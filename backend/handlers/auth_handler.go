package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"app-backend/middleware"
	"app-backend/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

type updateUsernameRequest struct {
	Username string `json:"username"`
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		writeJSONError(w, http.StatusBadRequest, "email is required and password must be at least 8 characters")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username != "" && (len(req.Username) < 2 || len(req.Username) > 32) {
		writeJSONError(w, http.StatusBadRequest, "username must be between 2 and 32 characters")
		return
	}

	user, err := h.authService.Register(r.Context(), req.Email, req.Password, req.Username)
	if err != nil {
		if errors.Is(err, services.ErrEmailInUse) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to register user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	token, user, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
		},
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to fetch profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
	})
}

func (h *AuthHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var req updateUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.authService.UpdateUsername(r.Context(), userID, req.Username)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
	})
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
