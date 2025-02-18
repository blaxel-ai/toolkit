Module blaxel.api.functions.get_function_logs
=============================================

Functions
---------

`asyncio(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> list[blaxel.models.resource_log.ResourceLog] | None`
:   Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`asyncio_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[list[blaxel.models.resource_log.ResourceLog]]`
:   Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]

`sync(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> list[blaxel.models.resource_log.ResourceLog] | None`
:   Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ResourceLog']

`sync_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient) ‑> blaxel.types.Response[list[blaxel.models.resource_log.ResourceLog]]`
:   Args:
        function_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ResourceLog']]