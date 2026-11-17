package server

import (
	"testing"

	"antifox/db"
)

func TestReportExcludesReviewedFalsePositivesPG(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()
	task, err := m.CreateTask("Review test", "Local regression", "Review findings", "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var acceptedID int64
	for _, status := range []string{"confirmed", "false_positive"} {
		id, err := task.Store.AddNode(db.KindFinding, map[string]any{
			"summary": status + " finding", "severity": "low", "vulnclass": "Test",
		}, 0, "confirmed", "test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.pg.UpdateFindingEnrichmentFields(id, "", "", "", status); err != nil {
			t.Fatal(err)
		}
		if status == "confirmed" {
			acceptedID = id
		}
	}
	s := &Server{m: m}
	in := s.buildReportInput(task, nil)
	if len(in.Findings) != 1 || in.Findings[0].NodeID != acceptedID {
		t.Fatalf("report included rejected evidence: %+v", in.Findings)
	}
}
