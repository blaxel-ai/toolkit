import asyncio
import warnings
from typing import Any, Callable

import pydantic
import pydantic_core
import requests
import typing_extensions as t
from beamlit.authentication.authentication import AuthenticatedClient
from beamlit.common.settings import get_settings
from langchain_core.tools.base import BaseTool, BaseToolkit, ToolException
from mcp.types import CallToolResult, ListToolsResult
from pydantic.json_schema import JsonSchemaValue
from pydantic_core import core_schema as cs

settings = get_settings()

TYPE_MAP = {
    "integer": int,
    "number": float,
    "array": list,
    "object": dict,
    "boolean": bool,
    "string": str,
    "null": type(None),
}

FIELD_DEFAULTS = {
    int: 0,
    float: 0.0,
    list: [],
    bool: False,
    str: "",
    type(None): None,
}

def configure_field(name: str, type_: dict[str, t.Any], required: list[str]) -> tuple[type, t.Any]:
    field_type = TYPE_MAP[type_["type"]]
    default_ = FIELD_DEFAULTS.get(field_type) if name not in required else ...
    return field_type, default_

def create_schema_model(name: str, schema: dict[str, t.Any]) -> type[pydantic.BaseModel]:
    # Create a new model class that returns our JSON schema.
    # LangChain requires a BaseModel class.
    class SchemaBase(pydantic.BaseModel):
        model_config = pydantic.ConfigDict(extra="allow")

        @t.override
        @classmethod
        def __get_pydantic_json_schema__(
            cls, core_schema: cs.CoreSchema, handler: pydantic.GetJsonSchemaHandler
        ) -> JsonSchemaValue:
            return schema

    # Since this langchain patch, we need to synthesize pydantic fields from the schema
    # https://github.com/langchain-ai/langchain/commit/033ac417609297369eb0525794d8b48a425b8b33
    required = schema.get("required", [])
    fields: dict[str, t.Any] = {
        name: configure_field(name, type_, required) for name, type_ in schema["properties"].items()
    }

    return pydantic.create_model(f"{name}Schema", __base__=SchemaBase, **fields)



class MCPClient:
    def __init__(self, client: AuthenticatedClient, url: str):
        self.client = client
        self.url = url

    def list_tools(self) -> requests.Response:
        client = self.client.get_httpx_client()
        response = client.request("GET", f"{self.url}/tools/list")
        response.raise_for_status()
        return response

    def call_tool(self, tool_name: str, arguments: dict[str, Any] = None) -> requests.Response:
        client = self.client.get_httpx_client()
        response = client.request(
            "POST",
            f"{self.url}/tools/call",
            json={"name": tool_name, "arguments": arguments},
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
        result = CallToolResult(**response)
        if result.isError:
            raise ToolException(result.content)
        content = pydantic_core.to_json(result.content).decode()
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
                args_schema=create_schema_model(tool.name, tool.inputSchema),
            )
            # list_tools returns a PaginatedResult, but I don't see a way to pass the cursor to retrieve more tools
            for tool in self._tools.tools
        ]