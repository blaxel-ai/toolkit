import { ChatOpenAI } from "@langchain/openai";
import settings from "../common/settings";

export const getLangchainModel = (model: string) : any => {
  const url = `${settings.runUrl}/${settings.workspace}/models/${model}/v1`
  return new ChatOpenAI({
    apiKey: settings.token,
    model: 'gpt-4o-mini',
    temperature: 0,
    configuration: {
        baseURL: url,
    },
  });
}
