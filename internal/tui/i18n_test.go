package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestTranslationKeysCoverage catches T("...") calls whose key is missing from
// either language table; missing keys render as raw identifiers in the UI.
func TestTranslationKeysCoverage(t *testing.T) {
	re := regexp.MustCompile(`\bT\("([^"]+)"\)`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			used[m[1]] = true
		}
	}
	for _, lang := range []Lang{LangEN, LangZH} {
		for key := range used {
			if _, ok := translations[lang][key]; !ok {
				t.Errorf("missing %v translation for %q", lang, key)
			}
		}
	}
}

func TestHelpOverlayHasNoRawKeys(t *testing.T) {
	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)

	a := &App{W: 120, H: 50}
	out := a.renderHelpOverlay()
	if strings.Contains(out, "action_") {
		t.Fatalf("help overlay leaks raw i18n keys:\n%s", out)
	}
}
