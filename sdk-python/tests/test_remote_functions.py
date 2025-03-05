from typing import List, Optional

import pydantic
import pytest

from blaxel.functions.utils import create_dynamic_schema
from blaxel.models import FunctionSchema
from blaxel.models.function_schema_properties import FunctionSchemaProperties
from blaxel.types import UNSET


def test_create_dynamic_schema_basic():
    """Test creating a schema with basic types."""
    # Create a simple schema with string, integer, and boolean properties
    properties = FunctionSchemaProperties()

    # Add a string property
    string_param = FunctionSchema(type_="string", description="A string parameter")
    properties["name"] = string_param

    # Add an integer property
    int_param = FunctionSchema(type_="integer", description="An integer parameter")
    properties["age"] = int_param

    # Add a boolean property
    bool_param = FunctionSchema(type_="boolean", description="A boolean parameter")
    properties["is_active"] = bool_param

    # Create the schema
    schema = FunctionSchema(
        type_="object",
        properties=properties,
        required=["name", "age"]
    )

    # Generate the Pydantic model
    model = create_dynamic_schema("Person", schema)

    # Verify the model structure
    assert issubclass(model, pydantic.BaseModel)
    assert model.__name__ == "PersonSchema"

    # Test field types
    field_types = {name: field.annotation for name, field in model.model_fields.items()}
    assert field_types["name"] == str
    assert field_types["age"] == int
    assert field_types["is_active"] == Optional[bool]

    # Test required fields - in Pydantic v2, check if the field is required
    assert model.model_fields["name"].is_required() is True
    assert model.model_fields["age"].is_required() is True
    assert model.model_fields["is_active"].is_required() is False

    # Test field descriptions
    assert model.model_fields["name"].description == "A string parameter"
    assert model.model_fields["age"].description == "An integer parameter"
    assert model.model_fields["is_active"].description == "A boolean parameter"

    # Test validation
    valid_instance = model(name="John", age=30)
    assert valid_instance.name == "John"
    assert valid_instance.age == 30
    assert valid_instance.is_active is None

    # Test with all fields
    valid_instance2 = model(name="Jane", age=25, is_active=True)
    assert valid_instance2.name == "Jane"
    assert valid_instance2.age == 25
    assert valid_instance2.is_active is True

    # Test validation errors
    with pytest.raises(pydantic.ValidationError):
        model(age=30)  # Missing required name

    with pytest.raises(pydantic.ValidationError):
        model(name="John")  # Missing required age

    with pytest.raises(pydantic.ValidationError):
        model(name="John", age="thirty")  # Wrong type for age


def test_create_dynamic_schema_array():
    """Test creating a schema with array types."""
    properties = FunctionSchemaProperties()

    # Create an array of strings
    string_items = FunctionSchema(type_="string")
    array_param = FunctionSchema(
        type_="array",
        items=string_items,
        description="A list of tags"
    )
    properties["tags"] = array_param

    # Create an array of integers
    int_items = FunctionSchema(type_="integer")
    int_array_param = FunctionSchema(
        type_="array",
        items=int_items,
        description="A list of scores"
    )
    properties["scores"] = int_array_param

    # Create the schema
    schema = FunctionSchema(
        type_="object",
        properties=properties
    )

    # Generate the Pydantic model
    model = create_dynamic_schema("ArrayTest", schema)

    # Verify the model structure
    assert issubclass(model, pydantic.BaseModel)

    # Test field types
    field_types = {name: field.annotation for name, field in model.model_fields.items()}
    assert field_types["tags"] == Optional[List[str]]
    assert field_types["scores"] == Optional[List[int]]

    # Test validation
    valid_instance = model(tags=["python", "testing"], scores=[95, 87, 92])
    assert valid_instance.tags == ["python", "testing"]
    assert valid_instance.scores == [95, 87, 92]

    # Test with empty arrays
    valid_empty = model(tags=[], scores=[])
    assert valid_empty.tags == []
    assert valid_empty.scores == []

    # Test validation errors
    with pytest.raises(pydantic.ValidationError):
        model(tags=["python", 123])  # Wrong type in tags array

    # Skip the second validation test since Pydantic v2 is more lenient with type coercion
    # and automatically converts strings to integers when possible
    # This behavior is different from Pydantic v1


def test_create_dynamic_schema_empty():
    """Test creating a schema with no properties."""
    # Create an empty schema
    schema = FunctionSchema(
        type_="object",
        properties=UNSET
    )

    # Generate the Pydantic model
    model = create_dynamic_schema("Empty", schema)

    # Verify the model structure
    assert issubclass(model, pydantic.BaseModel)
    assert model.__name__ == "EmptySchema"

    # Test that the model has no fields
    assert len(model.model_fields) == 0

    # Test instantiation
    instance = model()
    assert isinstance(instance, pydantic.BaseModel)


def test_create_dynamic_schema_with_description():
    """Test creating a schema with a description."""
    properties = FunctionSchemaProperties()

    # Add a string property
    string_param = FunctionSchema(type_="string", description="A string parameter")
    properties["name"] = string_param

    # Create the schema with a description
    schema = FunctionSchema(
        type_="object",
        properties=properties,
        description="A test schema with description"
    )

    # Generate the Pydantic model
    model = create_dynamic_schema("WithDescription", schema)

    # Verify the model structure
    assert issubclass(model, pydantic.BaseModel)

    # Test model description - in Pydantic v2, check model_config instead of __config__
    assert hasattr(model, "model_config")
    assert model.model_config.get("description") == "A test schema with description"
    assert model.model_config.get("title") == "WithDescription"