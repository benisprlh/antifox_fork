package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"antifox/db"
	"antifox/engine/harness"
)

func TestGoalRefusalSurfacesAnActionableError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":42}}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"refusal\"},\"usage\":{\"output_tokens\":0}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()
	var activities []db.Activity
	goals, err := DecomposeGoals(context.Background(), ConfigFrom("anthropic", "test-model", upstream.URL, "test-key", ""), "test", "test", func(a db.Activity) {
		activities = append(activities, a)
	})
	if !errors.Is(err, harness.ErrModelRefusal) || len(goals) != 0 {
		t.Fatalf("expected refusal without goals, got %v, %v", goals, err)
	}
	if len(activities) == 0 {
		t.Fatal("missing visible activity")
	}
	last := activities[len(activities)-1]
	if last.Kind != "result" || !last.IsError || !strings.Contains(last.Summary, "model declined") || strings.Contains(last.Summary, "completed") {
		t.Fatalf("refusal displayed as success: %+v", last)
	}
}
