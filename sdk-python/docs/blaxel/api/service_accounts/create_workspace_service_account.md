Module beamlit.api.service_accounts.create_workspace_service_account
====================================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.create_workspace_service_account_body.CreateWorkspaceServiceAccountBody) ‑> beamlit.models.create_workspace_service_account_response_200.CreateWorkspaceServiceAccountResponse200 | None`
:   Create workspace service account
    
     Creates a service account in the workspace.
    
    Args:
        body (CreateWorkspaceServiceAccountBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        CreateWorkspaceServiceAccountResponse200

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.create_workspace_service_account_body.CreateWorkspaceServiceAccountBody) ‑> beamlit.types.Response[beamlit.models.create_workspace_service_account_response_200.CreateWorkspaceServiceAccountResponse200]`
:   Create workspace service account
    
     Creates a service account in the workspace.
    
    Args:
        body (CreateWorkspaceServiceAccountBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[CreateWorkspaceServiceAccountResponse200]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.create_workspace_service_account_body.CreateWorkspaceServiceAccountBody) ‑> beamlit.models.create_workspace_service_account_response_200.CreateWorkspaceServiceAccountResponse200 | None`
:   Create workspace service account
    
     Creates a service account in the workspace.
    
    Args:
        body (CreateWorkspaceServiceAccountBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        CreateWorkspaceServiceAccountResponse200

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.create_workspace_service_account_body.CreateWorkspaceServiceAccountBody) ‑> beamlit.types.Response[beamlit.models.create_workspace_service_account_response_200.CreateWorkspaceServiceAccountResponse200]`
:   Create workspace service account
    
     Creates a service account in the workspace.
    
    Args:
        body (CreateWorkspaceServiceAccountBody):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[CreateWorkspaceServiceAccountResponse200]