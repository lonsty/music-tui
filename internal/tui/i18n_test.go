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
			// Registry-driven lookups are validated separately against both
			// language tables by TestRegistryTranslations.
			if arg == "emptyKey" || arg == "bd.DescKey" || arg == "bd.LabelKey" || arg == "labelKey" {
				continue
			}
			t.Errorf("%s: dynamic T() argument %q is not statically verifiable", name, m[1])
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

// TestRegistryTranslations covers the dynamic lookups in keymap.go: every
// registry label and description must exist in both languages.
func TestRegistryTranslations(t *testing.T) {
	for _, bd := range registry.bindings {
		for _, key := range []string{bd.LabelKey, bd.DescKey} {
			if key == "" {
				continue
			}
			for _, lang := range []Lang{LangEN, LangZH} {
				if strings.TrimSpace(translations[lang][key]) == "" {
					t.Errorf("action %s: missing %v translation for %q", bd.Action, lang, key)
				}
			}
		}
	}
	for key := range translations[LangEN] {
		if _, ok := translations[LangZH][key]; !ok {
			t.Errorf("missing ZH translation for %q", key)
		}
	}
	for key := range translations[LangZH] {
		if _, ok := translations[LangEN][key]; !ok {
			t.Errorf("missing EN translation for %q", key)
		}
	}
}

var keyNameInText = regexp.MustCompile(`❮|(?i)press [a-z0-9+\[\]/]{1,12}|按 [a-z0-9+\[\]/]{1,12}`)

// TestTranslationValuesHaveNoKeyNames keeps physical keys out of prose. All
// key chips come from the registry instead.
func TestTranslationValuesHaveNoKeyNames(t *testing.T) {
	for _, lang := range []Lang{LangEN, LangZH} {
		for key, val := range translations[lang] {
			if keyNameInText.MatchString(val) {
				t.Errorf("%v translation %q embeds a key name: %q", lang, key, val)
			}
		}
	}
}
