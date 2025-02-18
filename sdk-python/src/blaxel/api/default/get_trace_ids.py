from http import HTTPStatus
from typing import Any, Optional, Union

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.get_trace_ids_response_200 import GetTraceIdsResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    workload_id: Union[Unset, str] = UNSET,
    workload_type: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
    start_time: Union[Unset, str] = UNSET,
    end_time: Union[Unset, str] = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["workloadId"] = workload_id

    params["workloadType"] = workload_type

    params["limit"] = limit

    params["startTime"] = start_time

    params["endTime"] = end_time

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/traces",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: Union[AuthenticatedClient, Client], response: httpx.Response
) -> Optional[GetTraceIdsResponse200]:
    if response.status_code == 200:
        response_200 = GetTraceIdsResponse200.from_dict(response.json())

        return response_200
    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: Union[AuthenticatedClient, Client], response: httpx.Response
) -> Response[GetTraceIdsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    workload_id: Union[Unset, str] = UNSET,
    workload_type: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
    start_time: Union[Unset, str] = UNSET,
    end_time: Union[Unset, str] = UNSET,
) -> Response[GetTraceIdsResponse200]:
    """Get trace IDs

    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GetTraceIdsResponse200]
    """

    kwargs = _get_kwargs(
        workload_id=workload_id,
        workload_type=workload_type,
        limit=limit,
        start_time=start_time,
        end_time=end_time,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    workload_id: Union[Unset, str] = UNSET,
    workload_type: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
    start_time: Union[Unset, str] = UNSET,
    end_time: Union[Unset, str] = UNSET,
) -> Optional[GetTraceIdsResponse200]:
    """Get trace IDs

    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GetTraceIdsResponse200
    """

    return sync_detailed(
        client=client,
        workload_id=workload_id,
        workload_type=workload_type,
        limit=limit,
        start_time=start_time,
        end_time=end_time,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    workload_id: Union[Unset, str] = UNSET,
    workload_type: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
    start_time: Union[Unset, str] = UNSET,
    end_time: Union[Unset, str] = UNSET,
) -> Response[GetTraceIdsResponse200]:
    """Get trace IDs

    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GetTraceIdsResponse200]
    """

    kwargs = _get_kwargs(
        workload_id=workload_id,
        workload_type=workload_type,
        limit=limit,
        start_time=start_time,
        end_time=end_time,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    workload_id: Union[Unset, str] = UNSET,
    workload_type: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
    start_time: Union[Unset, str] = UNSET,
    end_time: Union[Unset, str] = UNSET,
) -> Optional[GetTraceIdsResponse200]:
    """Get trace IDs

    Args:
        workload_id (Union[Unset, str]):
        workload_type (Union[Unset, str]):
        limit (Union[Unset, str]):
        start_time (Union[Unset, str]):
        end_time (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GetTraceIdsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            workload_id=workload_id,
            workload_type=workload_type,
            limit=limit,
            start_time=start_time,
            end_time=end_time,
        )
    ).parsed
