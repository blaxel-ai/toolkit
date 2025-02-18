Module blaxel.api.store.list_store_functions
============================================

Functions
---------

`asyncio(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> list[blaxel.models.store_function.StoreFunction] | None`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreFunction']

`asyncio_detailed(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> blaxel.types.Response[list[blaxel.models.store_function.StoreFunction]]`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreFunction']]

`sync(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> list[blaxel.models.store_function.StoreFunction] | None`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreFunction']

`sync_detailed(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> blaxel.types.Response[list[blaxel.models.store_function.StoreFunction]]`
:   List all store agent functions
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreFunction']]