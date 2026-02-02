# Terra Sandbox VS Code Extension

A VS Code / code-server extension that displays sandbox environment information in a sidebar panel.

## Features

- **Timer Display**: Shows remaining time for the sandbox session with visual warnings
- **Task Description**: Displays the current task/assignment
- **Services Status**: Shows status of associated services (PostgreSQL, Redis, etc.)
- **Submit Work**: Button to submit completed work for review

## Environment Variables

The extension reads data from environment variables injected by sandbox-engine:

| Variable | Description | Example |
|----------|-------------|---------|
| `SANDBOX_ID` | Unique sandbox identifier | `sb-abc123` |
| `TASK_DESCRIPTION` | Task description text | `Implement a REST API...` |
| `SANDBOX_EXPIRES_AT` | ISO timestamp when sandbox expires | `2024-01-15T12:00:00Z` |
| `SANDBOX_SERVICES` | JSON array of services | `[{"name":"PostgreSQL","status":"online"}]` |

## Development

```bash
# Install dependencies
npm install

# Compile TypeScript
npm run compile

# Watch for changes
npm run watch

# Package extension
npm run package
```

## Installation in code-server

1. Build the extension: `npm run package`
2. Copy the `.vsix` file to the sandbox container
3. Install: `code-server --install-extension terra-sandbox-0.1.0.vsix`

## Service Status Types

- `online` - Service is running (green indicator)
- `offline` - Service is not available (red indicator)
- `pending` - Service is starting (yellow blinking indicator)
