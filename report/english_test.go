package report

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestEnglishReportFallbacksAndLegacyLocales(t *testing.T) {
	for _, locale := range []string{"", "en-US", "zh-CN", "zh-TW", "unknown"} {
		t.Run(locale, func(t *testing.T) {
			in := &GenInput{Locale: locale, ProjectName: "Example Assessment"}
			doc := BuildDoc(context.Background(), nil, in)
			if doc.Meta.Locale != DefaultLocale || !strings.Contains(doc.Meta.ReportTitle, "Penetration Test Report") {
				t.Fatalf("unexpected report metadata: %+v", doc.Meta)
			}
			body, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.ContainsFunc(string(body), func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Fatal("built-in report fallback contains Chinese text")
			}
		})
	}
}

func TestLegacyLocaleLoadsEnglishSkillEdits(t *testing.T) {
	dir := t.TempDir()
	if err := SeedSkillFiles(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, reportSkillName, "skeleton", "en-US", "methodology.json")
	if err := os.WriteFile(path, []byte(`{"intro":"English custom methodology","phases":[{"name":"Custom phase","desc":"English procedure"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"", "zh-CN", "zh-TW"} {
		if got := loadSkillContent(dir, locale).MethodologyIntro; got != "English custom methodology" {
			t.Fatalf("locale %q did not use the English skill: %q", locale, got)
		}
	}
}
