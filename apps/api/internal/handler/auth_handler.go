package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/suncrestlabs/nester/apps/api/internal/api"
	"github.com/suncrestlabs/nester/apps/api/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/challenge", h.handleChallenge)
	mux.HandleFunc("POST /api/v1/auth/verify", h.handleVerify)
}

type ChallengeRequest struct {
	WalletAddress string `json:"wallet_address"`
}

type ChallengeResponse struct {
	Challenge string `json:"challenge"`
}

func (h *AuthHandler) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.WalletAddress == "" {
		api.Error(w, http.StatusBadRequest, "wallet_address is required")
		return
	}

	challenge, err := h.authService.GenerateChallenge(r.Context(), req.WalletAddress)
	if err != nil {
		if errors.Is(err, service.ErrWalletInvalid) {
			api.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		api.Error(w, http.StatusInternalServerError, "failed to generate challenge")
		return
	}

	api.JSON(w, http.StatusOK, ChallengeResponse{Challenge: challenge})
}

type VerifyRequest struct {
	WalletAddress string `json:"wallet_address"`
	Signature     string `json:"signature"`
	Challenge     string `json:"challenge"`
}

type VerifyResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.WalletAddress == "" || req.Signature == "" || req.Challenge == "" {
		api.Error(w, http.StatusBadRequest, "wallet_address, signature, and challenge are required")
		return
	}

	token, err := h.authService.VerifyAndIssue(r.Context(), req.WalletAddress, req.Signature, req.Challenge)
	if err != nil {
		if errors.Is(err, service.ErrChallengeExpired) || errors.Is(err, service.ErrSignatureInvalid) || errors.Is(err, service.ErrWalletInvalid) {
			api.Error(w, http.StatusUnauthorized, err.Error())
			return
		}
		api.Error(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	api.JSON(w, http.StatusOK, VerifyResponse{Token: token})
}
