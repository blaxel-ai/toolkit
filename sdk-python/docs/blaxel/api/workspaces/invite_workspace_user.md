Module beamlit.api.workspaces.invite_workspace_user
===================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.invite_workspace_user_body.InviteWorkspaceUserBody) ‑> Any | beamlit.models.pending_invitation.PendingInvitation | None`
:   Invite user to workspace
    
     Invites a user to the workspace by email.
    
    Args:
        body (InviteWorkspaceUserBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, PendingInvitation]

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.invite_workspace_user_body.InviteWorkspaceUserBody) ‑> beamlit.types.Response[Any | beamlit.models.pending_invitation.PendingInvitation]`
:   Invite user to workspace
    
     Invites a user to the workspace by email.
    
    Args:
        body (InviteWorkspaceUserBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, PendingInvitation]]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.invite_workspace_user_body.InviteWorkspaceUserBody) ‑> Any | beamlit.models.pending_invitation.PendingInvitation | None`
:   Invite user to workspace
    
     Invites a user to the workspace by email.
    
    Args:
        body (InviteWorkspaceUserBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, PendingInvitation]

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.invite_workspace_user_body.InviteWorkspaceUserBody) ‑> beamlit.types.Response[Any | beamlit.models.pending_invitation.PendingInvitation]`
:   Invite user to workspace
    
     Invites a user to the workspace by email.
    
    Args:
        body (InviteWorkspaceUserBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, PendingInvitation]]