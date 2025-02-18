from dataclasses import dataclass

import pydantic
from langchain_core.tools.base import BaseTool

from blaxel.authentication.authentication import AuthenticatedClient
from blaxel.functions.mcp.mcp import MCPClient, MCPToolkit
from blaxel.models import Function


@dataclass
class LocalToolKit:
    """
    Toolkit for managing local tools.

    Attributes:
        client (AuthenticatedClient): The authenticated client instance.
        function (str): The name of the local function to integrate.
        _function (Function | None): Cached Function object after initialization.
    """
    client: AuthenticatedClient
    local_function: dict
    _function: Function | None = None
    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    async def initialize(self) -> None:
        """Initialize the session and retrieve the local function details."""
        if self._function is None:
            try:
                # For local functions, we directly create the Function object
                # based on the local function name
                self._function = Function(
                    metadata={"name": self.local_function['name']},
                    spec={
                        "configurations": {
                            "url": self.local_function['url'],
                        },
                        "description": self.local_function['description'] or "",
                    }
                )
            except Exception as e:
                raise RuntimeError(f"Failed to initialize local function: {e}")

    async def get_tools(self) -> list[BaseTool]:
        mcp_client = MCPClient(self.client, self._function.spec["configurations"]["url"], None)
        mcp_toolkit = MCPToolkit(client=mcp_client, url=self._function.spec["configurations"]["url"])
        await mcp_toolkit.initialize()
        return await mcp_toolkit.get_tools()
