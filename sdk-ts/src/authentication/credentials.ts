import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import * as yaml from "js-yaml";
import { homedir } from "os";
import { join } from "path";
import { Config, ContextConfig, Credentials, WorkspaceConfig } from "./types";

function loadConfig(): Config {
  const config: Config = {
    workspaces: [],
    context: {
      workspace: "",
      environment: "",
    },
  };

  const homeDir = homedir();
  if (homeDir) {
    const configPath = join(homeDir, ".beamlit", "config.yaml");
    if (existsSync(configPath)) {
      try {
        const data = yaml.load(readFileSync(configPath, "utf8")) as any;
        if (data) {
          const workspaces: WorkspaceConfig[] = [];
          for (const ws of data.workspaces || []) {
            const creds: Credentials = {
              apiKey: ws.credentials?.apiKey || "",
              access_token: ws.credentials?.access_token || "",
              refresh_token: ws.credentials?.refresh_token || "",
              expires_in: ws.credentials?.expires_in || 0,
              device_code: ws.credentials?.device_code || "",
              client_credentials: ws.credentials?.client_credentials || "",
            };
            workspaces.push({ name: ws.name, credentials: creds });
          }
          config.workspaces = workspaces;
          if (data.context) {
            config.context = data.context;
          }
        }
      } catch (e) {
        // Invalid YAML, use empty config
      }
    }
  }
  return config;
}

function saveConfig(config: Config): void {
  const homeDir = homedir();
  if (!homeDir) {
    throw new Error("Could not determine home directory");
  }

  const configDir = join(homeDir, ".beamlit");
  const configFile = join(configDir, "config.yaml");

  mkdirSync(configDir, { recursive: true, mode: 0o700 });
  writeFileSync(
    configFile,
    yaml.dump({
      workspaces: config.workspaces.map((ws) => ({
        name: ws.name,
        credentials: {
          apiKey: ws.credentials.apiKey,
          access_token: ws.credentials.access_token,
          refresh_token: ws.credentials.refresh_token,
          expires_in: ws.credentials.expires_in,
          device_code: ws.credentials.device_code,
        },
      })),
      context: config.context,
    })
  );
}

export function listWorkspaces(): string[] {
  const config = loadConfig();
  return config.workspaces.map((workspace) => workspace.name);
}

export function currentContext(): ContextConfig {
  const config = loadConfig();
  return config.context;
}

export function setCurrentWorkspace(
  workspaceName: string,
  environment: string
): void {
  const config = loadConfig();
  config.context.workspace = workspaceName;
  config.context.environment = environment;
  saveConfig(config);
}

export function loadCredentials(workspaceName: string): Credentials {
  const config = loadConfig();
  const workspace = config.workspaces.find((ws) => ws.name === workspaceName);
  if (workspace) {
    return workspace.credentials;
  }
  return {
    apiKey: "",
    access_token: "",
    refresh_token: "",
    expires_in: 0,
    device_code: "",
    client_credentials: "",
  };
}

export function loadCredentialsFromSettings(settings: any): Credentials {
  return {
    apiKey: settings.authentication?.apiKey || "",
    access_token: "",
    refresh_token: "",
    expires_in: 0,
    device_code: "",
    client_credentials: "",
  };
}

export function createHomeDirIfMissing(): void {
  const homeDir = homedir();
  if (!homeDir) {
    console.error("Error getting home directory");
    return;
  }

  const credentialsDir = join(homeDir, ".beamlit");
  const credentialsFile = join(credentialsDir, "credentials.json");

  if (existsSync(credentialsFile)) {
    console.warn(
      "You are already logged in. Enter a new API key to overwrite it."
    );
  } else {
    try {
      mkdirSync(credentialsDir, { recursive: true, mode: 0o700 });
    } catch (e) {
      console.error(`Error creating credentials directory: ${e}`);
    }
  }
}

export function saveCredentials(
  workspaceName: string,
  credentials: Credentials
): void {
  createHomeDirIfMissing();
  if (!credentials.access_token && !credentials.apiKey) {
    console.info("No credentials to save, error");
    return;
  }

  const config = loadConfig();
  let found = false;

  for (let i = 0; i < config.workspaces.length; i++) {
    if (config.workspaces[i].name === workspaceName) {
      config.workspaces[i].credentials = credentials;
      found = true;
      break;
    }
  }

  if (!found) {
    config.workspaces.push({ name: workspaceName, credentials });
  }

  saveConfig(config);
}

export function clearCredentials(workspaceName: string): void {
  const config = loadConfig();
  config.workspaces = config.workspaces.filter(
    (ws) => ws.name !== workspaceName
  );

  if (config.context.workspace === workspaceName) {
    config.context.workspace = "";
  }

  saveConfig(config);
}
