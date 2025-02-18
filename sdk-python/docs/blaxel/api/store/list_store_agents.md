Module blaxel.api.store.list_store_agents
=========================================

Functions
---------

`asyncio(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> list[blaxel.models.store_agent.StoreAgent] | None`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreAgent']

`asyncio_detailed(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> blaxel.types.Response[list[blaxel.models.store_agent.StoreAgent]]`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreAgent']]

`sync(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> list[blaxel.models.store_agent.StoreAgent] | None`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreAgent']

`sync_detailed(*, client: blaxel.client.AuthenticatedClient | blaxel.client.Client) ‑> blaxel.types.Response[list[blaxel.models.store_agent.StoreAgent]]`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreAgent']]