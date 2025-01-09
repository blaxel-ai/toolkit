import { Credentials } from "./types";

export class ApiKeyAuth {
  private credentials: Credentials;
  private workspaceName: string;

  constructor(credentials: Credentials, workspaceName: string) {
    this.credentials = credentials;
    this.workspaceName = workspaceName;
  }

  async getHeaders(): Promise<Record<string, string>> {
    return {
      "X-Beamlit-Api-Key": this.credentials.apiKey || "",
      "X-Beamlit-Workspace": this.workspaceName,
    };
  }

  intercept(req: Request): void {
    req.headers.set("X-Beamlit-Api-Key", this.credentials.apiKey || "");
    req.headers.set("X-Beamlit-Workspace", this.workspaceName);
  }
}
