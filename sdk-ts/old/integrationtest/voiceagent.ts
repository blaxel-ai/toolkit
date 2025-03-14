import { FastifyRequest } from "fastify";
import { createApp, logger, runApp, wrapAgent } from "../src";

const websocketHandler = async (
  ws: WebSocket,
  request: FastifyRequest,
  args
) => {
  const { agent, functions } = args;
  logger.info("Websocket connected, request: ", request);

  agent.bindTools(functions);
  await agent.connect(ws, ws.send.bind(ws));
  ws.onclose = () => {
    logger.info("Websocket closed");
  };
};

export const agent = async () => {
  return wrapAgent(websocketHandler, {
    agent: {
      metadata: {
        name: "voiceagent",
      },
      spec: {
        description: "voiceagent",
        model: "gpt-4o-mini-realtime-preview",
      },
    },
    remoteFunctions: ["linear"],
  });
};

const main = async () => {
  process.env.BL_ENV = "dev";
  process.env.BL_SERVER_MODULE = "customagent.voiceagent";
  process.env.BL_SERVER_PORT = "1338";
  const app = await createApp(agent);
  runApp(app);
};

main();
