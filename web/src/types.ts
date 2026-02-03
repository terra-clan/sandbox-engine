// Sandbox Engine - Types

export interface FileNode {
  id: string;
  name: string;
  type: 'file' | 'folder';
  children?: FileNode[];
  content?: string;
  language?: string;
}

export interface Service {
  name: string;
  port: number;
  status: 'running' | 'stopped' | 'starting';
  url?: string;
}

export interface SandboxInfo {
  id: string;
  templateId: string;
  status: 'creating' | 'running' | 'stopped' | 'error';
  createdAt: string;
  expiresAt: string;
  services: Service[];
  workDir: string;
}

export interface SandboxConfig {
  sandboxId: string;
  apiToken: string;
  apiBaseUrl: string;
  wsBaseUrl: string;
}

// API Response types
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface SandboxResponse {
  sandbox: SandboxInfo;
}

// Helper to get time remaining
export const getTimeRemaining = (expiresAt: string): { minutes: number; seconds: number; expired: boolean } => {
  const now = new Date().getTime();
  const expiry = new Date(expiresAt).getTime();
  const diff = expiry - now;

  if (diff <= 0) {
    return { minutes: 0, seconds: 0, expired: true };
  }

  const minutes = Math.floor(diff / 60000);
  const seconds = Math.floor((diff % 60000) / 1000);

  return { minutes, seconds, expired: false };
};

// Format time as MM:SS
export const formatTime = (minutes: number, seconds: number): string => {
  return `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
};
