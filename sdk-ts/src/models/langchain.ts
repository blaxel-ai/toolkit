import { ChatAnthropic } from "@langchain/anthropic";
import { ChatCohere } from "@langchain/cohere";
import { ChatDeepSeek } from "@langchain/deepseek";
import { ChatOpenAI } from "@langchain/openai";
import { CohereClient } from "cohere-ai";
import { getModel } from "../client";
import settings from "../common/settings";

export const getLangchainModel = async (model: string, options?: any) : Promise<any> => {
  const url = `${settings.runUrl}/${settings.workspace}/models/${model}`
  const {data:modelData} = await getModel({
    path: {
      modelName: model,
    },
  });
  const type = modelData?.spec?.runtime?.type || 'openai'
  switch(type) {
    case 'mistral':
      return new ChatOpenAI({
        apiKey: settings.token,
        model: modelData?.spec?.runtime?.model,
        configuration: {
          baseURL: `${url}/v1`,
        },
        ...options
      });
    case 'cohere':
      return new ChatCohere({
        apiKey: settings.token,
        model: modelData?.spec?.runtime?.model,
        client: new CohereClient({
          token: settings.token,
          environment: url,
        }),
      });
    case 'deepseek':
      return new ChatDeepSeek({
        apiKey: settings.token,
        model: modelData?.spec?.runtime?.model,
        configuration: {
          baseURL: `${url}/v1`,
        },
        ...options
      });
    case 'anthropic':
      return new ChatAnthropic({
        anthropicApiUrl: url,
        model: modelData?.spec?.runtime?.model,
        clientOptions: {
          defaultHeaders: settings.headers,
        },
        ...options
      });
    default:
      return new ChatOpenAI({
        apiKey: settings.token,
        model: modelData?.spec?.runtime?.model,
        configuration: {
          baseURL: `${url}/v1`,
        },
        ...options
      });
    }
}
