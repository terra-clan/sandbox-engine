export interface ServiceInfo {
    name: string;
    status: 'online' | 'offline' | 'pending';
    url?: string;
}

export interface SandboxInfo {
    sandboxId: string;
    templateId: string;
    expiresAt: string;
    services: ServiceInfo[];
    webUrl: string;
}

export function loadSandboxInfo(): SandboxInfo {
    // Загружаем из ENV переменных которые sandbox-engine инжектит в контейнер
    const sandboxId = process.env.SANDBOX_ID || 'unknown';
    const templateId = process.env.SANDBOX_TEMPLATE || 'unknown';
    const expiresAt = process.env.SANDBOX_EXPIRES_AT || new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString();
    const webUrl = process.env.SANDBOX_WEB_URL || '';

    // Парсим сервисы из JSON
    let services: ServiceInfo[] = [];
    try {
        const servicesJson = process.env.SANDBOX_SERVICES || '[]';
        services = JSON.parse(servicesJson);
    } catch (e) {
        services = [
            { name: 'PostgreSQL', status: 'online' },
            { name: 'Redis', status: 'online' },
        ];
    }

    return {
        sandboxId,
        templateId,
        expiresAt,
        services,
        webUrl
    };
}
