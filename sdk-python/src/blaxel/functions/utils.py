"""
This module provides functionalities to integrate remote functions into Blaxel.
It includes classes for creating dynamic schemas based on function schema and managing remote toolkits.
"""

import pydantic
import typing_extensions as t

from blaxel.models import FunctionSchema
from blaxel.types import Unset

FIELD_DEFAULTS = {
    int: 0,
    float: 0.0,
    list: [],
    bool: False,
    str: "",
    type(None): None,
}

def _get_field_default(param: FunctionSchema) -> t.Any:
    """
    Determine the default value based on the OpenAPI type in the schema parameter.
    """
    return FIELD_DEFAULTS.get(param.type_)

def _get_field_type(param: FunctionSchema) -> type:
    """
    Determine the Python type based on the OpenAPI type in the schema parameter.

    Args:
        param (FunctionSchema): The parameter schema.

    Returns:
        type: The corresponding Python type.
    """
    # Default to string type
    field_type = str

    # Handle different OpenAPI types
    if not isinstance(param.type_, Unset) and param.type_ is not None:
        if param.type_ == "number":
            field_type = float
        elif param.type_ == "integer":
            field_type = int
        elif param.type_ == "boolean":
            field_type = bool
        elif param.type_ == "array":
            field_type = _get_array_field_type(param)
        elif param.type_ == "object":
            field_type = dict

    return field_type


def _get_array_field_type(param: FunctionSchema) -> type:
    """
    Determine the Python type for array fields based on the items schema.

    Args:
        param (FunctionSchema): The parameter schema with array type.

    Returns:
        type: The corresponding Python List type with appropriate item type.
    """
    # Handle array types with item definitions
    if not isinstance(param.items, Unset) and param.items is not None:
        item_type = str  # Default item type
        if not isinstance(param.items.type_, Unset) and param.items.type_ is not None:
            if param.items.type_ == "number":
                item_type = float
            elif param.items.type_ == "integer":
                item_type = int
            elif param.items.type_ == "boolean":
                item_type = bool
            elif param.items.type_ == "object":
                item_type = dict
        return t.List[item_type]
    else:
        return t.List[str]


def _create_field_definitions(schema: FunctionSchema) -> dict:
    """
    Create field definitions for the Pydantic model based on schema properties.

    Args:
        schema (FunctionSchema): The OpenAPI schema.

    Returns:
        dict: Field definitions for the Pydantic model.
    """
    field_definitions = {}

    # Handle properties from the schema
    if not isinstance(schema.properties, Unset) and schema.properties is not None:
        for key, param in schema.properties.additional_properties.items():
            # Get the field type
            field_type = _get_field_type(param)

            # Get description
            description = ""
            if not isinstance(param.description, Unset) and param.description is not None:
                description = param.description

            # Check if parameter is required
            is_required = False
            if not isinstance(schema.required, Unset) and schema.required is not None:
                is_required = key in schema.required

            # Create field with appropriate settings
            if is_required:
                field_definitions[key] = (
                    field_type,
                    pydantic.Field(description=description)
                )
            else:
                field_definitions[key] = (
                    t.Optional[field_type],
                    pydantic.Field(default=_get_field_default(param), description=description)
                )

    return field_definitions


def create_dynamic_schema(name: str, schema: FunctionSchema) -> type[pydantic.BaseModel]:
    """
    Creates a dynamic Pydantic schema based on OpenAPI function schema.

    Args:
        name (str): The name of the schema.
        schema (FunctionSchema): The OpenAPI schema of the function.

    Returns:
        type[pydantic.BaseModel]: The dynamically created Pydantic model.
    """
    # Create field definitions from schema properties
    field_definitions = _create_field_definitions(schema)

    # Create the Pydantic model
    model_name = f"{name}Schema"

    # Add model description if available
    model_config = {}
    if not isinstance(schema.description, Unset) and schema.description is not None:
        model_config["title"] = name
        model_config["description"] = schema.description

    # Create the model with Pydantic v2 compatible configuration
    if model_config:
        # Use model_config for Pydantic v2
        model = pydantic.create_model(
            model_name,
            **field_definitions,
            model_config=pydantic.ConfigDict(**model_config)
        )
    else:
        model = pydantic.create_model(
            model_name,
            **field_definitions
        )

    return model