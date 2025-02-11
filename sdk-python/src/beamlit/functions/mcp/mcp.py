"""
This module provides functionalities to interact with MCP (Multi-Client Platform) servers.
It includes classes for managing MCP clients, creating dynamic schemas, and integrating MCP tools into Beamlit.
"""

import asyncio
import logging
import warnings
from typing import Any, AsyncIterator, Callable

import pydantic
import pydantic_core
import requests
import typing_extensions as t
from langchain_core.tools.base import BaseTool, BaseToolkit, ToolException
from mcp import ClientSession
from mcp.client.sse import sse_client
from mcp.types import CallToolResult, ListToolsResult

from beamlit.authentication.authentication import AuthenticatedClient
from beamlit.common.settings import get_settings

from .utils import create_schema_model

settings = get_settings()

logger = logging.getLogger(__name__)


class MCPClient:
    def __init__(self, client: AuthenticatedClient, url: str):
        self.client = client
        self.url = url
        self._sse = False

    async def list_sse_tools(self) -> ListToolsResult:
        # Create a new context for each SSE connection
        try:
            async with sse_client(f"{self.url}/sse") as (read_stream, write_stream):
                async with ClientSession(read_stream, write_stream) as session:
                    await session.initialize()
                    response = await session.list_tools()
                    return response
        except Exception:
            self._sse = False
            logger.info("SSE not available, trying HTTP")
            return None  # Signal to list_tools() to try HTTP instead

    def list_tools(self) -> ListToolsResult:
        try:
            loop = asyncio.get_event_loop()
            result = loop.run_until_complete(self.list_sse_tools())
            if result is None:  # SSE failed, try HTTP
                raise Exception("SSE failed")
            self._sse = True
            return result
        except Exception:  # Fallback to HTTP
            client = self.client.get_httpx_client()
            response = client.request("GET", f"{self.url}/tools/list")
            response.raise_for_status()
            return ListToolsResult(**response.json())

    async def call_tool(
        self,
        tool_name: str,
        arguments: dict[str, Any] = None,
    ) -> requests.Response | AsyncIterator[CallToolResult]:
        if self._sse:
            async with sse_client(f"{self.url}/sse") as (read_stream, write_stream):
                async with ClientSession(read_stream, write_stream) as session:
                    await session.initialize()
                    response = await session.call_tool(tool_name, arguments or {})
                    content = pydantic_core.to_json(response).decode()
                    return content
        else: # Fallback to HTTP
            client = self.client.get_httpx_client()
            response = client.request(
                "POST",
                f"{self.url}/tools/call",
                json={"name": tool_name, "arguments": arguments},
            )
            response.raise_for_status()
            result = CallToolResult(response.json())
            if result.isError:
                raise ToolException(result.content)
            content = pydantic_core.to_json(result.content).decode()
            return content

class MCPTool(BaseTool):
    """
    Tool for interacting with MCP server-hosted tools.

    Attributes:
        client (MCPClient): The MCP client instance.
        handle_tool_error (bool | str | Callable[[ToolException], str] | None): Error handling strategy.
        sse (bool): Whether to use SSE streaming for responses.
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
        return await self.client.call_tool(self.name, arguments=kwargs)

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

    def initialize(self) -> None:
        """Initialize the session and retrieve tools list"""
        if self._tools is None:
            response = self.client.list_tools()
            self._tools = response

    @t.override
    def get_tools(self) -> list[BaseTool]:
        if self._tools is None:
            raise RuntimeError("Must initialize the toolkit first")

        return [
            MCPTool(
                client=self.client,
                name=tool.name,
                description=tool.description or "",
                args_schema=create_schema_model(tool.name, tool.inputSchema),
            )
            # list_tools returns a PaginatedResult, but I don't see a way to pass the cursor to retrieve more tools
            for tool in self._tools.tools
        ]