import asyncio
import uuid

from blaxel.agents import agent
from blaxel.aimon import init

settings = init()

@agent(
    agent={
        "metadata": {
            "name": "agent-custom",
        },
        "spec": {
            "description": "A chat agent using Blaxel to handle your tasks.",
            "model": "sandbox-openai",
        },
    },
)
async def main(
    input, agent, functions
):
    agent_config = {"configurable": {"thread_id": str(uuid.uuid4())}}

    body = await input.json()

    agent_body = {"messages": [("user", body["inputs"])]}
    responses = []

    async for chunk in agent.astream(agent_body, config=agent_config):
        # if "agent" in chunk and "messages" in chunk["agent"]:
        #     print(chunk["agent"]["messages"][-1].content)
        print(chunk)
        responses.append(chunk)
    content = responses[-1]
    return content["agent"]["messages"][-1].content


if __name__ == "__main__":
    async def check():
        input = "Give me the content of the README.md file for blaxel/toolkit repository"
        response = await main(input)
        print(response)

    asyncio.run(check())
