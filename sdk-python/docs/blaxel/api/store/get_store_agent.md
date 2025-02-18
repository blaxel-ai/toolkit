Module beamlit.api.store.get_store_agent
========================================

Functions
---------

`asyncio(agent_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.models.store_agent.StoreAgent | None`
:   Get store agent by name
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        StoreAgent

`asyncio_detailed(agent_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[beamlit.models.store_agent.StoreAgent]`
:   Get store agent by name
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[StoreAgent]

`sync(agent_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.models.store_agent.StoreAgent | None`
:   Get store agent by name
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        StoreAgent

`sync_detailed(agent_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client) ‑> beamlit.types.Response[beamlit.models.store_agent.StoreAgent]`
:   Get store agent by name
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[StoreAgent]