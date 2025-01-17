"""Decorators for creating function tools with Beamlit and LangChain integration."""
import asyncio
import functools
from collections.abc import Callable
from logging import getLogger

from fastapi import Request

from beamlit.models import Function, FunctionKit

logger = getLogger(__name__)

def kit(bl_kit: FunctionKit = None, **kwargs: dict) -> Callable:
    """Create function tools with Beamlit and LangChain integration."""

    def wrapper(func: Callable) -> Callable:
        if bl_kit and not func.__doc__ and bl_kit.description:
            func.__doc__ = bl_kit.description
        return func

    return wrapper


def function(*args, function: Function | dict = None, kit=False, **kwargs: dict) -> Callable:
    """Create function tools with Beamlit and LangChain integration."""
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
            if len(args) > 0 and isinstance(args[0], Request):
                body = await args[0].json()
                args = [body.get(param) for param in func.__code__.co_varnames[:func.__code__.co_argcount]]
            return await func(*args, **kwargs) if asyncio.iscoroutinefunction(func) else func(*args, **kwargs)
        return wrapped

    return wrapper
