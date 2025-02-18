Module beamlit.functions.mcp.utils
==================================
This module provides functionalities to interact with MCP (Multi-Client Platform) servers.
It includes classes for managing MCP clients, creating dynamic schemas, and integrating MCP tools into Beamlit.

Functions
---------

`configure_field(name: str, type_: dict[str, typing.Any], required: list[str]) ‑> tuple[type, typing.Any]`
:   

`create_schema_model(name: str, schema: dict[str, typing.Any]) ‑> type[pydantic.main.BaseModel]`
: