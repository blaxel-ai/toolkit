Module beamlit.api.store.get_store_function
===========================================

Functions
---------

`asyncio(function_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.models.store_function.StoreFunction | None`
:   Get store agent function by name
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        StoreFunction

`asyncio_detailed(function_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[beamlit.models.store_function.StoreFunction]`
:   Get store agent function by name
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[StoreFunction]

`sync(function_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.models.store_function.StoreFunction | None`
:   Get store agent function by name
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        StoreFunction

`sync_detailed(function_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[beamlit.models.store_function.StoreFunction]`
:   Get store agent function by name
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[StoreFunction]