Module blaxel.functions.mcp.mcp
===============================
This module provides functionalities to interact with MCP (Multi-Client Platform) servers.
It includes classes for managing MCP clients, creating dynamic schemas, and integrating MCP tools into Blaxel.

Classes
-------

`MCPClient(client: blaxel.client.AuthenticatedClient, url: str, fallback_url: str | None = None)`
:   

    ### Methods

    `call_tool(self, tool_name: str, arguments: dict[str, typing.Any] = None, is_fallback: bool = False) ‑> requests.models.Response | AsyncIterator[mcp.types.CallToolResult]`
    :

    `list_tools(self) ‑> mcp.types.ListToolsResult`
    :

    `list_ws_tools(self, is_fallback: bool = False) ‑> mcp.types.ListToolsResult`
    :

`MCPTool(**kwargs: Any)`
:   Tool for interacting with MCP server-hosted tools.
    
    Attributes:
        client (MCPClient): The MCP client instance.
        handle_tool_error (bool | str | Callable[[ToolException], str] | None): Error handling strategy.
    
    Initialize the tool.

    ### Ancestors (in MRO)

    * langchain_core.tools.base.BaseTool
    * langchain_core.runnables.base.RunnableSerializable[Union[str, dict, ToolCall], Any]
    * langchain_core.runnables.base.RunnableSerializable
    * langchain_core.load.serializable.Serializable
    * pydantic.main.BaseModel
    * langchain_core.runnables.base.Runnable
    * typing.Generic
    * abc.ABC

    ### Class variables

    `client: blaxel.functions.mcp.mcp.MCPClient`
    :

    `handle_tool_error: bool | str | Callable[[langchain_core.tools.base.ToolException], str] | None`
    :

    `model_config`
    :

    ### Instance variables

    `tool_call_schema: type[pydantic.main.BaseModel]`
    :

`MCPToolkit(**data: Any)`
:   Toolkit for managing MCP server tools.
    
    Attributes:
        client (MCPClient): The MCP client instance.
        _tools (ListToolsResult | None): Cached list of tools from the MCP server.
    
    Create a new model by parsing and validating input data from keyword arguments.
    
    Raises [`ValidationError`][pydantic_core.ValidationError] if the input data cannot be
    validated to form a valid model.
    
    `self` is explicitly positional-only to allow `self` as a field name.

    ### Ancestors (in MRO)

    * langchain_core.tools.base.BaseToolkit
    * pydantic.main.BaseModel
    * abc.ABC

    ### Class variables

    `client: blaxel.functions.mcp.mcp.MCPClient`
    :   The MCP session used to obtain the tools

    `model_config`
    :

    ### Methods

    `get_tools(self) ‑> list[langchain_core.tools.base.BaseTool]`
    :   Get the tools in the toolkit.

    `initialize(self) ‑> None`
    :   Initialize the session and retrieve tools list

    `model_post_init(self: BaseModel, context: Any, /) ‑> None`
    :   This function is meant to behave like a BaseModel method to initialise private attributes.
        
        It takes context as an argument since that's what pydantic-core passes when calling it.
        
        Args:
            self: The BaseModel instance.
            context: The context.