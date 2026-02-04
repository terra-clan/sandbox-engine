#!/bin/bash
# Entrypoint script for sandbox workspace containers
# Copies Claude Code config from volume to expected location

# Fix permissions on mounted volumes
if [ -d /home/coder/.claude ]; then
    sudo chown -R coder:coder /home/coder/.claude 2>/dev/null || true
fi

# If config exists in the mounted volume, copy it to the expected location
if [ -f /home/coder/.claude/config.json ]; then
    cp /home/coder/.claude/config.json /home/coder/.claude.json 2>/dev/null || true
    echo "Claude Code config restored from volume"
fi

# Execute the main command
exec "$@"
