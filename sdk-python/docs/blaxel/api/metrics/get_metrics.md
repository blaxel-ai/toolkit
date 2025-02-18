Module blaxel.api.metrics.get_metrics
=====================================

Functions
---------

`asyncio(*, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.metrics.Metrics | None`
:   Get metrics for a workspace
    
     Returns metrics for the workspace's deployments.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Metrics

`asyncio_detailed(*, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.metrics.Metrics]`
:   Get metrics for a workspace
    
     Returns metrics for the workspace's deployments.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Metrics]

`sync(*, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.metrics.Metrics | None`
:   Get metrics for a workspace
    
     Returns metrics for the workspace's deployments.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Metrics

`sync_detailed(*, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.metrics.Metrics]`
:   Get metrics for a workspace
    
     Returns metrics for the workspace's deployments.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Metrics]