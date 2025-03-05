import { Client } from "@hey-api/client-fetch";
import { StructuredTool, tool } from "@langchain/core/tools";
import { Client as ModelContextProtocolClient } from "@modelcontextprotocol/sdk/client/index.js";
import { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import { z } from "zod";
import { getAuthenticationHeaders } from "../authentication/authentication.js";
import { getFunction, listFunctions } from "../client/sdk.gen.js";
import { Function } from "../client/types.gen.js";
import { getSettings, Settings } from "../common/settings.js";
import { RunClient } from "../run.js";
import { schemaToZodSchema } from "./common.js";
import { MCPClient, MCPToolkit } from "./mcp.js";
import { WebSocketClientTransport } from "./transport/websocket.js";

/**
 * Creates a StructuredTool for remote functions, enabling their invocation via the RunClient.
 *
 * @param {RunClient} client - The client instance used to execute the function.
 * @param {string} name - The name of the remote function.
 * @param {string} description - A description of what the function does.
 * @param {z.ZodType} schema - The Zod schema for the function's input parameters.
 * @returns {StructuredTool} The structured tool representing the remote function.
 */
export function getRemoteTool(
  client: RunClient,
  name: string,
  description: string,
  schema: z.ZodType
) {
  return tool(
    async (args: Record<string, any>) => {
      const data = await client.run({
        resourceType: "function",
        resourceName: name,
        method: "POST",
        json: args,
      });
      return data;
    },
    {
      name,
      description,
      schema,
    }
  );
}

/**
 * Toolkit for managing and interacting with remote toolkits and MCP services.
 */
export class RemoteToolkit {
  private client: Client;
  private modelContextProtocolClient: ModelContextProtocolClient;
  private _mcpToolkit: MCPToolkit | null = null;
  private fallbackUrl: string | null = null;
  private functionName: string;
  private _function: Function | null = null;
  private runClient: RunClient;
  private settings: Settings;

  /**
   * Creates an instance of RemoteToolkit.
   *
   * @param {Client} client - The HTTP client instance.
   * @param {string} functionName - The name of the remote function to manage.
   */
  constructor(client: Client, functionName: string) {
    this.settings = getSettings();
    this.client = client;
    this.functionName = functionName;
    this.runClient = new RunClient(client);
    this.modelContextProtocolClient = new ModelContextProtocolClient(
      {
        name: this.settings.name,
        version: "1.0.0",
      },
      {
        capabilities: {
          tools: {},
        },
      }
    );
  }

  /**
   * Initializes the toolkit by retrieving the specified function and its associated tools.
   *
   * @returns {Promise<void>} Resolves when initialization is complete.
   * @throws Will throw an error if the function retrieval fails.
   */
  async initialize(): Promise<void> {
    if (!this._function) {
      const { response, data } = await getFunction({
        client: this.client,
        path: { functionName: this.functionName },
      });
      if (response.status >= 400) {
        const { data: listData } = await listFunctions({
          client: this.client,
        });
        const names =
          listData?.map((f: Function) => f.metadata?.name || "") || [];
        throw new Error(
          `error ${response.status}. Available functions: ${names.join(", ")}`
        );
      }
      this._function = data || null;
    }

    if (
      this._function &&
      this._function.metadata &&
      this._function.spec?.integrationConnections
    ) {
      let url = `${this.settings.runUrl}/${this.settings.workspace}/functions/${this._function.metadata.name}`;
      let transport: Transport;

      const headers = await getAuthenticationHeaders();
      const envVar = toEnvVar(this._function.metadata.name || "");
      if (process.env[`BL_FUNCTION_${envVar}_SERVICE_NAME`]) {
        this.fallbackUrl = url;
        url = `https://${process.env[`BL_FUNCTION_${envVar}_SERVICE_NAME`]}.${
          this.settings.runInternalHostname
        }`;
        transport = new WebSocketClientTransport(new URL(url), headers);
      } else {
        transport = new WebSocketClientTransport(new URL(url), headers);
      }
      try {
        await this.modelContextProtocolClient.connect(transport);
        const mcpClient = new MCPClient(
          this.modelContextProtocolClient,
          transport
        );
        const mcpToolkit = new MCPToolkit(mcpClient);
        this._mcpToolkit = mcpToolkit;
        await mcpToolkit.initialize();
      } catch (error) {
        if (this.fallbackUrl) {
          transport = new WebSocketClientTransport(new URL(this.fallbackUrl), {
            "x-blaxel-authorization": headers?.["X-Blaxel-Authorization"] || "",
            "x-blaxel-workspace": headers?.["X-Blaxel-Workspace"] || "",
          });
          await this.modelContextProtocolClient.connect(transport);
          const mcpClient = new MCPClient(
            this.modelContextProtocolClient,
            transport
          );
          const mcpToolkit = new MCPToolkit(mcpClient);
          this._mcpToolkit = mcpToolkit;
          try {
            await mcpToolkit.initialize();
          } catch (error) {
            throw new Error(
              `Failed to initialize MCP toolkit, error: ${error}`
            );
          }
        } else {
          throw new Error(`Failed to initialize MCP toolkit, error: ${error}`);
        }
      }
    }
  }

  /**
   * Retrieves the list of structured tools from the remote function. If the function has integration connections,
   * it utilizes the MCPToolkit to manage them.
   *
   * @returns {Promise<StructuredTool[]>} An array of structured tools.
   * @throws Will throw an error if the toolkit has not been initialized.
   */
  async getTools(): Promise<StructuredTool[]> {
    if (!this._function) {
      throw new Error("Must initialize the toolkit first");
    }
    if (this._mcpToolkit) {
      return this._mcpToolkit.getTools();
    }
    return [
      getRemoteTool(
        this.runClient,
        this._function.metadata?.name || "",
        this._function.spec?.description || "",
        schemaToZodSchema(this._function.spec?.schema || {})
      ),
    ];
  }
}

function toEnvVar(name: string) {
  return name.replace(/-/g, "_").toUpperCase();
}
