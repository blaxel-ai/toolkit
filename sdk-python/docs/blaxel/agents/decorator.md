Module blaxel.agents.decorator
==============================
Module: decorator

Defines decorators for agent functionalities.

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

`initialize_agent(settings: blaxel.common.settings.Settings, agent: blaxel.models.agent.Agent | dict = None, override_model=None, override_agent=None, override_functions=None, remote_functions=None, local_functions=None)`
: