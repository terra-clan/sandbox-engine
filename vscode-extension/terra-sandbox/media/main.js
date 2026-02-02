// Terra Sandbox - Main JavaScript
// This file can be used for additional client-side logic if needed
// Currently, scripts are embedded in the webview HTML for simplicity

(function() {
    'use strict';

    // VS Code API
    const vscode = typeof acquireVsCodeApi === 'function' ? acquireVsCodeApi() : null;

    // Utility functions
    function formatTime(ms) {
        if (ms <= 0) return '00:00:00';

        const hours = Math.floor(ms / 3600000);
        const minutes = Math.floor((ms % 3600000) / 60000);
        const seconds = Math.floor((ms % 60000) / 1000);

        return [hours, minutes, seconds]
            .map(n => n.toString().padStart(2, '0'))
            .join(':');
    }

    function postMessage(command, data = {}) {
        if (vscode) {
            vscode.postMessage({ command, ...data });
        }
    }

    // Export for global use
    window.TerraSandbox = {
        formatTime,
        postMessage
    };
})();
