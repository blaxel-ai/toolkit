import ast
import json
import os
import sys
import uuid
from logging import getLogger
from typing import Literal

from beamlit.common.settings import Settings, get_settings, init
from beamlit.models import (
    Agent,
    AgentSpec,
    EnvironmentMetadata,
    Flavor,
    Function,
    FunctionSpec,
    Runtime,
)

from .format import arg_to_dict, format_agent_chain, format_parameters
from .parser import Resource, get_description, get_parameters, get_resources

sys.path.insert(0, os.getcwd())
sys.path.insert(0, os.path.join(os.getcwd(), "src"))

random_id = str(uuid.uuid4())[:8]
def slugify(name: str) -> str:
    return name.lower().replace(" ", "-").replace("_", "-")

def get_runtime_image(type: str, name: str) -> str:
    settings = get_settings()
    registry_url = settings.registry_url.replace("https://", "").replace("http://", "")
    image = f"{registry_url}/{settings.workspace}/{type}s/{name}"
    # Generate a random ID to ensure unique image tags
    image = f"{image}:{random_id}"
    return image


def set_default_values(resource: Resource, deployment: Agent | Function):
    settings = get_settings()
    deployment.metadata.workspace = settings.workspace
    deployment.metadata.environment = settings.environment
    if not deployment.metadata.name:
        deployment.metadata.name = resource.name
    if not deployment.metadata.display_name:
        deployment.metadata.display_name = deployment.metadata.name
    if not deployment.spec.description:
        deployment.spec.description = get_description(None, resource)
    if not deployment.spec.runtime:
        deployment.spec.runtime = Runtime()
    if not deployment.spec.runtime.image:
        deployment.spec.runtime.image = get_runtime_image(resource.type, deployment.metadata.name)
    return deployment

def get_beamlit_deployment_from_resource(
    resource: Resource,
) -> Agent | Function:
    """
    Creates a deployment configuration from a resource.

    Args:
        resource (Resource): The resource to create a deployment for

    Returns:
        Agent | Function: The deployment configuration
    """
    for arg in resource.decorator.keywords:
        if arg.arg == "agent":
            if isinstance(arg.value, ast.Dict):
                value = arg_to_dict(arg.value)
                metadata = EnvironmentMetadata(**value.get("metadata", {}))
                spec = AgentSpec(**value.get("spec", {}))
                agent = Agent(metadata=metadata, spec=spec)
                return set_default_values(resource, agent)
        if arg.arg == "function":
            if isinstance(arg.value, ast.Dict):
                value = arg_to_dict(arg.value)
                metadata = EnvironmentMetadata(**value.get("metadata", {}))
                spec = FunctionSpec(**value.get("spec", {}))
                func = Function(metadata=metadata, spec=spec)
                if not func.spec.parameters:
                    func.spec.parameters = get_parameters(resource)
                return set_default_values(resource, func)
    if resource.type == "agent":
        agent = Agent(metadata=EnvironmentMetadata(), spec=AgentSpec())
        return set_default_values(resource, agent)
    if resource.type == "function":
        func = Function(metadata=EnvironmentMetadata(), spec=FunctionSpec())
        func.spec.parameters = get_parameters(resource)
        return set_default_values(resource, func)
    return None


def get_flavors(flavors: list[Flavor]) -> str:
    """
    Converts a list of Flavor objects to JSON string.

    Args:
        flavors (list[Flavor]): List of Flavor objects

    Returns:
        str: JSON string representation of flavors
    """
    if not flavors:
        return "[]"
    return json.dumps([flavor.to_dict() for flavor in flavors])

def get_agent_yaml(
    agent: Agent, functions: list[tuple[Resource, Function]], settings: Settings
) -> str:
    """
    Generates YAML configuration for an agent deployment.

    Args:
        agent (Agent): Agent deployment configuration
        functions (list[tuple[Resource, FunctionDeployment]]): List of associated functions
        settings (Settings): Application settings

    Returns:
        str: YAML configuration string
    """
    template = f"""
apiVersion: beamlit.com/v1alpha1
kind: Agent
metadata:
  name: {slugify(agent.metadata.name)}
  displayName: {agent.metadata.display_name or agent.metadata.name}
  environment: {settings.environment}
  workspace: {settings.workspace}
spec:
  enabled: true
  policies: [{", ".join(agent.spec.policies or [])}]
  functions: [{", ".join([f"{function.metadata.name}" for (_, function) in functions])}]
  agentChain: {format_agent_chain(agent.spec.agent_chain)}
  model: {agent.spec.model}
  runtime:
    image: {agent.spec.runtime.image}
"""
    if agent.spec.description:
        template += f"""    description: |
      {agent.spec.description}"""
    return template


def get_function_yaml(function: Function, settings: Settings) -> str:
    """
    Generates YAML configuration for a function deployment.

    Args:
        function (FunctionDeployment): Function deployment configuration
        settings (Settings): Application settings

    Returns:
        str: YAML configuration string
    """
    return f"""
apiVersion: beamlit.com/v1alpha1
kind: Function
metadata:
  name: {slugify(function.metadata.name)}
  displayName: {function.metadata.display_name or function.metadata.name}
  environment: {settings.environment}
spec:
  enabled: true
  policies: [{", ".join(function.spec.policies or [])}]
  description: |
    {function.spec.description}
  parameters: {format_parameters(function.spec.parameters)}
  runtime:
    image: {function.spec.runtime.image}
"""


def dockerfile(
    type: Literal["agent", "function"],
    resource: Resource,
    deployment: Agent | Function,
) -> str:
    """
    Generates Dockerfile content for agent or function deployment.

    Args:
        type (Literal["agent", "function"]): Type of deployment
        resource (Resource): Resource to be deployed
        deployment (Agent | Function): Resource configuration

    Returns:
        str: Dockerfile content
    """
    settings = get_settings()
    if type == "agent":
        module = f"{resource.module.__file__.split('/')[-1].replace('.py', '')}.{resource.module.__name__}"
    else:
        module = f"functions.{resource.module.__file__.split('/')[-1].replace('.py', '')}.{resource.module.__name__}"
    cmd = ["bl", "serve", "--port", "80", "--module", module]
    if type == "agent":
        cmd.append("--remote")
    cmd_str = ",".join([f'"{c}"' for c in cmd])
    return f"""
FROM python:3.12-slim

ARG UV_VERSION="latest"
RUN apt update && apt install -y curl

# Install uv.
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
RUN curl -fsSL https://raw.githubusercontent.com/beamlit/toolkit/main/install.sh | BINDIR=/bin sh
WORKDIR /beamlit

# Install the application dependencies.
COPY pyproject.toml /beamlit/pyproject.toml
COPY uv.lock /beamlit/uv.lock
RUN uv sync --no-cache

COPY README.m[d] /beamlit/README.md
COPY LICENS[E] /beamlit/LICENSE
COPY {settings.server.directory} /beamlit/src

ENV PATH="/beamlit/.venv/bin:$PATH"

ENTRYPOINT [{cmd_str}]
"""


def generate_beamlit_deployment(directory: str):
    """
    Generates all necessary deployment files for Beamlit agents and functions.

    Args:
        directory (str): Target directory for generated files

    Creates:
        - Agent and function YAML configurations
        - Dockerfiles for each deployment
        - Directory structure for agents and functions
    """
    settings = init()
    logger = getLogger(__name__)
    logger.info(f"Importing server module: {settings.server.module}")
    functions: list[tuple[Resource, Function]] = []
    agents: list[tuple[Resource, Agent]] = []
    for resource in get_resources("agent", settings.server.directory):
        agent = get_beamlit_deployment_from_resource(resource)
        if agent:
            agents.append((resource, agent))
    for resource in get_resources("function", settings.server.directory):
        function = get_beamlit_deployment_from_resource(resource)
        if function:
            functions.append((resource, function))

    agents_dir = os.path.join(directory, "agents")
    functions_dir = os.path.join(directory, "functions")
    # Create directory if it doesn't exist
    os.makedirs(agents_dir, exist_ok=True)
    os.makedirs(functions_dir, exist_ok=True)
    for resource, agent in agents:
        # write deployment file
        agent_dir = os.path.join(agents_dir, agent.metadata.name)
        os.makedirs(agent_dir, exist_ok=True)
        with open(os.path.join(agent_dir, "agent.yaml"), "w") as f:
            content = get_agent_yaml(agent, functions, settings)
            f.write(content)
        # write dockerfile for build
        with open(os.path.join(agent_dir, "Dockerfile"), "w") as f:
            content = dockerfile("agent", resource, agent)
            f.write(content)
        # write destination docker
        with open(os.path.join(agent_dir, "destination.txt"), "w") as f:
            content = agent.spec.runtime.image
            f.write(content)
    for resource, function in functions:
        # write deployment file
        function_dir = os.path.join(functions_dir, function.metadata.name)
        os.makedirs(function_dir, exist_ok=True)
        with open(os.path.join(function_dir, "function.yaml"), "w") as f:
            content = get_function_yaml(function, settings)
            f.write(content)
        # write dockerfile for build
        with open(os.path.join(function_dir, "Dockerfile"), "w") as f:
            content = dockerfile("function", resource, function)
            f.write(content)
        # write destination docker
        with open(os.path.join(function_dir, "destination.txt"), "w") as f:
            content = function.spec.runtime.image
            f.write(content)