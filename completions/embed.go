// Package completions embeds shell completion scripts.
package completions

import "embed"

//go:embed muxwarp.bash muxwarp.zsh muxwarp.fish
var Scripts embed.FS
