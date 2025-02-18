"""
This module provides the ApiKeyProvider class, which handles API key-based authentication for Blaxel.
"""

from typing import Generator

from httpx import Auth, Request, Response


class ApiKeyProvider(Auth):
    """
    A provider that authenticates requests using an API key.
    """

    def __init__(self, credentials, workspace_name: str):
        """
        Initializes the ApiKeyProvider with the given credentials and workspace name.

        Parameters:
            credentials: Credentials containing the API key.
            workspace_name (str): The name of the workspace.
        """
        self.credentials = credentials
        self.workspace_name = workspace_name

    def get_headers(self):
        """
        Retrieves the authentication headers containing the API key and workspace information.

        Returns:
            dict: A dictionary of headers with API key and workspace.
        """
        return {
            "X-Blaxel-Api-Key": self.credentials.apiKey,
            "X-Blaxel-Workspace": self.workspace_name,
        }

    def auth_flow(self, request: Request) -> Generator[Request, Response, None]:
        """
        Authenticates the request by adding API key and workspace headers.

        Parameters:
            request (Request): The HTTP request to authenticate.

        Yields:
            Request: The authenticated request.
        """
        request.headers["X-Blaxel-Api-Key"] = self.credentials.apiKey
        request.headers["X-Blaxel-Workspace"] = self.workspace_name
        yield request
