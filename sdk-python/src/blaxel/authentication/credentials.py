"""
This module provides classes and functions for managing credentials and workspace configurations.
It includes functionalities to load, save, and manage authentication credentials, as well as to handle
workspace contexts and configurations.
"""

from dataclasses import dataclass
from logging import getLogger
from pathlib import Path
from typing import List

import yaml

from blaxel.aimon.settings import Settings

logger = getLogger(__name__)


@dataclass
class Credentials:
    """
    A dataclass representing user credentials for authentication.

    Attributes:
        apiKey (str): The API key.
        access_token (str): The access token.
        refresh_token (str): The refresh token.
        expires_in (int): Token expiration time in seconds.
        device_code (str): The device code for device authentication.
        client_credentials (str): The client credentials for authentication.
    """
    apiKey: str = ""
    access_token: str = ""
    refresh_token: str = ""
    expires_in: int = 0
    device_code: str = ""
    client_credentials: str = ""


@dataclass
class WorkspaceConfig:
    """
    A dataclass representing the configuration for a workspace.

    Attributes:
        name (str): The name of the workspace.
        credentials (Credentials): The credentials associated with the workspace.
    """
    name: str
    credentials: Credentials


@dataclass
class ContextConfig:
    """
    A dataclass representing the current context configuration.

    Attributes:
        workspace (str): The name of the current workspace.
    """
    workspace: str = ""


@dataclass
class Config:
    """
    A dataclass representing the overall configuration, including workspaces and context.

    Attributes:
        workspaces (List[WorkspaceConfig]): A list of workspace configurations.
        context (ContextConfig): The current context configuration.
    """
    workspaces: List[WorkspaceConfig] = None
    context: ContextConfig = None

    def __post_init__(self):
        """
        Post-initialization to ensure workspaces and context are initialized.
        """
        if self.workspaces is None:
            self.workspaces = []
        if self.context is None:
            self.context = ContextConfig()

    def to_json(self) -> dict:
        """
        Converts the Config dataclass to a JSON-compatible dictionary.

        Returns:
            dict: The JSON representation of the configuration.
        """
        return {
            "workspaces": [
                {
                    "name": ws.name,
                    "credentials": {
                        "apiKey": ws.credentials.apiKey,
                        "access_token": ws.credentials.access_token,
                        "refresh_token": ws.credentials.refresh_token,
                        "expires_in": ws.credentials.expires_in,
                        "device_code": ws.credentials.device_code,
                        "client_credentials": ws.credentials.client_credentials,
                    },
                }
                for ws in self.workspaces
            ],
            "context": {
                "workspace": self.context.workspace,
            },
        }


def load_config() -> Config:
    """
    Loads the configuration from the user's home directory.

    Returns:
        Config: The loaded configuration.
    """
    config = Config()
    home_dir = Path.home()
    if home_dir:
        config_path = home_dir / ".blaxel" / "config.yaml"
        if config_path.exists():
            try:
                with open(config_path) as f:
                    data = yaml.safe_load(f)
                    if data:
                        workspaces = []
                        for ws in data.get("workspaces", []):
                            creds = Credentials(**ws.get("credentials", {}))
                            workspaces.append(WorkspaceConfig(name=ws["name"], credentials=creds))
                        config.workspaces = workspaces
                        if "context" in data:
                            config.context = ContextConfig(workspace=data["context"].get("workspace", ""))
            except yaml.YAMLError:
                # Invalid YAML, use empty config
                pass
    return config


def save_config(config: Config):
    """
    Saves the provided configuration to the user's home directory.

    Parameters:
        config (Config): The configuration to save.

    Raises:
        RuntimeError: If the home directory cannot be determined.
    """
    home_dir = Path.home()
    if not home_dir:
        raise RuntimeError("Could not determine home directory")

    config_dir = home_dir / ".blaxel"
    config_file = config_dir / "config.yaml"

    config_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    with open(config_file, "w", encoding="utf-8") as f:
        yaml.dump(config.to_json(), f)


def list_workspaces() -> List[str]:
    """
    Lists all available workspace names from the configuration.

    Returns:
        List[str]: A list of workspace names.
    """
    config = load_config()
    return [workspace.name for workspace in config.workspaces]


def current_context() -> ContextConfig:
    """
    Retrieves the current context configuration.

    Returns:
        ContextConfig: The current context configuration.
    """
    config = load_config()
    return config.context


def set_current_workspace(workspace_name: str):
    """
    Sets the current workspace in the configuration.

    Parameters:
        workspace_name (str): The name of the workspace to set as current.
    """
    config = load_config()
    config.context.workspace = workspace_name
    save_config(config)


def load_credentials(workspace_name: str) -> Credentials:
    """
    Loads credentials for the specified workspace.

    Parameters:
        workspace_name (str): The name of the workspace whose credentials are to be loaded.

    Returns:
        Credentials: The credentials associated with the workspace. Returns empty credentials if not found.
    """
    config = load_config()
    for workspace in config.workspaces:
        if workspace.name == workspace_name:
            return workspace.credentials
    return Credentials()


def load_credentials_from_settings(settings: Settings) -> Credentials:
    """
    Loads credentials from the provided settings.

    Parameters:
        settings (Settings): The settings containing authentication information.

    Returns:
        Credentials: The loaded credentials from settings.
    """
    return Credentials(
        apiKey=settings.authentication.apiKey,
        client_credentials=settings.authentication.client.credentials,
    )


def create_home_dir_if_missing():
    """
    Creates the Blaxel home directory if it does not exist.

    Logs a warning if credentials already exist or an error if directory creation fails.
    """
    home_dir = Path.home()
    if not home_dir:
        logger.error("Error getting home directory")
        return

    credentials_dir = home_dir / ".blaxel"
    credentials_file = credentials_dir / "credentials.json"

    if credentials_file.exists():
        logger.warning("You are already logged in. Enter a new API key to overwrite it.")
    else:
        try:
            credentials_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        except Exception as e:
            logger.error(f"Error creating credentials directory: {e}")


def save_credentials(workspace_name: str, credentials: Credentials):
    """
    Saves the provided credentials for the specified workspace.

    Parameters:
        workspace_name (str): The name of the workspace.
        credentials (Credentials): The credentials to save.
    """
    create_home_dir_if_missing()
    if not credentials.access_token and not credentials.apiKey:
        logger.info("No credentials to save, error")
        return

    config = load_config()
    found = False

    for i, workspace in enumerate(config.workspaces):
        if workspace.name == workspace_name:
            config.workspaces[i].credentials = credentials
            found = True
            break

    if not found:
        config.workspaces.append(WorkspaceConfig(name=workspace_name, credentials=credentials))

    save_config(config)


def clear_credentials(workspace_name: str):
    """
    Clears the credentials for the specified workspace.

    Parameters:
        workspace_name (str): The name of the workspace whose credentials are to be cleared.
    """
    config = load_config()
    config.workspaces = [ws for ws in config.workspaces if ws.name != workspace_name]

    if config.context.workspace == workspace_name:
        config.context.workspace = ""

    save_config(config)
