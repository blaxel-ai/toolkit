"""Decorators for creating function tools with Blaxel and LangChain integration."""
import asyncio
import functools
from collections.abc import Callable
from logging import getLogger

from blaxel.models import Function, FunctionKit
from fastapi import Request

logger = getLogger(__name__)

def kit(parent: str, kit: FunctionKit = None, **kwargs: dict) -> Callable:
    """
    Decorator to create function tools with Blaxel and LangChain integration.

    Args:
        kit (FunctionKit | None): Optional FunctionKit to associate with the function.
        **kwargs (dict): Additional keyword arguments for function configuration.

    Returns:
        Callable: The decorated function.
    """

    def wrapper(func: Callable) -> Callable:
        name = kit.name if kit and kit.name else func.__name__
        logger.info(f"Toolkit call {name} with arguments: {kwargs}")
        if kit and not func.__doc__ and kit.description:
            func.__doc__ = kit.description
        return func

    return wrapper


def function(*args, function: Function | dict = None, kit=False, **kwargs: dict) -> Callable:
    """
    Decorator to create function tools with Blaxel and LangChain integration.

    Args:
        function (Function | dict): Function metadata or a dictionary representing it.
        kit (bool): Whether to associate a function kit.
        **kwargs (dict): Additional keyword arguments for function configuration.

    Returns:
        Callable: The decorated function.
    """
    if function is not None and not isinstance(function, dict):
        raise Exception(
            'function must be a dictionary, example: @function(function={"metadata": {"name": "my_function"}})'
        )
    if isinstance(function, dict):
        function = Function(**function)

    def wrapper(func: Callable) -> Callable:
        if function and not func.__doc__ and function.spec and function.spec.description:
            func.__doc__ = function.spec.description

        @functools.wraps(func)
        async def wrapped(*args, **kwargs):
            try:
                name = function.metadata.name if function and function.metadata and function.metadata.name else func.__name__
                if kit is True:
                    return await func(*args, **kwargs)
                if len(args) > 0 and isinstance(args[0], Request):
                    body = await args[0].json()
                    args = [body.get(param) for param in func.__code__.co_varnames[:func.__code__.co_argcount]]
                logger.info(f"Tool call {name} with arguments: {args}")
                return await func(*args, **kwargs) if asyncio.iscoroutinefunction(func) else func(*args, **kwargs)
            except Exception as e:
                logger.error(f"Error calling function {func.__name__}: {e}")
                raise e
        return wrapped

    return wrapper
