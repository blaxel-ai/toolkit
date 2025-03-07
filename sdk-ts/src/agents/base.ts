import { Client } from "@hey-api/client-fetch";
import { BaseMessage, SystemMessage } from "@langchain/core/messages";
import { StructuredTool } from "@langchain/core/tools";
import {
  LangGraphRunnableConfig,
  MemorySaver,
  MessagesAnnotation,
} from "@langchain/langgraph";
import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { FastifyRequest } from "fastify";
import { newClient } from "../authentication/authentication.js";
import {
  getIntegrationConnection,
  getKnowledgebase,
  getModel,
  listModels,
} from "../client/sdk.gen.js";
import { Agent } from "../client/types.gen.js";
import { logger } from "../common/logger.js";
import { getSettings, Settings } from "../common/settings.js";
import { getFunctions, reloadFunctions } from "../functions/common.js";
import { KnowledgebaseFactory } from "../knowledgebases/factory.js";
import { KnowledgebaseClass } from "../knowledgebases/types.js";
import { getChatModelFull } from "./chat.js";
import { OpenAIVoiceReactAgent } from "./voice/openai.js";

/**
 * A variadic callback function type for agents.
 * @param args - The arguments passed to the function.
 * @returns The result of the callback function.
 */
export type CallbackFunctionAgentVariadic = (...args: any[]) => any;

// Documentation for FunctionRun
/**
 * Represents a function that runs with a Fastify request.
 * @param request - The incoming Fastify request.
 * @returns A promise resolving to any type.
 */
export type FunctionRun = (request: FastifyRequest) => Promise<any>;

/**
 * Represents a function that runs with a WebSocket and Fastify request, returning an async generator.
 * @param ws - The WebSocket instance.
 * @param request - The incoming Fastify request.
 * @returns A promise resolving to an async generator of any type.
 */
export type FunctionRunStream = (
  ws: WebSocket,
  request: FastifyRequest
) => Promise<AsyncGenerator<any>>;

/**
 * A type for wrapping agent functions.
 * @param func - The callback function to wrap.
 * @param options - Optional agent configuration options.
 * @returns A promise resolving to an AgentBase object.
 */
export type WrapAgentType = (
  func: CallbackFunctionAgentVariadic,
  options?: AgentOptions
) => Promise<AgentBase>;

/**
 * Represents the base structure of an agent.
 */
export type AgentBase = {
  run: FunctionRun | FunctionRunStream;
  agent: Agent | null;
  stream?: boolean;
  remoteFunctions?: string[];
};

/**
 * Configuration options for wrapping agents.
 */
export type AgentOptions = {
  agent?: Agent;
  overrideAgent?: any;
  overrideModel?: any;
  remoteFunctions?: string[];
};

/**
 * Handles the context management for an agent by retrieving relevant memories and constructing messages.
 * @param agent - The agent configuration object.
 * @param state - The current state of messages in the conversation.
 * @param knowledgebase - The memory store instance for retrieving historical context.
 * @param embeddingModel - The embedding model used for semantic similarity search.
 * @returns A promise resolving to an array of BaseMessage objects containing the context and current messages.
 */
const handleContext = async (
  agent: Agent | undefined,
  state: typeof MessagesAnnotation.State,
  config: LangGraphRunnableConfig,
  knowledgebase: KnowledgebaseClass
) => {
  const messages: BaseMessage[] = [];
  const prompt = agent?.spec?.prompt || "";
  try {
    const memories = await knowledgebase.search(
      state.messages[state.messages.length - 1].content as string
    );
    if (memories.length > 0) {
      let context = "Relevant information from previous conversations:\n";

      memories.forEach((memory: { value: string; similarity: number }) => {
        context += `- ${memory.value} (score: ${memory.similarity})\n`;
      });
      const message = new SystemMessage(prompt + context);
      messages.push(message);
    } else {
      messages.push(new SystemMessage(prompt));
    }
  } catch (error) {
    let context = "";
    if (error instanceof Error && "status" in error) {
      context = ` Could not retrieve memories from store: ${
        (error as any).status
      } - ${error.message}`;
    } else {
      context = ` Could not retrieve memories from store: ${error}`;
    }
    logger.warn(context);
    const message = new SystemMessage(prompt + context);
    messages.push(message);
  }
  messages.push(...state.messages);
  return messages;
};

const initKnowledgebase = async (
  agent: Agent | undefined,
  client: Client,
  settings: Settings
) => {
  if (!agent?.spec?.knowledgebase) {
    return null;
  }
  let knowledgebase = null;
  const { data: kb } = await getKnowledgebase({
    client,
    path: { knowledgebaseName: agent.spec.knowledgebase },
  });
  if (kb && kb.spec) {
    let config = {
      ...(kb.spec.options || {}),
    };
    let secrets = {};
    if (kb.spec.integrationConnections && kb.spec.integrationConnections[0]) {
      const { data: integrationConnection } = await getIntegrationConnection({
        client,
        path: { connectionName: kb.spec?.integrationConnections[0] },
      });
      if (integrationConnection?.spec) {
        if (integrationConnection?.spec?.config) {
          config = { ...config, ...integrationConnection?.spec?.config };
        }
        if (integrationConnection?.spec?.secret) {
          secrets = {
            ...secrets,
            ...integrationConnection?.spec?.secret,
            apiKey:
              "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3MiOiJtIiwiZXhwIjoxNzQ2NDY3MzI0fQ.75MdQ62X0X3gLxgaJ6Du19_qsMVsdz6srMjD42IRd90",
          };
        }
        knowledgebase = await KnowledgebaseFactory.create({
          type: integrationConnection.spec.integration || "qdrant",
          knowledgeBase: kb,
          connection: {
            config,
            secrets,
          },
        });
      }
    }
  } else {
    logger.warn(
      `Knowledgebase ${agent.spec.knowledgebase} not found. Please create one at ${settings.appUrl}/${settings.workspace}/global-inference-network/knowledgebases/create`
    );
  }
  return knowledgebase;
};

/**
 * Wraps a callback function into an AgentBase, configuring it based on the provided options and settings.
 * @param func - The callback function to wrap.
 * @param options - Optional agent configuration options.
 * @returns A promise resolving to an AgentBase object.
 */
export const wrapAgent: WrapAgentType = async (
  func: CallbackFunctionAgentVariadic,
  options: AgentOptions | null = null
): Promise<AgentBase> => {
  const settings = getSettings();
  let functions: StructuredTool[] = [];

  if (settings.deploy) {
    return {
      async run(request: FastifyRequest): Promise<any> {
        return await func(request);
      },
      agent: options?.agent ?? null,
      remoteFunctions: options?.remoteFunctions ?? [],
    };
  }

  const client = newClient();
  const { agent, overrideAgent, overrideModel, remoteFunctions } =
    options ?? {};

  if (overrideModel) {
    settings.agent.model = overrideModel;
  }
  if (overrideAgent) {
    settings.agent.agent = overrideAgent;
  }
  if (agent?.spec?.model) {
    const { response, data } = await getModel({
      client,
      path: { modelName: agent.spec.model },
    });
    if (response.status === 200) {
      settings.agent.model = data;
    }
  }
  if (!overrideAgent) {
    functions = await getFunctions({
      client,
      dir: settings.agent.functionsDirectory,
      remoteFunctions,
      chain: agent?.spec?.agentChain,
      warning: settings.agent.model !== null,
    });
  }
  settings.agent.functions = functions;
  if (!settings.agent.agent) {
    if (!settings.agent.model) {
      const { response, data: models } = await listModels({
        client,
        throwOnError: false,
      });
      if (models?.length) {
        let modelError = "";
        if (agent?.spec?.model) {
          modelError = `Model ${agent.spec.model} not found.\n`;
        }
        throw new Error(
          `${modelError}You must provide a model.\n${models?.join(
            ", "
          )}\nYou can create one at ${settings.appUrl}/${
            settings.workspace
          }/global-inference-network/models/create`
        );
      } else {
        throw new Error(
          `Cannot initialize agent. No models found. Response: ${response.status}`
        );
      }
    }

    const { chat } = await getChatModelFull(
      settings.agent.model.metadata.name,
      settings.agent.model
    );
    settings.agent.chatModel = chat;
    if (chat instanceof OpenAIVoiceReactAgent) {
      settings.agent.agent = chat;
    } else {
      const knowledgebase = await initKnowledgebase(agent, client, settings);

      settings.agent.agent = createReactAgent({
        llm: chat,
        tools: settings.agent.functions ?? [],
        checkpointSaver: new MemorySaver(),
        stateModifier: async (
          state: typeof MessagesAnnotation.State,
          config: LangGraphRunnableConfig
        ) => {
          if (knowledgebase) {
            return await handleContext(agent, state, config, knowledgebase);
          }
          const prompt = agent?.spec?.prompt || "";
          const messages = [new SystemMessage(prompt), ...state.messages];
          return messages;
        },
      });
    }
  }

  if (functions.length === 0 && !overrideAgent) {
    logger.warn(`
      You can define this function in directory ${settings.agent.functionsDirectory}. Here is a sample function you can use:\n\n
      import { wrapFunction } from '@blaxel/sdk/functions'\n\n
      wrapFunction(() => return 'Hello, world!', { name: 'hello_world', description: 'This is a sample function' })
      `);
  }
  if (settings.agent.agent instanceof OpenAIVoiceReactAgent) {
    return {
      run: async (ws: WebSocket, request: FastifyRequest) => {
        const args = {
          agent: settings.agent.agent,
          model: settings.agent.model,
          functions: settings.agent.functions,
        };
        return await func(ws, request, args);
      },
      agent: options?.agent ?? null,
      remoteFunctions: options?.remoteFunctions ?? [],
      stream: true,
    };
  }
  return {
    run: async (request: FastifyRequest) => {
      if (!overrideAgent) {
        functions = functions.concat(await reloadFunctions({
          client,
          dir: settings.agent.functionsDirectory,
          remoteFunctions,
          chain: agent?.spec?.agentChain,
          warning: settings.agent.model !== null,
        }));
      }
      const args = {
        agent: settings.agent.agent,
        model: settings.agent.model,
        functions: settings.agent.functions,
      };
      if (func.constructor.name === "AsyncFunction") {
        return await func(request, args);
      }
      return func(request, args);
    },
    remoteFunctions: options?.remoteFunctions ?? [],
    agent: options?.agent ?? null,
  };
};
