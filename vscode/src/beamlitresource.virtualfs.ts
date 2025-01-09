import {
  getAgent,
  getEnvironment,
  getFunction,
  getIntegrationConnection,
  getModel,
  getPolicy,
} from "@beamlit/sdk";
import { newClient } from "@beamlit/sdk/src/authentication/authentication";
import * as yaml from "js-yaml";
import * as querystring from "querystring";
import {
  Disposable,
  Event,
  EventEmitter,
  FileChangeEvent,
  FileStat,
  FileSystemProvider,
  FileType,
  Uri,
  window,
} from "vscode";

export class BeamlitResourceVirtualFileSystemProvider
  implements FileSystemProvider
{
  constructor() {}

  private readonly onDidChangeFileEmitter: EventEmitter<FileChangeEvent[]> =
    new EventEmitter<FileChangeEvent[]>();

  onDidChangeFile: Event<FileChangeEvent[]> = this.onDidChangeFileEmitter.event;

  watch(
    _uri: Uri,
    _options: { recursive: boolean; excludes: string[] }
  ): Disposable {
    // It would be quite neat to implement this to watch for changes
    // in the cluster and update the doc accordingly.  But that is very
    // definitely a future enhancement thing!
    return new Disposable(() => {});
  }

  stat(_uri: Uri): FileStat {
    return {
      type: FileType.File,
      ctime: 0,
      mtime: 0,
      size: 65536, // These files don't seem to matter for us
    };
  }

  readDirectory(
    _uri: Uri
  ): [string, FileType][] | Thenable<[string, FileType][]> {
    return [];
  }

  createDirectory(_uri: Uri): void | Thenable<void> {
    // no-op
  }

  readFile(uri: Uri): Uint8Array | Thenable<Uint8Array> {
    return this.readFileAsync(uri);
  }

  async readFileAsync(uri: Uri): Promise<Uint8Array> {
    const content = await this.loadResource(uri);
    return Buffer.from(content, "utf8");
  }

  async loadResource(uri: Uri): Promise<string> {
    const query = querystring.parse(uri.query);
    const client = newClient();
    const resourceType = (query.resourceType as string) || "agents";
    const resourceId = (query.resourceId as string) || "";
    const environment = (query.environment as string) || "";
    const func: Record<string, CallableFunction> = {
      agents: getAgent,
      functions: getFunction,
      models: getModel,
      environments: getEnvironment,
      policies: getPolicy,
      integrations: getIntegrationConnection,
    };
    const kind: Record<string, string> = {
      agents: "Agent",
      functions: "Function",
      models: "Model",
      environments: "Environment",
      policies: "Policy",
      integrations: "Integration",
    };
    const status = window.setStatusBarMessage(
      `Loading ${kind[resourceType]} ${resourceId}...`
    );
    try {
      const query: Record<string, string> = {};
      if (environment) {
        query.environment = environment;
      }
      const resource = await func[resourceType]({
        client,
        path: {
          functionName: resourceId,
          agentName: resourceId,
          modelName: resourceId,
          environmentName: resourceId,
          policyName: resourceId,
          connectionName: resourceId,
        },
        query,
        throwOnError: true,
      });
      status.dispose();
      const yamlContent = yaml.dump({
        apiVersion: "beamlit.com/v1",
        kind: kind[resourceType],
        ...resource.data,
      });
      return yamlContent;
    } catch (err) {
      throw new Error(
        `Failed to load resource ${resourceId} because ${(err as Error).stack}`
      );
    }
  }

  writeFile(
    uri: Uri,
    content: Uint8Array,
    _options: { create: boolean; overwrite: boolean }
  ): void | Thenable<void> {
    // no-op
  }

  delete(_uri: Uri, _options: { recursive: boolean }): void | Thenable<void> {
    // no-op
  }

  rename(
    _oldUri: Uri,
    _newUri: Uri,
    _options: { overwrite: boolean }
  ): void | Thenable<void> {
    // no-op
  }
}
