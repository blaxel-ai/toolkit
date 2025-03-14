Module blaxel.functions
=======================
Functions package providing function decorators and utilities for Blaxel integration.
It includes decorators for creating function tools and utilities for managing and retrieving functions.

Sub-modules
-----------
* blaxel.functions.common
* blaxel.functions.decorator
* blaxel.functions.local
* blaxel.functions.mcp
* blaxel.functions.remote
* blaxel.functions.utils

Functions
---------

`function(*args, function: blaxel.models.function.Function | dict = None, kit=False, **kwargs: dict) ‑> <class 'collections.abc.Callable'>`
:   Decorator to create function tools with Blaxel and LangChain integration.
    
    Args:
        function (Function | dict): Function metadata or a dictionary representing it.
        kit (bool): Whether to associate a function kit.
        **kwargs (dict): Additional keyword arguments for function configuration.
    
    Returns:
        Callable: The decorated function.

`get_functions(remote_functions: list[str] | None = None, local_functions: list[dict] | None = None, client: blaxel.client.AuthenticatedClient | None = None, dir: str | None = None, chain: list[blaxel.models.agent_chain.AgentChain] | None = None, remote_functions_empty: bool = True, local_functions_empty: bool = True, from_decorator: str = 'function', warning: bool = True) ‑> list[langchain_core.tools.base.BaseTool]`
:   Discovers and loads function tools from Python files and remote sources.
    
    This function walks through Python files in a specified directory, looking for
    decorated functions to convert into LangChain tools. It also handles remote
    functions and chain toolkits.
    
    Args:
        remote_functions (Union[list[str], None]): List of remote function names to load
        client (Union[AuthenticatedClient, None]): Authenticated client instance for API calls
        dir (Union[str, None]): Directory to search for Python files containing functions
        chain (Union[list[AgentChain], None]): List of agent chains to include
        remote_functions_empty (bool): Whether to allow empty remote functions
        from_decorator (str): Name of the decorator to look for (default: "function")
        warning (bool): Whether to show warning messages
    
    Returns:
        list: List of discovered and loaded function tools
    
    The function performs the following steps:
    1. Walks through Python files in the specified directory
    2. Parses each file to find decorated functions
    3. Converts found functions into LangChain StructuredTools
    4. Handles both synchronous and asynchronous functions
    5. Processes remote functions if specified
    6. Integrates chain toolkits if provided
    
    Example:
        ```python
        tools = get_functions(
            dir="./functions",
            from_decorator="function",
            warning=True
        )
        ```

`kit(parent: str, kit: blaxel.models.function_kit.FunctionKit = None, **kwargs: dict) ‑> <class 'collections.abc.Callable'>`
:   Decorator to create function tools with Blaxel and LangChain integration.
    
    Args:
        kit (FunctionKit | None): Optional FunctionKit to associate with the function.
        **kwargs (dict): Additional keyword arguments for function configuration.
    
    Returns:
        Callable: The decorated function.