import * as vscode from 'vscode';
import { SidebarProvider } from './sidebarProvider';

export function activate(context: vscode.ExtensionContext) {
    console.log('Terra Sandbox extension is now active');

    const sidebarProvider = new SidebarProvider(context.extensionUri);

    // Регистрируем WebviewViewProvider для sidebar panel
    context.subscriptions.push(
        vscode.window.registerWebviewViewProvider(
            'terra-sandbox.panel',
            sidebarProvider
        )
    );

    // Команда для обновления данных
    context.subscriptions.push(
        vscode.commands.registerCommand('terra-sandbox.refresh', () => {
            sidebarProvider.refresh();
            vscode.window.showInformationMessage('Sandbox info refreshed');
        })
    );

    // Команда "Открыть Web UI"
    context.subscriptions.push(
        vscode.commands.registerCommand('terra-sandbox.openWeb', () => {
            const webUrl = process.env.SANDBOX_WEB_URL;
            if (webUrl) {
                vscode.env.openExternal(vscode.Uri.parse(webUrl));
            } else {
                vscode.window.showWarningMessage('Web UI URL not configured');
            }
        })
    );
}

export function deactivate() {
    console.log('Terra Sandbox extension deactivated');
}
