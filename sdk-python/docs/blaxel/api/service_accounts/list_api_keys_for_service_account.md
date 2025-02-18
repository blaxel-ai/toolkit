Module beamlit.api.service_accounts.list_api_keys_for_service_account
=====================================================================

Functions
---------

`asyncio(client_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.api_key.ApiKey] | None`
:   List API keys for service account
    
     Returns a list of all API keys for a service account.
    
    Args:
        client_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ApiKey']

`asyncio_detailed(client_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.api_key.ApiKey]]`
:   List API keys for service account
    
     Returns a list of all API keys for a service account.
    
    Args:
        client_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ApiKey']]

`sync(client_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> list[beamlit.models.api_key.ApiKey] | None`
:   List API keys for service account
    
     Returns a list of all API keys for a service account.
    
    Args:
        client_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        list['ApiKey']

`sync_detailed(client_id: str, *, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[list[beamlit.models.api_key.ApiKey]]`
:   List API keys for service account
    
     Returns a list of all API keys for a service account.
    
    Args:
        client_id (str):
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[list['ApiKey']]