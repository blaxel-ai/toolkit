import ast
import importlib
import os
from dataclasses import dataclass
from logging import getLogger
from typing import Callable, Literal

from beamlit.models import StoreFunctionParameter


@dataclass
class Resource:
    type: Literal["agent", "function"]
    module: Callable
    name: str
    decorator: ast.Call
    func: Callable


def get_resources(from_decorator, dir) -> list[Resource]:
    """
    Scans through Python files in a directory to find functions decorated with a specific decorator.

    Args:
        from_decorator (str): The name of the decorator to search for
        dir (str): The directory to scan, defaults to "src"

    Returns:
        list[Resource]: List of Resource objects containing information about decorated functions
    """
    resources = []
    logger = getLogger(__name__)

    # Walk through all Python files in resources directory and subdirectories
    for root, _, files in os.walk(dir):
        for file in files:
            if file.endswith(".py"):
                file_path = os.path.join(root, file)
                # Read and compile the file content
                with open(file_path) as f:
                    try:
                        file_content = f.read()
                        # Parse the file content to find decorated resources
                        tree = ast.parse(file_content)

                        # Look for function definitions with decorators
                        for node in ast.walk(tree):
                            if (
                                not isinstance(node, ast.FunctionDef)
                                and not isinstance(node, ast.AsyncFunctionDef)
                            ) or len(node.decorator_list) == 0:
                                continue
                            decorator = node.decorator_list[0]

                            decorator_name = ""
                            if isinstance(decorator, ast.Call):
                                decorator_name = decorator.func.id
                            if isinstance(decorator, ast.Name):
                                decorator_name = decorator.id
                            if decorator_name == from_decorator:
                                # Get the function name and decorator name
                                func_name = node.name

                                # Import the module to get the actual function
                                spec = importlib.util.spec_from_file_location(func_name, file_path)
                                module = importlib.util.module_from_spec(spec)
                                spec.loader.exec_module(module)

                                # Check if kit=True in the decorator arguments

                                # Get the decorated function
                                if hasattr(module, func_name) and isinstance(decorator, ast.Call):
                                    resources.append(
                                        Resource(
                                            type=decorator_name,
                                            module=module,
                                            name=func_name,
                                            func=getattr(module, func_name),
                                            decorator=decorator,
                                        )
                                    )
                    except Exception as e:
                        logger.warning(f"Error processing {file_path}: {e!s} at line {e.__traceback__.tb_lineno}")
    return resources


def get_parameters(resource: Resource) -> list[StoreFunctionParameter]:
    """
    Extracts parameter information from a function's signature and docstring.

    Args:
        resource (Resource): The resource object containing the function to analyze

    Returns:
        list[StoreFunctionParameter]: List of parameter objects with name, type, required status, and description
    """
    parameters = []
    # Get function signature
    import inspect

    sig = inspect.signature(resource.func)
    # Get docstring for parameter descriptions
    docstring = inspect.getdoc(resource.func)
    param_descriptions = {}
    if docstring:
        # Parse docstring for parameter descriptions
        lines = docstring.split("\n")
        for line in lines:
            line = line.strip().lower()
            if line.startswith(":param "):
                # Extract parameter name and description
                param_line = line[7:].split(":", 1)
                if len(param_line) == 2:
                    param_name = param_line[0].strip()
                    param_desc = param_line[1].strip()
                    param_descriptions[param_name] = param_desc
    for name, param in sig.parameters.items():
        # Skip *args and **kwargs parameters
        if param.kind in (inspect.Parameter.VAR_POSITIONAL, inspect.Parameter.VAR_KEYWORD):
            continue

        param_type = "string"  # Default type
        type_mapping = {
            "str": "string",
            "int": "integer",
            "float": "number",
            "bool": "boolean",
            "list": "array",
            "dict": "object",
            "none": "null",
        }
        if param.annotation != inspect.Parameter.empty:
            # Map Python types to OpenAPI types
            if hasattr(param.annotation, "__name__"):
                param_type = param.annotation.__name__.lower()
            else:
                # Handle special types like Union, Optional etc
                param_type = str(param.annotation).lower()
        parameter = StoreFunctionParameter(
            name=name,
            type_=type_mapping.get(param_type, "string"),
            required=param.default == inspect.Parameter.empty,
            description=param_descriptions.get(name, f"Parameter {name}"),
        )
        parameters.append(parameter)

    return parameters


def get_description(description: str | None, resource: Resource) -> str:
    """
    Gets the description of a function from either a provided description or the function's docstring.

    Args:
        description (str | None): Optional explicit description
        resource (Resource): The resource object containing the function

    Returns:
        str: The function description
    """
    if description:
        return description
    doc = resource.func.__doc__
    if doc:
        # Split docstring into sections and get only the description part
        doc_lines = doc.split("\n")
        description_lines = []
        for line in doc_lines:
            line = line.strip()
            # Stop when we hit param/return sections
            if line.startswith(":param") or line.startswith(":return"):
                break
            if line:
                description_lines.append(line)
        return " ".join(description_lines).strip()
    return ""