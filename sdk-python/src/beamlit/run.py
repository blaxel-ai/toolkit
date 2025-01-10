import urllib.parse
from typing import Any

import requests

from beamlit.client import AuthenticatedClient
from beamlit.common import HTTPError, get_settings


class RunClient:
    def __init__(self, client: AuthenticatedClient):
        self.client = client

    def run(
        self,
        resource_type: str,
        resource_name: str,
        environment: str,
        method: str,
        path: str = "",
        headers: dict[str, str] | None = None,
        json: dict[str, Any] | None = None,
        data: str | None = None,
        params: dict[str, str] | None = None,
    ) -> requests.Response:
        settings = get_settings()
        headers = headers or {}
        params = params or {}

        # Build the path
        if path:
            path = f"{settings.workspace}/{resource_type}s/{resource_name}/{path}"
        else:
            path = f"{settings.workspace}/{resource_type}s/{resource_name}"

        client = self.client.get_httpx_client()
        url = urllib.parse.urljoin(settings.run_url, path)

        kwargs = {
            "headers": headers,
            "params": {"environment": environment, **params},
        }
        if data:
            kwargs["data"] = data
        if json:
            kwargs["json"] = json

        response = client.request(method, url, **kwargs)
        if response.status_code >= 400:
            raise HTTPError(response.status_code, response.text)
        return response
