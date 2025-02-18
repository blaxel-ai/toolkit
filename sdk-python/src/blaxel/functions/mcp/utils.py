"""
This module provides functionalities to interact with MCP (Multi-Client Platform) servers.
It includes classes for managing MCP clients, creating dynamic schemas, and integrating MCP tools into Blaxel.
"""
import pydantic
import typing_extensions as t
from pydantic.json_schema import JsonSchemaValue
from pydantic_core import core_schema as cs

TYPE_MAP = {
    "integer": int,
    "number": float,
    "array": list,
    "object": dict,
    "boolean": bool,
    "string": str,
    "null": type(None),
}

FIELD_DEFAULTS = {
    int: 0,
    float: 0.0,
    list: [],
    bool: False,
    str: "",
    type(None): None,
}

def configure_field(name: str, type_: dict[str, t.Any], required: list[str]) -> tuple[type, t.Any]:
    field_type = TYPE_MAP[type_["type"]]
    default_ = FIELD_DEFAULTS.get(field_type) if name not in required else ...
    return field_type, default_

def create_schema_model(name: str, schema: dict[str, t.Any]) -> type[pydantic.BaseModel]:
    # Create a new model class that returns our JSON schema.
    # LangChain requires a BaseModel class.
    class SchemaBase(pydantic.BaseModel):
        model_config = pydantic.ConfigDict(extra="allow")

        @t.override
        @classmethod
        def __get_pydantic_json_schema__(
            cls, core_schema: cs.CoreSchema, handler: pydantic.GetJsonSchemaHandler
        ) -> JsonSchemaValue:
            return schema

    # Since this langchain patch, we need to synthesize pydantic fields from the schema
    # https://github.com/langchain-ai/langchain/commit/033ac417609297369eb0525794d8b48a425b8b33
    required = schema.get("required", [])
    fields: dict[str, t.Any] = {
        name: configure_field(name, type_, required) for name, type_ in schema["properties"].items()
    }

    return pydantic.create_model(f"{name}Schema", __base__=SchemaBase, **fields)


