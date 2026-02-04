#!/bin/bash

# Fix permissions on mounted volumes
if [ -d /home/coder/.claude ]; then
    sudo chown -R coder:coder /home/coder/.claude 2>/dev/null || true
fi

# Copy saved config to home if exists
if [ -f /home/coder/.claude/config.json ]; then
    cp /home/coder/.claude/config.json /home/coder/.claude.json 2>/dev/null || true
fi

# Execute the command
exec "$@"
