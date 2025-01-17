import os
from typing import Tuple, Type, Union

from langchain_core.language_models.chat_models import BaseChatModel
from langgraph.graph.graph import CompiledGraph
from pydantic import Field
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    YamlConfigSettingsSource,
)

from beamlit.common.logger import init as init_logger
from beamlit.models import Agent, Function, Model

global SETTINGS
SETTINGS = None

class SettingsAgent(BaseSettings):
    agent: Union[None, CompiledGraph] = None
    chain: Union[None, list[Agent]] = None
    model: Union[None, Model] = None
    functions: Union[None, list[Function]] = None
    functions_directory: str = Field(default="src/functions")
    chat_model: Union[None, BaseChatModel] = None
    module: str = Field(default="main.main")


class SettingsAuthenticationClient(BaseSettings):
    credentials: Union[None, str] = None


class SettingsAuthentication(BaseSettings):
    apiKey: Union[None, str] = None
    jwt: Union[None, str] = None
    client: SettingsAuthenticationClient = SettingsAuthenticationClient()


class SettingsServer(BaseSettings):
    module: str = Field(default="main.main")
    port: int = Field(default=80)
    host: str = Field(default="0.0.0.0")
    directory: str = Field(default="src")

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        yaml_file="beamlit.yaml",
        env_prefix="bl_",
        env_nested_delimiter="_",
        extra="ignore",
    )

    workspace: str
    environment: str = Field(default="production")
    remote: bool = Field(default=False)
    type: str = Field(default="agent")
    name: str = Field(default="beamlit-agent")
    base_url: str = Field(default="https://api.beamlit.com/v0")
    run_url: str = Field(default="https://run.beamlit.com")
    mcp_hub_url: str = Field(default="https://mcp-hub-server.beamlit.workers.com")
    registry_url: str = Field(default="https://us.registry.beamlit.com")
    log_level: str = Field(default="INFO")
    enable_opentelemetry: bool = Field(default=False)
    agent: SettingsAgent = SettingsAgent()
    server: SettingsServer = SettingsServer()
    authentication: SettingsAuthentication = SettingsAuthentication()

    def __init__(self, **data):
        super().__init__(**data)
        if os.getenv('BL_ENV') == 'dev':
            self.base_url = os.getenv('BL_BASE_URL') or "https://api.beamlit.dev/v0"
            self.run_url = os.getenv('BL_RUN_URL') or "https://run.beamlit.dev"
            self.mcp_hub_url = os.getenv('BL_MCP_HUB_URL') or "https://mcp-hub-server.beamlit.workers.dev"
            self.registry_url = os.getenv('BL_REGISTRY_URL') or "https://eu.registry.beamlit.dev"

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: Type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> Tuple[PydanticBaseSettingsSource, ...]:
        return (
            env_settings,
            dotenv_settings,
            file_secret_settings,
            YamlConfigSettingsSource(settings_cls),
            init_settings,
        )

def get_settings() -> Settings:
    return SETTINGS

def init() -> Settings:
    """Parse the beamlit.yaml file to get configurations."""
    from beamlit.authentication.credentials import current_context

    global SETTINGS

    context = current_context()
    kwargs = {}
    if context.workspace:
        kwargs["workspace"] = context.workspace
    if context.environment:
        kwargs["environment"] = context.environment

    SETTINGS = Settings(**kwargs)
    init_logger(SETTINGS.log_level)

    return SETTINGS
