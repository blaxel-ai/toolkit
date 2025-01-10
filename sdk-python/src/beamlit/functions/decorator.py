"""Decorators for creating function tools with Beamlit and LangChain integration."""

import json
from collections.abc import Callable
from logging import getLogger

from langchain_core.tools import create_schema_from_function

from beamlit.authentication import new_client
from beamlit.common.settings import get_settings
from beamlit.models import Function, FunctionKit
from beamlit.run import RunClient

logger = getLogger(__name__)


def get_remote_function(func: Callable, function: Function):
    settings = get_settings()
    name = (function and function.metadata and function.metadata.name) or func.__name__

    def _partial(*args, **kwargs):
        # Get function signature parameters
        try:
            client = new_client()
            run_client = RunClient(client)
            logger.debug(
                f"Calling remote function: NAME={name}"
                f" PARAMS={kwargs} ENVIRONMENT={settings.environment}"
            )
            response = run_client.run(
                resource_type="function",
                resource_name=name,
                environment=settings.environment,
                method="POST",
                headers={"Content-Type": "application/json"},
                data=json.dumps(kwargs),
            )
            content = response.text
            if response.status_code >= 400:
                content = f"{response.status_code}:{response.text}"
                logger.error(f"Error calling remote function: {content}")
                return f"Error calling remote function: {content}"
            logger.debug(
                f"Response from remote function: NAME={name}"
                f" RESPONSE={content} ENVIRONMENT={settings.environment}"
            )
            if response.headers.get("content-type") == "application/json":
                return response.json()
            return content
        except Exception as e:
            logger.error(f"Error calling function {name}: {e}")
            raise e

    remote_func = _partial
    remote_func.__name__ = func.__name__
    remote_func.__doc__ = func.__doc__
    return remote_func


def kit(bl_kit: FunctionKit = None, **kwargs: dict) -> Callable:
    """Create function tools with Beamlit and LangChain integration."""

    def wrapper(func: Callable) -> Callable:
        if bl_kit and not func.__doc__ and bl_kit.description:
            func.__doc__ = bl_kit.description
        return func

    return wrapper


def function(*args, function: Function | dict = None, kit=False, **kwargs: dict) -> Callable:
    """Create function tools with Beamlit and LangChain integration."""
    settings = get_settings()
    if function is not None and not isinstance(function, dict):
        raise Exception(
            'function must be a dictionary, example: @function(function={"metadata": {"name": "my_function"}})'
        )
    if isinstance(function, dict):
        function = Function(**function)
        
    def wrapper(func: Callable) -> Callable:
        if function and not func.__doc__ and function.spec and function.spec.description:
            func.__doc__ = function.spec.description
        if settings.remote:
            remote_func = get_remote_function(func, function)
            if not kwargs.get("args_schema"):
                kwargs["args_schema"] = create_schema_from_function(
                    func.__name__,
                    func,
                    parse_docstring=func.__doc__,
                )
            return remote_func
        return func

    return wrapper
