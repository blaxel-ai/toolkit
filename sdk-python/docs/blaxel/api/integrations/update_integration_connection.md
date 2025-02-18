Module beamlit.api.integrations.update_integration_connection
=============================================================

Functions
---------

`asyncio(connection_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client, body: beamlit.models.integration_connection.IntegrationConnection) ‑> beamlit.models.integration_connection.IntegrationConnection | None`
:   Update integration connection
    
     Update an integration connection by integration name and connection name.
    
    Args:
        connection_name (str):
        body (IntegrationConnection): Integration Connection
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        IntegrationConnection

`asyncio_detailed(connection_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client, body: beamlit.models.integration_connection.IntegrationConnection) ‑> beamlit.types.Response[beamlit.models.integration_connection.IntegrationConnection]`
:   Update integration connection
    
     Update an integration connection by integration name and connection name.
    
    Args:
        connection_name (str):
        body (IntegrationConnection): Integration Connection
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[IntegrationConnection]

`sync(connection_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client, body: beamlit.models.integration_connection.IntegrationConnection) ‑> beamlit.models.integration_connection.IntegrationConnection | None`
:   Update integration connection
    
     Update an integration connection by integration name and connection name.
    
    Args:
        connection_name (str):
        body (IntegrationConnection): Integration Connection
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        IntegrationConnection

`sync_detailed(connection_name: str, *, client: beamlit.client.AuthenticatedClient | beamlit.client.Client, body: beamlit.models.integration_connection.IntegrationConnection) ‑> beamlit.types.Response[beamlit.models.integration_connection.IntegrationConnection]`
:   Update integration connection
    
     Update an integration connection by integration name and connection name.
    
    Args:
        connection_name (str):
        body (IntegrationConnection): Integration Connection
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[IntegrationConnection]