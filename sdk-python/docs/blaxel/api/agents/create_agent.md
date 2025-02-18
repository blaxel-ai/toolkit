Module beamlit.api.agents.create_agent
======================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.agent.Agent) ‑> beamlit.models.agent.Agent | None`
:   Create agent by name
    
    Args:
        body (Agent): Agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Agent

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.agent.Agent) ‑> beamlit.types.Response[beamlit.models.agent.Agent]`
:   Create agent by name
    
    Args:
        body (Agent): Agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Agent]

`sync(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.agent.Agent) ‑> beamlit.models.agent.Agent | None`
:   Create agent by name
    
    Args:
        body (Agent): Agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Agent

`sync_detailed(*, client: beamlit.client.AuthenticatedClient, body: beamlit.models.agent.Agent) ‑> beamlit.types.Response[beamlit.models.agent.Agent]`
:   Create agent by name
    
    Args:
        body (Agent): Agent
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Agent]