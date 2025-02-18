Module beamlit.api.policies.list_policies
=========================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.policy.Policy] | None`
:   List policies
    
     Returns a list of all policies in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Policy']

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.policy.Policy]]`
:   List policies
    
     Returns a list of all policies in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Policy']]

`sync(*, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.policy.Policy] | None`
:   List policies
    
     Returns a list of all policies in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['Policy']

`sync_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.policy.Policy]]`
:   List policies
    
     Returns a list of all policies in the workspace.
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['Policy']]