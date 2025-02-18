Module beamlit.api.default.get_trace_ids
========================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, workload_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, workload_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.get_trace_ids_response_200.GetTraceIdsResponse200 | None`
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

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, workload_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, workload_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.get_trace_ids_response_200.GetTraceIdsResponse200]`
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

`sync(*, client: beamlit.client.AuthenticatedClient, workload_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, workload_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.get_trace_ids_response_200.GetTraceIdsResponse200 | None`
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

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, workload_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, workload_type: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.get_trace_ids_response_200.GetTraceIdsResponse200]`
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