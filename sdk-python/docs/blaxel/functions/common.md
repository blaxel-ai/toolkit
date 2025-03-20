Module blaxel.functions.common
==============================
Decorators for creating function tools with Blaxel and LangChain integration.

This module provides functionality to discover and load function tools from Python files,
supporting both local and remote function execution.

Key Features:
- Automatic function discovery in specified directories
- Support for both synchronous and asynchronous functions
- Integration with LangChain's StructuredTool system
- Remote function toolkit handling
- Chain toolkit integration

Main Components:
- get_functions(): Core function that discovers and loads function tools

Functions
---------

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

`initialize_with_retry(toolkit, function_name: str, max_retries: int) ‑> list[langchain_core.tools.base.BaseTool]`
: