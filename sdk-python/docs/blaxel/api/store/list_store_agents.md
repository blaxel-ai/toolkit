Module beamlit.api.store.list_store_agents
==========================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> list[beamlit.models.store_agent.StoreAgent] | None`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreAgent']

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[list[beamlit.models.store_agent.StoreAgent]]`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreAgent']]

`sync(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> list[beamlit.models.store_agent.StoreAgent] | None`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['StoreAgent']

`sync_detailed(*, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[list[beamlit.models.store_agent.StoreAgent]]`
:   List all store agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['StoreAgent']]