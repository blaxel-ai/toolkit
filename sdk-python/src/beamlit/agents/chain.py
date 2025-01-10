import asyncio
import warnings
from dataclasses import dataclass
from typing import Callable

import pydantic
import typing_extensions as t
from langchain_core.tools.base import BaseTool, ToolException

from beamlit.api.agents import list_agents
from beamlit.authentication.authentication import AuthenticatedClient
from beamlit.common.settings import get_settings
from beamlit.models import Agent, AgentChain
from beamlit.run import RunClient


class ChainTool(BaseTool):
    """
    Chain tool
    """

    client: RunClient
    handle_tool_error: bool | str | Callable[[ToolException], str] | None = True

    @t.override
    def _run(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        warnings.warn(
            "Invoke this tool asynchronousely using `ainvoke`. This method exists only to satisfy standard tests.",
            stacklevel=1,
        )
        return asyncio.run(self._arun(*args, **kwargs))

    @t.override
    async def _arun(self, *args: t.Any, **kwargs: t.Any) -> t.Any:
        settings = get_settings()
        result = self.client.run(
            "agent",
            self.name,
            settings.environment,
            "POST",
            json=kwargs,
        )
        return result.text

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema

class ChainInput(pydantic.BaseModel):
    inputs: str

@dataclass
class ChainToolkit:
    """
    Remote toolkit
    """
    client: AuthenticatedClient
    chain: list[AgentChain]
    _chain: list[Agent] | None = None

    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    def initialize(self) -> None:
        """Initialize the session and retrieve tools list"""
        if self._chain is None:
            agents = list_agents.sync_detailed(
                client=self.client,
            ).parsed
            chain_enabled = [chain for chain in self.chain if chain.enabled]
            agents_chain = []
            for chain in chain_enabled:
                agent = [agent for agent in agents if agent.metadata.name == chain.name]
                if agent:
                    agent[0].spec.description = chain.description or agent[0].spec.description
                    agents_chain.append(agent[0])
            self._chain = agents_chain

    @t.override
    def get_tools(self) -> list[BaseTool]:
        if self._chain is None:
            raise RuntimeError("Must initialize the toolkit first")

        return [
            ChainTool(
                client=RunClient(self.client),
                name=agent.metadata.name,
                description=agent.spec.description or "",
                args_schema=ChainInput,
            )
            for agent in self._chain
        ]