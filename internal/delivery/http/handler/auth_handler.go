package handler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"binary-core/internal/delivery/http/response"
	"binary-core/internal/domain"
)

type AuthHandler struct {
	authSvc domain.AuthService
}

func NewAuthHandler(authSvc domain.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

type credentialsPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload credentialsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}

	user, err := h.authSvc.RegisterClient(r.Context(), payload.Email, payload.Password)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	response.JSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload credentialsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}

	tokens, err := h.authSvc.Login(r.Context(), payload.Email, payload.Password, r.UserAgent(), readClientIP(r))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			response.JSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, domain.ErrAccountSuspended) {
			response.JSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var payload refreshPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.RefreshToken == "" {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "refresh_token obligatoire"})
		return
	}

	tokens, err := h.authSvc.RefreshSession(r.Context(), payload.RefreshToken, r.UserAgent(), readClientIP(r))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidSession) || errors.Is(err, domain.ErrAccountSuspended) {
			response.JSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var payload refreshPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err == nil && payload.RefreshToken != "" {
		_ = h.authSvc.Logout(r.Context(), payload.RefreshToken)
	}
	response.JSON(w, http.StatusNoContent, nil)
}

func readClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
