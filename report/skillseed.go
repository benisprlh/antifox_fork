package report

// Seeds the editable report-generation skill on startup by materializing the
// built-in Go defaults (defaults.go) into files under <skillDir>/report-generation/.
// Only writes files that are MISSING, so user edits in the Skills page are never
// overwritten. This makes the built-in skeleton + prompts editable without ever
// requiring a hand-maintained copy: the Go defaults remain the single source, and
// this projects them to disk once so they can be tuned and hot-reloaded.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var seedLocales = []string{"en-US"}

// SeedSkillFiles writes any missing report-generation skill files. Best-effort:
// returns the first write error but always attempts every file.
func SeedSkillFiles(skillDir string) error {
	if strings.TrimSpace(skillDir) == "" {
		return nil
	}
	base := filepath.Join(skillDir, reportSkillName)
	var firstErr error
	put := func(rel, content string) {
		if err := writeIfMissing(filepath.Join(base, rel), content); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	putJSON := func(rel string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		put(rel, string(b)+"\n")
	}

	put("SKILL.md", skillMD)
	put(filepath.Join("prompts", "report-level.tmpl"), promptReportLevelTmpl)
	put(filepath.Join("prompts", "vuln-narrative.tmpl"), promptVulnTmpl)

	for _, loc := range seedLocales {
		d := filepath.Join("skeleton", loc)
		putJSON(filepath.Join(d, "methodology.json"), map[string]any{
			"intro": methodologyIntro(loc), "phases": methodology(loc),
		})
		putJSON(filepath.Join(d, "tools.json"), tools(loc))
		putJSON(filepath.Join(d, "standards.json"), standards(loc))
		putJSON(filepath.Join(d, "checklist.json"), checklist(loc))
		putJSON(filepath.Join(d, "personnel.json"), defaultPersonnel(loc))
	}
	return firstErr
}

// writeIfMissing creates parent dirs and writes content only when path is absent.
func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // exists → keep user edits
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

const skillMD = `---
name: report-generation
description: Editable penetration testing report content, including AI rewriting prompts and fixed sections (methodology, checklist, tools, standards, personnel), in English. The report engine reads files on each generation, so edits take effect immediately.
---

# Report Generation (Editable Content)

This skill stores **editable penetration testing report content** for the Report Management generation pipeline. Changes take effect **the next time a report is generated**, without restarting or rebuilding. Missing or damaged files automatically fall back to built-in defaults.

## Structure

- ` + "`prompts/report-level.tmpl`" + ` — AI prompts for report-level narrative (background, service results, remediation conclusions, matched checklist items).
- ` + "`prompts/vuln-narrative.tmpl`" + ` — AI prompts for rewriting individual findings (description, reproduction, risk, remediation, CVSS).
- ` + "`skeleton/en-US/methodology.json`" + ` — Section 2.2, testing process: ` + "`{intro, phases:[{name,desc}]}`" + `.
- ` + "`skeleton/en-US/checklist.json`" + ` — Section 2.3, testing checklist: ` + "`[{category,name,desc}]`" + `. Items identified by AI as affected are rendered as "Fail".
- ` + "`skeleton/en-US/tools.json`" + ` — Section 2.4, testing tools: ` + "`[{name,desc}]`" + `.
- ` + "`skeleton/en-US/standards.json`" + ` — Section 2.5, reference standards: ` + "`[\"...\"]`" + `.
- ` + "`skeleton/en-US/personnel.json`" + ` — Section 2.1, default testing personnel: ` + "`[{name,role,contact}]`" + `. Values entered in report settings take precedence.

Report content is stored in the en-US directory. Legacy locale selections fall back to English.

## Prompt Template Variables

Prompts use Go ` + "`text/template`" + `, with ` + "`===USER===`" + ` separating the system and user sections:

- Report level: ` + "`{{.Company}}` `{{.Project}}` `{{.Client}}` `{{.FindingsList}}` `{{.LangDirective}}`" + `
- Finding level: ` + "`{{.Company}}` `{{.VulnClass}}` `{{.Severity}}` `{{.Summary}}` `{{.PoC}}` `{{.LangDirective}}`" + `

The LangDirective variable requires professional English narrative while preserving evidence verbatim.
`

const promptReportLevelTmpl = `You are a senior penetration-testing report author at {{.Company}}. Write professional, client-readable, technically rigorous reports in English. Return only JSON without Markdown fences.
===USER===
Write the report overview from the following project and findings.

Project name: {{.Project}}
Client: {{.Client}}
Findings:
{{.FindingsList}}
Use a professional delivery tone; avoid phrases such as 'testing took place today'.
- background: two concise paragraphs in a string array. First explain that the client commissioned {{.Company}} and describe objectives; second explain scope and manual/tool-assisted testing.
- service_result: one paragraph summarizing systems tested, finding counts, and severity distribution.
- remediation_conclusion: one paragraph; initial findings default to pending remediation, with prompt fixes and retesting recommended.
- checklist_hits: array of standard test-item names inferred from finding types, such as 'SQL injection', 'Unauthorized access', 'Authentication bypass', 'Sensitive information disclosure', 'Command injection', 'Cross-site scripting (XSS)', or 'Vertical privilege escalation'.

Return only JSON: {"background":["...","..."],"service_result":"...","remediation_conclusion":"...","checklist_hits":["...","..."]}

Output language requirement: {{.LangDirective}}
`

const promptVulnTmpl = `You are a senior penetration-testing report author at {{.Company}}. Turn raw findings into concise, professional, client-ready English report content. Return only JSON without Markdown fences, pleasantries, disclaimers, or generic boilerplate. Preserve actual requests, commands, and responses; never invent evidence.
===USER===
Rewrite this finding as report content. Be concise, specific, and restrained: every sentence must convey real information about this target and vulnerability. Do not pad the report.

Vulnerability class / original title: {{.VulnClass}}
Severity: {{.Severity}}
Original summary:
{{.Summary}}
Original reproduction / evidence:
{{.PoC}}

Field requirements:
- title: concise, professional vulnerability name, e.g. 'Authentication bypass permits administrator takeover'. Preserve or lightly refine suitable titles including CVEs; do not copy the summary.
- target: affected host or URL extracted from the summary/evidence; empty if unknown.
- description: one paragraph, 2–3 sentences explaining the vulnerability, affected point, and root cause. Avoid classification lists, repeated remediation, and filler.
- module: short functional module name, such as 'External account login / unauthenticated endpoint'.
- risk_analysis: 2–4 specific consequences, one sentence each, describing who can do what and obtain which data or privileges. Use fewer items if evidence is limited; avoid generic claims.
- remediation: 2–4 actionable recommendations, one sentence each, specifying what to change and validate. Avoid vague advice such as general awareness or regular audits.
- CVSS 3.1: cvss_score (number), cvss_vector (e.g. 'AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H'), and eight English cvss_metrics values, e.g. {"AV":"Network","AC":"Low","PR":"None","UI":"None","S":"Unchanged","C":"High","I":"High","A":"High"}.

Do not reconstruct test_process; the platform generates it directly from real evidence. Return "test_process":[].

Return only JSON: {"title":"...","target":"...","description":["..."],"module":"...","test_process":[],"risk_analysis":["..."],"remediation":["..."],"cvss_score":0,"cvss_vector":"...","cvss_metrics":{...}}

Output language requirement: {{.LangDirective}}
`
