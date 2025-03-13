import { client } from "../client/index.js";
import { telemetryManager } from "../instrumentation/telemetryManager.js";

import settings from "./settings.js";

async function autoload() {
  client.setConfig({
    baseUrl: settings.baseUrl,
  })
  client.interceptors.request.use(async (request,options: any) => {
    if (options.authenticated === false) {
      return request;
    }
    await onLoad()
    for(const header in settings.headers) {
      request.headers.set(header, settings.headers[header])
    }
    return request;
  })
  await settings.authenticate();
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
