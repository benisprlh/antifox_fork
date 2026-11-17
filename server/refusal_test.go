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

	"antifox/db"
)

func TestRefusalFailsTaskPG(t *testing.T) {
	for _, phase := range []string{"goals", "planner"} {
		t.Run(phase, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":42}}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"refusal\"},\"usage\":{\"output_tokens\":0}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
			}))
			defer upstream.Close()
			m, err := NewManager(t.TempDir(), "")
			if err != nil {
				t.Skipf("postgres unavailable: %v", err)
			}
			defer m.Close()
			profileID, err := m.pg.SaveProfile(&db.LLMProfile{
				Name: "refusal-test-" + phase, Format: "anthropic", Model: "test-model", BaseURL: upstream.URL, APIKey: "test-key",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.pg.SetActiveProfile(profileID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := New(ctx, m, t.TempDir(), t.TempDir())
			var task *Task
			if phase == "goals" {
				token, err := signJWT(s.jwtKey, "test-admin", "admin")
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(`{"name":"refusal test","description":"Local regression test","goal":"Verify refusal handling"}`))
				req.Header.Set("Authorization", "Bearer "+token)
				resp := httptest.NewRecorder()
				s.Handler().ServeHTTP(resp, req)
				if resp.Code != http.StatusCreated {
					t.Fatalf("create task: %d %s", resp.Code, resp.Body.String())
				}
				task = m.ActiveTask()
			} else {
				task, err = m.CreateTask("refusal test", "Local regression test", "Verify refusal handling", "", &profileID, 0)
				if err != nil {
					t.Fatal(err)
				}
				s.engine.Run(ctx, task)
			}
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) && m.TaskStatus(task.ID) != "failed" {
				time.Sleep(20 * time.Millisecond)
			}
			if status := m.TaskStatus(task.ID); status != "failed" {
				t.Fatalf("refused task stayed %q", status)
			}
			stats := httptest.NewRecorder()
			s.stats(stats, httptest.NewRequest(http.MethodGet, "/api/stats?task="+task.ID, nil))
			var state struct {
				ActiveTask struct {
					Running bool `json:"running"`
				} `json:"active_task"`
			}
			if err := json.Unmarshal(stats.Body.Bytes(), &state); err != nil || state.ActiveTask.Running {
				t.Fatalf("failed task reported as running: %s (err=%v)", stats.Body.String(), err)
			}
			if calls.Load() != 1 {
				t.Fatalf("refusal was retried: %d requests", calls.Load())
			}
			if phase == "goals" {
				goals, err := task.Store.ListByKind(db.KindGoal, 20)
				if err != nil || len(goals) != 0 || s.engine.Started(task.ID) {
					t.Fatalf("refusal started fallback planning: goals=%v err=%v started=%v", goals, err, s.engine.Started(task.ID))
				}
			}
			var visibleErrors int
			if err := m.pg.QueryRow(`SELECT count(*) FROM activity WHERE exploration_id=$1 AND is_error AND summary LIKE '%model declined%'`, task.ExpID).Scan(&visibleErrors); err != nil || visibleErrors == 0 {
				t.Fatalf("missing visible refusal: count=%d err=%v", visibleErrors, err)
			}
		})
	}
}
