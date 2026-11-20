package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"binary-core/internal/domain"
	"binary-core/internal/pkg/crypto"
	"binary-core/internal/repository/postgres"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jackc/pgx/v5"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

type authService struct {
	txManager postgres.TxManager
	queries   postgres.Querier
	jwtSecret []byte
}

func NewAuthService(txManager postgres.TxManager, queries postgres.Querier, jwtSecret []byte) domain.AuthService {
	return &authService{
		txManager: txManager,
		queries:   queries,
		jwtSecret: jwtSecret,
	}
}

func (s *authService) RegisterClient(ctx context.Context, email, password string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 10 {
		return nil, errors.New("email requis et mot de passe de 10 caractères minimum")
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateUser(ctx, postgres.CreateUserParams{
		ID:           userID,
		Email:        email,
		PasswordHash: hash,
		Role:         string(domain.RoleClient),
		Status:       string(domain.UserStatusActive),
	})
	if err != nil {
		return nil, fmt.Errorf("création utilisateur: %w", err)
	}

	return &domain.User{
		ID:        row.ID,
		Email:     row.Email,
		Role:      domain.Role(row.Role),
		Status:    domain.UserStatus(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (s *authService) Login(ctx context.Context, email, password, userAgent, ipAddress string) (*domain.TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if user.Status != string(domain.UserStatusActive) {
		return nil, domain.ErrAccountSuspended
	}

	valid, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil || !valid {
		return nil, domain.ErrInvalidCredentials
	}

	return s.issueSessionTokens(ctx, s.queries, user.ID, domain.Role(user.Role), userAgent, ipAddress)
}

func (s *authService) RefreshSession(ctx context.Context, rawRefreshToken, userAgent, ipAddress string) (*domain.TokenPair, error) {
	tokenHash := hashTokenSHA256(rawRefreshToken)
	var newPair *domain.TokenPair

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		session, err := q.GetSessionByTokenHashForUpdate(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidSession
			}
			return err
		}

		// Protection anti-vol : si un jeton déjà révoqué est rejoué, on coupe toutes les sessions du compte
		if session.IsRevoked {
			_ = q.RevokeAllUserSessions(ctx, session.UserID)
			return domain.ErrInvalidSession
		}

		if time.Now().After(session.ExpiresAt.Time) {
			_ = q.RevokeSession(ctx, session.ID)
			return domain.ErrInvalidSession
		}

		user, err := q.GetUserByID(ctx, session.UserID)
		if err != nil || user.Status != string(domain.UserStatusActive) {
			return domain.ErrAccountSuspended
		}

		// Rotation : révocation immédiate de l'ancien jeton
		if err := q.RevokeSession(ctx, session.ID); err != nil {
			return err
		}

		pair, err := s.issueSessionTokens(ctx, q, user.ID, domain.Role(user.Role), userAgent, ipAddress)
		if err != nil {
			return err
		}

		newPair = pair
		return nil
	})

	if err != nil {
		return nil, err
	}

	return newPair, nil
}

func (s *authService) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := hashTokenSHA256(rawRefreshToken)
	return s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		session, err := q.GetSessionByTokenHashForUpdate(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // Déjà déconnecté
			}
			return err
		}
		return q.RevokeSession(ctx, session.ID)
	})
}

func (s *authService) issueSessionTokens(
	ctx context.Context,
	q postgres.Querier,
	userID uuid.UUID,
	role domain.Role,
	userAgent, ipAddress string,
) (*domain.TokenPair, error) {
	accessExp := time.Now().Add(accessTokenTTL)
	accessToken, err := signHS256JWT(userID, role, accessExp, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	// Génération d'un Refresh Token aléatoire de 32 octets (256 bits)
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, err
	}
	rawRefreshToken := base64.RawURLEncoding.EncodeToString(rawBytes)
	refreshHash := hashTokenSHA256(rawRefreshToken)

	ipAddr, err := netip.ParseAddr(ipAddress)
	if err != nil {
		ipAddr = netip.MustParseAddr("127.0.0.1")
	}

	sessionID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	_, err = q.CreateSession(ctx, postgres.CreateSessionParams{
		ID:               sessionID,
		UserID:           userID,
		RefreshTokenHash: refreshHash,
		UserAgent:        userAgent,
		IpAddress:        ipAddr,
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(refreshTokenTTL), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("création session: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		ExpiresAt:    accessExp,
	}, nil
}

func signHS256JWT(userID uuid.UUID, role domain.Role, exp time.Time, secret []byte) (string, error) {
	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	payloadBytes, err := json.Marshal(map[string]any{
		"sub":  userID.String(),
		"role": string(role),
		"exp":  exp.Unix(),
	})
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := encodedHeader + "." + encodedPayload

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func hashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
