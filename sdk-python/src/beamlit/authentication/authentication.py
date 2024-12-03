from dataclasses import dataclass
from typing import Generator

from httpx import Auth, Request, Response

from ..client import AuthenticatedClient
from .apikey import ApiKeyProvider
from .credentials import Credentials
from .device_mode import BearerToken


class PublicProvider(Auth):
    def auth_flow(self, request: Request) -> Generator[Request, Response, None]:
        yield request



@dataclass
class RunClientWithCredentials:
    credentials: Credentials
    workspace: str
    api_url: str = "https://api.beamlit.dev/v0"
    run_url: str = "https://run.beamlit.dev/v0"


def new_client_with_credentials(config: RunClientWithCredentials):
    provider: Auth = None
    if config.credentials.api_key:
        provider = ApiKeyProvider(config.credentials, config.workspace)
    elif config.credentials.accessToken:
        provider = BearerToken(config.credentials, config.workspace, config.api_url)
    else:
        provider = PublicProvider()

    return AuthenticatedClient(base_url=config.api_url, provider=provider)
