Module blaxel.api.default.get_trace_ids
=======================================

Functions
---------

`asyncio(*, client: blaxel.client.AuthenticatedClient, workload_id: blaxel.types.Unset | str = <blaxel.types.Unset object>, workload_type: blaxel.types.Unset | str = <blaxel.types.Unset object>, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.models.get_trace_ids_response_200.GetTraceIdsResponse200 | None`
:   Get trace IDs
    
    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceIdsResponse200

`asyncio_detailed(*, client: blaxel.client.AuthenticatedClient, workload_id: blaxel.types.Unset | str = <blaxel.types.Unset object>, workload_type: blaxel.types.Unset | str = <blaxel.types.Unset object>, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.types.Response[blaxel.models.get_trace_ids_response_200.GetTraceIdsResponse200]`
:   Get trace IDs
    
    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceIdsResponse200]

`sync(*, client: blaxel.client.AuthenticatedClient, workload_id: blaxel.types.Unset | str = <blaxel.types.Unset object>, workload_type: blaxel.types.Unset | str = <blaxel.types.Unset object>, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.models.get_trace_ids_response_200.GetTraceIdsResponse200 | None`
:   Get trace IDs
    
    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceIdsResponse200

`sync_detailed(*, client: blaxel.client.AuthenticatedClient, workload_id: blaxel.types.Unset | str = <blaxel.types.Unset object>, workload_type: blaxel.types.Unset | str = <blaxel.types.Unset object>, limit: blaxel.types.Unset | str = <blaxel.types.Unset object>, start_time: blaxel.types.Unset | str = <blaxel.types.Unset object>, end_time: blaxel.types.Unset | str = <blaxel.types.Unset object>) ‑> blaxel.types.Response[blaxel.models.get_trace_ids_response_200.GetTraceIdsResponse200]`
:   Get trace IDs
    
    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceIdsResponse200]