Module beamlit.functions.remote.remote
======================================
This module provides functionalities to integrate remote functions into Beamlit.
It includes classes for creating dynamic schemas based on function parameters and managing remote toolkits.

Functions
---------

`create_dynamic_schema(name: str, parameters: list[beamlit.models.store_function_parameter.StoreFunctionParameter]) ‑> type[pydantic.main.BaseModel]`
:   Creates a dynamic Pydantic schema based on function parameters.
    
    Args:
        name (str): The name of the schema.
        parameters (list[StoreFunctionParameter]): List of parameter objects.
    
    Returns:
        type[pydantic.BaseModel]: The dynamically created Pydantic model.

Classes
-------

`RemoteTool(**kwargs: Any)`
:   Tool for interacting with remote functions.
    
    Attributes:
        client (RunClient): The client used to execute remote function calls.
        resource_name (str): The name of the remote resource.
        kit (bool): Indicates whether the tool is part of a function kit.
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

    `client: beamlit.run.RunClient`
    :

    `cloud: bool`
    :

    `handle_tool_error: bool | str | Callable[[langchain_core.tools.base.ToolException], str] | None`
    :

    `kit: bool`
    :

    `model_config`
    :

    `resource_name: str`
    :

    `service_name: str | None`
    :

    ### Instance variables

    `tool_call_schema: type[pydantic.main.BaseModel]`
    :

`RemoteToolkit(client: beamlit.client.AuthenticatedClient, function: str)`
:   Toolkit for managing remote function tools.
    
    Attributes:
        client (AuthenticatedClient): The authenticated client instance.
        function (str): The name of the remote function to integrate.
        _function (Function | None): Cached Function object after initialization.

    ### Class variables

    `client: beamlit.client.AuthenticatedClient`
    :

    `function: str`
    :

    `model_config`
    :

    ### Methods

    `get_tools(self) ‑> list[langchain_core.tools.base.BaseTool]`
    :

    `initialize(self) ‑> None`
    :   Initialize the session and retrieve the remote function details.