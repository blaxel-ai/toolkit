Module beamlit.api.workspaces.update_workspace_user_role
========================================================

Functions
---------

`asyncio(sub_or_email: str, *, client: beamlit.client.AuthenticatedClient, body: beamlit.models.update_workspace_user_role_body.UpdateWorkspaceUserRoleBody) ‑> Any | beamlit.models.workspace_user.WorkspaceUser | None`
:   Update user role in workspace
    
     Updates the role of a user in the workspace.
    
    Args:
        sub_or_email (str):
        body (UpdateWorkspaceUserRoleBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, WorkspaceUser]

`asyncio_detailed(sub_or_email: str, *, client: beamlit.client.AuthenticatedClient, body: beamlit.models.update_workspace_user_role_body.UpdateWorkspaceUserRoleBody) ‑> beamlit.types.Response[Any | beamlit.models.workspace_user.WorkspaceUser]`
:   Update user role in workspace
    
     Updates the role of a user in the workspace.
    
    Args:
        sub_or_email (str):
        body (UpdateWorkspaceUserRoleBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, WorkspaceUser]]

`sync(sub_or_email: str, *, client: beamlit.client.AuthenticatedClient, body: beamlit.models.update_workspace_user_role_body.UpdateWorkspaceUserRoleBody) ‑> Any | beamlit.models.workspace_user.WorkspaceUser | None`
:   Update user role in workspace
    
     Updates the role of a user in the workspace.
    
    Args:
        sub_or_email (str):
        body (UpdateWorkspaceUserRoleBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, WorkspaceUser]

`sync_detailed(sub_or_email: str, *, client: beamlit.client.AuthenticatedClient, body: beamlit.models.update_workspace_user_role_body.UpdateWorkspaceUserRoleBody) ‑> beamlit.types.Response[Any | beamlit.models.workspace_user.WorkspaceUser]`
:   Update user role in workspace
    
     Updates the role of a user in the workspace.
    
    Args:
        sub_or_email (str):
        body (UpdateWorkspaceUserRoleBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, WorkspaceUser]]