Module beamlit.api.workspaces.create_worspace
=============================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.workspace.Workspace) ‑> beamlit.models.workspace.Workspace | None`
:   Create worspace
    
     Creates a workspace.
    
    Args:
        body (Workspace): Workspace
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Workspace

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.workspace.Workspace) ‑> beamlit.types.Response[beamlit.models.workspace.Workspace]`
:   Create worspace
    
     Creates a workspace.
    
    Args:
        body (Workspace): Workspace
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Workspace]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.workspace.Workspace) ‑> beamlit.models.workspace.Workspace | None`
:   Create worspace
    
     Creates a workspace.
    
    Args:
        body (Workspace): Workspace
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Workspace

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.workspace.Workspace) ‑> beamlit.types.Response[beamlit.models.workspace.Workspace]`
:   Create worspace
    
     Creates a workspace.
    
    Args:
        body (Workspace): Workspace
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Workspace]