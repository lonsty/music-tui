// Command genkeys regenerates the keyboard-shortcut tables in README.md from
// the TUI key registry. Run it with: go generate ./...
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/lonsty/music-tui/internal/tui"
)

const (
	begin = "<!-- BEGIN GENERATED KEYS -->"
	end   = "<!-- END GENERATED KEYS -->"
	path  = "../../README.md"
)

func main() {
	tui.SetLang(tui.LangZH)
	table := tui.ReadmeKeyTables()

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read README:", err)
		os.Exit(1)
	}
	src := string(data)
	i := strings.Index(src, begin)
	j := strings.Index(src, end)
	if i < 0 || j < 0 || j < i {
		fmt.Fprintln(os.Stderr, "README missing generated-key markers")
		os.Exit(1)
	}
	out := src[:i+len(begin)] + "\n\n" + strings.TrimRight(table, "\n") + "\n\n" + src[j:]
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write README:", err)
		os.Exit(1)
	}
}
