Module beamlit.functions.decorator
==================================
Decorators for creating function tools with Beamlit and LangChain integration.

Functions
---------

`function(*args, function: beamlit.models.function.Function | dict = None, kit=False, **kwargs: dict) ‑> <class 'collections.abc.Callable'>`
:   Decorator to create function tools with Beamlit and LangChain integration.
    
    Args:
        function (Function | dict): Function metadata or a dictionary representing it.
        kit (bool): Whether to associate a function kit.
        **kwargs (dict): Additional keyword arguments for function configuration.
    
    Returns:
        Callable: The decorated function.

`kit(bl_kit: beamlit.models.function_kit.FunctionKit = None, **kwargs: dict) ‑> <class 'collections.abc.Callable'>`
:   Decorator to create function tools with Beamlit and LangChain integration.
    
    Args:
        bl_kit (FunctionKit | None): Optional FunctionKit to associate with the function.
        **kwargs (dict): Additional keyword arguments for function configuration.
    
    Returns:
        Callable: The decorated function.