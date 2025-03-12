import { client } from "../client";
import { telemetryManager } from "../instrumentation/telemetryManager";

import settings from "./settings";

async function delay(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function autoload() {
  client.setConfig({
    baseUrl: settings.baseUrl,
  })
  client.interceptors.request.use(async (request) => {
    console.log('beforeRequest',request)
    await delay(1000);
    return request;
  })
  await settings.authenticate();
  client.setConfig({
    headers: {
      Authorization: settings.authorization,
    },
  });
  telemetryManager.initialize(settings);
}

const autoloadPromise = autoload()

export const onLoad = function(): Promise<void> {
  return autoloadPromise;
}

autoloadPromise.catch((err) => {
  console.error(err);
  process.exit(1);
});
