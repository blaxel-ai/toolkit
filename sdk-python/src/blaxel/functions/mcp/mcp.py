"""
This module provides functionalities to interact with MCP (Multi-Client Platform) servers.
It includes classes for managing MCP clients, creating dynamic schemas, and integrating MCP tools into Blaxel.
"""

import asyncio
import logging
import warnings
from typing import Any, AsyncIterator, Callable

import pydantic
import pydantic_core
import requests
import typing_extensions as t
from blaxel.authentication import get_authentication_headers
from blaxel.authentication.authentication import AuthenticatedClient
from blaxel.common.settings import get_settings
from blaxel.functions.mcp.client import websocket_client
from blaxel.functions.utils import create_dynamic_schema
from blaxel.models import FunctionSchema, FunctionSchemaProperties
from langchain_core.tools.base import BaseTool, BaseToolkit, ToolException
from mcp import ClientSession
from mcp.types import CallToolResult, ListToolsResult

settings = get_settings()

logger = logging.getLogger(__name__)


class MCPClient:
    def __init__(self, client: AuthenticatedClient, url: str, fallback_url: str | None = None):
        self.client = client
        self.url = url
        self.fallback_url = fallback_url

    async def list_ws_tools(self, is_fallback: bool = False) -> ListToolsResult:
        if is_fallback:
            url = self.fallback_url
        else:
            url = self.url
        try:
            async with websocket_client(url, headers=get_authentication_headers(settings)) as (read_stream, write_stream):
                logger.debug("WebSocket connection established")
                async with ClientSession(read_stream, write_stream) as client:
                    await client.initialize()
                    response = await client.list_tools()
                    logger.debug(f"WebSocket tools: {response}")
                    return response
        except Exception as e:
            logger.error(f"Error listing tools: {e}")
            logger.debug("WebSocket not available, trying HTTP")
            return None  # Signal to list_tools() to try HTTP instead

    async def list_tools(self) -> ListToolsResult:
        logger.debug(f"Listing tools for {self.url}")
        try:
            result = await self.list_ws_tools(is_fallback=False)
            return result
        except Exception as e: # Fallback to Public endpoint
            if self.fallback_url:
                try:
                    result = await self.list_ws_tools(is_fallback=True)
                    return result
                except Exception as e:
                    raise e
            else:
                raise e


    async def call_tool(
        self,
        tool_name: str,
        arguments: dict[str, Any] = None,
        is_fallback: bool = False,
    ) -> requests.Response | AsyncIterator[CallToolResult]:
        if is_fallback:
            url = self.fallback_url
        else:
            url = self.url
        try:
            logger.info(f"MCP tool call {tool_name} with arguments: {arguments}")
            async with websocket_client(url, headers=get_authentication_headers(settings)) as (read_stream, write_stream):
                async with ClientSession(read_stream, write_stream) as session:
                    await session.initialize()
                    response = await session.call_tool(tool_name, arguments or {})
                    content = pydantic_core.to_json(response).decode()
                    if response.isError:
                        raise ToolException(content)
                    return content
        except Exception as e:
            raise e


class MCPTool(BaseTool):
    """
    Tool for interacting with MCP server-hosted tools.

    Attributes:
        client (MCPClient): The MCP client instance.
        handle_tool_error (bool | str | Callable[[ToolException], str] | None): Error handling strategy.
    """

    client: MCPClient
    handle_tool_error: bool | str | Callable[[ToolException], str] | None = True

    @t.override
    def _run(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        warnings.warn(
            "Invoke this tool asynchronousely using `ainvoke`. This method exists only to satisfy standard tests.",
            stacklevel=1,
        )
        return asyncio.run(self._arun(*args, **kwargs))

    @t.override
    async def _arun(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        try:
            return await self.client.call_tool(self.name, arguments=kwargs)
        except Exception as e:
            if self.client.fallback_url:
                try:
                    return await self.client.call_tool(self.name, arguments=kwargs, is_fallback=True) # Fallback to Public endpoint
                except Exception as e:
                    raise e
            else:
                raise e

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema

class MCPToolkit(BaseToolkit):
    """
    Toolkit for managing MCP server tools.

    Attributes:
        client (MCPClient): The MCP client instance.
        _tools (ListToolsResult | None): Cached list of tools from the MCP server.
    """

    client: MCPClient
    """The MCP session used to obtain the tools"""

    _tools: ListToolsResult | None = None
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    async def initialize(self) -> None:
        """Initialize the session and retrieve tools list"""
        if self._tools is None:
            response = await self.client.list_tools()
            self._tools = response

    @t.override
    async def get_tools(self) -> list[BaseTool]:
        if self._tools is None:
            raise RuntimeError("Must initialize the toolkit first")

        return [
            MCPTool(
                client=self.client,
                name=tool.name,
                description=tool.description or "",
                args_schema=create_dynamic_schema(
                    tool.name,
                    FunctionSchema(
                        properties=FunctionSchemaProperties.from_dict(tool.inputSchema.get("properties", {})),
                        required=tool.inputSchema.get("required", []),
                    ),
                ),
            )
            # list_tools returns a PaginatedResult, but I don't see a way to pass the cursor to retrieve more tools
            for tool in self._tools.tools
        ]