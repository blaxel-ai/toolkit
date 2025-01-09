import { saveCredentials } from "./credentials";
import { Credentials } from "./types";

interface DeviceLoginFinalizeResponse {
  access_token: string;
  expires_in: number;
  refresh_token: string;
  token_type: string;
}

export class ClientCredentials {
  private credentials: Credentials;
  private workspace_name: string;
  private base_url: string;

  constructor(
    credentials: Credentials,
    workspace_name: string,
    base_url: string
  ) {
    this.credentials = credentials;
    this.workspace_name = workspace_name;
    this.base_url = base_url;
  }

  async getHeaders(): Promise<Record<string, string>> {
    const err = await this.refreshIfNeeded();
    if (err) {
      throw err;
    }

    return {
      "X-Beamlit-Authorization": `Bearer ${this.credentials.access_token}`,
      "X-Beamlit-Workspace": this.workspace_name,
    };
  }

  async refreshIfNeeded(): Promise<null> {
    if (
      this.credentials.client_credentials &&
      !this.credentials.refresh_token
    ) {
      const body = { grant_type: "client_credentials" };

      try {
        const response = await fetch(`${this.base_url}/oauth/token`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Basic ${this.credentials.client_credentials}`,
          },
          body: JSON.stringify(body),
        });
        const data = (await response.json()) as DeviceLoginFinalizeResponse;
        this.credentials.access_token = data.access_token;
        this.credentials.refresh_token = data.refresh_token;
        this.credentials.expires_in = data.expires_in;
      } catch (e) {
        throw new Error(`Failed to get client credentials: ${e}`);
      }
    }

    // Need to refresh token if expires in less than 10 minutes
    const parts = this.credentials.access_token?.split(".") || [];
    if (parts.length !== 3) {
      throw new Error("Invalid JWT token format");
    }

    try {
      const claims = JSON.parse(
        Buffer.from(parts[1], "base64").toString("utf-8")
      );
      const expTime = claims.exp * 1000; // Convert to milliseconds
      // Refresh if token expires in less than 10 minutes
      if (Date.now() + 10 * 60 * 1000 > expTime) {
        return await this.doRefresh();
      }
    } catch (e) {
      throw new Error(`Failed to decode/parse JWT claims: ${e}`);
    }

    return null;
  }

  intercept(req: Request): void {
    const err = this.refreshIfNeeded();
    if (err) {
      throw err;
    }

    req.headers.set(
      "X-Beamlit-Authorization",
      `Bearer ${this.credentials.access_token}`
    );
    req.headers.set("X-Beamlit-Workspace", this.workspace_name);
  }

  private async doRefresh(): Promise<null> {
    if (!this.credentials.refresh_token) {
      throw new Error("No refresh token to refresh");
    }

    const url = `${this.base_url}/oauth/token`;
    const refresh_data = {
      grant_type: "refresh_token",
      refresh_token: this.credentials.refresh_token,
      device_code: this.credentials.device_code,
      client_id: "beamlit",
    };

    try {
      const response = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(refresh_data),
      });

      const finalize_response =
        (await response.json()) as DeviceLoginFinalizeResponse;

      if (!finalize_response.refresh_token) {
        finalize_response.refresh_token = this.credentials.refresh_token;
      }

      const creds: Credentials = {
        access_token: finalize_response.access_token,
        refresh_token: finalize_response.refresh_token,
        expires_in: finalize_response.expires_in,
        device_code: this.credentials.device_code,
        client_credentials: this.credentials.client_credentials,
      };

      this.credentials = creds;
      saveCredentials(this.workspace_name, creds);
      return null;
    } catch (e) {
      throw new Error(`Failed to refresh token: ${e}`);
    }
  }
}
