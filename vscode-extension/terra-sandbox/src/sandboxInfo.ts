export interface ServiceInfo {
    name: string;
    status: 'online' | 'offline' | 'pending';
    url?: string;
}

export interface SandboxInfo {
    sandboxId: string;
    taskDescription: string;
    expiresAt: string;
    services: ServiceInfo[];
}

export function loadSandboxInfo(): SandboxInfo {
    // Загружаем из ENV переменных которые sandbox-engine инжектит в контейнер
    const sandboxId = process.env.SANDBOX_ID || 'unknown';
    const taskDescription = process.env.TASK_DESCRIPTION || 'Описание задания не загружено';
    const expiresAt = process.env.SANDBOX_EXPIRES_AT || new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString();

    // Парсим сервисы из JSON
    let services: ServiceInfo[] = [];
    try {
        const servicesJson = process.env.SANDBOX_SERVICES || '[]';
        services = JSON.parse(servicesJson);
    } catch (e) {
        // Fallback для демонстрации
        services = [
            { name: 'PostgreSQL', status: 'online' },
            { name: 'Redis', status: 'online' },
        ];
    }

    return {
        sandboxId,
        taskDescription,
        expiresAt,
        services
    };
}
