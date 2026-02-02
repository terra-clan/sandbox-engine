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

    // Команда "Сдать работу"
    context.subscriptions.push(
        vscode.commands.registerCommand('terra-sandbox.submit', async () => {
            const confirm = await vscode.window.showWarningMessage(
                'Are you sure you want to submit your work?',
                { modal: true },
                'Yes, Submit'
            );

            if (confirm === 'Yes, Submit') {
                // TODO: API call to sandbox-engine
                // const sandboxId = process.env.SANDBOX_ID;
                // const apiUrl = process.env.SANDBOX_API_URL || 'http://localhost:8080';
                // await fetch(`${apiUrl}/api/v1/sandboxes/${sandboxId}/submit`, { method: 'POST' });

                vscode.window.showInformationMessage('Work submitted for review!');
            }
        })
    );
}

export function deactivate() {
    console.log('Terra Sandbox extension deactivated');
}
