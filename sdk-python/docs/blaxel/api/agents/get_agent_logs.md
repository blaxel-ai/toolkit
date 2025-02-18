Module blaxel.api.agents.get_agent_logs
=======================================

Functions
---------

`asyncio(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> list[blaxel.models.resource_log.ResourceLog] | None`
:   Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`asyncio_detailed(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[list[blaxel.models.resource_log.ResourceLog]]`
:   Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]

`sync(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> list[blaxel.models.resource_log.ResourceLog] | None`
:   Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`sync_detailed(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[list[blaxel.models.resource_log.ResourceLog]]`
:   Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]