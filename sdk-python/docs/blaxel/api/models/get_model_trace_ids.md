Module beamlit.api.models.get_model_trace_ids
=============================================

Functions
---------

`asyncio(model_name: str, *, client: beamlit.client.AuthenticatedClient, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.trace_ids_response.TraceIdsResponse | None`
:   Get model trace IDs
    
    Args:
        model_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        TraceIdsResponse

`asyncio_detailed(model_name: str, *, client: beamlit.client.AuthenticatedClient, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.trace_ids_response.TraceIdsResponse]`
:   Get model trace IDs
    
    Args:
        model_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[TraceIdsResponse]

`sync(model_name: str, *, client: beamlit.client.AuthenticatedClient, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.models.trace_ids_response.TraceIdsResponse | None`
:   Get model trace IDs
    
    Args:
        model_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        TraceIdsResponse

`sync_detailed(model_name: str, *, client: beamlit.client.AuthenticatedClient, limit: beamlit.types.Unset | str = <beamlit.types.Unset object>, start_time: beamlit.types.Unset | str = <beamlit.types.Unset object>, end_time: beamlit.types.Unset | str = <beamlit.types.Unset object>) ‑> beamlit.types.Response[beamlit.models.trace_ids_response.TraceIdsResponse]`
:   Get model trace IDs
    
    Args:
        model_name (str):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[TraceIdsResponse]