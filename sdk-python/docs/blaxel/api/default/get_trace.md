Module beamlit.api.default.get_trace
====================================

Functions
---------

`asyncio(trace_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.get_trace_response_200.GetTraceResponse200 | None`
:   Get trace by ID
    
    Args:
        trace_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceResponse200

`asyncio_detailed(trace_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.get_trace_response_200.GetTraceResponse200]`
:   Get trace by ID
    
    Args:
        trace_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceResponse200]

`sync(trace_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.get_trace_response_200.GetTraceResponse200 | None`
:   Get trace by ID
    
    Args:
        trace_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        GetTraceResponse200

`sync_detailed(trace_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.get_trace_response_200.GetTraceResponse200]`
:   Get trace by ID
    
    Args:
        trace_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[GetTraceResponse200]