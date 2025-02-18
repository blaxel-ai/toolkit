Module blaxel.api.functions.get_function_trace_ids
==================================================

Functions
---------

`asyncio(function_name: str, *, client: blaxel.client.AuthenticatedClient, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.models.trace_ids_response.TraceIdsResponse | None`
:   Get function trace IDs
    
    Args:
        function_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        TraceIdsResponse

`asyncio_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.types.Response[blaxel.models.trace_ids_response.TraceIdsResponse]`
:   Get function trace IDs
    
    Args:
        function_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[TraceIdsResponse]

`sync(function_name: str, *, client: blaxel.client.AuthenticatedClient, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.models.trace_ids_response.TraceIdsResponse | None`
:   Get function trace IDs
    
    Args:
        function_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        TraceIdsResponse

`sync_detailed(function_name: str, *, client: blaxel.client.AuthenticatedClient, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.types.Response[blaxel.models.trace_ids_response.TraceIdsResponse]`
:   Get function trace IDs
    
    Args:
        function_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[TraceIdsResponse]