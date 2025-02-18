import asyncio
import os
import warnings
from dataclasses import dataclass
from typing import Callable

import pydantic
import typing_extensions as t
from langchain_core.tools.base import BaseTool, ToolException

from blaxel.aimon.settings import get_settings
from blaxel.api.agents import list_agents
from blaxel.authentication.authentication import AuthenticatedClient
from blaxel.models import Agent, AgentChain
from blaxel.run import RunClient


class ChainTool(BaseTool):
    """
    A tool that allows chaining of agent actions. Extends LangChain's BaseTool.
    """

    client: RunClient
    handle_tool_error: bool | str | Callable[[ToolException], str] | None = True
    _cloud: bool = False
    _service_name: str | None = None

    @t.override
    def _run(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        """
        Executes the tool synchronously.

        Parameters:
            *args (Any): Positional arguments.
            **kwargs (Any): Keyword arguments.

        Returns:
            Any: The result of the tool execution.
        """
        warnings.warn(
            "Invoke this tool asynchronousely using `ainvoke`. This method exists only to satisfy standard tests.",
            stacklevel=1,
        )
        return asyncio.run(self._arun(*args, **kwargs))

    @t.override
    async def _arun(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        """
        Executes the tool asynchronously.

        Parameters:
            *args (Any): Positional arguments.
            **kwargs (Any): Keyword arguments.

        Returns:
            Any: The result of the asynchronous tool execution.
        """
        result = self.client.run(
            "agent",
            self.name,
            "POST",
            cloud=self._cloud,
            service_name=self._service_name,
            json=kwargs,
        )
        return result.text

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        """
        Defines the schema for tool calls based on the provided argument schema.

        Returns:
            type[pydantic.BaseModel]: The Pydantic model representing the tool call schema.
        """
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema


class ChainInput(pydantic.BaseModel):
    """
    A Pydantic model representing the input structure for a chain.
    """

    inputs: str


@dataclass
class ChainToolkit:
    """
    A toolkit for managing and initializing a chain of agents.
    """

    client: AuthenticatedClient
    chain: list[AgentChain]
    _cloud: bool = False
    _service_name: str | None = None
    _chain: list[Agent] | None = None

    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    def initialize(self) -> None:
        """
        Initializes the toolkit by retrieving and configuring the list of agents based on the provided chains.

        Raises:
            RuntimeError: If initialization fails due to missing agents.
        """
        """Initialize the session and retrieve tools list"""
        settings = get_settings()
        self._cloud = settings.cloud
        if self._chain is None:
            agents = list_agents.sync_detailed(
                client=self.client,
            ).parsed
            chain_enabled = [chain for chain in self.chain if chain.enabled]
            agents_chain = []
            for chain in chain_enabled:
                agent = [agent for agent in agents if agent.metadata.name == chain.name]
                agent_name = agent[0].metadata.name.upper().replace("-", "_")
                if os.getenv(f"BL_AGENT_{agent_name}_SERVICE_NAME"):
                    self._service_name = os.getenv(f"BL_AGENT_{agent_name}_SERVICE_NAME")
                if agent:
                    agent[0].spec.prompt = chain.prompt or agent[0].spec.prompt
                    agent[0].spec.description = chain.description or agent[0].spec.description
                    agents_chain.append(agent[0])
            self._chain = agents_chain

    def get_tools(self) -> list[BaseTool]:
        """
        Retrieves a list of tools corresponding to the initialized agents.

        Returns:
            list[BaseTool]: A list of initialized `ChainTool` instances.

        Raises:
            RuntimeError: If the toolkit has not been initialized.
        """
        if self._chain is None:
            raise RuntimeError("Must initialize the toolkit first")

        return [
            ChainTool(
                client=RunClient(self.client),
                name=agent.metadata.name,
                description=agent.spec.description or agent.spec.prompt or "",
                args_schema=ChainInput,
                cloud=self._cloud,
                service_name=self._service_name,
            )
            for agent in self._chain
        ]