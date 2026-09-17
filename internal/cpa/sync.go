package cpa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"OpenCodeGoPool/internal/model"
	"OpenCodeGoPool/internal/store"

	"github.com/google/uuid"
)

type Syncer struct {
	store  store.Store
	client *http.Client
	mu     sync.Mutex
}

func NewSyncer(s store.Store) *Syncer {
	return &Syncer{
		store:  s,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Syncer) InitDefaults(ctx context.Context, cfg model.CPASettings) error {
	existing, err := s.store.GetCPASettings(ctx)
	if err != nil {
		return err
	}
	if existing.Endpoint == "" {
		return s.store.SaveCPASettings(ctx, &cfg)
	}
	return nil
}

func (s *Syncer) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.store.GetCPASettings(ctx)
	if err != nil {
		return fmt.Errorf("get CPA settings: %w", err)
	}
	if settings.Endpoint == "" {
		return fmt.Errorf("CPA endpoint not configured")
	}
	if settings.ProviderName == "" {
		return fmt.Errorf("CPA provider name not configured")
	}

	accounts, err := s.store.ListActiveAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list active accounts: %w", err)
	}

	remote, err := s.fetchRemoteProviders(ctx, settings)
	if err != nil {
		s.logSync(ctx, "error", err.Error(), 0)
		return fmt.Errorf("fetch remote providers: %w", err)
	}

	// Build a map of apiKey -> authIndex from the existing remote provider so
	// we can carry auth-index values over instead of losing them on each sync.
	existingAuthIndex := map[string]string{}
	for _, p := range remote {
		if p.Name == settings.ProviderName {
			for _, e := range p.APIKeyEntries {
				if e.AuthIndex != "" {
					existingAuthIndex[e.APIKey] = e.AuthIndex
				}
			}
			break
		}
	}

	var opencodeKeys, ollamaKeys []model.CPAKeyEntry
	for _, acc := range accounts {
		if acc.APIKey == "" {
			continue
		}
		entry := model.CPAKeyEntry{APIKey: acc.APIKey, AuthIndex: existingAuthIndex[acc.APIKey]}
		if acc.Type == "ollama" {
			ollamaKeys = append(ollamaKeys, entry)
		} else {
			opencodeKeys = append(opencodeKeys, entry)
		}
	}

	models := make([]model.CPAModel, len(settings.Models))
	for i, m := range settings.Models {
		models[i] = model.CPAModel{Name: m}
	}

	ownProvider := model.CPAProvider{
		Name:          settings.ProviderName,
		BaseURL:       settings.BaseURL,
		APIKeyEntries: opencodeKeys,
		Disabled:      false,
		Models:        models,
	}

	merged := mergeProvider(remote, ownProvider)

	// Ollama accounts are synced to a separate provider so they never
	// share the OpenCode credential pool.
	if settings.OllamaProviderName != "" {
		ollamaModels := make([]model.CPAModel, len(settings.OllamaModels))
		for i, m := range settings.OllamaModels {
			ollamaModels[i] = model.CPAModel{Name: m}
		}
		merged = mergeProvider(merged, model.CPAProvider{
			Name:          settings.OllamaProviderName,
			BaseURL:       settings.OllamaBaseURL,
			APIKeyEntries: ollamaKeys,
			Disabled:      false,
			Models:        ollamaModels,
		})
	}

	body, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", settings.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.BearerToken)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logSync(ctx, "error", err.Error(), len(opencodeKeys))
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
		s.logSync(ctx, "error", msg, len(opencodeKeys))
		return fmt.Errorf("%s", msg)
	}

	s.logSync(ctx, "success", string(respBody), len(opencodeKeys)+len(ollamaKeys))
	slog.Info("CPA sync success", "keys", len(opencodeKeys)+len(ollamaKeys), "models", len(models))
	return nil
}

// fetchRemoteProviders fetches the current remote provider list so that Sync
// can merge into it instead of overwriting unrelated providers. A 404 or
// empty body is treated as "no existing remote list yet" and returns (nil, nil).
// Any other failure aborts the sync without sending a PUT, since a destructive
// overwrite is worse than failing loudly.
func (s *Syncer) fetchRemoteProviders(ctx context.Context, settings *model.CPASettings) ([]model.CPAProvider, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", settings.Endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+settings.BearerToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		io.Copy(io.Discard, resp.Body)
		return nil, nil
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if len(bytes.TrimSpace(respBody)) == 0 {
		return nil, nil
	}

	// The CPA API wraps the provider list: {"openai-compatibility": [...]}
	var wrapper map[string][]model.CPAProvider
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return wrapper["openai-compatibility"], nil
}

// mergeProvider replaces the entry in remote whose Name matches own.Name,
// or appends own if no such entry exists. Every other entry passes through
// untouched, in its original order.
func mergeProvider(remote []model.CPAProvider, own model.CPAProvider) []model.CPAProvider {
	for i, p := range remote {
		if p.Name == own.Name {
			remote[i] = own
			return remote
		}
	}
	return append(append([]model.CPAProvider{}, remote...), own)
}

func (s *Syncer) logSync(ctx context.Context, status, message string, keyCount int) {
	log := &model.CPASyncLog{
		ID:       uuid.New().String(),
		Status:   status,
		Message:  message,
		KeyCount: keyCount,
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.store.SaveSyncLog(ctx, log); err != nil {
		slog.Error("save sync log", "error", err)
	}
}
