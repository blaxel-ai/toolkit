import asyncio
import urllib.parse
import warnings
from typing import Any, Callable

import pydantic
import pydantic_core
import requests
import typing_extensions as t
from langchain_core.tools.base import BaseTool, BaseToolkit, ToolException
from mcp import ListToolsResult

from beamlit.authentication.authentication import AuthenticatedClient
from beamlit.common.settings import get_settings

settings = get_settings()

def create_dynamic_schema(name: str, schema: dict[str, t.Any]) -> type[pydantic.BaseModel]:
    field_definitions = {}
    for k, v in schema["properties"].items():
        field_type = str
        if v["type"] == "number":
            field_type = float
        elif v["type"] == "integer":
            field_type = int
        elif v["type"] == "boolean":
            field_type = bool
        description = v.get("description") or ""
        default_ = v.get("default")
        fields = {}
        if default_ is not None:
            fields["default"] = default_
        if description is not None:
            fields["description"] = description
        field_definitions[k] = (
            field_type,
            pydantic.Field(**fields)
        )
    return pydantic.create_model(
        f"{name}Schema",
        **field_definitions
    )



class MCPClient:
    def __init__(self, client: AuthenticatedClient, server_name: str):
        self.client = client
        self.server_name = server_name
        self.headers = {"Api-Key": "1234567890"}

    def list_tools(self) -> requests.Response:
        client = self.client.get_httpx_client()
        url = urllib.parse.urljoin(settings.mcp_hub_url, f"{self.server_name}/tools/list")
        response = client.request("GET", url, headers=self.headers)
        response.raise_for_status()
        return response

    def call_tool(self, tool_name: str, arguments: dict[str, Any] = None) -> requests.Response:
        client = self.client.get_httpx_client()
        url = urllib.parse.urljoin(settings.mcp_hub_url, f"{self.server_name}/tools/call")
        response = client.request(
            "POST",
            url,
            json={"name": tool_name, "arguments": arguments},
            headers=self.headers,
        )
        response.raise_for_status()
        return response

class MCPTool(BaseTool):
    """
    MCP server tool
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
        result = self.client.call_tool(self.name, arguments=kwargs)
        response = result.json()
        content = pydantic_core.to_json(response["content"]).decode()
        if response["isError"]:
            raise ToolException(content)
        return content

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema

class MCPToolkit(BaseToolkit):
    """
    MCP server toolkit
    """

    client: MCPClient
    """The MCP session used to obtain the tools"""

    _tools: ListToolsResult | None = None

    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    def initialize(self) -> None:
        """Initialize the session and retrieve tools list"""
        if self._tools is None:
            response = self.client.list_tools()
            self._tools = ListToolsResult(**response.json())

    @t.override
    def get_tools(self) -> list[BaseTool]:
        if self._tools is None:
            raise RuntimeError("Must initialize the toolkit first")

        return [
            MCPTool(
                client=self.client,
                name=tool.name,
                description=tool.description or "",
                args_schema=create_dynamic_schema(tool.name, tool.inputSchema),
            )
            # list_tools returns a PaginatedResult, but I don't see a way to pass the cursor to retrieve more tools
            for tool in self._tools.tools
        ]