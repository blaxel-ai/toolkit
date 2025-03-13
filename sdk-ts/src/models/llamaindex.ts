/* eslint-disable @typescript-eslint/no-require-imports */

import settings from "../common/settings";

export const getLlamaIndexModel = async (model: string) => {
  const { openai } = require('llamaindex')
  const url = `${settings.runUrl}/${settings.workspace}/models/${model}/v1`
  return openai({
    model: 'gpt-4o-mini',
    apiKey: settings.token,
    baseURL: url,
  });
}