import { HumanMessage } from "@langchain/core/messages";
import { CompiledGraph, MemorySaver } from "@langchain/langgraph";
import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { ChatOpenAI } from "@langchain/openai";
import { FastifyRequest } from "fastify";
import { v4 as uuidv4 } from "uuid";
import { getChatModel, getDefaultThread, getFunctions, logger, wrapAgent } from "../src";
import "../src/common/instrumentation.js"; // Ensure instrumentation is initialized
import { createApp, runApp } from "../src/serve/app";
import { helloworld } from "./customfunctions/helloworld";

type InputType = {
  inputs: string | null;
  input: string | null;
};

type AgentType = {
  agent: CompiledGraph<any, any, any, any, any, any>;
};

const handleRequest = async (request: FastifyRequest, args: AgentType) => {
  const { agent } = args;
  const body = (await request.body) as InputType;
  const thread_id = getDefaultThread(request) || uuidv4();
  const input = body.inputs || body.input || "";
  const responses: any[] = [];

  logger.info(`Received request ${input}`);
  const stream = await agent.stream(
    { messages: [new HumanMessage(input)] },
    { configurable: { thread_id } }
  );

  for await (const chunk of stream) {
    responses.push(chunk);
  }
  const content = responses[responses.length - 1];
  return content.agent.messages[content.agent.messages.length - 1].content;
};

export const agent = async () => {
  const functions = await getFunctions({
    localFunctions: [{ name: "brave-search-2", description: "brave-search-2", url: "http://localhost:1400" }],
  });
  functions.push(helloworld);
  console.log(functions);

  const model = await getChatModel("gpt-4o-mini");
  return wrapAgent(handleRequest, {
    agent: {
      metadata: {
        name: "{{.ProjectName}}",
      },
      spec: {
        description: "{{.ProjectDescription}}",
      },
    },
    overrideAgent: createReactAgent({
      llm: model,
      tools: functions,
      checkpointSaver: new MemorySaver(),
    }),
  });
};

const main = async () => {
  process.env.BL_ENV = "dev";
  process.env.BL_SERVER_MODULE = "customagent.agent";
  process.env.BL_SERVER_PORT = "1338";
  const app = await createApp(agent);
  runApp(app);
};

main();
