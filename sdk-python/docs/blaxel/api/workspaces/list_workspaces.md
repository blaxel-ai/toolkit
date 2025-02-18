Module beamlit.api.workspaces.list_workspaces
=============================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.workspace.Workspace] | None`
:   List workspaces
    
     Returns a list of all workspaces.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Workspace']

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.workspace.Workspace]]`
:   List workspaces
    
     Returns a list of all workspaces.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Workspace']]

`sync(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.workspace.Workspace] | None`
:   List workspaces
    
     Returns a list of all workspaces.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Workspace']

`sync_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.workspace.Workspace]]`
:   List workspaces
    
     Returns a list of all workspaces.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Workspace']]