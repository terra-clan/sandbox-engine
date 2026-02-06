#!/bin/bash
# Entrypoint script for sandbox workspace containers
# Sets up Claude Code auth from mounted volume

# Fix permissions on mounted volumes
if [ -d /home/coder/.claude ]; then
    sudo chown -R coder:coder /home/coder/.claude 2>/dev/null || true
fi

# Claude Code stores credentials in ~/.claude/.credentials.json
# The volume is mounted at /home/coder/.claude with pre-populated auth
if [ -f /home/coder/.claude/.credentials.json ]; then
    echo "Claude Code credentials found"
fi

# Execute the main command
exec "$@"
