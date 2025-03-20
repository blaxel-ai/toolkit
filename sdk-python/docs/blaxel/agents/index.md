Module blaxel.agents
====================

Sub-modules
-----------
* blaxel.agents.chain
* blaxel.agents.chat
* blaxel.agents.decorator
* blaxel.agents.thread
* blaxel.agents.voice

Functions
---------

`agent(agent: blaxel.models.agent.Agent | dict = None, override_model=None, override_agent: Any | Callable = None, override_functions=None, remote_functions=None, local_functions=None) ‑> Callable`
:   A decorator factory that configures and wraps functions to integrate with Blaxel agents.
    Handles model initialization, function retrieval, and agent setup.
    
    Parameters:
        agent (Agent | dict, optional): An `Agent` instance or a dictionary containing agent metadata and specifications.
        override_model (Any, optional): An optional model to override the default agent model.
        override_agent (Any, Callable, optional): An optional agent instance to override the default agent, or a function which returns an agent instance.
        mcp_hub (Any, optional): An optional MCP hub configuration.
        remote_functions (Any, optional): An optional list of remote functions to be integrated.
        local_functions (Any, optional): An optional list of local functions to be integrated.
    
    Returns:
        Callable: A decorator that wraps the target function, injecting agent-related configurations and dependencies.
    
    Behavior:
        - Validates and initializes the agent configuration.
        - Retrieves and sets up the appropriate chat model based on the agent's specifications.
        - Retrieves functions from the specified directories or remote sources.
        - Wraps the target function, injecting agent, model, and function dependencies as needed.
        - Logs relevant information and handles exceptions during the setup process.
    
    Raises:
        ValueError: If required configurations such as the model are missing.
        Re-raises exceptions encountered during model retrieval and agent setup.

`get_chat_model(name: str, agent_model: blaxel.models.model.Model | None = None) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Gets a chat model instance for the specified model name.
    
    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.
    
    Returns:
        BaseChatModel: An instance of the appropriate chat model.

`get_chat_model_full(name: str, agent_model: blaxel.models.model.Model | None = None) ‑> Tuple[langchain_core.language_models.chat_models.BaseChatModel, str, str]`
:   Gets a chat model instance along with provider and model information.
    
    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.
    
    Returns:
        Tuple[BaseChatModel, str, str]: A tuple containing:
            - The chat model instance
            - The provider name (e.g., 'openai', 'anthropic', etc.)
            - The specific model name (e.g., 'gpt-4o-mini')

`get_default_thread(request: starlette.requests.Request) ‑> str`
:   Extracts the default thread identifier from an incoming HTTP request.
    Prioritizes the `X-Blaxel-Sub` header and falls back to decoding the JWT
    from the `Authorization` or `X-Blaxel-Authorization` headers.
    
    Parameters:
        request (Request): The incoming HTTP request object.
    
    Returns:
        str: The extracted thread identifier. Returns an empty string if no valid identifier is found.