import os
from logging import getLogger
from typing import AsyncGenerator

from blaxel.models import bl_model
from blaxel.tools import bl_tools
from google.adk.agents import Agent
from google.adk.runners import Runner
from google.adk.sessions import InMemorySessionService
from google.genai import types

logger = getLogger(__name__)
session_service = InMemorySessionService()

# @title Define the get_weather Tool
def weather(city: str) -> str:
    """Get the weather in a given city

    Args:
        city (str): The name of the city (e.g., "New York", "London", "Tokyo").

    Returns:
        str: A string containing the weather information.
    """
    return f"The weather in {city} is sunny"

APP_NAME = "research_assistant"

async def agent(input: str, user_id: str, session_id: str) -> AsyncGenerator[str, None]:
    description = "You are a helpful assistant that can answer questions and help with tasks."
    prompt = """
You are a helpful assistant that can answer questions about weather, places and more generic questions about real time information.
"""
    tools = await bl_tools(["blaxel-search"], timeout_enabled=False).to_google_adk() + [weather]
    model = await bl_model("sandbox-openai").to_google_adk()

    agent = Agent(model=model, name=APP_NAME, description=description, instruction=prompt, tools=tools)

    # Create the specific session where the conversation will happen
    session = await session_service.get_session(app_name=APP_NAME, user_id=user_id, session_id=session_id)
    if not session:
        session = await session_service.create_session(
            app_name=APP_NAME,
            user_id=user_id,
            session_id=session_id
        )

    logger.info(f"Session created: App='{APP_NAME}', User='{user_id}', Session='{session_id}'")

    runner = Runner(
        agent=agent,
        app_name=APP_NAME,
        session_service=session_service,
    )
    logger.info(f"Runner created for agent '{runner.agent.name}'.")
    content = types.Content(role="user", parts=[types.Part(text=input)])
    async for event in runner.run_async(user_id=user_id, session_id=session_id, new_message=content):
      # Key Concept: is_final_response() marks the concluding message for the turn.
      if event.is_final_response():
        if event.content and event.content.parts:
            # Assuming text response in the first part
            yield event.content.parts[0].text
        elif event.actions and event.actions.escalate: # Handle potential errors/escalations
            yield f"Agent escalated: {event.error_message or 'No specific message.'}"
