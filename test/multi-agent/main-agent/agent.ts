import { blModel, blTools } from "@blaxel/langgraph";
import { HumanMessage } from "@langchain/core/messages";
import { tool } from "@langchain/core/tools";
import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { z } from "zod";
interface Stream {
  write: (data: string) => void;
  end: () => void;
}

export default async function agent(
  input: string,
  stream: Stream
): Promise<void> {
  const llm = await blModel("sandbox-openai");
  const streamResponse = await createReactAgent({
    llm,
    prompt: "If the user ask for the weather, use the weather tool.",
    tools: [
      ...(await blTools(["blaxel-search"])),
      tool(
        async (input: any) => {
          console.debug("TOOLCALLING: local weather", input);
          return `The weather in ${input.city} is sunny`;
        },
        {
          name: "weather",
          description: "Get the weather in a specific city",
          schema: z.object({
            city: z.string(),
          }),
        }
      ),
    ],
  }).stream({
    messages: [new HumanMessage(input)],
  });

  for await (const chunk of streamResponse) {
    if (chunk.agent)
      for (const message of chunk.agent.messages) {
        stream.write(message.content);
      }
  }
  stream.end();
}
