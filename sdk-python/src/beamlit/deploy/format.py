
import ast

from beamlit.models import AgentChain, StoreFunctionParameter


def arg_to_list(arg: ast.List):
    value = []
    for v in arg.elts:
        value.append(format_value(v))
    return value

def format_value(v):
    if isinstance(v, ast.Constant):
        return v.value
    elif isinstance(v, ast.Dict):
        return arg_to_dict(v)
    elif isinstance(v, ast.List):
        return arg_to_list(v)

def arg_to_dict(arg: ast.keyword):
    value = {}
    for k, v in zip(arg.keys, arg.values):
        if isinstance(k, ast.Constant):
            value[k.value] = format_value(v)
    return value

def format_parameters(parameters: list[StoreFunctionParameter]) -> str:
    """
    Formats function parameters into YAML-compatible string.

    Args:
        parameters (list[StoreFunctionParameter]): List of parameter objects

    Returns:
        str: YAML-formatted string of parameters
    """
    if not parameters:
        return "[]"

    formatted = []
    for param in parameters:
        formatted.append(f"""
    - name: {param.name}
      type: {param.type_}
      required: {str(param.required).lower()}
      description: {param.description}""")

    return "\n".join(formatted)


def format_agent_chain(agentChain: list[AgentChain]) -> str:
    """
    Formats agent chain configuration into YAML-compatible string.

    Args:
        agentChain (list[AgentChain]): List of agent chain configurations

    Returns:
        str: YAML-formatted string of agent chain
    """
    if not agentChain:
        return "[]"
    formatted = []

    for agent in agentChain:
        formatted.append(f"""
      - agent: {agent.name}
        enabled: {agent.enabled}""")
        if agent.description:
            formatted.append(f"        description: {agent.description}")
    return "\n".join(formatted)