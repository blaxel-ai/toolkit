"""Module: decorator

Defines decorators for agent functionalities.
"""

# Import necessary modules
import asyncio
import functools
import inspect
from logging import getLogger
from typing import Any, Callable

from langgraph.checkpoint.memory import MemorySaver
from langgraph.prebuilt import create_react_agent

from blaxel.api.models import get_model, list_models
from blaxel.authentication import new_client
from blaxel.aimon.settings import Settings, init
from blaxel.errors import UnexpectedStatus
from blaxel.functions import get_functions
from blaxel.models import Agent, AgentSpec, Metadata

from .chat import get_chat_model_full
from .voice.openai import OpenAIVoiceReactAgent


async def initialize_agent(
    settings: Settings,
    agent: Agent | dict = None,
    override_model=None,
    override_agent=None,
    override_functions=None,
    remote_functions=None,
    local_functions=None,
):
    logger = getLogger(__name__)
    client = new_client()
    chat_model = override_model or None

    if asyncio.iscoroutinefunction(override_agent):
        override_agent = await override_agent()

    if agent is not None:
        metadata = Metadata(**agent.get("metadata", {}))
        spec = AgentSpec(**agent.get("spec", {}))
        agent = Agent(metadata=metadata, spec=spec)
        if agent.spec.model and chat_model is None:
            try:
                response = get_model.sync_detailed(
                    agent.spec.model, client=client
                )
                settings.agent.model = response.parsed
            except UnexpectedStatus as e:
                if e.status_code == 404:
                    if e.status_code == 404:
                        raise ValueError(f"Model {agent.spec.model} not found")
                raise e
            except Exception as e:
                raise e

            if settings.agent.model:
                chat_model, provider, model = get_chat_model_full(agent.spec.model, settings.agent.model)
                settings.agent.chat_model = chat_model
                logger.info(f"Chat model configured, using: {provider}:{model}")

    if override_functions is not None:
        functions = override_functions
    else:
        functions = await get_functions(
            client=client,
            dir=settings.agent.functions_directory,
            remote_functions=remote_functions,
            chain=agent.spec.agent_chain,
            local_functions=local_functions,
            remote_functions_empty=not remote_functions,
            warning=chat_model is not None,
        )
    settings.agent.functions = functions

    if override_agent is None:
        if chat_model is None:
            models_select = ""
            try:
                models = list_models.sync_detailed(
                    client=client
                )
                models = ", ".join([model.metadata.name for model in models.parsed])
                models_select = f"You can select one from your models: {models}"
            except Exception:
                pass

            raise ValueError(
                f"You must provide a model.\n"
                f"{models_select}\n"
                f"You can create one at {settings.app_url}/{settings.workspace}/global-inference-network/models/create\n"
                "Add it to your agent spec\n"
                "agent={\n"
                '    "metadata": {\n'
                f'        "name": "{agent.metadata.name}",\n'
                "    },\n"
                '    "spec": {\n'
                '        "model": "MODEL_NAME",\n'
                f'        "description": "{agent.spec.description}",\n'
                f'        "prompt": "{agent.spec.prompt}",\n'
                "    },\n"
                "}")
        if isinstance(chat_model, OpenAIVoiceReactAgent):
            _agent = chat_model
        else:
            memory = MemorySaver()
            if len(functions) == 0:
                raise ValueError("You can define this function in directory "
                    f'"{settings.agent.functions_directory}". Here is a sample function you can use:\n\n'
                    "from blaxel.functions import function\n\n"
                    "@function()\n"
                    "def hello_world(query: str):\n"
                    "    return 'Hello, world!'\n")
            try:
                _agent = create_react_agent(chat_model, functions, checkpointer=memory, state_modifier=agent.spec.prompt or "")
            except AttributeError: # special case for azure-marketplace where it uses the old OpenAI interface (no tools)
                logger.warning("Using the old OpenAI interface for Azure Marketplace, no tools available")
                _agent = create_react_agent(chat_model, [], checkpointer=memory, state_modifier=(agent and agent.spec and agent.spec.prompt) or "")

        settings.agent.agent = _agent
    else:
        settings.agent.agent = override_agent

def agent(
    agent: Agent | dict = None,
    override_model=None,
    override_agent: Any | Callable = None,
    override_functions=None,
    remote_functions=None,
    local_functions=None,
) -> Callable:
    """
    A decorator factory that configures and wraps functions to integrate with Blaxel agents.
    Handles model initialization, function retrieval, and agent setup.

    Parameters:
        agent (Agent | dict, optional): An `Agent` instance or a dictionary containing agent metadata and specifications.
        override_model (Any, optional): An optional model to override the default agent model.
        override_agent (Any, Callable, optional): An optional agent instance to override the default agent, or a function which returns an agent instance.
        mcp_hub (Any, optional): An optional MCP hub configuration.
        remote_functions (Any, optional): An optional list of remote functions to be integrated.
        local_functions (Any, optional): An optional list of local functions to be integrated.

    Returns:
        Callable: A decorator that wraps the target function, injecting agent-related configurations and dependencies.

    Behavior:
        - Validates and initializes the agent configuration.
        - Retrieves and sets up the appropriate chat model based on the agent's specifications.
        - Retrieves functions from the specified directories or remote sources.
        - Wraps the target function, injecting agent, model, and function dependencies as needed.
        - Logs relevant information and handles exceptions during the setup process.

    Raises:
        ValueError: If required configurations such as the model are missing.
        Re-raises exceptions encountered during model retrieval and agent setup.
    """
    logger = getLogger(__name__)
    settings = init()
    _is_initialized = False
    try:
        if agent is not None and not isinstance(agent, dict):
            raise Exception(
                'agent must be a dictionary, example: @agent(agent={"metadata": {"name": "my_agent"}})'
            )


        def wrapper(func):

            agent_kwargs = any(
                param.name == "agent"
                for param in inspect.signature(func).parameters.values()
            )
            model_kwargs = any(
                param.name == "model"
                for param in inspect.signature(func).parameters.values()
            )
            functions_kwargs = any(
                param.name == "functions"
                for param in inspect.signature(func).parameters.values()
            )

            @functools.wraps(func)
            async def wrapped(*args, **kwargs):
                nonlocal _is_initialized
                if not _is_initialized:
                    async with asyncio.Lock():
                        if not _is_initialized:
                            await initialize_agent(settings, agent, override_model, override_agent, override_functions, remote_functions, local_functions)
                            _is_initialized = True
                if agent_kwargs:
                    kwargs["agent"] = settings.agent.agent
                if model_kwargs:
                    kwargs["model"] = settings.agent.chat_model
                if functions_kwargs:
                    kwargs["functions"] = settings.agent.functions
                return await func(*args, **kwargs)

            return wrapped

        return wrapper
    except Exception as e:
        logger.error(f"Error in agent decorator: {e!s} at line {e.__traceback__.tb_lineno}")
        raise e
