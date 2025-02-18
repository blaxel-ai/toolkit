from logging import getLogger

from starlette.websockets import WebSocket

from blaxel.agents import agent
from blaxel.common import init

settings = init()
logger = getLogger(__name__)


@agent(
    agent={
        "metadata": {
            "name": "voice-agent",
        },
        "spec": {
            "description": "A chat agent using Blaxel to handle your tasks.",
            "model": "gpt-4o-mini-realtime-preview",
        },
    },
    remote_functions=["brave-search"],
)
async def main(
    websocket: WebSocket, agent, functions,
):
    try:
        agent.bind_tools(functions)
        await agent.aconnect(websocket)
    except Exception as e:
        logger.error(f"Error connecting to agent: {str(e)}")
        await websocket.send_text(str(e))
        await websocket.close()
