package server

// Retroactive extraction uses main-agent/worker transcripts to create confirmed finding nodes
// that were described but never persisted through report_finding. Graph, findings, and reports
// share these nodes, so one extraction updates all three views.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"antifox/db"
	"antifox/report"
)

// POST /api/tasks/{id}/extract-findings — back-fill findings from the transcript.
func (s *Server) extractFindings(w http.ResponseWriter, r *http.Request) {
	t := s.m.ResolveTask(r.PathValue("id"))
	if t == nil {
		writeErr(w, 404, "no active task")
		return
	}
	expID := t.Store.ID()
	transcript := s.m.pg.ExplorationSynthesis(expID, 48000)
	if strings.TrimSpace(transcript) == "" {
		writeErr(w, 400, "This task has no conversation records to analyze")
		return
	}

	// Include recorded exploration facts to recover verified exposures that were recorded
	// with record_fact but never promoted to findings.
	if facts, ferr := t.Store.ListByKind(db.KindFact, 200); ferr == nil && len(facts) > 0 {
		var fb strings.Builder
		fb.WriteString("\n\n[Exploration facts, potentially including exposures not yet recorded as findings]\n")
		for _, n := range facts {
			var p struct {
				Summary string `json:"summary"`
				Detail  string `json:"detail"`
			}
			_ = json.Unmarshal(n.Payload, &p)
			sm := strings.TrimSpace(p.Summary)
			if sm == "" {
				continue
			}
			fb.WriteString("- ")
			fb.WriteString(sm)
			if d := strings.TrimSpace(p.Detail); d != "" {
				if rr := []rune(d); len(rr) > 200 {
					d = string(rr[:200])
				}
				fb.WriteString("：")
				fb.WriteString(d)
			}
			fb.WriteString("\n")
		}
		transcript += fb.String()
	}

	// existing findings → dedup labels for the AI + a server-side guard
	existingNodes, _ := t.Store.ListByKind(db.KindFinding, 500)
	labels := make([]string, 0, len(existingNodes))
	existingKeys := make([]string, 0, len(existingNodes))
	for _, n := range existingNodes {
		var p struct {
			VulnClass string `json:"vulnclass"`
			Summary   string `json:"summary"`
		}
		_ = json.Unmarshal(n.Payload, &p)
		labels = append(labels, strings.TrimSpace(p.VulnClass+" — "+p.Summary))
		existingKeys = append(existingKeys, normalizeVuln(p.VulnClass+p.Summary))
	}

	extracted, err := report.ExtractFindings(s.ctx, s.m.pg, transcript, labels)
	if err != nil {
		writeErr(w, 502, "AI extraction failed: "+err.Error())
		return
	}

	taskID, _ := strconv.ParseInt(t.ID, 10, 64)
	begin := s.m.pg.ExplorationBeginNode(expID)
	created := 0
	for _, e := range extracted {
		if strings.TrimSpace(e.Summary) == "" && strings.TrimSpace(e.VulnClass) == "" {
			continue
		}
		key := normalizeVuln(e.VulnClass + e.Summary)
		if containsSimilar(existingKeys, key) {
			continue // already recorded
		}
		payload := map[string]any{
			"vulnclass": e.VulnClass, "severity": normalizeSeverity(e.Severity), "summary": e.Summary,
			"evidence": map[string]any{"by": "extracted", "poc": e.Evidence},
		}
		id, aerr := t.Store.AddNode(db.KindFinding, payload, 9, "confirmed", "extracted", nil)
		if aerr != nil {
			continue
		}
		if begin > 0 {
			_ = t.Store.Link(begin, db.RelYields, id) // connect into the graph
		}
		_, _ = t.Store.AddStandaloneFinding(taskID, id, e.VulnClass, normalizeSeverity(e.Severity), e.Summary, e.Evidence, "extracted", nil)
		existingKeys = append(existingKeys, key)
		created++
	}
	writeJSON(w, 200, map[string]any{"extracted": len(extracted), "created": created})
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "\u4e25\u91cd", "high", "\u9ad8\u5371", "\u9ad8":
		if strings.Contains(strings.ToLower(s), "crit") || strings.Contains(s, "\u4e25\u91cd") {
			return "high" // AntiFox severity set is high|medium|low
		}
		return "high"
	case "medium", "\u4e2d\u5371", "\u4e2d", "med":
		return "medium"
	case "low", "\u4f4e\u5371", "\u4f4e", "info", "\u4fe1\u606f":
		return "low"
	}
	return "medium"
}

func normalizeVuln(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 { // keep CJK
			b.WriteRune(r)
		}
	}
	return b.String()
}

// containsSimilar reports whether key strongly overlaps any existing key (either
// contains the other on a ≥8-char prefix) — a light dedup on top of the AI's.
func containsSimilar(existing []string, key string) bool {
	if key == "" {
		return false
	}
	for _, e := range existing {
		if e == "" {
			continue
		}
		if strings.Contains(e, key) || strings.Contains(key, e) {
			return true
		}
		n := 12
		if len([]rune(key)) >= n && len([]rune(e)) >= n {
			if string([]rune(key)[:n]) == string([]rune(e)[:n]) {
				return true
			}
		}
	}
	return false
}
