import asyncio
import uuid

from beamlit.agents import agent


@agent(
    agent={
        "metadata": {
            "name": "agent-custom",
            "environment": "production",
        },
        "spec": {
            "description": "A chat agent using Beamlit to handle your tasks.",
            "model": "gpt-4o-mini",
            "policies": ["only-us"],
        },
    },
)
async def main(agent, chat_model, tools, body, headers=None, query_params=None, **_):
    agent_config = {"configurable": {"thread_id": str(uuid.uuid4())}}
    if body.get("inputs"):
        body["input"] = body["inputs"]

    agent_body = {"messages": [("user", body["input"])]}
    responses = []

    async for chunk in agent.astream(agent_body, config=agent_config):
        responses.append(chunk)
    content = responses[-1]
    return content["agent"]["messages"][-1].content


if __name__ == "__main__":

    async def check():
        response = await main(
            {"input": "What does 4+2 ?"},
            headers={"X-Beamlit-Sub": "123"},
            query_params={"debug": "false"},
        )
        print(response)

    asyncio.run(check())
