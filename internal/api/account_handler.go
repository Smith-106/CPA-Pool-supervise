package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"OpenCodeGoPool/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.ListAccounts(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if accounts == nil {
		accounts = []model.Account{}
	}
	jsonOK(w, accounts)
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string `json:"type"`
		Email        string `json:"email"`
		Cookie       string `json:"cookie"`
		WorkspaceID  string `json:"workspace_id"`
		APIKey       string `json:"api_key"`
		LimitRolling *int   `json:"limit_rolling"`
		LimitWeekly  *int   `json:"limit_weekly"`
		LimitMonthly *int   `json:"limit_monthly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Type == "ollama" {
		if req.Email == "" || req.APIKey == "" {
			jsonError(w, "email and api_key are required for ollama accounts", http.StatusBadRequest)
			return
		}
	} else {
		if req.Email == "" || req.Cookie == "" || req.WorkspaceID == "" {
			jsonError(w, "email, cookie, and workspace_id are required", http.StatusBadRequest)
			return
		}
	}
	if err := validateLimits(req.LimitRolling, req.LimitWeekly, req.LimitMonthly); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	acc := &model.Account{
		ID:           uuid.New().String(),
		Type:         req.Type,
		Email:        req.Email,
		Cookie:       req.Cookie,
		WorkspaceID:  req.WorkspaceID,
		APIKey:       req.APIKey,
		LimitRolling: req.LimitRolling,
		LimitWeekly:  req.LimitWeekly,
		LimitMonthly: req.LimitMonthly,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateAccount(r.Context(), acc); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go s.cpa.Sync(r.Context())

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, acc)
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	acc, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if acc == nil {
		jsonError(w, "account not found", http.StatusNotFound)
		return
	}
	jsonOK(w, acc)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	acc, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if acc == nil {
		jsonError(w, "account not found", http.StatusNotFound)
		return
	}

	var req struct {
		Type         *string `json:"type"`
		Email        *string `json:"email"`
		Cookie       *string `json:"cookie"`
		WorkspaceID  *string `json:"workspace_id"`
		APIKey       *string `json:"api_key"`
		LimitRolling *int    `json:"limit_rolling"`
		LimitWeekly  *int    `json:"limit_weekly"`
		LimitMonthly *int    `json:"limit_monthly"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := validateLimits(req.LimitRolling, req.LimitWeekly, req.LimitMonthly); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Type != nil {
		acc.Type = *req.Type
	}
	if req.Email != nil {
		acc.Email = *req.Email
	}
	if req.Cookie != nil {
		acc.Cookie = *req.Cookie
	}
	if req.WorkspaceID != nil {
		acc.WorkspaceID = *req.WorkspaceID
	}
	if req.APIKey != nil {
		acc.APIKey = *req.APIKey
	}
	// Always apply limit fields so the user can clear them by sending null.
	acc.LimitRolling = req.LimitRolling
	acc.LimitWeekly = req.LimitWeekly
	acc.LimitMonthly = req.LimitMonthly

	if err := s.store.UpdateAccount(r.Context(), acc); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go s.cpa.Sync(r.Context())

	jsonOK(w, acc)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteAccount(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go s.cpa.Sync(r.Context())

	jsonOK(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleToggleStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	acc, err := s.store.GetAccount(r.Context(), id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if acc == nil {
		jsonError(w, "account not found", http.StatusNotFound)
		return
	}

	newStatus := "disabled"
	if acc.Status == "disabled" {
		newStatus = "active"
	}

	if err := s.store.UpdateAccountStatus(r.Context(), id, newStatus, ""); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go s.cpa.Sync(r.Context())

	jsonOK(w, map[string]string{"status": newStatus})
}

func validateLimits(rolling, weekly, monthly *int) error {
	for _, v := range []*int{rolling, weekly, monthly} {
		if v != nil && (*v < 1 || *v > 100) {
			return errors.New("limit values must be between 1 and 100")
		}
	}
	return nil
}
