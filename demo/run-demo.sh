#!/bin/bash
set -euo pipefail

# Wrapper for demo recording that sets up SSH config for demo containers.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

tmpdir=$(mktemp -d)
export HOME="$tmpdir"
mkdir -p "$HOME/.ssh"

# Use demo SSH key without host key checking.
cat > "$HOME/.ssh/config" <<SSHEOF
Host *
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    IdentityFile $repo_root/demo/id_ed25519
    LogLevel ERROR
SSHEOF
chmod 600 "$HOME/.ssh/config"

# Fix SSH key permissions (git doesn't preserve 600).
chmod 600 "$repo_root/demo/id_ed25519"

# Copy demo config as the user's config.
cp "$repo_root/demo/muxwarp.config.yaml" "$HOME/.muxwarp.config.yaml"

exec "$repo_root/bin/muxwarp" "$@"
