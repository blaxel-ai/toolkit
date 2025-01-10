import os
from logging import getLogger
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

from beamlit.api.agents import get_agent
from beamlit.api.functions import get_function
from beamlit.api.models import get_model
from beamlit.client import AuthenticatedClient
from beamlit.common.logger import init as init_logger
from beamlit.models import Agent, Function, Model
from beamlit.types import UNSET, Unset

global SETTINGS
SETTINGS = None

class SettingsAgent(BaseSettings):
    agent: Union[None, CompiledGraph, BaseChatModel] = None
    chain: Union[Unset, list[Agent]] = UNSET
    model: Union[Unset, Model] = UNSET
    functions: Union[Unset, list[Function]] = UNSET
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


def init_agent(
    client: AuthenticatedClient,
    destination: str = f"{os.getcwd()}/src/beamlit_generated.py",
):
    from beamlit.common.generate import generate

    logger = getLogger(__name__)
    settings = get_settings()
    # Init configuration from environment variables
    if settings.agent.functions or settings.agent.chain:
        return

    # Init configuration from beamlit control plane
    name = settings.name
    env = settings.environment

    agent = get_agent.sync(name, environment=env, client=client)
    if not agent:
        raise ValueError(f"Agent {name} not found")
    functions: list[Function] = []
    agents_chain: list[Agent] = []
    if agent.spec.functions:
        for function in agent.spec.functions:
            function = get_function.sync(function, environment=env, client=client)
            if function:
                functions.append(function)
        settings.agent.functions = functions

    if agent.spec.agentChain:
        for chain in agent.spec.agentChain:
            if chain.enabled:
                agentChain = get_agent.sync(chain.name, environment=env, client=client)
                if chain.description:
                    agentChain.spec.description = chain.description
                agents_chain.append(agentChain)
        settings.agent.chain = agents_chain
    if agent.spec.model:
        model = get_model.sync(agent.spec.model, environment=env, client=client)
        settings.agent.model = model

    content_generate = generate(destination, dry_run=True)
    compared_content = None
    if os.path.exists(destination):
        compared_content = open(destination).read()

    if not os.path.exists(destination) or (
        compared_content and content_generate != compared_content
    ):
        logger.info("Generating agent code")
        generate(destination)


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
