Module beamlit.api.store.list_store_functions
=============================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> list[beamlit.models.store_function.StoreFunction] | None`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreFunction']

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[list[beamlit.models.store_function.StoreFunction]]`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreFunction']]

`sync(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> list[beamlit.models.store_function.StoreFunction] | None`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreFunction']

`sync_detailed(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[list[beamlit.models.store_function.StoreFunction]]`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreFunction']]