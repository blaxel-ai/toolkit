Module beamlit.api.models.get_model_logs
========================================

Functions
---------

`asyncio(model_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.resource_log.ResourceLog] | None`
:   Returns logs for a model deployment by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`asyncio_detailed(model_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.resource_log.ResourceLog]]`
:   Returns logs for a model deployment by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]

`sync(model_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.resource_log.ResourceLog] | None`
:   Returns logs for a model deployment by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`sync_detailed(model_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.resource_log.ResourceLog]]`
:   Returns logs for a model deployment by name.
    
    Args:
        model_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]