package cpa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"OpenCodeGoPool/internal/model"
	"OpenCodeGoPool/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedSettings(t *testing.T, s store.Store, endpoint string) {
	t.Helper()
	ctx := context.Background()
	if err := s.SaveCPASettings(ctx, &model.CPASettings{
		Endpoint:     endpoint,
		BearerToken:  "test-token",
		ProviderName: "OpenCode Go",
		BaseURL:      "https://example.com",
		Models:       []string{"model-a"},
	}); err != nil {
		t.Fatalf("SaveCPASettings: %v", err)
	}
	if err := s.CreateAccount(ctx, &model.Account{
		ID:     "acc-1",
		Email:  "acc1@example.com",
		Status: "active",
		APIKey: "sk-acc-1",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
}

func TestSync_MergePreservesUnrelatedProviders(t *testing.T) {
	remoteList := []model.CPAProvider{
		{
			Name:    "zyloo.io",
			BaseURL: "https://zyloo.io",
			APIKeyEntries: []model.CPAKeyEntry{
				{APIKey: "zyloo-key"},
			},
			Models: []model.CPAModel{{Name: "zyloo-model"}},
		},
		{
			Name:    "OpenCode Go",
			BaseURL: "https://stale.example.com",
			APIKeyEntries: []model.CPAKeyEntry{
				{APIKey: "stale-key"},
			},
		},
	}

	var capturedPUTBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/management/openai-compatibility", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string][]model.CPAProvider{"openai-compatibility": remoteList})
		case http.MethodPut:
			buf, _ := io.ReadAll(r.Body)
			capturedPUTBody = buf
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := newTestStore(t)
	seedSettings(t, s, srv.URL+"/v0/management/openai-compatibility")

	syncer := NewSyncer(s)
	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	var wrapped map[string][]model.CPAProvider
	if err := json.Unmarshal(capturedPUTBody, &wrapped); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	merged := wrapped["openai-compatibility"]
	if len(merged) != 2 {
		t.Fatalf("expected 2 providers in merged payload, got %d", len(merged))
	}

	var zyloo, ours *model.CPAProvider
	for i := range merged {
		switch merged[i].Name {
		case "zyloo.io":
			zyloo = &merged[i]
		case "OpenCode Go":
			ours = &merged[i]
		}
	}

	if zyloo == nil {
		t.Fatal("zyloo.io entry was removed by sync")
	}
	if zyloo.BaseURL != "https://zyloo.io" || len(zyloo.APIKeyEntries) != 1 || zyloo.APIKeyEntries[0].APIKey != "zyloo-key" {
		t.Errorf("zyloo.io entry was modified: %+v", zyloo)
	}

	if ours == nil {
		t.Fatal("OpenCode Go entry missing from merged payload")
	}
	if len(ours.APIKeyEntries) != 1 || ours.APIKeyEntries[0].APIKey != "sk-acc-1" {
		t.Errorf("OpenCode Go entry does not reflect current local account keys: %+v", ours)
	}
}

func TestSync_FirstSyncWhenRemoteEmpty(t *testing.T) {
	mux := http.NewServeMux()
	var putCalled bool
	mux.HandleFunc("/v0/management/openai-compatibility", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			putCalled = true
			buf, _ := io.ReadAll(r.Body)
			var wrapped map[string][]model.CPAProvider
			if err := json.Unmarshal(buf, &wrapped); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			providers := wrapped["openai-compatibility"]
			if len(providers) != 1 {
				t.Errorf("expected 1 provider, got %d", len(providers))
			}
			w.WriteHeader(http.StatusOK)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := newTestStore(t)
	seedSettings(t, s, srv.URL+"/v0/management/openai-compatibility")

	syncer := NewSyncer(s)
	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if !putCalled {
		t.Error("expected PUT to be called for first sync")
	}
}

func TestSync_FailsWithEmptyProviderName(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.SaveCPASettings(ctx, &model.CPASettings{
		Endpoint:     "http://example.com/v0/providers",
		BearerToken:  "tok",
		ProviderName: "",
		BaseURL:      "https://example.com",
		Models:       []string{"model-a"},
	}); err != nil {
		t.Fatalf("SaveCPASettings: %v", err)
	}

	syncer := NewSyncer(s)
	err := syncer.Sync(ctx)
	if err == nil {
		t.Fatal("expected error when ProviderName is empty")
	}
	if !strings.Contains(err.Error(), "provider name not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_AbortsWithoutOverwriteOnFetchFailure(t *testing.T) {
	mux := http.NewServeMux()
	var putCalled bool
	mux.HandleFunc("/v0/management/openai-compatibility", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusOK)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := newTestStore(t)
	seedSettings(t, s, srv.URL+"/v0/management/openai-compatibility")

	syncer := NewSyncer(s)
	if err := syncer.Sync(context.Background()); err == nil {
		t.Fatal("expected Sync to return an error when fetch fails")
	}
	if putCalled {
		t.Error("PUT must not be called when the remote fetch fails")
	}
}
