Module beamlit.api.models.create_model
======================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.model.Model) ‑> beamlit.models.model.Model | None`
:   Create model
    
     Creates a model.
    
    Args:
        body (Model): Logical object representing a model
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Model

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.model.Model) ‑> beamlit.types.Response[beamlit.models.model.Model]`
:   Create model
    
     Creates a model.
    
    Args:
        body (Model): Logical object representing a model
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Model]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.model.Model) ‑> beamlit.models.model.Model | None`
:   Create model
    
     Creates a model.
    
    Args:
        body (Model): Logical object representing a model
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Model

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.model.Model) ‑> beamlit.types.Response[beamlit.models.model.Model]`
:   Create model
    
     Creates a model.
    
    Args:
        body (Model): Logical object representing a model
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Model]