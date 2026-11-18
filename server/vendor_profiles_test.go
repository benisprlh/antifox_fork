package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"antifox/db"
)

func TestVendorProfilesAndCredentialIsolationPG(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer new-vendor-key" {
			t.Error("unexpected credential reached the vendor")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		switch r.URL.Path {
		case "/grok/responses":
			if body["max_output_tokens"] != float64(4096) {
				t.Error("connection test left no room for Grok reasoning")
			}
			fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
		case "/deepseek/chat/completions":
			if body["max_tokens"] != float64(4096) {
				t.Error("connection test left no room for DeepSeek reasoning")
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		default:
			t.Errorf("unexpected outbound destination: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()
	oldID, err := m.pg.SaveProfile(&db.LLMProfile{Name: "existing provider", Format: "anthropic", Model: "test-model", BaseURL: upstream.URL + "/original", APIKey: "original-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.pg.SetActiveProfile(oldID); err != nil {
		t.Fatal(err)
	}
	// Simulate upgrading an existing installation, including its saved profile.
	if _, err := m.pg.Exec(`ALTER TABLE llm_profiles DROP CONSTRAINT llm_profiles_format_check;
		ALTER TABLE llm_profiles ADD CONSTRAINT llm_profiles_format_check CHECK (format IN ('openai','anthropic'))`); err != nil {
		t.Fatal(err)
	}
	dsn, _, err := db.DSN()
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	migrated.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, m, t.TempDir(), t.TempDir())
	call := func(handler http.HandlerFunc, body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRecorder()
		handler(r, httptest.NewRequest(http.MethodPost, "/api/llm", bytes.NewReader(b)))
		return r
	}
	for _, provider := range []string{"grok", "deepseek"} {
		res := call(s.pgSaveProfile, map[string]any{"name": provider, "format": provider, "base_url": upstream.URL + "/" + provider, "api_key": "new-vendor-key"})
		if res.Code != 200 {
			t.Fatalf("save %s: %d %s", provider, res.Code, res.Body.String())
		}
		var saved struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &saved); err != nil {
			t.Fatal(err)
		}
		p, err := m.pg.ProfileByID(saved.ID)
		if err != nil || p == nil || p.Format != provider || p.Model == "" {
			t.Fatalf("profile did not round-trip: provider=%s err=%v", provider, err)
		}
		res = call(s.testLLM, map[string]any{"provider": provider, "model": p.Model, "base_url": p.BaseURL, "profile_id": p.ID})
		if !strings.Contains(res.Body.String(), `"ok":true`) {
			t.Fatalf("test %s: %s", provider, res.Body.String())
		}
		res = call(s.pgSaveProfile, map[string]any{"id": p.ID, "name": provider + " renamed", "format": provider, "model": p.Model, "base_url": p.BaseURL})
		if res.Code != 200 {
			t.Fatalf("same-vendor key retention rejected: %s", res.Body.String())
		}
		retained, _ := m.pg.ProfileByID(p.ID)
		if retained.APIKey != "new-vendor-key" {
			t.Fatal("normal edit erased the saved key")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected two connection probes, got %d", calls.Load())
	}
	emptyID, err := m.pg.SaveProfile(&db.LLMProfile{Name: "keyless draft", Format: "grok", Model: "grok-4.7", BaseURL: upstream.URL + "/grok"})
	if err != nil {
		t.Fatal(err)
	}
	emptyTest := call(s.testLLM, map[string]any{"provider": "grok", "base_url": upstream.URL + "/grok", "profile_id": emptyID})
	if !strings.Contains(emptyTest.Body.String(), "No API key provided") {
		t.Fatalf("keyless draft borrowed the active credential: %s", emptyTest.Body.String())
	}
	for _, body := range []map[string]any{
		{"provider": "grok", "base_url": upstream.URL + "/grok", "profile_id": oldID},
		{"provider": "anthropic", "base_url": upstream.URL + "/another-destination", "profile_id": oldID},
		{"provider": "deepseek", "base_url": upstream.URL + "/deepseek"},
	} {
		res := call(s.testLLM, body)
		if !strings.Contains(res.Body.String(), providerKeyRequired) {
			t.Fatalf("credential reuse was not rejected: %s", res.Body.String())
		}
	}
	res := call(s.pgSaveProfile, map[string]any{"id": oldID, "name": "wrong destination", "format": "grok", "base_url": upstream.URL + "/grok"})
	if res.Code != 400 {
		t.Fatalf("provider change kept a foreign key: %d %s", res.Code, res.Body.String())
	}
	res = call(s.setLLM, map[string]any{"provider": "deepseek", "base_url": upstream.URL + "/deepseek"})
	if res.Code != 400 {
		t.Fatal("legacy configuration endpoint reused a foreign key")
	}
	if calls.Load() != 2 {
		t.Fatal("rejected credential was sent over HTTP")
	}
	old, _ := m.pg.ProfileByID(oldID)
	if old.Format != "anthropic" || old.APIKey != "original-secret" {
		t.Fatal("rejected edit changed the existing profile")
	}
	res = httptest.NewRecorder()
	s.pgListProfiles(res, httptest.NewRequest(http.MethodGet, "/api/llm/profiles", nil))
	if strings.Contains(res.Body.String(), "original-secret") || strings.Contains(res.Body.String(), "new-vendor-key") {
		t.Fatal("profile listing exposed a credential")
	}
}
