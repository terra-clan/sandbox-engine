import * as vscode from 'vscode';
import { SandboxInfo, loadSandboxInfo } from './sandboxInfo';

export class SidebarProvider implements vscode.WebviewViewProvider {
    private _view?: vscode.WebviewView;
    private _sandboxInfo?: SandboxInfo;
    private _timerInterval?: ReturnType<typeof setInterval>;

    constructor(private readonly _extensionUri: vscode.Uri) {}

    public resolveWebviewView(webviewView: vscode.WebviewView) {
        this._view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [this._extensionUri]
        };

        this._sandboxInfo = loadSandboxInfo();
        webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);

        // Обновляем таймер каждую секунду
        this._timerInterval = setInterval(() => {
            this._updateTimer();
        }, 1000);

        webviewView.onDidDispose(() => {
            if (this._timerInterval) {
                clearInterval(this._timerInterval);
            }
        });

        // Обработка сообщений от webview
        webviewView.webview.onDidReceiveMessage(message => {
            switch (message.command) {
                case 'submit':
                    vscode.commands.executeCommand('terra-sandbox.submit');
                    break;
                case 'openService':
                    vscode.env.openExternal(vscode.Uri.parse(message.url));
                    break;
            }
        });
    }

    public refresh() {
        this._sandboxInfo = loadSandboxInfo();
        if (this._view) {
            this._view.webview.html = this._getHtmlForWebview(this._view.webview);
        }
    }

    private _updateTimer() {
        if (this._view && this._sandboxInfo) {
            const remaining = this._calculateTimeRemaining();
            this._view.webview.postMessage({
                command: 'updateTimer',
                time: remaining
            });
        }
    }

    private _calculateTimeRemaining(): string {
        if (!this._sandboxInfo?.expiresAt) {
            return '--:--:--';
        }

        const now = Date.now();
        const expires = new Date(this._sandboxInfo.expiresAt).getTime();
        const diff = expires - now;

        if (diff <= 0) {
            return '00:00:00';
        }

        const hours = Math.floor(diff / 3600000);
        const minutes = Math.floor((diff % 3600000) / 60000);
        const seconds = Math.floor((diff % 60000) / 1000);

        return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    }

    private _getHtmlForWebview(webview: vscode.Webview): string {
        const info = this._sandboxInfo;

        const servicesHtml = info?.services?.map(s => `
            <div class="service ${s.status}">
                <span class="status-dot"></span>
                <span class="name">${this._escapeHtml(s.name)}</span>
                ${s.url ? `<a href="#" onclick="openService('${this._escapeHtml(s.url)}')">Open</a>` : ''}
            </div>
        `).join('') || '<p class="no-services">No services</p>';

        return `<!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
            <style>
                * {
                    box-sizing: border-box;
                }
                body {
                    padding: 12px;
                    font-family: var(--vscode-font-family);
                    color: var(--vscode-foreground);
                    font-size: 13px;
                    line-height: 1.4;
                }
                .section {
                    margin-bottom: 20px;
                }
                .section-title {
                    font-weight: 600;
                    margin-bottom: 8px;
                    color: var(--vscode-textLink-foreground);
                    font-size: 12px;
                    text-transform: uppercase;
                    letter-spacing: 0.5px;
                }
                .timer {
                    font-size: 28px;
                    font-weight: bold;
                    text-align: center;
                    padding: 16px;
                    background: var(--vscode-editor-background);
                    border-radius: 6px;
                    font-family: 'SF Mono', Monaco, 'Courier New', monospace;
                    border: 1px solid var(--vscode-panel-border);
                }
                .timer.warning {
                    color: #f59e0b;
                    border-color: #f59e0b;
                    background: rgba(245, 158, 11, 0.1);
                }
                .timer.danger {
                    color: #ef4444;
                    border-color: #ef4444;
                    background: rgba(239, 68, 68, 0.1);
                    animation: pulse 1s infinite;
                }
                @keyframes pulse {
                    0%, 100% { opacity: 1; }
                    50% { opacity: 0.7; }
                }
                .task-description {
                    padding: 12px;
                    background: var(--vscode-editor-background);
                    border-radius: 6px;
                    max-height: 200px;
                    overflow-y: auto;
                    border: 1px solid var(--vscode-panel-border);
                    white-space: pre-wrap;
                    word-wrap: break-word;
                }
                .service {
                    display: flex;
                    align-items: center;
                    padding: 8px 12px;
                    background: var(--vscode-editor-background);
                    border-radius: 4px;
                    margin-bottom: 6px;
                    border: 1px solid var(--vscode-panel-border);
                }
                .status-dot {
                    width: 8px;
                    height: 8px;
                    border-radius: 50%;
                    margin-right: 10px;
                    flex-shrink: 0;
                }
                .service.online .status-dot {
                    background: #22c55e;
                    box-shadow: 0 0 6px #22c55e;
                }
                .service.offline .status-dot {
                    background: #ef4444;
                }
                .service.pending .status-dot {
                    background: #f59e0b;
                    animation: blink 1s infinite;
                }
                @keyframes blink {
                    0%, 100% { opacity: 1; }
                    50% { opacity: 0.3; }
                }
                .service .name {
                    flex: 1;
                    font-weight: 500;
                }
                .service a {
                    color: var(--vscode-textLink-foreground);
                    text-decoration: none;
                    font-size: 12px;
                    padding: 2px 8px;
                    border-radius: 3px;
                    background: var(--vscode-button-secondaryBackground);
                }
                .service a:hover {
                    background: var(--vscode-button-secondaryHoverBackground);
                }
                .no-services {
                    color: var(--vscode-descriptionForeground);
                    font-style: italic;
                }
                .submit-btn {
                    width: 100%;
                    padding: 12px;
                    background: var(--vscode-button-background);
                    color: var(--vscode-button-foreground);
                    border: none;
                    border-radius: 6px;
                    cursor: pointer;
                    font-size: 14px;
                    font-weight: 600;
                    transition: background 0.2s;
                }
                .submit-btn:hover {
                    background: var(--vscode-button-hoverBackground);
                }
                .submit-btn:active {
                    transform: scale(0.98);
                }
                .sandbox-id {
                    font-size: 11px;
                    color: var(--vscode-descriptionForeground);
                    text-align: center;
                    margin-top: 16px;
                    padding-top: 12px;
                    border-top: 1px solid var(--vscode-panel-border);
                }
                .sandbox-id code {
                    font-family: 'SF Mono', Monaco, 'Courier New', monospace;
                    background: var(--vscode-editor-background);
                    padding: 2px 6px;
                    border-radius: 3px;
                }
            </style>
        </head>
        <body>
            <div class="section">
                <div class="section-title">Time Remaining</div>
                <div class="timer" id="timer">${this._calculateTimeRemaining()}</div>
            </div>

            <div class="section">
                <div class="section-title">Task</div>
                <div class="task-description">${this._escapeHtml(info?.taskDescription || 'Loading...')}</div>
            </div>

            <div class="section">
                <div class="section-title">Services</div>
                ${servicesHtml}
            </div>

            <div class="section">
                <button class="submit-btn" onclick="submit()">Submit Work</button>
            </div>

            <div class="sandbox-id">
                Sandbox ID: <code>${this._escapeHtml(info?.sandboxId || 'unknown')}</code>
            </div>

            <script>
                const vscode = acquireVsCodeApi();

                function submit() {
                    vscode.postMessage({ command: 'submit' });
                }

                function openService(url) {
                    vscode.postMessage({ command: 'openService', url: url });
                }

                window.addEventListener('message', event => {
                    const message = event.data;
                    if (message.command === 'updateTimer') {
                        const timer = document.getElementById('timer');
                        if (timer) {
                            timer.textContent = message.time;

                            // Подсветка при малом времени
                            const parts = message.time.split(':');
                            const h = parseInt(parts[0], 10);
                            const m = parseInt(parts[1], 10);
                            const totalMinutes = h * 60 + m;

                            timer.className = 'timer';
                            if (totalMinutes < 10) {
                                timer.classList.add('danger');
                            } else if (totalMinutes < 30) {
                                timer.classList.add('warning');
                            }
                        }
                    }
                });
            </script>
        </body>
        </html>`;
    }

    private _escapeHtml(text: string): string {
        return text
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }
}
