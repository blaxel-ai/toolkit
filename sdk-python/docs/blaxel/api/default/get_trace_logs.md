Module beamlit.api.default.get_trace_logs
=========================================

Functions
---------

`asyncio(trace_id: str, *, client: beamlit.client.AuthenticatedClient, span_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.get_trace_logs_response_200.GetTraceLogsResponse200 | None`
:   Get trace logs
    
    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceLogsResponse200

`asyncio_detailed(trace_id: str, *, client: beamlit.client.AuthenticatedClient, span_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.get_trace_logs_response_200.GetTraceLogsResponse200]`
:   Get trace logs
    
    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceLogsResponse200]

`sync(trace_id: str, *, client: beamlit.client.AuthenticatedClient, span_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.get_trace_logs_response_200.GetTraceLogsResponse200 | None`
:   Get trace logs
    
    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceLogsResponse200

`sync_detailed(trace_id: str, *, client: beamlit.client.AuthenticatedClient, span_id: beamlit.types.Unset | str = <beamlit.types.Unset object>, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.get_trace_logs_response_200.GetTraceLogsResponse200]`
:   Get trace logs
    
    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceLogsResponse200]