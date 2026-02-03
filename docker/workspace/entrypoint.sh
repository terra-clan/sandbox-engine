#!/bin/bash
# Entrypoint script for sandbox workspace containers
# Copies Claude Code config from volume to expected location

# If config exists in the mounted volume, copy it to the expected location
if [ -f /home/coder/.claude/config.json ]; then
    cp /home/coder/.claude/config.json /home/coder/.claude.json
    echo "Claude Code config restored from volume"
fi

# Execute the main command
exec "$@"
