package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestTranslationKeysCoverage catches T("...") calls whose key is missing from
// either language table, empty translations, and indirect keys passed through
// TrackListContext.EmptyMsg.  Missing keys render as raw identifiers in the UI.
func TestTranslationKeysCoverage(t *testing.T) {
	literalCall := regexp.MustCompile(`\bT\("([^"]+)"\)`)
	dynamicCall := regexp.MustCompile(`\bT\(([^")][^)]*)\)`)
	emptyMsg := regexp.MustCompile(`EmptyMsg:\s*"([^"]+)"`)

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
		src := string(data)

		for _, m := range literalCall.FindAllStringSubmatch(src, -1) {
			used[m[1]] = true
		}
		// EmptyMsg keys are resolved indirectly via T(emptyKey).  They are
		// declared as literals, so collect them here and reject any other
		// dynamic T() argument below.
		for _, m := range emptyMsg.FindAllStringSubmatch(src, -1) {
			used[m[1]] = true
		}
		for _, m := range dynamicCall.FindAllStringSubmatch(src, -1) {
			arg := strings.TrimSpace(m[1])
			// "func T(key string)" is the declaration, not a call site.
			if arg == "key string" {
				continue
			}
			if arg != "emptyKey" {
				t.Errorf("%s: dynamic T() argument %q is not statically verifiable", name, m[1])
			}
		}
	}

	for _, lang := range []Lang{LangEN, LangZH} {
		for key := range used {
			val, ok := translations[lang][key]
			if !ok {
				t.Errorf("missing %v translation for %q", lang, key)
				continue
			}
			if strings.TrimSpace(val) == "" {
				t.Errorf("empty %v translation for %q", lang, key)
			}
		}
		for key, val := range translations[lang] {
			if strings.TrimSpace(val) == "" {
				t.Errorf("empty %v translation for %q", lang, key)
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
