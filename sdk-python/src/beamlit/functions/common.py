"""Decorators for creating function tools with Beamlit and LangChain integration."""
import ast
import asyncio
import importlib.util
import os
import traceback

from logging import getLogger
from typing import Union

from langchain_core.tools import StructuredTool
from langchain_core.tools.base import create_schema_from_function

from beamlit.authentication import new_client
from beamlit.client import AuthenticatedClient
from beamlit.common import slugify
from beamlit.common.settings import get_settings
from beamlit.functions.remote.remote import RemoteToolkit
from beamlit.models import AgentChain

logger = getLogger(__name__)

def get_functions(
    remote_functions:Union[list[str], None]=None,
    client:Union[AuthenticatedClient, None]=None,
    dir:Union[str, None]=None,
    chain:Union[list[AgentChain], None]=None,
    remote_functions_empty:bool=True,
    from_decorator:str="function",
    warning:bool=True,
):
    from beamlit.agents.chain import ChainToolkit

    settings = get_settings()
    if client is None:
        client = new_client()
    if dir is None:
        dir = settings.agent.functions_directory

    functions = []
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
                                        kit_functions = get_functions(
                                            client=client,
                                            dir=os.path.join(root),
                                            remote_functions_empty=remote_functions_empty,
                                            from_decorator="kit",
                                        )
                                        functions.extend(kit_functions)

                                    # Get the decorated function
                                    if not is_kit and hasattr(module, func_name):
                                        func = getattr(module, func_name)
                                        if settings.remote:
                                            toolkit = RemoteToolkit(client, slugify(func.__name__))
                                            toolkit.initialize()
                                            functions.extend(toolkit.get_tools())
                                        else:
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

    if remote_functions:
        for function in remote_functions:
            try:
                toolkit = RemoteToolkit(client, function)
                toolkit.initialize()
                functions.extend(toolkit.get_tools())
            except Exception as e:
                logger.debug(
                    f"Failed to initialize remote function {function}: {e!s}\n"
                    f"Traceback:\n{traceback.format_exc()}"
                )
                logger.warn(f"Failed to initialize remote function {function}: {e!s}")

    if chain:
        toolkit = ChainToolkit(client, chain)
        toolkit.initialize()
        functions.extend(toolkit.get_tools())

    return functions

