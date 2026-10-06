"""Start bl mcp the way a coding agent does after bl setup, from its MCP
configuration, and check it answers over stdio with nothing but JSON-RPC.

    python3 test/install/test_bl_mcp.py HOME

HOME is a home where bl setup ran, without a Blaxel login: the bridge must
still connect with an empty tool list and terminal-login instructions.
"""

import json
import os
import subprocess
import sys

home = sys.argv[1]
with open(os.path.join(home, ".cursor", "mcp.json"), encoding="utf-8") as file:
    entry = json.load(file)["mcpServers"]["blaxel"]
command = [entry["command"], *entry["args"]]
assert command[1:] == ["mcp"], f"setup wrote {command}"

messages = "\n".join([
    json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
        "protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "ci", "version": "1"}}}),
    json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}),
    json.dumps({"jsonrpc": "2.0", "id": 2, "method": "tools/list"}),
]) + "\n"
# No BL_* variable (API key, workspace, ...) may log the bridge in or redirect it.
environment = {key: value for key, value in os.environ.items() if not key.startswith("BL_")}
environment.update({"HOME": home, "USERPROFILE": home, "BL_INSTALL_SKILLS": "false"})
result = subprocess.run(command, input=messages, capture_output=True, text=True, timeout=60, env=environment)
assert result.returncode == 0, result.stderr

answers = {}
for line in result.stdout.splitlines():
    message = json.loads(line)  # every line is one JSON-RPC message
    answers[message["id"]] = message
assert sorted(answers) == [1, 2], result.stdout
assert "tools" in answers[1]["result"]["capabilities"], answers[1]
tools = [tool["name"] for tool in answers[2]["result"]["tools"]]
assert tools == [], tools
assert "bl login" in answers[1]["result"]["instructions"]
print(f"bl mcp answered from {command[0]}: {tools}")
