# Import necessary modules
import ast
import asyncio
import functools
import importlib
import os
from logging import getLogger

from langchain_core.tools import Tool
from langgraph.checkpoint.memory import MemorySaver
from langgraph.prebuilt import create_react_agent

from beamlit.api.models import get_model
from beamlit.authentication import new_client
from beamlit.common.settings import init
from beamlit.errors import UnexpectedStatus
from beamlit.functions.mcp.mcp import MCPClient, MCPToolkit
from beamlit.functions.remote.remote import RemoteToolkit
from beamlit.models import Agent, AgentMetadata, AgentSpec

from .chain import ChainToolkit
from .chat import get_chat_model


def get_functions(dir="src/functions", from_decorator="function", remote_functions_empty=True):
    functions = []
    logger = getLogger(__name__)

    # Walk through all Python files in functions directory and subdirectories
    if not os.path.exists(dir):
        if remote_functions_empty:
            logger.warn(f"Functions directory {dir} not found")
        return []
    for root, _, files in os.walk(dir):
        for file in files:
            if file.endswith(".py"):
                file_path = os.path.join(root, file)
                # Read and compile the file content
                with open(file_path) as f:
                    try:
                        file_content = f.read()
                        # Parse the file content to find decorated functions
                        tree = ast.parse(file_content)

                        # Look for function definitions with decorators
                        for node in ast.walk(tree):
                            if (
                                not isinstance(node, ast.FunctionDef)
                                and not isinstance(node, ast.AsyncFunctionDef)
                            ) or len(node.decorator_list) == 0:
                                continue
                            decorator = node.decorator_list[0]

                            decorator_name = ""
                            if isinstance(decorator, ast.Call):
                                decorator_name = decorator.func.id
                            if isinstance(decorator, ast.Name):
                                decorator_name = decorator.id

                            if decorator_name == from_decorator:
                                # Get the function name and decorator name
                                func_name = node.name

                                # Import the module to get the actual function
                                spec = importlib.util.spec_from_file_location(func_name, file_path)
                                module = importlib.util.module_from_spec(spec)
                                spec.loader.exec_module(module)
                                # Check if kit=True in the decorator arguments
                                is_kit = False
                                if isinstance(decorator, ast.Call):
                                    for keyword in decorator.keywords:
                                        if keyword.arg == "kit" and isinstance(
                                            keyword.value, ast.Constant
                                        ):
                                            is_kit = keyword.value.value
                                if is_kit:
                                    kit_functions = get_functions(
                                        dir=os.path.join(root),
                                        from_decorator="kit",
                                        remote_functions_empty=remote_functions_empty,
                                    )
                                    functions.extend(kit_functions)

                                # Get the decorated function
                                if not is_kit and hasattr(module, func_name):
                                    func = getattr(module, func_name)
                                    if asyncio.iscoroutinefunction(func):
                                        functions.append(
                                            Tool(
                                                name=func.__name__,
                                                description=func.__doc__,
                                                func=func,
                                                coroutine=func,
                                            )
                                        )
                                    else:
                                        functions.append(
                                            Tool(
                                                name=func.__name__,
                                                description=func.__doc__,
                                                func=func,
                                            )
                                        )
                    except Exception as e:
                        logger.warning(f"Error processing {file_path}: {e!s}")
    return functions


def agent(
    agent: Agent | dict = None,
    override_chat_model=None,
    override_agent=None,
    mcp_hub=None,
    remote_functions=None,
):
    logger = getLogger(__name__)
    try:
        if agent is not None and not isinstance(agent, dict):
            raise Exception(
                'agent must be a dictionary, example: @agent(agent={"metadata": {"name": "my_agent"}})'
            )

        client = new_client()
        chat_model = override_chat_model or None
        settings = init()

        def wrapper(func):
            @functools.wraps(func)
            def wrapped(*args, **kwargs):
                return func(
                    settings.agent.agent,
                    settings.agent.chat_model,
                    settings.agent.functions,
                    *args,
                    **kwargs,
                )

            return wrapped

        # Initialize functions array to store decorated functions
        functions = get_functions(
            dir=settings.agent.functions_directory,
            remote_functions_empty=not remote_functions,
        )
        settings.agent.functions = functions

        if agent is not None:
            metadata = AgentMetadata(**agent.get("metadata", {}))
            spec = AgentSpec(**agent.get("spec", {}))
            agent = Agent(metadata=metadata, spec=spec)
            if agent.spec.model and chat_model is None:
                try:
                    response = get_model.sync_detailed(
                        agent.spec.model, environment=settings.environment, client=client
                    )
                    settings.agent.model = response.parsed
                except UnexpectedStatus as e:
                    if e.status_code == 404 and settings.environment != "production":
                        try:
                            response = get_model.sync_detailed(
                                agent.spec.model, environment="production", client=client
                            )
                            settings.agent.model = response.parsed
                        except UnexpectedStatus as e:
                            if e.status_code == 404:
                                raise ValueError(f"Model {agent.spec.model} not found")
                    else:
                        raise e
                except Exception as e:
                    raise e

                if settings.agent.model:
                    chat_model, provider, model = get_chat_model(agent.spec.model, settings.agent.model)
                    settings.agent.chat_model = chat_model
                    logger.info(f"Chat model configured, using: {provider}:{model}")

        if mcp_hub:
            for server in mcp_hub:
                try:
                    mcp_client = MCPClient(client, server)
                    toolkit = MCPToolkit(client=mcp_client)
                    toolkit.initialize()
                    functions.extend(toolkit.get_tools())
                except Exception as e:
                    logger.warn(f"Failed to initialize MCP server {server}: {e!s}")

        if remote_functions:
            for function in remote_functions:
                try:
                    toolkit = RemoteToolkit(client, function)
                    toolkit.initialize()
                    functions.extend(toolkit.get_tools())
                except Exception as e:
                    logger.warn(f"Failed to initialize remote function {function}: {e!s}")

        if agent.spec.agent_chain:
            toolkit = ChainToolkit(client, agent.spec.agent_chain)
            toolkit.initialize()
            functions.extend(toolkit.get_tools())

        if override_agent is None and len(functions) == 0:
            raise ValueError(
                "You must define at least one function, you can define this function in directory "
                f'"{settings.agent.functions_directory}". Here is a sample function you can use:\n\n'
                "from beamlit.functions import function\n\n"
                "@function()\n"
                "def hello_world(query: str):\n"
                "    return 'Hello, world!'\n"
            )

        if override_agent is None and chat_model is not None:
            memory = MemorySaver()
            agent = create_react_agent(chat_model, functions, checkpointer=memory)
            settings.agent.agent = agent
        else:
            settings.agent.agent = override_agent
        return wrapper
    except Exception as e:
        logger.error(f"Error in agent decorator: {e!s} at line {e.__traceback__.tb_lineno}")
        raise e
