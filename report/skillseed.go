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

var seedLocales = []string{"zh-CN", "zh-TW", "en-US"}

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
description: Editable penetration testing report content, including AI rewriting prompts and fixed sections (methodology, checklist, tools, standards, personnel), organized by language. The report engine reads files on each generation, so edits take effect immediately.
---

# Report Generation (Editable Content)

This skill stores **editable penetration testing report content** for the Report Management generation pipeline. Changes take effect **the next time a report is generated**, without restarting or rebuilding. Missing or damaged files automatically fall back to built-in defaults.

## Structure

- ` + "`prompts/report-level.tmpl`" + ` — AI prompts for report-level narrative (background, service results, remediation conclusions, matched checklist items).
- ` + "`prompts/vuln-narrative.tmpl`" + ` — AI prompts for rewriting individual findings (description, reproduction, risk, remediation, CVSS).
- ` + "`skeleton/<language>/methodology.json`" + ` — Section 2.2, testing process: ` + "`{intro, phases:[{name,desc}]}`" + `.
- ` + "`skeleton/<language>/checklist.json`" + ` — Section 2.3, testing checklist: ` + "`[{category,name,desc}]`" + `. Items identified by AI as affected are rendered as "Fail".
- ` + "`skeleton/<language>/tools.json`" + ` — Section 2.4, testing tools: ` + "`[{name,desc}]`" + `.
- ` + "`skeleton/<language>/standards.json`" + ` — Section 2.5, reference standards: ` + "`[\"...\"]`" + `.
- ` + "`skeleton/<language>/personnel.json`" + ` — Section 2.1, default testing personnel: ` + "`[{name,role,contact}]`" + `. Values entered in report settings take precedence.

Language directories: ` + "`zh-CN`" + ` (Simplified Chinese), ` + "`zh-TW`" + ` (Traditional Chinese), and ` + "`en-US`" + ` (English).

## Prompt Template Variables

Prompts use Go ` + "`text/template`" + `, with ` + "`===USER===`" + ` separating the system and user sections:

- Report level: ` + "`{{.Company}}` `{{.Project}}` `{{.Client}}` `{{.FindingsList}}` `{{.LangDirective}}`" + `
- Finding level: ` + "`{{.Company}}` `{{.VulnClass}}` `{{.Severity}}` `{{.Summary}}` `{{.PoC}}` `{{.LangDirective}}`" + `

` + "`{{.LangDirective}}`" + ` injects the target language instruction (Simplified Chinese, Traditional Chinese, or English), so the same language-independent prompt is shared across all three languages.
`

const promptReportLevelTmpl = `你是{{.Company}}的资深渗透测试报告撰写专家，擅长撰写专业、客户可读、技术严谨的渗透测试报告。严格只输出 JSON，不要 markdown 代码块。
===USER===
根据以下项目与漏洞清单撰写报告综述内容。

项目名称：{{.Project}}
客户名称：{{.Client}}
漏洞清单：
{{.FindingsList}}
要求（专业交付口吻，不要出现"本次测试于今天"之类措辞）：
- background：字符串数组，2 段。第一段说明客户委托{{.Company}}开展渗透测试及测试目标；第二段说明测试范围与人工+工具结合的方式。每段 60-130 字。
- service_result：1 段，概述测试了哪些系统、发现漏洞的数量与严重度分布。
- remediation_conclusion：1 段整改结论（首轮渗透默认全部待修复，建议尽快整改并复测）。
- checklist_hits：字符串数组，根据漏洞类型推断命中的标准测试项名称（例如 "SQL 注入"、"未授权访问"、"登录认证绕过"、"敏感信息泄露"、"命令注入"、"跨站脚本攻击（XSS）"、"垂直越权访问" 等标准名称）。

只输出 JSON：{"background":["...","..."],"service_result":"...","remediation_conclusion":"...","checklist_hits":["...","..."]}

输出语言要求：{{.LangDirective}}
`

const promptVulnTmpl = `你是{{.Company}}的资深渗透测试报告撰写专家。把漏洞原始记录改写为【交付级】报告正文——像资深顾问亲笔写给客户，精炼、专业、就事论事。严格只输出 JSON，不要 markdown 代码块，不要客套话、免责声明或放之四海皆准的套话。HTTP 请求/命令/响应等原文必须真实保留，不得编造。
===USER===
把下面这条漏洞改写成报告正文。核心要求：【精炼、具体、克制】——宁缺毋滥，不凑条数、不注水、不说正确的废话；每一句都要是针对本目标本漏洞的真信息。

漏洞类型/原始名称：{{.VulnClass}}
严重程度：{{.Severity}}
原始摘要：
{{.Summary}}
原始复现过程/证据：
{{.PoC}}

字段要求：
- title：规范、专业的漏洞名称（10-26 字，如"外部账户认证绕过致管理员接管漏洞"）。原始名已规范（含 CVE 等）则保留或微调，不要照抄摘要。
- target：受影响主机或 URL，从摘要/证据中提取；无法确定留空字符串。
- description：1 段、2-3 句。只说清三件事——这是什么漏洞、出在哪个点、本质成因。不堆 OWASP 分类、不复述修复、不空泛铺垫。
- module：所属功能模块（简短，如"外部账户登录 / 未授权接口"）。
- risk_analysis：2-4 条，每条一句，必须是【本目标本漏洞】能造成的具体后果（谁、能做什么、拿到什么数据或权限）。没有具体后果就少写，不要"可能影响系统安全"这类套话。
- remediation：2-4 条，每条一句，具体可落地（改哪个点、怎么改、校验什么）。不要"加强安全意识""定期安全审计"这类空话。
- CVSS 3.1：cvss_score(数字)、cvss_vector（如 "AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"）、cvss_metrics（八项英文取值，如 {"AV":"Network","AC":"Low","PR":"None","UI":"None","S":"Unchanged","C":"High","I":"High","A":"High"}）。

不要输出 test_process——复现过程由平台用真实证据直接生成，返回 "test_process":[] 即可。

只输出 JSON：{"title":"...","target":"...","description":["..."],"module":"...","test_process":[],"risk_analysis":["..."],"remediation":["..."],"cvss_score":0,"cvss_vector":"...","cvss_metrics":{...}}

输出语言要求：{{.LangDirective}}
`
