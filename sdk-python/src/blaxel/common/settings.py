"""
This module defines the configuration management system for Blaxel applications using Pydantic.
It includes dataclasses for various configuration aspects, such as agents, authentication, and server settings.
The module provides functions to initialize settings, load configurations from YAML files, and customize settings sources.
"""

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

from blaxel.common.logger import init as init_logger
from blaxel.models import Agent, Function, Model

global SETTINGS
SETTINGS = None

class SettingsAgent(BaseSettings):
    """
    Configuration settings for agents within Blaxel.

    Attributes:
        agent (Union[None, CompiledGraph]): The compiled agent graph.
        chain (Union[None, list[Agent]]): A list of agent chains.
        model (Union[None, Model]): The model configuration.
        functions (Union[None, list[Function]]): A list of functions available to agents.
        functions_directory (str): The directory path where agent functions are located.
        chat_model (Union[None, BaseChatModel]): The chat model used by agents.
        module (str): The module path to the main application.
    """
    agent: Union[None, CompiledGraph] = None
    chain: Union[None, list[Agent]] = None
    model: Union[None, Model] = None
    functions: Union[None, list[Function]] = None
    functions_directory: str = Field(default="src/functions")
    chat_model: Union[None, BaseChatModel] = None
    module: str = Field(default="main.main")


class SettingsAuthenticationClient(BaseSettings):
    """
    Configuration settings for authentication clients.

    Attributes:
        credentials (Union[None, str]): Client credentials for authentication.
    """
    credentials: Union[None, str] = None


class SettingsAuthentication(BaseSettings):
    apiKey: Union[None, str] = None
    jwt: Union[None, str] = None
    client: SettingsAuthenticationClient = SettingsAuthenticationClient()


class SettingsServer(BaseSettings):
    module: str = Field(default="agent.main")
    port: int = Field(default=80)
    host: str = Field(default="0.0.0.0")
    directory: str = Field(default="src")

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        yaml_file="blaxel.yaml",
        env_prefix="bl_",
        env_nested_delimiter="_",
        extra="ignore",
    )

    workspace: str
    remote: bool = Field(default=False)
    type: str = Field(default="agent")
    name: str = Field(default="blaxel-agent")
    base_url: str = Field(default="https://api.blaxel.ai/v0")
    app_url: str = Field(default="https://app.blaxel.ai")
    run_url: str = Field(default="https://run.blaxel.ai")
    run_internal_hostname: str = Field(default="internal.run.blaxel.ai")
    registry_url: str = Field(default="https://us.registry.blaxel.ai")
    log_level: str = Field(default="INFO")
    enable_opentelemetry: bool = Field(default=False)
    cloud: bool = Field(default=False)
    agent: SettingsAgent = SettingsAgent()
    server: SettingsServer = SettingsServer()
    authentication: SettingsAuthentication = SettingsAuthentication()
    deploy: bool = Field(default=False)

    def __init__(self, **data):
        super().__init__(**data)
        if os.getenv('BL_ENV') == 'dev':
            self.base_url = os.getenv('BL_BASE_URL') or "https://api.blaxel.dev/v0"
            self.run_url = os.getenv('BL_RUN_URL') or "https://run.blaxel.dev"
            self.registry_url = os.getenv('BL_REGISTRY_URL') or "https://eu.registry.blaxel.dev"
            self.app_url = os.getenv('BL_APP_URL') or "https://app.blaxel.dev"

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
    """
    Retrieves the current settings instance.

    Returns:
        Settings: The current settings configuration.
    """
    return SETTINGS

def init() -> Settings:
    """
    Initializes the settings by parsing the `blaxel.yaml` file and setting up logging.

    This function reads workspace configuration from the current context,
    initializes the global SETTINGS variable, and configures the logger based on the log level.

    Returns:
        Settings: The initialized settings configuration.
    """
    from blaxel.authentication.credentials import current_context

    global SETTINGS

    context = current_context()
    kwargs = {}
    if context.workspace:
        kwargs["workspace"] = context.workspace

    SETTINGS = Settings(**kwargs)
    init_logger(SETTINGS.log_level)

    return SETTINGS
