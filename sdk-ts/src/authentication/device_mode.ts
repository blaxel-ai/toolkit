import { saveCredentials } from "./credentials";
import { Credentials } from "./types";

interface DeviceLogin {
  client_id: string;
  scope: string;
}

interface DeviceLoginResponse {
  client_id: string;
  device_code: string;
  user_code: string;
  expires_in: number;
  interval: number;
  verification_uri: string;
  verification_uri_complete: string;
}

interface DeviceLoginFinalizeRequest {
  grant_type: string;
  client_id: string;
  device_code: string;
}

interface DeviceLoginFinalizeResponse {
  access_token: string;
  expires_in: number;
  refresh_token: string;
  token_type: string;
}

export class BearerToken {
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

  private async refreshIfNeeded(): Promise<null> {
    // Need to refresh token if expires in less than 10 minutes
    const parts = this.credentials.access_token?.split(".") || [];
    if (parts.length !== 3) {
      throw new Error("Invalid JWT token format");
    }

    try {
      const claims_bytes = Buffer.from(parts[1], "base64url");
      const claims = JSON.parse(claims_bytes.toString());

      // Refresh if token expires in less than 10 minutes
      if (Date.now() / 1000 + 10 * 60 > claims.exp) {
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
        apiKey: "",
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
