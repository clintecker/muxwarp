#!/bin/bash
set -e

# Create tmux sessions as the demo user if SESSIONS is set.
# Format: "name:windows name:windows ..."
if [ -n "$SESSIONS" ]; then
    for entry in $SESSIONS; do
        name="${entry%%:*}"
        windows="${entry##*:}"
        su - demo -c "tmux new-session -d -s '$name'" 2>/dev/null || true
        # Add extra windows (session starts with 1).
        for ((i=1; i<windows; i++)); do
            su - demo -c "tmux new-window -t '$name'" 2>/dev/null || true
        done
    done
fi

# Start sshd in foreground.
exec /usr/sbin/sshd -D -e
