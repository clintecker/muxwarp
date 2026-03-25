// Command tuidemo runs the muxwarp TUI with hardcoded fake session data.
// This is a temporary visual testing tool; it will be removed once the real
// scanner is wired in.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/clint/muxwarp/internal/tui"
)

func main() {
	m := tui.NewModelWithFakeData()
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If the user selected a session, print it.
	if model, ok := result.(tui.Model); ok {
		if t := model.WarpTarget(); t != nil {
			fmt.Printf("Warp target: %s on %s\n", t.Name, t.Host)
		} else {
			fmt.Println("No session selected.")
		}
	}
}
