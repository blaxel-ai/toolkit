/* eslint-disable @typescript-eslint/no-require-imports */
/* eslint-disable no-case-declarations */
import { BaseChatModel } from "@langchain/core/language_models/chat_models";
import {
  getAuthenticationHeaders,
  newClient,
} from "../authentication/authentication.js";
import { getModel } from "../client/sdk.gen.js";
import { Model } from "../client/types.gen.js";
import { logger } from "../common/logger.js";
import { getSettings } from "../common/settings.js";
import { OpenAIVoiceReactAgent } from "./voice/openai.js";

/**
 * Retrieves the base URL for a given model name based on the current settings.
 * @param name - The name of the model.
 * @returns The base URL as a string.
 */
function getBaseUrl(name: string): string {
  const settings = getSettings();
  return `${settings.runUrl}/${settings.workspace}/models/${name}/v1`;
}

/**
 * Dynamically imports and returns the ChatOpenAI class from @langchain/openai.
 * @returns The ChatOpenAI class.
 * @throws If the import fails.
 */
async function getOpenAIChatModel() {
  try {
    const { ChatOpenAI } = require("@langchain/openai");
    return ChatOpenAI;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/openai. Please install it with: npm install @langchain/openai"
    );
    throw e;
  }
}

async function getDeepSeekChatModel() {
  try {
    const { ChatDeepSeek } = require("@langchain/deepseek");
    return ChatDeepSeek;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/deepseek. Please install it with: npm install @langchain/deepseek"
    );
    throw e;
  }
}

async function getMistralChatModel() {
  try {
    const { ChatMistralAI } = require("@langchain/mistralai");
    return ChatMistralAI;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/mistralai. Please install it with: npm install @langchain/mistralai"
    );
    throw e;
  }
}

async function getAnthropicChatModel() {
  try {
    const { ChatAnthropic } = require("@langchain/anthropic");
    return ChatAnthropic;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/anthropic. Please install it with: npm install @langchain/anthropic"
    );
    throw e;
  }
}

async function getXAIChatModel() {
  try {
    const { ChatXAI } = require("./providers/xai");

    return ChatXAI;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/openai. Please install it with: npm install @langchain/openai"
    );
    throw e;
  }
}

async function getCohereModel() {
  try {
    const { ChatCohere } = require("@langchain/cohere");
    return ChatCohere;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/openai. Please install it with: npm install @langchain/openai"
    );
    throw e;
  }
}

async function getAzurAIInferenceModel() {
  try {
    const { ChatOpenAI } = require("@langchain/openai");
    return ChatOpenAI;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/openai. Please install it with: npm install @langchain/openai"
    );
    throw e;
  }
}

async function getAzureMarketplaceModel() {
  try {
    const { OpenAI } = require("@langchain/openai");
    return OpenAI;
  } catch (e) {
    logger.warn(
      "Could not import @langchain/openai. Please install it with: npm install @langchain/openai"
    );
    throw e;
  }
}

async function getGeminiChatModel() {
  try {
    const {
      ChatGoogleGenerativeAI,
    } = require("./providers/google-genai/chat_models.js");
    return ChatGoogleGenerativeAI;
  } catch (e) {
    logger.warn(
      "Could not import @google/generative-ai. Please install it with: npm install @google/generative-ai"
    );
    throw e;
  }
}

/**
 * Retrieves the chat model and its details based on the provided name and agent model.
 * @param name - The name of the model.
 * @param agentModel - Optional Model object to override the default.
 * @returns An object containing the chat model, provider, and model name.
 */
export async function getChatModel(name: string, agentModel?: Model) {
  const { chat } = await getChatModelFull(name, agentModel);
  return chat;
}

/**
 * Retrieves the full chat model details, including the provider and model configuration.
 * @param name - The name of the model.
 * @param agentModel - Optional Model object to override the default.
 * @returns An object containing the chat model, provider, and model name.
 */
export async function getChatModelFull(
  name: string,
  agentModel?: Model
): Promise<{
  chat: BaseChatModel | OpenAIVoiceReactAgent;
  provider: string;
  model: string;
}> {
  const client = newClient();

  if (!agentModel) {
    try {
      const { data } = await getModel({
        client,
        path: {
          modelName: name,
        },
      });
      agentModel = data;
    } catch {
      logger.warn(`Model ${name} not found, defaulting to gpt-4o-mini`);
    }
  }

  const headers = await getAuthenticationHeaders();
  const jwt =
    headers["X-Blaxel-Authorization"]?.replace("Bearer ", "") ||
    headers["X-Blaxel-Api-Key"] ||
    "";
  let provider = agentModel?.spec?.runtime?.type;
  if (!provider) {
    logger.warn("Provider not found in agent model, defaulting to OpenAI");
    provider = "openai";
  }

  let model = agentModel?.spec?.runtime?.model;
  if (!model) {
    logger.warn("Model not found in agent model, defaulting to gpt-4o-mini");
    model = "gpt-4o-mini";
  }

  if (["openai"].includes(provider) && model.includes("realtime")) {
    logger.info("Starting OpenAI Realtime Agent");
    return {
      chat: new OpenAIVoiceReactAgent({
        url: getBaseUrl(name),
        model,
        headers,
      }),
      provider,
      model,
    };
  }

  const chatClasses = [
    "openai",
    "anthropic",
    "mistral",
    "xai",
    "cohere",
    "deepseek",
    "azure-ai-inference",
    "azure-marketplace",
    "gemini",
  ];
  if (!chatClasses.includes(provider)) {
    logger.warn(
      `Provider ${provider} not currently supported, defaulting to OpenAI`
    );
    provider = "openai";
  }

  let chat: BaseChatModel;

  switch (provider) {
    case "openai":
      // const chatClassOpenAI = await getOpenAIChatModel();
      const chatClassOpenAI = await getOpenAIChatModel();
      const chatOpenAI = new chatClassOpenAI({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        configuration: {
          baseURL: getBaseUrl(name),
          defaultHeaders: headers,
        },
      });
      return { chat: chatOpenAI, provider, model };
    case "anthropic":
      const chatClassAnthropic = await getAnthropicChatModel();
      const chatAnthropic = new chatClassAnthropic({
        apiKey: "fake_api_key",
        anthropicApiUrl: getBaseUrl(name).replace("/v1", ""),
        temperature: 0,
        model,
        clientOptions: {
          defaultHeaders: headers,
        },
      });
      chat = chatAnthropic;
      break;
    case "mistral":
      const chatClassMistral = await getMistralChatModel();
      const chatMistral = new chatClassMistral({
        apiKey: jwt,
        temperature: 0,
        model,
        serverURL: getBaseUrl(name).replace("/v1", ""),
      });
      chat = chatMistral;
      break;
    case "xai":
      const chatClassXAI = await getXAIChatModel();
      const chatXAI = new chatClassXAI({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        configuration: {
          baseURL: getBaseUrl(name),
          defaultHeaders: headers,
        },
      });
      chat = chatXAI;
      break;
    case "cohere":
      const chatClassCohere = await getCohereModel();
      try {
        const { CohereClient } = require("cohere-ai");
        const chatCohere = new chatClassCohere({
          apiKey: jwt,
          temperature: 0,
          model,
          client: new CohereClient({
            token: jwt,
            environment: getBaseUrl(name).replace("/v1", ""),
          }),
        });
        chat = chatCohere;
      } catch (e) {
        logger.warn(
          "Could not import cohere-ai. Please install it with: npm install cohere-ai"
        );
        throw e;
      }
      break;
    case "deepseek":
      const chatClassDeepSeek = await getDeepSeekChatModel();
      const chatDeepSeek = new chatClassDeepSeek({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        configuration: {
          baseURL: getBaseUrl(name),
          defaultHeaders: headers,
        },
      });
      chat = chatDeepSeek;
      break;
    case "azure-ai-inference":
      const chatClassAzureAIInference = await getAzurAIInferenceModel();
      const chatAzureAIInference = new chatClassAzureAIInference({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        configuration: {
          baseURL: getBaseUrl(name).replace("/v1", ""),
          defaultHeaders: headers,
        },
      });
      chat = chatAzureAIInference;
      break;
    case "azure-marketplace":
      const chatClassAzureMarketplace = await getAzureMarketplaceModel();
      const chatAzureMarketplace = new chatClassAzureMarketplace({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        configuration: {
          defaultHeaders: headers,
        },
      });
      chat = chatAzureMarketplace;
      break;
    case "gemini":
      const chatClassGemini = await getGeminiChatModel();
      const chatGemini = new chatClassGemini({
        apiKey: "fake_api_key",
        temperature: 0,
        model,
        baseUrl: getBaseUrl(name).replace("/v1", ""),
        customHeaders: headers,
      });
      chat = chatGemini;
      break;
    default:
      logger.warn(
        `Provider ${provider} not currently supported, defaulting to OpenAI`
      );
      const chatDefaultClass = await getOpenAIChatModel();
      const chatDefault = new chatDefaultClass(
        { apiKey: "fake_api_key", temperature: 0, model },
        {
          baseURL: getBaseUrl(name),
          defaultHeaders: headers,
        }
      );
      chat = chatDefault;
      break;
  }
  return { chat, provider, model };
}
