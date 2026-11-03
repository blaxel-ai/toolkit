"""Exercise the built CLI's MCP setup in temporary homes, without login.

    python3 test/install/test_setup_mcp_agents.py ./blaxel
"""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


binary = Path(sys.argv[1]).absolute()
assert binary.is_file(), binary
agents = {
    "github-copilot": (".copilot/mcp-config.json", "mcpServers", "url", "local", "http"),
    "vscode": ("vscode/mcp.json", "servers", "url", "stdio", "http"),
    "amp": ("xdg/amp/settings.json", "amp.mcpServers", "url", None, None),
    "kiro-cli": (".kiro/settings/mcp.json", "mcpServers", "url", None, None),
    "qwen-code": (".qwen/settings.json", "mcpServers", "httpUrl", None, None),
    "cline": (".cline/data/settings/cline_mcp_settings.json", "mcpServers", "url", None, "streamableHttp"),
    "continue": (".continue/mcpServers/blaxel.json", "mcpServers", "url", "stdio", "http"),
    "junie": (".junie/mcp/mcp.json", "mcpServers", "url", None, None),
    "augment": (".augment/settings.json", "mcpServers", "url", None, "http"),
    "openhands": (".openhands/mcp.json", "mcpServers", "url", None, None),
    "crush": ("xdg/crush/crush.json", "mcp", "url", "stdio", "http"),
    "openclaw": (".openclaw/openclaw.json", "servers", "url", "stdio", "streamable-http"),
}


def environment(home):
    env = {key: value for key, value in os.environ.items() if not key.startswith(
        ("BL_", "COPILOT_", "AMP_", "GOOSE_", "OPENCLAW_", "CLINE_", "CONTINUE_", "CRUSH_", "XDG_")
    )}
    env.update(HOME=str(home), USERPROFILE=str(home), APPDATA=str(home / "roaming"),
               XDG_CONFIG_HOME=str(home / "xdg"), DO_NOT_TRACK="true")
    return env


def setup(home, agent=None, **env):
    try:
        executable = os.path.relpath(binary, home)
    except ValueError:  # Windows homes and build outputs can use different drives.
        executable = str(binary)
    args = [executable, "setup", "--yes", "--skip-skills", "--skip-login"]
    if agent:
        args += ["--agent", agent]
    return subprocess.run(args, cwd=home, env={**environment(home), **env}, text=True,
                          capture_output=True, timeout=30)


def refresh(home):
    """The setup refresh that bl upgrade and the install scripts run."""
    return setup(home, BL_INSTALL_REFRESH="true", BL_INSTALL_MCP="true")


def uninstall(home, *flags):
    return subprocess.run([str(binary), "uninstall", "--skip-skills", *flags], cwd=home, env=environment(home),
                          text=True, capture_output=True, stdin=subprocess.DEVNULL, timeout=30)


def servers(config, agent, container):
    return (config["mcp"] if agent == "openclaw" else config)[container]


for agent, (relative, container, url_key, local_type, remote_type) in agents.items():
    with tempfile.TemporaryDirectory(prefix="bl-setup-agents-") as temp:
        home = Path(temp).resolve()
        if agent == "vscode":
            relative = {"darwin": "Library/Application Support/Code/User/mcp.json",
                        "win32": "roaming/Code/User/mcp.json"}.get(sys.platform, "xdg/Code/User/mcp.json")
        file = home / relative
        file.parent.mkdir(parents=True)
        other = {"command": "my-server", "args": ["keep"], "env": {"MY_SETTING": "keep"}}
        config = {"theme": "dark", container: {"other": other}}
        if agent == "openclaw":
            config = {"theme": "dark", "mcp": {"setting": "keep", "servers": {"other": other}}}
        file.write_text(json.dumps(config), encoding="utf-8")
        result = setup(home)  # directory detection, through the actual command
        assert result.returncode == 0, (agent, result.stdout, result.stderr)
        written = json.loads(file.read_text(encoding="utf-8"))
        assert written["theme"] == "dark", written
        entries = servers(written, agent, container)
        assert entries["other"] == other, entries
        expected = {"command": str(binary), "args": ["mcp"]}
        type_key = "transport" if agent == "openclaw" else "type"
        if local_type:
            expected[type_key] = local_type
        if agent == "github-copilot":
            expected["tools"] = ["*"]
        assert entries["blaxel"] == expected, (agent, entries["blaxel"])
        docs = {url_key: "https://docs.blaxel.ai/mcp"}
        if remote_type:
            docs[type_key] = remote_type
        if agent == "github-copilot":
            docs["tools"] = ["*"]
        assert entries["blaxel-docs"] == docs, (agent, entries["blaxel-docs"])
        before = file.read_bytes()
        result = setup(home)
        assert result.returncode == 0, result.stdout + result.stderr
        assert before == file.read_bytes(), agent

        # bl uninstall lists both entries, removes them only with --yes, and
        # leaves the rest of the file as it was.
        for flags in [("--dry-run",), ()]:
            result = uninstall(home, *flags)
            assert result.returncode == (0 if flags else 1), (agent, flags, result.stdout, result.stderr)
            assert "blaxel, blaxel-docs" in result.stdout, (agent, result.stdout)
            assert before == file.read_bytes(), (agent, flags)
        result = uninstall(home, "--yes")
        assert result.returncode == 0, (agent, result.stdout, result.stderr)
        assert json.loads(file.read_text(encoding="utf-8")) == config, (agent, file.read_text(encoding="utf-8"))
        # Upgrades and install scripts then leave the servers out; bl setup
        # adds them back.
        uninstalled = file.read_bytes()
        result = refresh(home)
        assert result.returncode == 0, (agent, result.stdout, result.stderr)
        assert "MCP skipped" in result.stderr, (agent, result.stderr)
        assert file.read_bytes() == uninstalled, agent
        result = setup(home)
        assert result.returncode == 0, result.stdout + result.stderr
        written = json.loads(file.read_text(encoding="utf-8"))
        entries = servers(written, agent, container)

        # A user-selected workspace and environment remain byte-for-byte intact.
        entries["blaxel"]["args"] += ["--workspace", "my-workspace"]
        entries["blaxel"]["env"] = {"BL_ENV": "dev"}
        file.write_text(json.dumps(written), encoding="utf-8")
        before = file.read_bytes()
        result = setup(home, agent)
        assert result.returncode == 0, result.stdout + result.stderr
        assert before == file.read_bytes(), agent

        # bl uninstall keeps a server the user changed, and says so.
        result = uninstall(home, "--yes")
        assert result.returncode == 0, (agent, result.stdout, result.stderr)
        assert "blaxel: not what setup writes" in result.stdout, (agent, result.stdout)
        left = servers(json.loads(file.read_text(encoding="utf-8")), agent, container)
        assert left == {"other": other, "blaxel": entries["blaxel"]}, (agent, left)

        # Invalid JSON and comment-bearing JSON are never rewritten.
        for original in ['{"broken":', '{\n// keep my comment\n"' + container + '": {}\n}']:
            file.write_text(original, encoding="utf-8")
            result = setup(home, agent)
            assert result.returncode != 0, (agent, result.stdout)
            assert file.read_text(encoding="utf-8") == original, agent
        print(f"PASS {agent}: detected, merged, absolute executable, repeated, uninstalled, not refreshed, set up again, custom kept, malformed/commented")


with tempfile.TemporaryDirectory(prefix="bl-setup-goose-") as temp:
    home = Path(temp).resolve()
    relative = "roaming/Block/goose/config/config.yaml" if sys.platform == "win32" else "xdg/goose/config.yaml"
    file = home / relative
    file.parent.mkdir(parents=True)
    original = "# keep my settings\ntheme: dark # keep my theme\nextensions:\n  other:\n    name: other\n    type: stdio\n    cmd: my-server\n    enabled: false\n"
    file.write_text(original, encoding="utf-8")
    result = setup(home)
    assert result.returncode == 0, result.stdout + result.stderr
    written = file.read_text(encoding="utf-8")
    assert "# keep my settings" in written and "dark # keep my theme" in written, written
    assert "cmd: my-server" in written and "enabled: false" in written, written
    resource = written.split("  blaxel:\n", 1)[1].split("  blaxel-docs:", 1)[0]
    assert re.search(r"args:\s*\n\s*- mcp\n", resource), resource
    assert "type: stdio" in resource and str(binary) in resource, resource
    docs = written.split("  blaxel-docs:\n", 1)[1]
    assert "type: streamable_http" in docs and "uri: https://docs.blaxel.ai/mcp" in docs, docs
    result = setup(home)
    assert result.returncode == 0, result.stdout + result.stderr
    assert file.read_text(encoding="utf-8") == written
    result = uninstall(home, "--yes")
    assert result.returncode == 0, result.stdout + result.stderr
    assert file.read_text(encoding="utf-8") == original, file.read_text(encoding="utf-8")
    result = refresh(home)
    assert result.returncode == 0, result.stdout + result.stderr
    assert file.read_text(encoding="utf-8") == original, file.read_text(encoding="utf-8")
    for original in ["extensions: [broken", "extensions: []\n", "extensions: {}\n---\ntheme: dark\n"]:
        file.write_text(original, encoding="utf-8")
        result = setup(home, "goose")
        assert result.returncode != 0, result.stdout
        assert file.read_text(encoding="utf-8") == original
    print("PASS goose: detected, YAML comments/settings preserved, absolute executable, repeated, uninstalled, not refreshed, malformed")
