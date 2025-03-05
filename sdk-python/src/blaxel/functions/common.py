"""Decorators for creating function tools with Blaxel and LangChain integration.

This module provides functionality to discover and load function tools from Python files,
supporting both local and remote function execution.

Key Features:
- Automatic function discovery in specified directories
- Support for both synchronous and asynchronous functions
- Integration with LangChain's StructuredTool system
- Remote function toolkit handling
- Chain toolkit integration

Main Components:
- get_functions(): Core function that discovers and loads function tools
"""
import ast
import asyncio
import importlib.util
import os
import traceback
from logging import getLogger
from typing import Union

from blaxel.authentication import new_client
from blaxel.client import AuthenticatedClient
from blaxel.common import slugify
from blaxel.common.settings import get_settings
from blaxel.functions.local.local import LocalToolKit
from blaxel.functions.remote.remote import RemoteToolkit
from blaxel.models import AgentChain
from langchain_core.tools import StructuredTool
from langchain_core.tools.base import BaseTool, create_schema_from_function

logger = getLogger(__name__)

MAX_RETRIES = 1
RETRY_DELAY = 1  # 1 second delay between retries

async def initialize_with_retry(toolkit, function_name: str, max_retries: int) -> list[BaseTool]:
    for attempt in range(1, max_retries + 1):
        try:
            await toolkit.initialize()
            return await toolkit.get_tools()
        except Exception as e:
            if attempt == max_retries:
                logger.warn(f"Failed to initialize function {function_name} after {max_retries} attempts: {e!s}")
                raise
            logger.info(f"Attempt {attempt} failed for {function_name}, retrying...")
            await asyncio.sleep(RETRY_DELAY)
    return []

async def get_functions(
    remote_functions: Union[list[str], None] = None,
    local_functions: Union[list[dict], None] = None,
    client: Union[AuthenticatedClient, None] = None,
    dir: Union[str, None] = None,
    chain: Union[list[AgentChain], None] = None,
    remote_functions_empty: bool = True,
    local_functions_empty: bool = True,
    from_decorator: str = "function",
    warning: bool = True,
) -> list[BaseTool]:
    """Discovers and loads function tools from Python files and remote sources.

    This function walks through Python files in a specified directory, looking for
    decorated functions to convert into LangChain tools. It also handles remote
    functions and chain toolkits.

    Args:
        remote_functions (Union[list[str], None]): List of remote function names to load
        client (Union[AuthenticatedClient, None]): Authenticated client instance for API calls
        dir (Union[str, None]): Directory to search for Python files containing functions
        chain (Union[list[AgentChain], None]): List of agent chains to include
        remote_functions_empty (bool): Whether to allow empty remote functions
        from_decorator (str): Name of the decorator to look for (default: "function")
        warning (bool): Whether to show warning messages

    Returns:
        list: List of discovered and loaded function tools

    The function performs the following steps:
    1. Walks through Python files in the specified directory
    2. Parses each file to find decorated functions
    3. Converts found functions into LangChain StructuredTools
    4. Handles both synchronous and asynchronous functions
    5. Processes remote functions if specified
    6. Integrates chain toolkits if provided

    Example:
        ```python
        tools = get_functions(
            dir="./functions",
            from_decorator="function",
            warning=True
        )
        ```
    """
    from blaxel.agents.chain import ChainToolkit

    settings = get_settings()
    if client is None:
        client = new_client()
    if dir is None:
        dir = settings.agent.functions_directory

    functions: list[BaseTool] = []
    logger = getLogger(__name__)
    settings = get_settings()

    # Walk through all Python files in functions directory and subdirectories
    if not os.path.exists(dir):
        if remote_functions_empty and warning:
            logger.warn(f"Functions directory {dir} not found")
    if os.path.exists(dir):
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
                                    if is_kit and not settings.remote:
                                        kit_functions = await get_functions(
                                            client=client,
                                            dir=os.path.join(root),
                                            remote_functions_empty=remote_functions_empty,
                                            local_functions_empty=local_functions_empty,
                                            from_decorator="kit",
                                        )
                                        functions.extend(kit_functions)

                                    # Get the decorated function
                                    if hasattr(module, func_name):
                                        func = getattr(module, func_name)
                                        if settings.remote:
                                            toolkit = RemoteToolkit(client, slugify(func.__name__))
                                            tools = await initialize_with_retry(toolkit, func.__name__, MAX_RETRIES)
                                            functions.extend(tools)
                                        elif not is_kit:
                                            if asyncio.iscoroutinefunction(func):
                                                functions.append(
                                                    StructuredTool(
                                                        name=func.__name__,
                                                        description=func.__doc__,
                                                        func=func,
                                                        coroutine=func,
                                                        args_schema=create_schema_from_function(func.__name__, func)
                                                    )
                                                )
                                            else:
                                                functions.append(
                                                    StructuredTool(
                                                        name=func.__name__,
                                                        description=func.__doc__,
                                                        func=func,
                                                        args_schema=create_schema_from_function(func.__name__, func)
                                                    )
                                                )
                        except Exception as e:
                            logger.warning(f"Error processing {file_path}: {e!s}")
    if remote_functions and not settings.deploy:
        for function in remote_functions:
            try:
                toolkit = RemoteToolkit(client, function)
                tools = await initialize_with_retry(toolkit, function, MAX_RETRIES)
                functions.extend(tools)
            except Exception as e:
                if not isinstance(e, RuntimeError):
                    logger.debug(
                        f"Failed to initialize remote function {function}: {e!s}\n"
                        f"Traceback:\n{traceback.format_exc()}"
                    )
                    logger.warn(f"Failed to initialize remote function {function}: {e!s}")
    if local_functions and not settings.deploy:
        for function in local_functions:
            try:
                toolkit = LocalToolKit(client, function)
                tools = await initialize_with_retry(toolkit, function["name"], MAX_RETRIES)
                functions.extend(tools)
            except Exception as e:
                logger.debug(
                    f"Failed to initialize local function {function}: {e!s}\n"
                    f"Traceback:\n{traceback.format_exc()}"
                )
                logger.warn(f"Failed to initialize local function {function}: {e!s}")

    if chain:
        toolkit = ChainToolkit(client, chain)
        await toolkit.initialize()
        functions.extend(await toolkit.get_tools())
    logger.info(f"Functions available to your agent: {', '.join([f.name for f in functions])}")
    return functions

