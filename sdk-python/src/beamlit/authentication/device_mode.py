import base64
import json
import time
from dataclasses import dataclass
from typing import Generator, Optional

from httpx import Auth, Request, Response, post


@dataclass
class DeviceLogin:
    client_id: str
    scope: str


@dataclass
class DeviceLoginResponse:
    client_id: str
    device_code: str
    user_code: str
    expires_in: int
    interval: int
    verification_uri: str
    verification_uri_complete: str


@dataclass
class DeviceLoginFinalizeRequest:
    grant_type: str
    client_id: str
    device_code: str


@dataclass
class DeviceLoginFinalizeResponse:
    accessToken: str
    expires_in: int
    refresh_token: str
    token_type: str


class BearerToken(Auth):
    def __init__(self, credentials, workspace_name: str, base_url: str):
        self.credentials = credentials
        self.workspace_name = workspace_name
        self.base_url = base_url

    def refresh_if_needed(self) -> Optional[Exception]:
        # Need to refresh token if expires in less than 10 minutes
        parts = self.credentials.accessToken.split('.')
        if len(parts) != 3:
            return Exception("Invalid JWT token format")

        try:
            claims_bytes = base64.urlsafe_b64decode(parts[1] + '=' * (-len(parts[1]) % 4))
            claims = json.loads(claims_bytes)
        except Exception as e:
            return Exception(f"Failed to decode/parse JWT claims: {str(e)}")

        exp_time = time.gmtime(claims['exp'])
        # Refresh if token expires in less than 10 minutes
        if time.time() + (10 * 60) > time.mktime(exp_time):
            return self.do_refresh()

        return None

    def auth_flow(self, request: Request) -> Generator[Request, Response, None]:
        err = self.refresh_if_needed()
        if err:
            return err

        request.headers['X-Beamlit-Authorization'] = f'Bearer {self.credentials.accessToken}'
        request.headers['X-Beamlit-Workspace'] = self.workspace_name
        yield request

    def do_refresh(self) -> Optional[Exception]:
        if not self.credentials.refresh_token:
            return Exception("No refresh token to refresh")

        url = f"{self.base_url}/oauth/token"
        refresh_data = {
            "grant_type": "refresh_token",
            "refresh_token": self.credentials.refresh_token,
            "device_code": self.credentials.device_code,
            "client_id": "beamlit"
        }

        try:
            print(refresh_data)
            response = post(
                url,
                json=refresh_data,
                headers={"Content-Type": "application/json"}
            )
            print(response.text)
            response.raise_for_status()
            finalize_response = DeviceLoginFinalizeResponse(**response.json())

            if not finalize_response.refresh_token:
                finalize_response.refresh_token = self.credentials.refresh_token

            from .credentials import Credentials, save_credentials
            creds = Credentials(
                accessToken=finalize_response.accessToken,
                refresh_token=finalize_response.refresh_token,
                expires_in=finalize_response.expires_in,
                device_code=self.credentials.device_code
            )

            self.credentials = creds
            save_credentials(self.workspace_name, creds)
            return None

        except Exception as e:
            return Exception(f"Failed to refresh token: {str(e)}")
