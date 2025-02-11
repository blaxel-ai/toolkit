import asyncio
import uuid

from beamlit.agents import agent
from beamlit.common import init

settings = init()

@agent(
    agent={
        "metadata": {
            "name": "agent-custom",
            "environment": "production",
        },
        "spec": {
            "description": "A chat agent using Beamlit to handle your tasks.",
            "model": "gpt-4o-mini",
        },
    },
    remote_functions=["brave-search"],
    # local_functions=[
    #     {
    #         "name": "github-search",
    #         "description": "A tool that searches GitHub for a given query",
    #         "url": "http://0.0.0.0:8000",
    #         "sse": True,
    #     }
    # ],
)
async def main(
    input, agent, functions
):
    agent_config = {"configurable": {"thread_id": str(uuid.uuid4())}}
    print(functions)

    body = await input.json()

    agent_body = {"messages": [("user", body["inputs"])]}
    responses = []

    async for chunk in agent.astream(agent_body, config=agent_config):
        if "agent" in chunk and "messages" in chunk["agent"]:
            print(chunk["agent"]["messages"][-1].content)
        responses.append(chunk)
    content = responses[-1]
    return content["agent"]["messages"][-1].content


if __name__ == "__main__":
    async def check():
        input = "Give me the content of the README.md file for beamlit/toolkit repository"
        response = await main(input)
        print(response)

    asyncio.run(check())
