"""
This module provides classes and functions for handling various authentication methods,
including public access, API key authentication, client credentials, and bearer tokens.
It also includes utilities for creating authenticated clients and managing authentication headers.
"""

from dataclasses import dataclass
from typing import Dict, Generator

from blaxel.aimon.settings import Settings, get_settings
from httpx import Auth, Request, Response

from ..client import AuthenticatedClient
from .apikey import ApiKeyProvider
from .clientcredentials import ClientCredentials
from .credentials import (Credentials, current_context, load_credentials,
                          load_credentials_from_settings)
from .device_mode import BearerToken

global provider_singleton
provider_singleton = None

class PublicProvider(Auth):
    """
    A provider that allows public access without any authentication.
    """

    def auth_flow(self, request: Request) -> Generator[Request, Response, None]:
        """
        Processes the authentication flow for public access by yielding the request as-is.

        Parameters:
            request (Request): The HTTP request to process.

        Yields:
            Request: The unmodified request.
        """
        yield request


@dataclass
class RunClientWithCredentials:
    """
    A dataclass that holds credentials and workspace information for initializing an authenticated client.

    Attributes:
        credentials (Credentials): The credentials used for authentication.
        workspace (str): The name of the workspace.
        api_url (str): The base API URL.
        run_url (str): The run-specific URL.
    """
    credentials: Credentials
    workspace: str
    api_url: str = ""
    run_url: str = ""

    def __post_init__(self):
        """
        Post-initialization to set the API and run URLs from settings.
        """
        from ..common.settings import get_settings

        settings = get_settings()
        self.api_url = settings.base_url
        self.run_url = settings.run_url


def new_client_from_settings(settings: Settings):
    """
    Creates a new authenticated client using the provided settings.

    Parameters:
        settings (Settings): The settings containing authentication and workspace information.

    Returns:
        AuthenticatedClient: An instance of AuthenticatedClient configured with the provided settings.
    """
    credentials = load_credentials_from_settings(settings)

    client_config = RunClientWithCredentials(
        credentials=credentials,
        workspace=settings.workspace,
    )
    return new_client_with_credentials(client_config)


def new_client():
    """
    Creates a new authenticated client based on the current context and settings.

    Returns:
        AuthenticatedClient: An instance of AuthenticatedClient configured with the current context or settings.
    """
    settings = get_settings()
    context = current_context()
    if context.workspace and not settings.authentication.client.credentials:
        credentials = load_credentials(context.workspace)
        client_config = RunClientWithCredentials(
            credentials=credentials,
            workspace=context.workspace,
        )
    else:
        credentials = load_credentials_from_settings(settings)
        client_config = RunClientWithCredentials(
            credentials=credentials,
            workspace=settings.workspace,
        )
    return new_client_with_credentials(client_config)


def new_client_with_credentials(config: RunClientWithCredentials):
    """
    Creates a new authenticated client using the provided client configuration.

    Parameters:
        config (RunClientWithCredentials): The client configuration containing credentials and workspace information.

    Returns:
        AuthenticatedClient: An instance of AuthenticatedClient configured with the provided credentials.
    """
    provider: Auth = None
    if config.credentials.apiKey:
        provider = ApiKeyProvider(config.credentials, config.workspace)
    elif config.credentials.access_token:
        provider = BearerToken(config.credentials, config.workspace, config.api_url)
    elif config.credentials.client_credentials:
        provider = ClientCredentials(config.credentials, config.workspace, config.api_url)
    else:
        provider = PublicProvider()

    return AuthenticatedClient(base_url=config.api_url, provider=provider)


def get_authentication_headers(settings: Settings) -> Dict[str, str]:
    """
    Retrieves authentication headers based on the current context and settings.

    Parameters:
        settings (Settings): The settings containing authentication and workspace information.

    Returns:
        Dict[str, str]: A dictionary of authentication headers.
    """
    global provider_singleton

    if provider_singleton:
        return provider_singleton.get_headers()

    context = current_context()
    if context.workspace and not settings.authentication.client.credentials:
        credentials = load_credentials(context.workspace)
    else:
        settings = get_settings()
        credentials = load_credentials_from_settings(settings)

    config = RunClientWithCredentials(
        credentials=credentials,
        workspace=settings.workspace,
    )
    provider = None
    if config.credentials.apiKey:
        provider = ApiKeyProvider(config.credentials, config.workspace)
    elif config.credentials.access_token:
        provider = BearerToken(config.credentials, config.workspace, config.api_url)
    elif config.credentials.client_credentials:
        provider = ClientCredentials(config.credentials, config.workspace, config.api_url)

    if provider is None:
        return None
    provider_singleton = provider
    return provider.get_headers()
