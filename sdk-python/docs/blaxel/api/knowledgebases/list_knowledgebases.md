Module beamlit.api.knowledgebases.list_knowledgebases
=====================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.knowledgebase.Knowledgebase] | None`
:   List knowledgebases
    
     Returns a list of all knowledgebases in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Knowledgebase']

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.knowledgebase.Knowledgebase]]`
:   List knowledgebases
    
     Returns a list of all knowledgebases in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Knowledgebase']]

`sync(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.knowledgebase.Knowledgebase] | None`
:   List knowledgebases
    
     Returns a list of all knowledgebases in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Knowledgebase']

`sync_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.knowledgebase.Knowledgebase]]`
:   List knowledgebases
    
     Returns a list of all knowledgebases in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Knowledgebase']]