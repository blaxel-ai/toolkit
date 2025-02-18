Module beamlit.api.workspaces.get_workspace
===========================================

Functions
---------

`asyncio(workspace_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.workspace.Workspace | None`
:   Get workspace
    
     Returns a workspace by name.
    
    Args:
        workspace_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Workspace

`asyncio_detailed(workspace_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.workspace.Workspace]`
:   Get workspace
    
     Returns a workspace by name.
    
    Args:
        workspace_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Workspace]

`sync(workspace_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.workspace.Workspace | None`
:   Get workspace
    
     Returns a workspace by name.
    
    Args:
        workspace_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Workspace

`sync_detailed(workspace_name: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.workspace.Workspace]`
:   Get workspace
    
     Returns a workspace by name.
    
    Args:
        workspace_name (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Workspace]