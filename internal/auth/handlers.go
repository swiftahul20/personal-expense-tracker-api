package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/swiftahul20/expense-tracker/internal/user"
)

func NewHandler(users user.Store, jwtManager *JWTManager, refreshTTL time.Duration) *Handler {
	return &Handler{users: users, jwtManager: jwtManager, refreshTTL: refreshTTL}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := user.ValidateRegistration(req.Email, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.users.GetByEmail(req.Email); err == nil {
		writeError(w, http.StatusConflict, user.ErrEmailTaken.Error())
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	created, err := h.users.Create(req.Email, hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.jwtManager.Generate(created.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	plainRefresh, hashedRefresh, err := GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	if err := h.users.SaveRefreshToken(created.ID, hashedRefresh, time.Now().Add(h.refreshTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save refresh token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{AccessToken: token, User: created, RefreshToken: plainRefresh})
}

// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        credentials body registerRequest true "Login credentials"
// @Success      200 {object} authResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	u, err := h.users.GetByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, user.ErrInvalidLogin.Error())
		return
	}

	if err := CheckPassword(req.Password, u.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, user.ErrInvalidLogin.Error())
		return
	}

	token, err := h.jwtManager.Generate(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	plainRefresh, hashedRefresh, err := GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	if err := h.users.SaveRefreshToken(u.ID, hashedRefresh, time.Now().Add(h.refreshTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save refresh token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{AccessToken: token, User: u, RefreshToken: plainRefresh})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	u, err := h.users.GetByID(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, u)
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	hash := HashToken(req.RefreshToken)
	_ = h.users.DeleteRefreshToken(hash)

	w.WriteHeader(http.StatusNoContent)
}
