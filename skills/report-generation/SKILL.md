---
name: report-generation
description: Editable penetration testing report content, including AI rewriting prompts and fixed sections (methodology, checklist, tools, standards, personnel), organized by language. The report engine reads files on each generation, so edits take effect immediately.
---

# Report Generation (Editable Content)

This skill stores **editable penetration testing report content** for the Report Management generation pipeline. Changes take effect **the next time a report is generated**, without restarting or rebuilding. Missing or damaged files automatically fall back to built-in defaults.

## Structure

- `prompts/report-level.tmpl` — AI prompts for report-level narrative (background, service results, remediation conclusions, matched checklist items).
- `prompts/vuln-narrative.tmpl` — AI prompts for rewriting individual findings (description, reproduction, risk, remediation, CVSS).
- `skeleton/<language>/methodology.json` — Section 2.2, testing process: `{intro, phases:[{name,desc}]}`.
- `skeleton/<language>/checklist.json` — Section 2.3, testing checklist: `[{category,name,desc}]`. Items identified by AI as affected are rendered as "Fail".
- `skeleton/<language>/tools.json` — Section 2.4, testing tools: `[{name,desc}]`.
- `skeleton/<language>/standards.json` — Section 2.5, reference standards: `["..."]`.
- `skeleton/<language>/personnel.json` — Section 2.1, default testing personnel: `[{name,role,contact}]`. Values entered in report settings take precedence.

Language directories: `zh-CN` (Simplified Chinese), `zh-TW` (Traditional Chinese), and `en-US` (English).

## Prompt Template Variables

Prompts use Go `text/template`, with `===USER===` separating the system and user sections:

- Report level: `{{.Company}}` `{{.Project}}` `{{.Client}}` `{{.FindingsList}}` `{{.LangDirective}}`
- Finding level: `{{.Company}}` `{{.VulnClass}}` `{{.Severity}}` `{{.Summary}}` `{{.PoC}}` `{{.LangDirective}}`

`{{.LangDirective}}` injects the target language instruction (Simplified Chinese, Traditional Chinese, or English), so the same language-independent prompt is shared across all three languages.
