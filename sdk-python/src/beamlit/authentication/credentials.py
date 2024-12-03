from dataclasses import dataclass
from pathlib import Path
from typing import List

import yaml


@dataclass
class Credentials:
    api_key: str = ""
    accessToken: str = ""
    refresh_token: str = ""
    expires_in: int = 0
    device_code: str = ""

@dataclass
class WorkspaceConfig:
    name: str
    credentials: Credentials


@dataclass
class ContextConfig:
    workspace: str = ""
    environment: str = ""


@dataclass
class Config:
    workspaces: List[WorkspaceConfig] = None
    context: ContextConfig = None

    def __post_init__(self):
        if self.workspaces is None:
            self.workspaces = []
        if self.context is None:
            self.context = ContextConfig()


def load_config() -> Config:
    config = Config()
    home_dir = Path.home()
    if home_dir:
        config_path = home_dir / ".beamlit" / "config.yaml"
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
                            config.context = ContextConfig(**data["context"])
            except yaml.YAMLError:
                # Invalid YAML, use empty config
                pass
    return config


def save_config(config: Config):
    data = {
        "workspaces": [
            {
                "name": ws.name,
                "credentials": {
                    "accessToken": ws.credentials.accessToken,
                    "api_key": ws.credentials.api_key
                }
            }
            for ws in config.workspaces
        ],
        "context": {
            "workspace": config.context.workspace,
            "environment": config.context.environment
        }
    }

    home_dir = Path.home()
    if not home_dir:
        raise RuntimeError("Could not determine home directory")

    config_dir = home_dir / ".beamlit"
    config_file = config_dir / "config.yaml"

    config_dir.mkdir(mode=0o700, parents=True, exist_ok=True)

    with open(config_file, "w", encoding="utf-8") as f:
        yaml.dump(data, f)


def list_workspaces() -> List[str]:
    config = load_config()
    return [workspace.name for workspace in config.workspaces]


def current_context() -> ContextConfig:
    config = load_config()
    return config.context


def set_current_workspace(workspace_name: str, environment: str):
    config = load_config()
    config.context.workspace = workspace_name
    config.context.environment = environment
    save_config(config)


def load_credentials(workspace_name: str) -> Credentials:
    config = load_config()
    for workspace in config.workspaces:
        if workspace.name == workspace_name:
            return workspace.credentials
    return Credentials()


def create_home_dir_if_missing():
    home_dir = Path.home()
    if not home_dir:
        print("Error getting home directory")
        return

    credentials_dir = home_dir / ".beamlit"
    credentials_file = credentials_dir / "credentials.json"

    if credentials_file.exists():
        print("You are already logged in. Enter a new API key to overwrite it.")
    else:
        try:
            credentials_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        except Exception as e:
            print(f"Error creating credentials directory: {e}")


def save_credentials(workspace_name: str, credentials: Credentials):
    create_home_dir_if_missing()
    if not credentials.accessToken and not credentials.api_key:
        print("No credentials to save, error")
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
    config = load_config()
    config.workspaces = [ws for ws in config.workspaces if ws.name != workspace_name]

    if config.context.workspace == workspace_name:
        config.context.workspace = ""

    save_config(config)
