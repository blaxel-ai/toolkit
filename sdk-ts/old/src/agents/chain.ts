import { StructuredTool, tool } from "@langchain/core/tools";
import { z } from "zod";
import { listAgents } from "../client/sdk.gen.js";
import { Agent, AgentChain } from "../client/types.gen.js";
import { RunClient } from "../run.js";

/**
 * Creates a chain tool for managing agent chains.
 * @param client - The RunClient instance.
 * @param name - The name of the tool.
 * @param description - A description of the tool.
 * @param schema - The Zod schema for the tool's input.
 * @returns A StructuredTool instance.
 */
export function getChainTool(
  client: RunClient,
  name: string,
  description: string,
  schema: z.ZodType
) {
  return tool(
    async (args: Record<string, any>) => {
      const result = await client.run({
        resourceType: "agent",
        resourceName: name,
        method: "POST",
        json: args,
      });
      return result;
    },
    {
      name,
      description,
      schema,
    }
  );
}

/**
 * Schema for chain input.
 */
export const ChainInputSchema = z.object({
  inputs: z.string(),
});

/**
 * Remote toolkit for managing agent chains
 */
export class ChainToolkit {
  private client: RunClient;
  private chain: AgentChain[];
  private _chain: Agent[] | null = null;

  /**
   * Initializes the ChainToolkit with a client and a chain configuration.
   * @param client - The RunClient instance.
   * @param chain - An array of AgentChain configurations.
   */
  constructor(client: RunClient, chain: AgentChain[]) {
    this.client = client;
    this.chain = chain;
  }

  /**
   * Initializes the session and retrieves the list of tools.
   * @returns A promise that resolves when initialization is complete.
   */
  async initialize(): Promise<void> {
    if (!this._chain) {
      const agents = await listAgents({ client: this.client.client });
      const chainEnabled = this.chain.filter((chain) => chain.enabled);
      const agentsChain: Agent[] = [];
      if (!agents.data) {
        throw new Error("No agents found");
      }
      for (const chain of chainEnabled) {
        const agent = agents.data.find(
          (agent: Agent) => agent.metadata?.name === chain.name
        );
        if (agent && agent.spec) {
          agent.spec.prompt = chain.prompt || agent.spec.prompt;
          agent.spec.description = chain.description || agent.spec.description;
          agentsChain.push(agent);
        }
      }
      this._chain = agentsChain;
    }
  }

  /**
   * Retrieves the list of StructuredTools based on the initialized chain.
   * @returns An array of StructuredTool instances.
   */
  getTools(): StructuredTool[] {
    if (!this._chain) {
      throw new Error("Must initialize the toolkit first");
    }

    return this._chain
      .map((agent) =>
        agent.metadata?.name && agent.spec
          ? getChainTool(
              this.client,
              agent.metadata.name,
              agent.spec.description || agent.spec.prompt || "",
              ChainInputSchema
            )
          : null
      )
      .filter((tool) => tool !== null);
  }
}
