#!/bin/bash
# Wrapper for demo recording that sets up SSH config for demo containers.
export HOME=$(mktemp -d)
mkdir -p "$HOME/.ssh"

# Use demo SSH key without host key checking.
cat > "$HOME/.ssh/config" <<SSHEOF
Host *
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    IdentityFile $(pwd)/demo/id_ed25519
    LogLevel ERROR
SSHEOF
chmod 600 "$HOME/.ssh/config"

# Fix SSH key permissions (git doesn't preserve 600).
chmod 600 "$(pwd)/demo/id_ed25519"

# Copy demo config as the user's config.
cp demo/muxwarp.config.yaml "$HOME/.muxwarp.config.yaml"

exec ./bin/muxwarp "$@"
