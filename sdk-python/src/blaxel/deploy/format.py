"""
This module provides utility functions to format deployment configurations into YAML-compatible strings.
It includes functions to convert arguments, parameters, dictionaries, and agent chains into properly formatted JSON or YAML strings.
"""

import ast


def arg_to_list(arg: ast.List):
    value = []
    for v in arg.elts:
        value.append(format_value(v))
    return value

def format_value(v):
    """
    Formats an AST node value into its Python equivalent.

    Args:
        v (ast.AST): The AST node to format.

    Returns:
        Any: The formatted Python value.
    """
    if isinstance(v, ast.Constant):
        return v.value
    elif isinstance(v, ast.Dict):
        return arg_to_dict(v)
    elif isinstance(v, ast.List):
        return arg_to_list(v)

def arg_to_dict(arg: ast.keyword):
    """
    Converts an AST keyword argument to a dictionary.

    Args:
        arg (ast.keyword): The AST keyword argument.

    Returns:
        dict: The resulting dictionary.
    """
    value = {}
    for k, v in zip(arg.keys, arg.values):
        if isinstance(k, ast.Constant):
            value[k.value] = format_value(v)
    return value