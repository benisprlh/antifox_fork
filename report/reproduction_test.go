package report

import (
	"encoding/json"
	"strings"
	"testing"
)

// sampleRepro mirrors what the agent records in-situ at report_finding time.
const sampleRepro = `{
  "entry_point": "POST /user/validate/otp/forgetPasswordVerify",
  "prerequisites": [
    "Register a test account: POST /user/register {\"mobile\":\"13800000000\",\"pwd\":\"P@ssw0rd123\"} → 200 {\"code\":0}",
    "Log in for a session: POST /user/login → Set-Cookie: SESSION=abc"
  ],
  "steps": [
    {"request": "POST /user/validate/otp/forgetPasswordVerify HTTP/1.1\nHost: t.example.com\nContent-Type: application/json\n\n{\"mobile\":\"13800000000\",\"otp\":\"000000\"}", "expected": "200 {\"code\":0,\"token\":\"...\"}", "proves": "Reset token returned without OTP validation"},
    {"request": "POST /user/resetPassword HTTP/1.1\n\n{\"token\":\"...\",\"pwd\":\"Hacked1.\"}", "expected": "200 {\"code\":0}", "proves": "Any account password can be reset"}
  ],
  "provenance": "Discovered by the authentication-bypass intent using password-reset flow enumeration"
}`

func TestReproToBlocks_LegacyLocaleUsesEnglish(t *testing.T) {
	blocks := reproToBlocks(json.RawMessage(sampleRepro), "zh-CN", nil)
	if len(blocks) == 0 {
		t.Fatal("expected blocks, got none")
	}
	joined := ""
	codeCount := 0
	for _, b := range blocks {
		joined += b.Type + ":" + b.Content + "\n"
		if b.Type == "code" {
			codeCount++
		}
	}
	for _, want := range []string{"Origin:", "Entry point:", "Prerequisites:", "Reproduction steps:", "Register a test account", "without OTP validation", "Any account password can be reset"} {
		if !strings.Contains(joined, want) {
			t.Errorf("rendered blocks missing %q\n---\n%s", want, joined)
		}
	}
	// two steps → two verbatim request code blocks
	if codeCount != 2 {
		t.Errorf("expected 2 code blocks (verbatim requests), got %d", codeCount)
	}
	// verbatim request must be preserved untouched
	if !strings.Contains(joined, "POST /user/validate/otp/forgetPasswordVerify HTTP/1.1") {
		t.Errorf("verbatim request not preserved:\n%s", joined)
	}
}

func TestReproToBlocks_EnUS_Labels(t *testing.T) {
	blocks := reproToBlocks(json.RawMessage(sampleRepro), "en-US", nil)
	joined := ""
	for _, b := range blocks {
		joined += b.Content + "\n"
	}
	for _, want := range []string{"Origin:", "Entry point:", "Prerequisites:", "Reproduction steps:", "Step 1", "Step 2"} {
		if !strings.Contains(joined, want) {
			t.Errorf("en-US labels missing %q\n---\n%s", want, joined)
		}
	}
}

func TestReproToBlocks_Empty(t *testing.T) {
	for _, raw := range []string{"", "{}", "null"} {
		if b := reproToBlocks(json.RawMessage(raw), "zh-CN", nil); len(b) != 0 {
			t.Errorf("reproToBlocks(%q) = %d blocks, want 0", raw, len(b))
		}
	}
}

// assembleVuln must prefer the captured reproduction over AI test_process and PoC.
func TestAssembleVuln_PrefersReproduction(t *testing.T) {
	f := FindingInput{
		VulnClass:    "OTP validation bypass",
		Severity:     "critical",
		Summary:      "OTP not validated in password-reset flow",
		PoC:          "some raw poc that should NOT win",
		Reproduction: json.RawMessage(sampleRepro),
	}
	enr := Enrichment{
		Title:       "OTP validation bypass allows arbitrary account takeover",
		TestProcess: []procBlock{{Type: "text", Content: "AI-reconstructed step that should NOT win"}},
	}
	d := assembleVuln("zh-CN", f, enr, "3.1.1", "", nil)
	joined := ""
	for _, b := range d.TestProcess {
		joined += b.Content + "\n"
	}
	if strings.Contains(joined, "should NOT win") {
		t.Errorf("captured reproduction did not take priority; got AI/PoC content:\n%s", joined)
	}
	if !strings.Contains(joined, "Entry point:") || !strings.Contains(joined, "Any account password can be reset") {
		t.Errorf("captured reproduction not rendered into test_process:\n%s", joined)
	}
}
