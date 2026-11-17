package server

// Finding enrichment: AI-generated or human-edited structured
// overlay on a raw finding node, keyed by exploration node id. Shared by the
// findings page, the task-detail tab, and the report pipeline. The node payload
// (what agent tools see) is never mutated — enrichment is a management layer.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"antifox/db"
	"antifox/report"
	"antifox/traffic"
)

// enrichmentDTO is the wire shape (report.Enrichment fields + status/source/severity).
type enrichmentDTO struct {
	NodeID       string              `json:"node_id"`
	Name         string              `json:"name"`
	Target       string              `json:"target"`
	Module       string              `json:"module"`
	Severity     string              `json:"severity"`
	Status       string              `json:"status"`
	CVSSScore    float64             `json:"cvss_score"`
	CVSSVector   string              `json:"cvss_vector"`
	CVSSMetrics  map[string]string   `json:"cvss_metrics"`
	Description  []string            `json:"description"`
	TestProcess  []map[string]string `json:"test_process"`
	RiskAnalysis []string            `json:"risk_analysis"`
	Remediation  []string            `json:"remediation"`
	Source       string              `json:"source"`
	UpdatedAt    string              `json:"updated_at,omitempty"`
}

func dtoFromEnrichment(e *db.FindingEnrichment) *enrichmentDTO {
	if e == nil {
		return nil
	}
	d := &enrichmentDTO{
		NodeID: i64s(e.NodeID), Name: e.Name, Target: e.Target, Module: e.Module,
		Severity: e.Severity, Status: e.Status, CVSSScore: e.CVSSScore, CVSSVector: e.CVSSVector,
		Source: e.Source, UpdatedAt: rfc3339(e.UpdatedAt),
	}
	_ = json.Unmarshal(e.CVSSMetrics, &d.CVSSMetrics)
	_ = json.Unmarshal(e.Description, &d.Description)
	_ = json.Unmarshal(e.TestProcess, &d.TestProcess)
	_ = json.Unmarshal(e.RiskAnalysis, &d.RiskAnalysis)
	_ = json.Unmarshal(e.Remediation, &d.Remediation)
	return d
}

// dbEnrichmentFromReport converts a report.Enrichment (AI output) into the DB row.
func dbEnrichmentFromReport(nodeID int64, r report.Enrichment) *db.FindingEnrichment {
	j := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	tp := make([]map[string]string, 0, len(r.TestProcess))
	for _, b := range r.TestProcess {
		tp = append(tp, map[string]string{"type": b.Type, "content": b.Content})
	}
	return &db.FindingEnrichment{
		NodeID: nodeID, Name: r.Title, Target: r.Target, Module: r.Module,
		CVSSScore: r.CVSSScore, CVSSVector: r.CVSSVector, Status: "open", Source: "ai",
		CVSSMetrics:  j(r.CVSSMetrics),
		Description:  j(r.Description),
		TestProcess:  j(tp),
		RiskAnalysis: j(r.RiskAnalysis),
		Remediation:  j(r.Remediation),
	}
}

// reportEnrichmentFromDB reconstructs a report.Enrichment from a stored row by
// composing the JSON the report type expects and unmarshaling it — avoids
// exporting the report package's internal block type.
func reportEnrichmentFromDB(e *db.FindingEnrichment) *report.Enrichment {
	if e == nil {
		return nil
	}
	mj := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	obj := map[string]json.RawMessage{
		"title": mj(e.Name), "target": mj(e.Target), "module": mj(e.Module),
		"cvss_score": mj(e.CVSSScore), "cvss_vector": mj(e.CVSSVector),
		"cvss_metrics":  rawOrRM(e.CVSSMetrics, "{}"),
		"description":   rawOrRM(e.Description, "[]"),
		"test_process":  rawOrRM(e.TestProcess, "[]"),
		"risk_analysis": rawOrRM(e.RiskAnalysis, "[]"),
		"remediation":   rawOrRM(e.Remediation, "[]"),
	}
	b, _ := json.Marshal(obj)
	var enr report.Enrichment
	_ = json.Unmarshal(b, &enr)
	return &enr
}

func rawOrRM(r json.RawMessage, def string) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage(def)
	}
	return r
}

func nodeIDFromPath(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// GET /api/findings/{id} — finding + enrichment (enrichment null when not analyzed).
func (s *Server) findingDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := nodeIDFromPath(r)
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	f, err := s.m.pg.GetFindingByNodeID(id)
	if err != nil || f == nil {
		writeErr(w, 404, "Finding not found")
		return
	}
	enr, _ := s.m.pg.GetFindingEnrichment(id)
	// Pass structured reproduction directly to the frontend for explicit steps and evidence attachments.
	// When absent, fall back to the flat enrichment.test_process display.
	var repro any
	if len(f.Reproduction) > 2 {
		// Resolve each step's verbatim request back to the exact recorded traffic
		// exchange (stateless, through the body index), giving each step a traffic link.
		// Done here (view time) so it works for findings recorded before this feature,
		// no agent re-run needed. The traffic store may be nil (capture off) → steps
		// just carry no exchange_id and the UI shows no jump.
		var rp report.Reproduction
		if json.Unmarshal(f.Reproduction, &rp) == nil {
			// Use persisted exchange IDs if present; otherwise resolve at view time within the task window
			// using the same logic as report_finding, providing links for older findings too.
			hasResolved := false
			for _, st := range rp.Steps {
				if st.ExchangeID != "" {
					hasResolved = true
					break
				}
			}
			if !hasResolved {
				if tr := s.m.Traffic(); tr != nil && f.TaskID != nil {
					since := f.CreatedAt
					if tk, err := s.m.pg.GetTask(*f.TaskID); err == nil && tk.FirstRunAt != nil {
						since = *tk.FirstRunAt
					}
					refs := make([]traffic.StepRef, len(rp.Steps))
					for i, st := range rp.Steps {
						refs[i] = traffic.StepRef{Request: st.Request, Response: st.Response}
					}
					ids := tr.ResolveSteps(since.Unix(), time.Now().Unix(), refs)
					for i, id := range ids {
						if id != "" {
							rp.Steps[i].ExchangeID = id
						}
					}
				}
			}
			repro = rp
		} else {
			_ = json.Unmarshal(f.Reproduction, &repro)
		}
	}
	writeJSON(w, 200, map[string]any{"finding": findingFromDB(f), "enrichment": dtoFromEnrichment(enr), "reproduction": repro})
}

// resolveFindingTraffic resolves a finding's reproduction steps to real recorded
// traffic exchange ids AT RECORD TIME. It is the host-side callback wired into the
// worker's report_finding tool: when an agent records a finding, each step's verbatim
// request is matched against the traffic captured in THIS task's time window (start
// → now), in step order — so every step gets a jump-to-traffic link up front, with
// the full time/order context that view-time guessing lacks. Returns the reproduction
// with steps[].exchange_id filled where matched; unchanged on any failure.
func (s *Server) resolveFindingTraffic(taskID int64, repro json.RawMessage) json.RawMessage {
	tr := s.m.Traffic()
	if tr == nil || taskID <= 0 {
		return repro
	}
	var rp report.Reproduction
	if json.Unmarshal(repro, &rp) != nil || len(rp.Steps) == 0 {
		return repro
	}
	// Task traffic window: [first_run_at|created_at, now]. The window plus step order
	// disambiguates repeated identical requests (e.g. many identical requests to one endpoint).
	since := time.Now().Add(-24 * time.Hour) // fallback if the task row is missing
	if tk, err := s.m.pg.GetTask(taskID); err == nil {
		if tk.FirstRunAt != nil {
			since = *tk.FirstRunAt
		} else {
			since = tk.CreatedAt
		}
	}
	refs := make([]traffic.StepRef, len(rp.Steps))
	for i, st := range rp.Steps {
		refs[i] = traffic.StepRef{Request: st.Request, Response: st.Response}
	}
	ids := tr.ResolveSteps(since.Unix(), time.Now().Unix(), refs)
	for i, id := range ids {
		if id != "" {
			rp.Steps[i].ExchangeID = id
		}
	}
	if out, err := json.Marshal(rp); err == nil {
		return out
	}
	return repro
}

// POST /api/findings/{id}/enrich — run AI enrichment, persist, return it.
// clipUTF8 truncates s to at most n bytes, keeping it valid UTF-8.
func clipUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}

// findingSetupTraffic pulls the target's REAL account-registration / login / OTP /
// captcha request packets from recorded traffic (bounded), so the reproduction recipe's
// Prerequisites can explain registration/login with verbatim packets.
// the platform's edge over AI-reconstructed PoC (which errs). Empty when no traffic.
func (s *Server) findingSetupTraffic(taskID int64) string {
	if s.m == nil || s.m.traffic == nil {
		return ""
	}
	hosts, _ := s.m.Assets().HostsForTask(taskID)
	kws := []string{"register", "signup", "regist", "authenticate", "/login", "forgetPassword", "sendOtp", "/otp", "captcha", "addUsernamePassw", "createUser", "user/add"}
	var b strings.Builder
	seen := map[string]bool{}
	for _, kw := range kws {
		var rows []traffic.ExchangeMeta
		var err error
		if len(hosts) > 0 {
			rows, _, err = s.m.traffic.PageHosts(hosts, "", kw, traffic.TrafficFilter{}, 0, 3)
		} else {
			rows, _, err = s.m.traffic.Page("", "", kw, traffic.TrafficFilter{}, 0, 3)
		}
		if err != nil {
			continue
		}
		for _, ex := range rows {
			if seen[ex.ID] || b.Len() > 6000 {
				continue
			}
			seen[ex.ID] = true
			req, resp, e := s.m.traffic.Get(ex.ID)
			if e != nil {
				continue
			}
			fmt.Fprintf(&b, "\n### %s %s (HTTP %d)\n[Request]\n%s\n[Response]\n%s\n", ex.Method, ex.URL, ex.Status, clipUTF8(req, 1400), clipUTF8(resp, 900))
		}
	}
	return b.String()
}

// enrichJob tracks an asynchronous AI-analysis run for a finding node.
type enrichJob struct {
	Status    string         `json:"status"` // analyzing | done | failed
	Error     string         `json:"error,omitempty"`
	StartedAt time.Time      `json:"started_at"`
	Result    *enrichmentDTO `json:"enrichment,omitempty"` // set when Status==done
}

// POST /api/findings/{id}/enrich — start (or re-attach to) async AI enrichment.
// Returns immediately with {status:"analyzing"}; the generation runs on s.ctx so a
// page switch / client disconnect can't abort it, and re-clicks dedup to the running
// job. Poll GET /api/findings/{id}/enrich/status for the result.
func (s *Server) findingEnrich(w http.ResponseWriter, r *http.Request) {
	id, ok := nodeIDFromPath(r)
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	f, err := s.m.pg.GetFindingByNodeID(id)
	if err != nil || f == nil {
		writeErr(w, 404, "Finding not found")
		return
	}
	company := strings.TrimSpace(r.URL.Query().Get("company"))
	locale := report.DefaultLocale // Accept legacy requests but generate English narrative.

	s.enrichMu.Lock()
	if j := s.enrichJobs[id]; j != nil && j.Status == "analyzing" {
		snap := *j // already running — re-attach, don't start a second generation
		s.enrichMu.Unlock()
		writeJSON(w, 202, snap)
		return
	}
	job := &enrichJob{Status: "analyzing", StartedAt: time.Now()}
	s.enrichJobs[id] = job
	snap := *job
	s.enrichMu.Unlock()

	go s.runEnrich(id, f, locale, company)
	writeJSON(w, 202, snap)
}

// GET /api/findings/{id}/enrich/status — poll the async enrichment. Reports the
// live job if one exists; otherwise falls back to the persisted enrichment (so a
// return-to-page after completion shows the result even after job state is dropped).
func (s *Server) findingEnrichStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := nodeIDFromPath(r)
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	s.enrichMu.Lock()
	j := s.enrichJobs[id]
	s.enrichMu.Unlock()
	if j != nil {
		writeJSON(w, 200, *j)
		return
	}
	// no live job → report persisted result (done) or idle
	if saved, _ := s.m.pg.GetFindingEnrichment(id); saved != nil {
		writeJSON(w, 200, enrichJob{Status: "done", Result: dtoFromEnrichment(saved)})
		return
	}
	writeJSON(w, 200, enrichJob{Status: "idle"})
}

// runEnrich performs the actual AI enrichment on the server's background ctx and
// records the outcome in the job (and PG). It never touches the request lifecycle.
func (s *Server) runEnrich(id int64, f *db.DBFinding, locale, company string) {
	fail := func(msg string) {
		s.enrichMu.Lock()
		if j := s.enrichJobs[id]; j != nil {
			j.Status, j.Error = "failed", msg
		}
		s.enrichMu.Unlock()
	}
	defer func() {
		if rec := recover(); rec != nil {
			fail(fmt.Sprintf("AI analysis error: %v", rec))
		}
	}()
	in := report.FindingInput{VulnClass: f.VulnClass, Severity: f.Severity, Summary: f.Summary, PoC: f.Evidence, CreatedAt: f.CreatedAt, Reproduction: f.Reproduction}
	// Ground the AI PROSE (description/risk/remediation) in the finding's real work
	// transcript (the intent that yielded it), so enrichment reflects what actually
	// happened instead of expanding the one-line summary. Enrichment is prose-only
	// test_process comes deterministically from recorded reproduction or raw PoC, so we
	// no longer retrieve setup traffic for AI reconstruction of prerequisites.
	// Include full requests, truncated responses, and summaries without thinking noise.
	in.Transcript = s.m.pg.FindingReproTranscript(id, 22000)
	enr := report.EnrichFinding(s.ctx, s.m.pg, s.skillDir, locale, in, company)
	if strings.TrimSpace(enr.Title) == "" && len(enr.Description) == 0 {
		fail("AI analysis failed: LLM is unconfigured or returned no content, possibly due to timeout")
		return
	}
	row := dbEnrichmentFromReport(id, enr)
	// keep any prior human status if this finding was already triaged
	if prev, _ := s.m.pg.GetFindingEnrichment(id); prev != nil && prev.Status != "" {
		row.Status = prev.Status
	}
	if err := s.m.pg.UpsertFindingEnrichment(row); err != nil {
		fail("Save failed: " + err.Error())
		return
	}
	saved, _ := s.m.pg.GetFindingEnrichment(id)
	s.enrichMu.Lock()
	if j := s.enrichJobs[id]; j != nil {
		j.Status, j.Result = "done", dtoFromEnrichment(saved)
	}
	s.enrichMu.Unlock()
}

// enrichmentHasProse reports whether an enrichment already carries AI-generated
// narrative (description / risk / remediation). A freshly-triaged row that only
// has only name/target/severity/status; this triggers enrichment on confirmation.
func enrichmentHasProse(e *db.FindingEnrichment) bool {
	if e == nil {
		return false
	}
	nonEmptyArr := func(raw json.RawMessage) bool {
		s := strings.TrimSpace(string(raw))
		return s != "" && s != "[]" && s != "null"
	}
	return nonEmptyArr(e.Description) || nonEmptyArr(e.RiskAnalysis) || nonEmptyArr(e.Remediation)
}

// PUT /api/findings/{id} — patch human-editable fields (name/target/severity/status).
// When confirmed without narrative, start background enrichment through runEnrich/enrichJob.
// Existing enrich/status polling updates the frontend automatically.
// Confirmation prepares structured report content so confirmed findings are deliverable
// without fragile last-minute generation during export.
func (s *Server) findingUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := nodeIDFromPath(r)
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	var body struct {
		Name     string `json:"name"`
		Target   string `json:"target"`
		Severity string `json:"severity"`
		Status   string `json:"status"`
		Locale   string `json:"locale"` // Optional report language supplied by the frontend.
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, 400, "Invalid request format")
		return
	}
	if err := s.m.pg.UpdateFindingEnrichmentFields(id, strings.TrimSpace(body.Name),
		strings.TrimSpace(body.Target), strings.TrimSpace(body.Severity), strings.TrimSpace(body.Status)); err != nil {
		writeErr(w, 500, "Save failed: "+err.Error())
		return
	}
	saved, _ := s.m.pg.GetFindingEnrichment(id)

	enriching := false
	if strings.TrimSpace(body.Status) == "confirmed" && !enrichmentHasProse(saved) {
		if f, err := s.m.pg.GetFindingByNodeID(id); err == nil && f != nil {
			s.enrichMu.Lock()
			if j := s.enrichJobs[id]; j == nil || j.Status != "analyzing" {
				s.enrichJobs[id] = &enrichJob{Status: "analyzing", StartedAt: time.Now()}
				enriching = true
			}
			s.enrichMu.Unlock()
			if enriching {
				go s.runEnrich(id, f, strings.TrimSpace(body.Locale), "")
			}
		}
	}
	writeJSON(w, 200, map[string]any{"enrichment": dtoFromEnrichment(saved), "enriching": enriching})
}
