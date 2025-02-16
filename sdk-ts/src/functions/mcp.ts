import { StructuredTool, tool } from "@langchain/core/tools";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import { ListToolsResult } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";

/**
 * Represents a property expected by MCP tools.
 *
 * @typedef {Object} MCPProperty
 * @property {string} type - The type of the property.
 * @property {boolean} [required] - Whether the property is required.
 * @property {string} [description] - A description of the property.
 * @property {*} [default] - The default value of the property.
 */
type MCPProperty = {
  type: string;
  required?: boolean;
  description?: string;
  default?: any;
};

/**
 * Creates a StructuredTool for MCP tools based on their specifications.
 *
 * @param {MCPClient} client - The MCP client instance.
 * @param {string} name - The name of the tool.
 * @param {string} description - A description of the tool.
 * @param {z.ZodType} schema - The Zod schema for the tool's input.
 * @returns {StructuredTool} The structured tool instance.
 */
export function getMCPTool(
  client: MCPClient,
  name: string,
  description: string,
  schema: z.ZodType
) {
  return tool(
    async (args: any) => {
      const result = await client.callTool(name, args);
      if (result.isError) {
        throw new Error(JSON.stringify(result.content));
      }
      return JSON.stringify(result.content);
    },
    {
      name,
      description,
      schema,
    }
  );
}

/**
 * Client for interacting with MCP (Model Context Protocol) services.
 */
export class MCPClient {
  private client: Client;
  private transport: Transport;
  /**
   * Creates an instance of MCPClient.
   *
   * @param {Client} client - The Model Context Protocol client instance.
   */
  constructor(client: Client, transport: Transport) {
    this.client = client;
    this.transport = transport;
  }

  /**
   * Retrieves a list of available tools from the MCP service.
   *
   * @returns {Promise<ListToolsResult>} The result containing the list of tools.
   * @throws Will throw an error if the request fails.
   */
  async listTools(): Promise<ListToolsResult> {
    if (this.client.transport === undefined) {
      try {
        await this.client.connect(this.transport);
      } catch (error) {
        throw new Error(`Failed to connect to MCP: ${error}`);
      }
    }
    return this.client.listTools();
  }

  /**
   * Calls a specific tool with provided arguments.
   *
   * @param {string} toolName - The name of the tool to invoke.
   * @param {any} args - Arguments to pass to the tool.
   * @returns {Promise<any>} The result from the tool invocation.
   * @throws Will throw an error if the call fails or if the tool returns an error.
   */
  async callTool(toolName: string, args: any): Promise<any> {
    if (this.client.transport === undefined) {
      try {
        await this.client.connect(this.transport);
      } catch (error) {
        throw new Error(`Failed to connect to MCP: ${error}`);
      }
    }
    return this.client.callTool({
      name: toolName,
      arguments: args,
    });
  }
}

/**
 * Toolkit for managing and interacting with MCP tools.
 */
export class MCPToolkit {
  private client: MCPClient;
  private _tools: ListToolsResult | null = null;

  /**
   * Creates an instance of MCPToolkit.
   *
   * @param {MCPClient} client - The MCP client instance.
   */
  constructor(client: MCPClient) {
    this.client = client;
  }

  /**
   * Initializes the toolkit by retrieving the list of available tools.
   *
   * @returns {Promise<void>} Resolves when initialization is complete.
   */
  async initialize(): Promise<void> {
    if (!this._tools) {
      this._tools = await this.client.listTools();
    }
  }

  /**
   * Retrieves the list of structured tools managed by the toolkit.
   *
   * @returns {Promise<StructuredTool[]>} An array of structured tools.
   * @throws Will throw an error if the toolkit has not been initialized.
   */
  async getTools(): Promise<StructuredTool[]> {
    if (!this._tools) {
      throw new Error("Must initialize the toolkit first");
    }

    return this._tools.tools.map((tool) => {
      const shape: { [key: string]: z.ZodType } = {};
      if (tool.inputSchema?.properties) {
        if (tool.inputSchema.type === "object") {
          for (const key in tool.inputSchema.properties) {
            const property = tool.inputSchema.properties[key] as MCPProperty;
            let zodType: z.ZodType;
            switch (property.type) {
              case "boolean":
                zodType = z.boolean();
                break;
              case "number":
                zodType = z.number();
                break;
              default:
                zodType = z.string();
            }
            if (property.description) {
              zodType = zodType.describe(property.description);
            }
            if (property.default) {
              zodType = zodType.default(property.default);
            }
            shape[key] = property.required ? zodType : zodType.optional();
          }
        }
      }
      return getMCPTool(
        this.client,
        tool.name,
        tool.description || "",
        z.object(shape)
      );
    });
  }
}
