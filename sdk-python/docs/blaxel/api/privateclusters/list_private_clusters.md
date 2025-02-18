Module beamlit.api.privateclusters.list_private_clusters
========================================================

Functions
---------

`asyncio(*, client: beamlit.client.AuthenticatedClient) ‑> Any | list[beamlit.models.private_cluster.PrivateCluster] | None`
:   List all private clusters
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, list['PrivateCluster']]

`asyncio_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[Any | list[beamlit.models.private_cluster.PrivateCluster]]`
:   List all private clusters
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, list['PrivateCluster']]]

`sync(*, client: beamlit.client.AuthenticatedClient) ‑> Any | list[beamlit.models.private_cluster.PrivateCluster] | None`
:   List all private clusters
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Union[Any, list['PrivateCluster']]

`sync_detailed(*, client: beamlit.client.AuthenticatedClient) ‑> beamlit.types.Response[Any | list[beamlit.models.private_cluster.PrivateCluster]]`
:   List all private clusters
    
    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.
    
    Returns:
        Response[Union[Any, list['PrivateCluster']]]