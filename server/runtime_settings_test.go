package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antifox/agent"
	"antifox/db"
)

// Changing capture must affect the next run of an already-cached, pinned worker.
// Exercise its real Bash environment, not just the cache's implementation.
func TestCaptureSettingsRefreshPinnedWorkerPG(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n")
		if calls.Add(1)%2 == 1 {
			fmt.Fprint(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"capture-check\",\"name\":\"Bash\",\"input\":{}}}\n\n")
			input, _ := json.Marshal(map[string]string{"command": `printf 'captured_proxy=%s' "$HTTPS_PROXY"`})
			fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":%q}}\n\n", input)
			fmt.Fprint(w, "data: {\"type\":\"content_block_stop\",\"index\":0}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n")
		} else {
			fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Checked.\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
		}
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()
	m, err := NewManager(t.TempDir(), ":0")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()
	if m.Traffic() == nil {
		t.Fatal("recording proxy did not initialize")
	}
	if err := m.SetTrafficEnabled(false); err != nil {
		t.Fatal(err)
	}
	profileID, err := m.pg.SaveProfile(&db.LLMProfile{
		Name: "capture-settings-test", Format: "anthropic", Model: "test-model", BaseURL: upstream.URL, APIKey: "test-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s := New(ctx, m, t.TempDir(), t.TempDir())
	// This regression needs only the built-in Bash tool, without external MCPs.
	augment := agent.ToolAugment
	agent.ToolAugment = nil
	t.Cleanup(func() { agent.ToolAugment = augment })
	task, err := m.CreateTask("capture test", "Local regression", "Check proxy environment", "", &profileID, 0)
	if err != nil {
		t.Fatal(err)
	}
	intent := &db.Node{ID: 0, Kind: db.KindIntent, Payload: json.RawMessage(`{"text":"Check the local proxy environment"}`)}
	for i, on := range []bool{false, true, false} {
		if i > 0 {
			resp := httptest.NewRecorder()
			s.putSettings(resp, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(fmt.Sprintf(`{"traffic_capture":%t}`, on))))
			if resp.Code != http.StatusOK {
				t.Fatalf("update capture: %d %s", resp.Code, resp.Body.String())
			}
		}
		stats := httptest.NewRecorder()
		s.stats(stats, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
		var state struct {
			TrafficEnabled bool `json:"traffic_enabled"`
		}
		if err := json.Unmarshal(stats.Body.Bytes(), &state); err != nil || state.TrafficEnabled != on {
			t.Fatalf("capture=%t reported incorrectly: %s (err=%v)", on, stats.Body.String(), err)
		}
		_, worker := s.agentsForProfile(profileID)
		if worker == nil {
			t.Fatal("missing pinned worker")
		}
		var output string
		_, _, err := worker.Execute(ctx, "capture-test", task.ExpID, nil, task.Store, intent, nil, func(a db.Activity) {
			if a.Kind == "tool_result" && a.Tool == "Bash" {
				if a.IsError {
					t.Errorf("Bash failed: %s", a.Detail)
				}
				output = a.Detail
			}
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.TrimSpace(output), "captured_proxy="+m.ProxyAddr(); got != want {
			t.Fatalf("capture=%t: worker environment %q, want %q", on, got, want)
		}
	}
	if calls.Load() != 6 {
		t.Fatalf("unexpected provider requests: %d", calls.Load())
	}
}
