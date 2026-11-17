package report

// Retroactive finding extraction: pull confirmed vulnerabilities out of a task's
// agent transcript (synthesis + worker conclusions) that were described but never
// recorded via report_finding — so they can be back-filled as finding nodes.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"antifox/db"
)

// ExtractedFinding is one vuln pulled from the transcript.
type ExtractedFinding struct {
	VulnClass string `json:"vulnclass"`
	Severity  string `json:"severity"` // high|medium|low
	Summary   string `json:"summary"`
	Target    string `json:"target"`
	Evidence  string `json:"evidence"`
}

const sysExtract = "You are a senior penetration-testing expert. Extract confirmed findings from execution and summary records. Write English and return only JSON without Markdown fences. Preserve actual evidence verbatim; never invent it."

const userExtract = `Extract all confirmed vulnerabilities and verified exposure/information-disclosure risks from these records. Exclude unverified guesses and negative conclusions.

Already recorded findings (do not duplicate):
%s
Execution / summary records:
%s

Requirements:
- One confirmed finding per item: vulnclass (e.g. 'Stored XSS', 'Hardcoded credentials', 'Unauthorized access'), severity (critical|high|medium|low; critical means complete system/data compromise), summary (one sentence including affected target), target (host/URL), and evidence (actual PoC/request/command/response).
- Extract only explicitly verified vulnerabilities/exposures. Exclude semantic duplicates of existing findings.
- Include unauthorized sensitive/management/debug interfaces, Swagger/API docs, public configuration/backups/source, and sensitive data when actually accessed with response evidence. Assign low for ordinary disclosure, medium for unauthorized sensitive/management access, and high for takeover or credentials/keys. Exclude version/CVE guesses, apparently injectable parameters, and interface existence without unauthorized exposure.
- Return an empty findings array when there are no new findings.

Return only JSON: {"findings":[{"vulnclass":"...","severity":"high","summary":"...","target":"...","evidence":"..."}]}`

// ExtractFindings asks the active LLM to extract confirmed vulns from the
// transcript, excluding ones already recorded (passed as short labels).
func ExtractFindings(ctx context.Context, pg *db.DB, transcript string, existing []string) ([]ExtractedFinding, error) {
	if strings.TrimSpace(transcript) == "" {
		return nil, fmt.Errorf("no conversation records available for analysis")
	}
	var ex strings.Builder
	for _, e := range existing {
		ex.WriteString("- ")
		ex.WriteString(e)
		ex.WriteString("\n")
	}
	if ex.Len() == 0 {
		ex.WriteString("(None)\n")
	}
	raw, err := complete(ctx, pg, sysExtract, fmt.Sprintf(userExtract, ex.String(), transcript), 4000)
	if err != nil {
		return nil, err
	}
	var out struct {
		Findings []ExtractedFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("failed to parse AI output")
	}
	return out.Findings, nil
}
