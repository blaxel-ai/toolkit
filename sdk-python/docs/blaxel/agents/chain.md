Module blaxel.agents.chain
==========================

Classes
-------

`ChainInput(**data: Any)`
:   A Pydantic model representing the input structure for a chain.
    
    Create a new model by parsing and validating input data from keyword arguments.
    
    Raises [`ValidationError`][pydantic_core.ValidationError] if the input data cannot be
    validated to form a valid model.
    
    `self` is explicitly positional-only to allow `self` as a field name.

    ### Ancestors (in MRO)

    * pydantic.main.BaseModel

    ### Class variables

    `inputs: str`
    :

    `model_config`
    :

`ChainTool(**kwargs: Any)`
:   A tool that allows chaining of agent actions. Extends LangChain's BaseTool.
    
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

    `client: blaxel.run.RunClient`
    :

    `handle_tool_error: bool | str | Callable[[langchain_core.tools.base.ToolException], str] | None`
    :

    `model_config`
    :

    ### Instance variables

    `tool_call_schema: type[pydantic.main.BaseModel]`
    :   Defines the schema for tool calls based on the provided argument schema.
        
        Returns:
            type[pydantic.BaseModel]: The Pydantic model representing the tool call schema.

    ### Methods

    `model_post_init(self: BaseModel, context: Any, /) ‑> None`
    :   This function is meant to behave like a BaseModel method to initialise private attributes.
        
        It takes context as an argument since that's what pydantic-core passes when calling it.
        
        Args:
            self: The BaseModel instance.
            context: The context.

`ChainToolkit(client: blaxel.client.AuthenticatedClient, chain: list[blaxel.models.agent_chain.AgentChain])`
:   A toolkit for managing and initializing a chain of agents.

    ### Class variables

    `chain: list[blaxel.models.agent_chain.AgentChain]`
    :

    `client: blaxel.client.AuthenticatedClient`
    :

    `model_config`
    :

    ### Methods

    `get_tools(self) ‑> list[langchain_core.tools.base.BaseTool]`
    :   Retrieves a list of tools corresponding to the initialized agents.
        
        Returns:
            list[BaseTool]: A list of initialized `ChainTool` instances.
        
        Raises:
            RuntimeError: If the toolkit has not been initialized.

    `initialize(self) ‑> None`
    :   Initializes the toolkit by retrieving and configuring the list of agents based on the provided chains.
        
        Raises:
            RuntimeError: If initialization fails due to missing agents.