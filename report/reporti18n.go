package report

// English report labels and fallback narrative. Legacy locale requests resolve to English.

import (
	"fmt"
	"strings"
	"time"
)

// DefaultLocale is the only output locale supported by this fork.
const DefaultLocale = "en-US"

func normLocale(_ string) string { return DefaultLocale }

// ---- severity ----

var sevNames = map[string]map[string]string{
	"critical": {"en-US": "Critical"},
	"high":     {"en-US": "High"},
	"medium":   {"en-US": "Medium"},
	"low":      {"en-US": "Low"},
	"info":     {"en-US": "Info"},
}

// severityName renders a canonical/localized severity word for the given locale.
func severityName(locale, sev string) string {
	c := canonSeverity(sev)
	if m, ok := sevNames[c]; ok {
		if v := m[normLocale(locale)]; v != "" {
			return v
		}
		return m["en-US"]
	}
	return sevNames["info"][normLocale(locale)]
}

// ---- status / small words ----

func statusPending(locale string) string {
	return "Pending"
}
func statusRemarkFirst(locale string) string {
	return "First identified in this assessment"
}
func defaultTargetName(locale string) string {
	return "Target System"
}
func reportWord(locale string) string {
	return "Penetration Test Report"
}

// defaultReportTitle builds the fallback cover title "<project> <report word>",
// localized. When project is empty it's just the report word.
func defaultReportTitle(locale, project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return reportWord(locale)
	}
	return project + " " + reportWord(locale)
}
func clientDefault(locale string) string {
	return "Client"
}
func projectDefault(locale string) string {
	return "Penetration Testing Project"
}
func moduleDash() string { return "—" }

// ---- Composite formatters ----

// sectionSystemHeader renders a title such as "3.1 example.com (example.com)".
func sectionSystemHeader(locale string, idx int, name, addr string) string {
	return fmt.Sprintf("3.%d %s (%s)", idx, name, addr)
}

// vulnObject renders a remediation item as "Title (host)".
func vulnObject(locale, title, target string) string {
	if strings.TrimSpace(target) == "" {
		return title
	}
	return fmt.Sprintf("%s (%s)", title, target)
}

// localizedDate formats today's date in English.
func localizedDate(locale string) string { return time.Now().Format("January 2, 2006") }

// ---- fallback prose (used only when the LLM is unavailable) ----

func fallbackBackground(locale, client, company, project string) string {
	return fmt.Sprintf("%s commissioned %s to perform a penetration test of the information systems related to %s. Security engineers simulated real-world attacker behaviour, combining manual testing with tooling to surface security risks from multiple angles and to provide hardening recommendations.", client, company, project)
}

// infoSeg / infoSegParen append an "info"-severity segment to a breakdown only
// when there are informational findings, so the enumerated counts reconcile with
// the total (which includes informational findings). infoSeg carries the count,
// infoSegParen is the unit-less form used inside parentheses.
func infoSeg(locale string, info int) string {
	if info <= 0 {
		return ""
	}
	return fmt.Sprintf(", %d Info", info)
}
func infoSegParen(locale string, info int) string {
	if info <= 0 {
		return ""
	}
	return fmt.Sprintf(", %d Info", info)
}

func fallbackServiceResult(locale string, total, crit, high, med, low, info int) string {
	return fmt.Sprintf("A total of %d security issues were identified: %d Critical, %d High, %d Medium, %d Low%s.", total, crit, high, med, low, infoSeg(locale, info))
}

func fallbackRemediationConclusion(locale string) string {
	return "This was an initial penetration assessment; all identified issues are currently pending remediation. Remediation should follow the recommendations in this report, with retesting performed afterwards to confirm closure."
}

func fallbackFindingsIntro(locale, company string, crit, high, med, low, info int) string {
	return fmt.Sprintf("%s performed penetration testing against the target systems and identified %d Critical, %d High, %d Medium, %d Low%s findings, as detailed in the table below:", company, crit, high, med, low, infoSeg(locale, info))
}

// overallConclusion builds the chapter-5 overall conclusion sentence.
func overallConclusion(locale string, total, crit, high, med, low, info int, worst string) string {
	s := fmt.Sprintf("A total of %d security issues were identified (%d Critical, %d High, %d Medium, %d Low%s). The overall system security rating is \"%s\". ", total, crit, high, med, low, infoSegParen(locale, info), worst)
	if crit > 0 || high > 0 {
		s += fmt.Sprintf("The environment contains %d Critical and %d High findings; immediate remediation followed by retesting is strongly recommended.", crit, high)
	} else if med > 0 {
		s += "No Critical or High findings were identified; Medium findings should be remediated on schedule."
	} else {
		s += "The overall security posture is good; only low-risk or informational issues were found."
	}
	return s
}

// ---- AI output-language directive (injected into prompts) ----

// outputLangDirective tells the LLM which language to write all prose in.
func outputLangDirective(locale string) string {
	return "Write ALL narrative fields (background, description, risk_analysis, remediation, etc.) in professional, client-ready English. Keep the JSON keys in English exactly as specified."
}
