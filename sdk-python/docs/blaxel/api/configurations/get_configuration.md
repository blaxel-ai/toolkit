Module beamlit.api.configurations.get_configuration
===================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.configuration.Configuration | None`
:   List all configurations
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Configuration

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.configuration.Configuration]`
:   List all configurations
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Configuration]

`sync(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.models.configuration.Configuration | None`
:   List all configurations
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Configuration

`sync_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[beamlit.models.configuration.Configuration]`
:   List all configurations
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Configuration]