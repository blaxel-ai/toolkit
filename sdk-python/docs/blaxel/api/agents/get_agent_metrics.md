Module blaxel.api.agents.get_agent_metrics
==========================================

Functions
---------

`asyncio(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get agent metrics
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`asyncio_detailed(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get agent metrics
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]

`sync(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get agent metrics
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`sync_detailed(agent_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get agent metrics
    
    Args:
        agent_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]