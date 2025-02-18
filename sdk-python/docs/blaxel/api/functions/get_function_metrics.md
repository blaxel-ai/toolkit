Module blaxel.api.functions.get_function_metrics
================================================

Functions
---------

`asyncio(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get function metrics
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`asyncio_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get function metrics
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]

`sync(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.models.resource_metrics.ResourceMetrics | None`
:   Get function metrics
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        ResourceMetrics

`sync_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[blaxel.models.resource_metrics.ResourceMetrics]`
:   Get function metrics
    
    Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[ResourceMetrics]