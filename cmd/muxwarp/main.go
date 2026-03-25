package main

import (
	"fmt"
	"os"

	"github.com/clint/muxwarp/internal/config"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("muxwarp %s (commit %s, built %s)\n", version, commit, date)
		os.Exit(0)
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nExample config (%s):\n\n%s",
			err, config.DefaultPath(), config.ExampleConfig())
		os.Exit(1)
	}

	fmt.Printf("Loaded %d hosts\n", len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		fmt.Printf("  - %s\n", h)
	}
}
