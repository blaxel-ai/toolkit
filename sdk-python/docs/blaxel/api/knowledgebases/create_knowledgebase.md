Module beamlit.api.knowledgebases.create_knowledgebase
======================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.knowledgebase.Knowledgebase) ‑> beamlit.models.knowledgebase.Knowledgebase | None`
:   Create knowledgebase
    
     Creates an knowledgebase.
    
    Args:
        body (Knowledgebase): Knowledgebase
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Knowledgebase

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.knowledgebase.Knowledgebase) ‑> beamlit.types.Response[beamlit.models.knowledgebase.Knowledgebase]`
:   Create knowledgebase
    
     Creates an knowledgebase.
    
    Args:
        body (Knowledgebase): Knowledgebase
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Knowledgebase]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.knowledgebase.Knowledgebase) ‑> beamlit.models.knowledgebase.Knowledgebase | None`
:   Create knowledgebase
    
     Creates an knowledgebase.
    
    Args:
        body (Knowledgebase): Knowledgebase
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Knowledgebase

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.knowledgebase.Knowledgebase) ‑> beamlit.types.Response[beamlit.models.knowledgebase.Knowledgebase]`
:   Create knowledgebase
    
     Creates an knowledgebase.
    
    Args:
        body (Knowledgebase): Knowledgebase
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Knowledgebase]