export interface Config {
  context: ContextConfig;
  workspaces: WorkspaceConfig[];
}

export interface WorkspaceConfig {
  name: string;
  credentials: Credentials;
}

export interface ContextConfig {
  workspace: string;
  environment: string;
}

export interface Credentials {
  apiKey?: string;
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  device_code?: string;
  client_credentials?: string;
}
