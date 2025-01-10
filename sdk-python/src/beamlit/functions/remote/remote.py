import asyncio
import warnings
from dataclasses import dataclass
from typing import Callable

import pydantic
import typing_extensions as t
from langchain_core.tools.base import BaseTool, ToolException

from beamlit.api.functions import get_function
from beamlit.authentication.authentication import AuthenticatedClient
from beamlit.common.settings import get_settings
from beamlit.models import Function, StoreFunctionParameter
from beamlit.run import RunClient


def create_dynamic_schema(name: str, parameters: list[StoreFunctionParameter]) -> type[pydantic.BaseModel]:
    field_definitions = {}
    for param in parameters:
        field_type = str
        if param.type_ == "number":
            field_type = float
        elif param.type_ == "integer":
            field_type = int
        elif param.type_ == "boolean":
            field_type = bool

        field_definitions[param.name] = (
            field_type,
            pydantic.Field(description=param.description or "")
        )
    return pydantic.create_model(
        f"{name}Schema",
        **field_definitions
    )


class RemoteTool(BaseTool):
    """
    Remote tool
    """

    client: RunClient
    resource_name: str
    kit: bool = False
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
        body = {**kwargs}
        if self.kit:
            body["name"] = self.name
        result = self.client.run(
            "function",
            self.resource_name,
            settings.environment,
            "POST",
            json=body
        )
        return result.text

    @t.override
    @property
    def tool_call_schema(self) -> type[pydantic.BaseModel]:
        assert self.args_schema is not None  # noqa: S101
        return self.args_schema

@dataclass
class RemoteToolkit:
    """
    Remote toolkit
    """

    client: AuthenticatedClient
    function: str
    _function: Function | None = None

    model_config = pydantic.ConfigDict(arbitrary_types_allowed=True)

    def initialize(self) -> None:
        """Initialize the session and retrieve tools list"""
        if self._function is None:
            self._function = get_function.sync_detailed(self.function, client=self.client).parsed

    @t.override
    def get_tools(self) -> list[BaseTool]:
        if self._function is None:
            raise RuntimeError("Must initialize the toolkit first")

        if self._function.spec.kit:
            return [
                RemoteTool(
                    client=RunClient(self.client),
                    name=func.name,
                    resource_name=self._function.metadata.name,
                    kit=True,
                    description=func.description or "",
                    args_schema=create_dynamic_schema(func.name, func.parameters),
                )
                for func in self._function.spec.kit
            ]

        return [
            RemoteTool(
                client=RunClient(self.client),
                name=self._function.metadata.name,
                resource_name=self._function.metadata.name,
                description=self._function.spec.description or "",
                args_schema=create_dynamic_schema(
                    self._function.metadata.name,
                    self._function.spec.parameters
                ),
            )
        ]