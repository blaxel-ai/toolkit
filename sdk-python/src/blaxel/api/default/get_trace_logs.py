from http import HTTPStatus
from typing import Any, Optional, Union

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.get_trace_logs_response_200 import GetTraceLogsResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    trace_id: str,
    *,
    span_id: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
) -> dict[str, Any]:
    params: dict[str, Any] = {}

    params["spanId"] = span_id

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": f"/traces/{trace_id}/logs",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: Union[AuthenticatedClient, Client], response: httpx.Response
) -> Optional[GetTraceLogsResponse200]:
    if response.status_code == 200:
        response_200 = GetTraceLogsResponse200.from_dict(response.json())

        return response_200
    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: Union[AuthenticatedClient, Client], response: httpx.Response
) -> Response[GetTraceLogsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    trace_id: str,
    *,
    client: AuthenticatedClient,
    span_id: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
) -> Response[GetTraceLogsResponse200]:
    """Get trace logs

    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GetTraceLogsResponse200]
    """

    kwargs = _get_kwargs(
        trace_id=trace_id,
        span_id=span_id,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    trace_id: str,
    *,
    client: AuthenticatedClient,
    span_id: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
) -> Optional[GetTraceLogsResponse200]:
    """Get trace logs

    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GetTraceLogsResponse200
    """

    return sync_detailed(
        trace_id=trace_id,
        client=client,
        span_id=span_id,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    trace_id: str,
    *,
    client: AuthenticatedClient,
    span_id: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
) -> Response[GetTraceLogsResponse200]:
    """Get trace logs

    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[GetTraceLogsResponse200]
    """

    kwargs = _get_kwargs(
        trace_id=trace_id,
        span_id=span_id,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    trace_id: str,
    *,
    client: AuthenticatedClient,
    span_id: Union[Unset, str] = UNSET,
    limit: Union[Unset, str] = UNSET,
) -> Optional[GetTraceLogsResponse200]:
    """Get trace logs

    Args:
        trace_id (str):
        span_id (Union[Unset, str]):
        limit (Union[Unset, str]):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        GetTraceLogsResponse200
    """

    return (
        await asyncio_detailed(
            trace_id=trace_id,
            client=client,
            span_id=span_id,
            limit=limit,
        )
    ).parsed
