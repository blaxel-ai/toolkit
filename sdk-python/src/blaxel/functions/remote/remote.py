"""
This module provides functionalities to integrate remote functions into Blaxel.
It includes classes for creating dynamic schemas based on function schema and managing remote toolkits.
"""

import asyncio
import os
import warnings
from dataclasses import dataclass
from logging import getLogger
from typing import Callable

import pydantic
import typing_extensions as t
from blaxel.api.functions import get_function, list_functions
from blaxel.authentication.authentication import AuthenticatedClient
from blaxel.common.settings import get_settings
from blaxel.errors import UnexpectedStatus
from blaxel.functions.mcp.mcp import MCPClient, MCPToolkit
from blaxel.functions.utils import create_dynamic_schema
from blaxel.models.function import Function
from blaxel.run import RunClient
from langchain_core.tools.base import BaseTool, ToolException

logger = getLogger(__name__)

class RemoteTool(BaseTool):
    """
    Tool for interacting with remote functions.

    Attributes:
        client (RunClient): The client used to execute remote function calls.
        resource_name (str): The name of the remote resource.
        kit (bool): Indicates whether the tool is part of a function kit.
        handle_tool_error (bool | str | Callable[[ToolException], str] | None): Error handling strategy.
    """

    client: RunClient
    resource_name: str
    kit: bool = False
    handle_tool_error: bool | str | Callable[[ToolException], str] | None = True
    service_name: str | None = None
    cloud: bool = False
    @t.override
    def _run(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        warnings.warn(
            "Invoke this tool asynchronousely using `ainvoke`. This method exists only to satisfy standard tests.",
            stacklevel=1,
        )
        return asyncio.run(self._arun(*args, **kwargs))

    @t.override
    async def _arun(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        logger.info(f"Remote tool call {self.name} with arguments: {kwargs}")
        body = {"arguments": {**kwargs}}
        if self.kit:
            body["name"] = self.name
        result = self.client.run(
            "function",
            self.resource_name,
            "POST",
            cloud=self.cloud,
            service_name=self.service_name,
            json=body,
        )
        return result.text

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema

@dataclass
class RemoteToolkit:
    """
    Toolkit for managing remote function tools.

    Attributes:
        client (AuthenticatedClient): The authenticated client instance.
        function (str): The name of the remote function to integrate.
        _function (Function | None): Cached Function object after initialization.
    """
    client: AuthenticatedClient
    function: str
    _function: Function | None = None
    _service_name: str | None = None
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    async def initialize(self) -> None:
        """Initialize the session and retrieve the remote function details."""
        if self._function is None:
            try:
                response = get_function.sync_detailed(self.function, client=self.client)
                function_name = self.function.upper().replace("-", "_")
                if os.getenv(f"BL_FUNCTION_{function_name}_SERVICE_NAME"):
                    self._service_name = os.getenv(f"BL_FUNCTION_{function_name}_SERVICE_NAME")
                self._function = response.parsed
            except UnexpectedStatus as e:
                functions = list_functions.sync_detailed(
                    client=self.client,
                ).parsed
                names = [
                    f.metadata.name
                    for f in functions
                ]
                raise RuntimeError(
                    f"error: {e.status_code}. Available functions: {', '.join(names)}"
                )

    async def get_tools(self) -> list[BaseTool]:
        settings = get_settings()
        if self._function is None:
            raise RuntimeError("Must initialize the toolkit first")

        if self._function.spec.integration_connections:
            fallback_url = None
            url = f"{settings.run_url}/{settings.workspace}/functions/{self._function.metadata.name}"
            if self._service_name:
                fallback_url = f"https://{self._service_name}.{settings.run_internal_hostname}"
                url = f"https://{self._service_name}.{settings.run_internal_hostname}"
            mcp_client = MCPClient(self.client, url, fallback_url)
            mcp_toolkit = MCPToolkit(client=mcp_client, url=url)
            await mcp_toolkit.initialize()
            return await mcp_toolkit.get_tools()

        if self._function.spec.kit:
            return [
                RemoteTool(
                    client=RunClient(self.client),
                    name=func.name,
                    resource_name=self._function.metadata.name,
                    kit=True,
                    description=func.description or "",
                    args_schema=create_dynamic_schema(func.name, func.schema),
                    cloud=settings.cloud,
                    service_name=self._service_name,
                )
                for func in self._function.spec.kit
            ]

        return [
            RemoteTool(
                client=RunClient(self.client),
                name=self._function.metadata.name,
                resource_name=self._function.metadata.name,
                description=self._function.spec.description or "",
                args_schema=create_dynamic_schema(
                    self._function.metadata.name,
                    self._function.spec.schema
                ),
                cloud=settings.cloud,
                service_name=self._service_name,
            )
        ]