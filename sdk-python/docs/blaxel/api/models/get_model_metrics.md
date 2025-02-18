Module blaxel.api.models.get_model_metrics
==========================================

Functions
---------

`asyncio(model_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get model metrics
    
     Returns metrics for a model by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`asyncio_detailed(model_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get model metrics
    
     Returns metrics for a model by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]

`sync(model_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get model metrics
    
     Returns metrics for a model by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`sync_detailed(model_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get model metrics
    
     Returns metrics for a model by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]