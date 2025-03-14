import { newClientWithCredentials } from "../src";
import { init } from "../src/common/settings";
import { RunClient } from "../src/run";

const main = async () => {
  const settings = init();
  const workspace = settings.workspace;
  const apiKey = settings.authentication.apiKey;
  if (!apiKey) {
    throw new Error("apiKey is not set");
  }
  const client = await newClientWithCredentials({
    credentials: {
      apiKey,
    },
    workspace,
  });
  const runClient = new RunClient(client);
  const data = await runClient.run({
    resourceType: "agents",
    resourceName: "agent-gpt-4o-mini",
    method: "POST",
    json: {
      input: "Hello world",
    },
  });
  console.log(data);
};

main();
