package handlers

import (
	"binary-core/internal/middlewares/auth"
	"binary-core/internal/models"
	"binary-core/internal/transaction"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// HTTPHandler contient le service injecté
type HTTPHandler struct {
	service *transaction.Service
}

type AuthHandler struct {
	pool *pgxpool.Pool
}

func NewHTTPHandler(s *transaction.Service) *HTTPHandler {
	return &HTTPHandler{service: s}
}

func NewAuthHandler(pool *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{pool: pool}
}

// Définition de la structure avec embedding de jwt.RegisteredClaims
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Register gère la route POST /register pour l'inscription des utilisateurs
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Format JSON invalide"})
		return
	}

	if req.Email == "" || len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Email valide et mot de passe de 8 caractères minimum requis"})
		return
	}

	// Hachage du mot de passe avec bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Erreur lors du traitement du mot de passe"})
		return
	}

	userID := uuid.New()
	//queries := db.New(h.pool)

	// Sauvegarde de l'utilisateur en base PostgreSQL
	// Assurez-vous d'avoir une colonne password_hash dans votre table users
	_, err = h.pool.Exec(r.Context(),
		"INSERT INTO users (id, email, password_hash, kyc_status) VALUES ($1, $2, $3, $4)",
		userID, req.Email, string(hashedPassword), "pending",
	)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Cet email est déjà utilisé"})
		return
	}

	// Création automatique d'un compte bancaire principal associé
	accountID := uuid.New()
	_, err = h.pool.Exec(r.Context(),
		"INSERT INTO accounts (id, user_id, currency) VALUES ($1, $2, 'EUR')",
		accountID, userID,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Compte utilisateur créé mais échec de l'ouverture du compte financier"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Compte créé avec succès",
		"user_id":    userID,
		"account_id": accountID,
	})
}

// HandleTransfer gère la route POST /transfers
func (h *HTTPHandler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	var req models.HTTPRequest
	w.Header().Set("Content-Type", "application/json")

	// Extraction de l'ID de l'utilisateur authentifié depuis le contexte du JWT
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Authentification requise"})
		return
	}

	// Décodage du corps de la requête JSON
	//var req HTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Format JSON invalide"}`))
		return
	}

	//Appel du service de transaction (Ici naissent txID et err)
	txID, err := h.service.ExecuteTransfer(r.Context(), transaction.TransferParams{
		AuthenticatedUserID: userID,
		IdempotencyKey:      req.IdempotencyKey,
		SourceAccount:       req.SourceAccount,
		DestAccount:         req.DestAccount,
		Amount:              req.Amount,
		Description:         req.Description,
	})

	//Traitement de l'erreur retournée par le Ledger
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	//Réponse en cas de succès (Utilisation de txID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Transfert effectué avec succès",
	})
}

// -----------------------------------------------------------------------------
// 2. CONNEXION (POST /auth/login)
// -----------------------------------------------------------------------------
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Format JSON invalide"})
		return
	}

	// Recherche de l'utilisateur par email
	var userID uuid.UUID
	var storedHash string
	err := h.pool.QueryRow(r.Context(),
		"SELECT id, password_hash FROM users WHERE email = $1",
		req.Email,
	).Scan(&userID, &storedHash)

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Identifiants incorrects"})
		return
	}

	// Comparaison du mot de passe saisi avec le hash enregistré
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Identifiants incorrects"})
		return
	}

	// Génération du jeton JWT
	token, err := generateJWT(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Impossible de générer le jeton de session"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"type":  "Bearer",
	})
}

// Génération du JWT signé d'une durée de validité de 24 heures
func generateJWT(userID uuid.UUID) (string, error) {
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))

	claims := &Claims{
		UserID: userID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
