

import { anthropic, AnthropicSession } from "@llamaindex/anthropic";
import { openai } from "@llamaindex/openai";
import { getModel } from "../client";
import settings from "../common/settings";

export const getLlamaIndexModel = async (model: string, options?: any) => {
  const url = `${settings.runUrl}/${settings.workspace}/models/${model}`
  const {data:modelData} = await getModel({
    path: {
      modelName: model,
    },
  });
  const type = modelData?.spec?.runtime?.type || 'openai'
  switch(type) {
    // case 'mistral':
    //   return mistral({
    //     model: modelData?.spec?.runtime?.model,
    //     apiKey: settings.token,
    //     baseURL: `${url}/v1`,
    //     ...options
    //   });
    // case 'cohere':
    //   throw new Error("Cohere is not supported in LlamaIndex integration with blaxel")
    // case 'deepseek':
    //   return openai({
    //     model: modelData?.spec?.runtime?.model,
    //     apiKey: settings.token,
    //     baseURL: `${url}/v1`,
    //     ...options
    //   });
    case 'anthropic':
      return anthropic({
        model: modelData?.spec?.runtime?.model,
        session: new AnthropicSession({
          baseURL: url,
          defaultHeaders: settings.headers,
        }),
        ...options
      });
    default:
      return openai({
        model: modelData?.spec?.runtime?.model,
        apiKey: settings.token,
        baseURL: `${url}/v1`,
        ...options
      });
  }

}