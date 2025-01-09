import { createClient, createConfig } from "@hey-api/client-fetch";
import { ApiKeyAuth } from "./apikey";
import { ClientCredentials } from "./clientcredentials";
import {
  currentContext,
  loadCredentials,
  loadCredentialsFromSettings,
} from "./credentials";
import { BearerToken } from "./device_mode";
import { Credentials } from "./types";

interface RunClientWithCredentials {
  credentials: Credentials;
  workspace: string;
  apiUrl?: string;
  runUrl?: string;
}

class PublicAuth {
  async getHeaders(): Promise<Record<string, string>> {
    return {};
  }

  intercept(req: Request): void {}
}

export function newClientFromSettings(settings: any) {
  const credentials = loadCredentialsFromSettings(settings);

  const clientConfig: RunClientWithCredentials = {
    credentials,
    workspace: settings.workspace,
  };
  return newClientWithCredentials(clientConfig);
}

export function newClient() {
  const context = currentContext();
  let clientConfig: RunClientWithCredentials;

  if (context.workspace) {
    const credentials = loadCredentials(context.workspace);
    clientConfig = {
      credentials,
      workspace: context.workspace,
    };
  } else {
    throw new Error(
      "No workspace found, use `bl login [WORKSPACE]` to set one"
    );
    // TODO: implement settings like in python sdk
    // const settings = getSettings();
    // const credentials = loadCredentialsFromSettings(settings);
    // clientConfig = {
    //   credentials,
    //   workspace: settings.workspace,
    // };
  }
  return newClientWithCredentials(clientConfig);
}

export function newClientWithCredentials(config: RunClientWithCredentials) {
  let provider: ApiKeyAuth | BearerToken | ClientCredentials | PublicAuth;

  if (config.credentials.apiKey) {
    provider = new ApiKeyAuth(config.credentials, config.workspace);
  } else if (config.credentials.access_token) {
    provider = new BearerToken(
      config.credentials,
      config.workspace,
      config.apiUrl || "https://api.beamlit.com/v0"
    );
  } else if (config.credentials.client_credentials) {
    provider = new ClientCredentials(
      config.credentials,
      config.workspace,
      config.apiUrl || "https://api.beamlit.com/v0"
    );
  } else {
    provider = new PublicAuth();
  }
  return createClient(
    createConfig({
      baseUrl: config.apiUrl || "https://api.beamlit.com/v0",
      fetch: async (req) => {
        const headers = await provider.getHeaders();
        Object.entries(headers).forEach(([key, value]) => {
          req.headers.set(key, value);
        });
        return fetch(req);
      },
    })
  );
}

export async function getAuthenticationHeaders(
  settings: any
): Promise<Record<string, string> | null> {
  const context = currentContext();
  let credentials: Credentials;

  if (context.workspace) {
    credentials = loadCredentials(context.workspace);
  } else {
    throw new Error(
      "No workspace found, use `bl login [WORKSPACE]` to set one"
    );
  }

  const config: RunClientWithCredentials = {
    credentials,
    workspace: settings.workspace,
  };

  let provider: ApiKeyAuth | BearerToken | ClientCredentials | null = null;

  if (config.credentials.apiKey) {
    provider = new ApiKeyAuth(config.credentials, config.workspace);
  } else if (config.credentials.access_token) {
    provider = new BearerToken(
      config.credentials,
      config.workspace,
      config.apiUrl || "https://api.beamlit.com/v0"
    );
  } else if (config.credentials.client_credentials) {
    provider = new ClientCredentials(
      config.credentials,
      config.workspace,
      config.apiUrl || "https://api.beamlit.com/v0"
    );
  }

  if (!provider) {
    return null;
  }

  return await provider.getHeaders();
}
