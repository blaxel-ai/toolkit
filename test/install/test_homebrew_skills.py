#!/usr/bin/env python3
"""Exercise a prebuilt CLI in temporary Homebrew kegs without invoking brew.

Usage: python3 test/install/test_homebrew_skills.py /tmp/blaxel [--real-network]
Skills are served from a local fixture archive. The optional network test
downloads the real skills from GitHub into a disposable home directory.
"""

import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile
import textwrap
import threading


SKILLS = {"blaxel-cli", "blaxel-sdk"}
PROJECT_CANARIES = {
    ".npmrc": "registry=https://invalid.example.invalid\nignore-scripts=false\n",
    "package.json": '{"name":"untouched-user-project","private":true}\n',
    "package-lock.json": '{"name":"untouched-user-project","lockfileVersion":3}\n',
    ".tool-versions": "nodejs 22.20.0\n",
}


def fixture_archive():
    """A small tarball laid out like github.com/blaxel-ai/agent-skills."""
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as archive:
        for name in sorted(SKILLS):
            body = f"---\nname: {name}\ndescription: Use {name}.\n---\n\n# {name}\n".encode()
            info = tarfile.TarInfo(f"agent-skills-HEAD/skills/{name}/SKILL.md")
            info.size, info.mode = len(body), 0o664
            archive.addfile(info, io.BytesIO(body))
    return buffer.getvalue()


class ArchiveServer:
    """Serves the fixture archive and counts downloads per installation."""

    def __init__(self):
        self.archive = fixture_archive()
        self.requests = {}
        self.failing = set()
        server = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                name = self.path.strip("/")
                server.requests[name] = server.requests.get(name, 0) + 1
                if name in server.failing:
                    self.send_error(503, "fixture outage")
                    return
                self.send_response(200)
                self.send_header("Content-Length", str(len(server.archive)))
                self.end_headers()
                self.wfile.write(server.archive)

            def log_message(self, *_):
                pass

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()

    def url(self, name):
        return f"http://127.0.0.1:{self.httpd.server_port}/{name}"

    def close(self):
        self.httpd.shutdown()


class Installation:
    def __init__(self, root, binary, server):
        self.root = root
        self.binary = binary
        self.server = server
        self.name = root.name
        self.home = root / "home"
        self.prefix = root / "prefix"
        self.tools = root / "tools"
        self.cwd = root / "work"
        for directory in (self.home, self.tools, self.cwd, self.prefix / "bin", self.prefix / "opt"):
            directory.mkdir(parents=True)
        # PATH holds only the disposable tools directory: the skills install
        # must not need Node.js, npm, git or anything else from the machine.
        self.env = {
            "HOME": str(self.home),
            "USERPROFILE": str(self.home),
            "XDG_CONFIG_HOME": str(self.home / ".config"),
            "XDG_CACHE_HOME": str(self.home / ".cache"),
            "XDG_DATA_HOME": str(self.home / ".local/share"),
            "CODEX_HOME": str(self.home / ".codex"),
            "CLAUDE_CONFIG_DIR": str(self.home / ".claude"),
            "BL_SKILLS_ARCHIVE_URL": server.url(self.name),
            "PATH": str(self.tools),
            "TMPDIR": str(root),
            "LANG": "en_US.UTF-8",
        }
        self.use_keg("1.0.0")

    def use_keg(self, version):
        keg = self.prefix / "Cellar/blaxel" / version
        (keg / "bin").mkdir(parents=True)
        shutil.copy2(self.binary, keg / "bin/blaxel")
        for link, target in ((self.prefix / "bin/bl", keg / "bin/blaxel"),
                             (self.prefix / "opt/blaxel", keg)):
            link.unlink(missing_ok=True)
            link.symlink_to(target)

    def fail_downloads(self):
        self.server.failing.add(self.name)

    def downloads(self):
        return self.server.requests.get(self.name, 0)

    def installed_skills(self):
        return {path.parent.name for path in (self.home / ".agents/skills").glob("*/SKILL.md")}

    def assert_installs(self, count):
        assert self.downloads() == count, (self.name, self.downloads())
        if count:
            assert self.installed_skills() == SKILLS, self.installed_skills()

    def marker(self, version="1.0.0"):
        return self.home / ".blaxel/setup/homebrew" / version

    def seed_project(self):
        for name, content in PROJECT_CANARIES.items():
            (self.cwd / name).write_text(content)

    def assert_project_unchanged(self):
        for name, content in PROJECT_CANARIES.items():
            assert (self.cwd / name).read_text() == content, name
        assert sorted(path.name for path in self.cwd.iterdir()) == sorted(PROJECT_CANARIES)

    def run(self, args=("--help",), extra_env=None, direct=False, check=True):
        binary = self.binary if direct else self.prefix / "bin/bl"
        result = subprocess.run(
            [str(binary), *args], cwd=self.cwd,
            env={**self.env, **(extra_env or {})}, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=150,
        )
        if not check:
            return result
        assert result.returncode == 0, (args, result.returncode, result.stdout, result.stderr)
        assert "Installing Blaxel skills" not in result.stdout, result.stdout
        return result


def fake_tests(root, binary, server):
    def installation(name):
        return Installation(root / name, binary, server)

    install = installation("no-tools")
    install.seed_project()
    install.run()
    install.assert_installs(1)
    install.assert_project_unchanged()
    print("PASS installs with an empty PATH and leaves project files untouched", flush=True)

    for index, args in enumerate((
        ("__complete", ""), ("__completeNoDesc", ""),
        ("--verbose", "__complete", ""), ("--verbose", "__completeNoDesc", ""),
    )):
        install = installation(f"completion-{index}")
        result = install.run(args)
        assert not install.downloads() and not install.marker().exists(), result
        install.run()
        install.assert_installs(1)
        assert install.marker().exists()
        print(f"PASS completion skips setup: bl {' '.join(args)}", flush=True)

    for index, args in enumerate((("--help",), ("--version",), ("help",), ("version",), ())):
        install = installation(f"command-{index}")
        first = install.run(args)
        install.assert_installs(1)
        assert install.marker().exists()
        # Users see one concise summary.
        assert "Blaxel setup refresh: 2 skills refreshed" in first.stderr, first.stderr
        assert len(first.stderr.splitlines()) == 1, first.stderr
        second = install.run(args)
        install.assert_installs(1)
        assert first.stdout == second.stdout
        assert "Installing Blaxel skills" not in second.stderr and "bl setup" not in second.stderr
        print(f"PASS first invocation and repeat: bl {' '.join(args)}", flush=True)

    install = installation("upgrade")
    install.run()
    install.use_keg("1.0.1")
    install.run()
    install.run()
    install.assert_installs(2)
    assert install.marker("1.0.1").exists()
    print("PASS new keg refresh", flush=True)

    install = installation("new-agent")
    install.run()
    (install.home / ".cursor").mkdir()
    install.use_keg("1.0.1")
    result = install.run()
    servers = json.loads((install.home / ".cursor/mcp.json").read_text())["mcpServers"]
    assert servers["blaxel"] == {"command": str(install.prefix / "bin/bl"), "args": ["mcp"]}, servers
    assert "blaxel-docs" in servers
    assert "MCP 2 added" in result.stderr, result.stderr
    print("PASS a Homebrew upgrade adds MCP to a newly detected agent", flush=True)

    for index, args in enumerate((("mcp",), ("--workspace", "test", "mcp"))):
        install = installation(f"mcp-{index}")
        messages = '\n'.join([
            '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}',
            '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
        ]) + '\n'
        result = subprocess.run([str(install.prefix / "bin/bl"), *args], input=messages,
                                cwd=install.cwd, env=install.env, capture_output=True, text=True, timeout=10)
        assert result.returncode == 0, result.stderr
        answers = [json.loads(line) for line in result.stdout.splitlines()]
        assert len(answers) == 2 and all(answer["jsonrpc"] == "2.0" for answer in answers), answers
        assert install.downloads() == 0 and not install.marker().exists()
        assert not (install.home / ".blaxel").exists()
    print("PASS bl mcp skips refresh and keeps stdout JSON-RPC, including global flags", flush=True)

    install = installation("upgrade-command")
    # Only these disposable scripts can service brew/git calls. The fake brew
    # changes both links while the old CLI process is still running.
    brew = install.tools / "brew"
    brew.write_text(
        f"#!{sys.executable}\n"
        "import pathlib, shutil, sys\n"
        f"prefix = pathlib.Path({str(install.prefix)!r})\n"
        "if sys.argv[1:] == ['--prefix']:\n"
        "    print(prefix)\n"
        "elif sys.argv[1:] == ['tap']:\n"
        "    print('blaxel-ai/blaxel')\n"
        "elif sys.argv[1] == '--repository':\n"
        "    print(prefix)\n"
        "elif sys.argv[1:] == ['upgrade', 'blaxel']:\n"
        "    keg = prefix / 'Cellar/blaxel/1.0.1'\n"
        "    (keg / 'bin').mkdir(parents=True)\n"
        "    shutil.copy2(prefix / 'bin/bl', keg / 'bin/blaxel')\n"
        "    for link, target in [(prefix / 'bin/bl', keg / 'bin/blaxel'), (prefix / 'opt/blaxel', keg)]:\n"
        "        link.unlink()\n"
        "        link.symlink_to(target)\n"
        "else:\n"
        "    sys.exit(2)\n"
    )
    brew.chmod(0o755)
    git = install.tools / "git"
    git.write_text(f"#!{sys.executable}\n")
    git.chmod(0o755)
    install.run(("upgrade",))
    install.assert_installs(1)
    assert not install.marker().exists() and install.marker("1.0.1").exists()
    install.run()
    install.assert_installs(1)
    print("PASS first bl upgrade installs once and marks the new keg", flush=True)

    # The replacement executable must run refresh, even after the old keg is removed.
    replacement = install.root / "replacement"
    receipt = install.root / "refresh.json"
    replacement.write_text(
        f"#!{sys.executable}\n"
        "import json, os, pathlib, sys\n"
        "if sys.argv[1:] == ['setup', '--refresh-check']:\n    print('blaxel-setup-refresh-v1')\n    sys.exit(0)\n"
        f"pathlib.Path({str(receipt)!r}).write_text(json.dumps({{'args': sys.argv[1:], 'refresh': os.getenv('BL_INSTALL_REFRESH'), 'skills': os.getenv('BL_INSTALL_SKILLS'), 'mcp': os.getenv('BL_INSTALL_MCP'), 'input': sys.stdin.read()}}))\n"
    )
    replacement.chmod(0o755)
    for force in (False, True):
        install.use_keg("2.0." + str(int(force)))
        brew.write_text(
            f"#!{sys.executable}\n"
            "import pathlib, shutil, sys\n"
            f"prefix = pathlib.Path({str(install.prefix)!r})\n"
            "if sys.argv[1:] == ['--prefix']:\n    print(prefix)\n"
            "elif sys.argv[1:] == ['tap']:\n    print('blaxel-ai/blaxel')\n"
            "elif sys.argv[1] == '--repository':\n    sys.exit(2)\n"
            f"elif sys.argv[1:] == {[('reinstall' if force else 'upgrade'), 'blaxel']!r}:\n"
            "    old = (prefix / 'opt/blaxel').resolve()\n"
            "    keg = prefix / 'Cellar/blaxel/3.0.0'\n"
            "    (keg / 'bin').mkdir(parents=True, exist_ok=True)\n"
            f"    shutil.copy2({str(replacement)!r}, keg / 'bin/blaxel')\n"
            "    for link, target in [(prefix / 'bin/bl', keg / 'bin/blaxel'), (prefix / 'opt/blaxel', keg)]:\n"
            "        link.unlink()\n        link.symlink_to(target)\n"
            "    shutil.rmtree(old)\n"
            "else:\n    sys.exit(2)\n"
        )
        args = ("--skip-version-warning", "upgrade", "--force") if force else ("upgrade",)
        code, output = run_in_terminal([str(install.prefix / "bin/bl"), *args], install.cwd,
                                      {**install.env, "BL_INSTALL_SKILLS": "false", "BL_INSTALL_MCP": "true"}, timeout=15)
        assert code == 0 and "enable tracking" not in output, output
        assert json.loads(receipt.read_text()) == {
            "args": ["setup", "--yes", "--skip-login"], "refresh": "true",
            "skills": "false", "mcp": "true", "input": "",
        }
        receipt.unlink()
    print("PASS terminal brew upgrade and reinstall invoke the new binary without prompts after old keg removal", flush=True)

    install = installation("manual-upgrade")
    bindir = install.home / "bin"
    bindir.mkdir()
    shutil.copy2(binary, bindir / "blaxel")
    manual_receipt = install.root / "manual-refresh.json"
    replacement.write_text(replacement.read_text().replace(str(receipt), str(manual_receipt)))
    curl = install.tools / "curl"
    script = 'cp -f ' + shlex.quote(str(replacement)) + ' "$BINDIR/blaxel"\n'
    curl.write_text(f"#!{sys.executable}\nimport sys\nsys.stdout.write({script!r})\n")
    curl.chmod(0o755)
    (install.tools / "sh").symlink_to("/bin/sh")
    (install.tools / "cp").symlink_to("/bin/cp")
    result = subprocess.run([str(bindir / "blaxel"), "upgrade", "--version", "1.2.3"],
                            cwd=install.cwd, env={**install.env, "BL_INSTALL_SKILLS": "true", "BL_INSTALL_MCP": "false"},
                            stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=30)
    assert result.returncode == 0, result.stdout + result.stderr
    assert json.loads(manual_receipt.read_text()) == {
        "args": ["setup", "--yes", "--skip-login"], "refresh": "true",
        "skills": "true", "mcp": "false", "input": "",
    }
    print("PASS curl upgrade invokes the replacement binary with the original opt-outs", flush=True)

    # A pre-contract CLI would interpret the old handoff as ordinary setup.
    # Its unknown-flag response must stop the caller before that side effect.
    for probe_output in ("", "ordinary setup"):
        shutil.copy2(binary, bindir / "blaxel")
        manual_receipt.unlink()
        replacement.write_text(
            f"#!{sys.executable}\n"
            "import json, os, pathlib, sys\n"
            "if sys.argv[1:] == ['setup', '--refresh-check']:\n"
            f"    print({probe_output!r})\n    sys.exit({0 if probe_output else 2})\n"
            f"pathlib.Path({str(manual_receipt)!r}).write_text('ordinary setup enabled tracking')\n"
        )
        result = subprocess.run([str(bindir / "blaxel"), "upgrade", "--version", "1.2.3"],
                                cwd=install.cwd, env={**install.env, "BL_INSTALL_SKILLS": "true", "BL_INSTALL_MCP": "false"},
                                stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=30)
        assert result.returncode == 0, result.stdout + result.stderr
        assert "does not support headless setup refresh" in result.stderr, result.stderr
        assert not manual_receipt.exists(), "unsupported CLI must never run ordinary setup"
        assert not (install.home / ".blaxel").exists(), "unsupported upgrade must leave consent and login state alone"
        # Keep unlinking uniform on the second iteration.
        manual_receipt.write_text("fixture")
    print("PASS unsupported replacement binaries leave setup and consent unchanged", flush=True)

    install = installation("installer-refresh")
    (install.home / ".cursor").mkdir()
    (install.tools / "bl").symlink_to(binary)
    for _ in range(2):
        result = subprocess.run(["/bin/sh", "-c", "BL_INSTALL_REFRESH=true bl setup --yes --skip-login </dev/null"],
                                cwd=install.cwd, env=install.env, capture_output=True, text=True, timeout=30)
        assert result.returncode == 0, result.stderr
        assert result.stdout == "" and len(result.stderr.splitlines()) == 1, result
        assert "Blaxel setup refresh:" in result.stderr
        assert "login" not in result.stderr and "error reports" not in result.stderr
    install.assert_installs(2)
    assert (install.home / ".cursor/mcp.json").is_file()
    assert not (install.home / ".blaxel").exists()
    print("PASS installer hook refreshes every invocation with one line and no authentication", flush=True)

    for name, skipped, enabled in (
        ("optout", {"BL_INSTALL_SKILLS": "false", "BL_INSTALL_MCP": "false"}, {}),
        ("ci", {"CI": "true"}, {"CI": "true", "BL_INSTALL_SKILLS": "true"}),
    ):
        install = installation(name)
        install.run(extra_env=skipped)
        assert not install.downloads() and not install.marker().exists()
        install.run(extra_env=enabled)
        install.assert_installs(1)
        assert install.marker().exists()
        print(f"PASS {name} skip then enable", flush=True)

    install = installation("failure")
    install.fail_downloads()
    result = install.run()
    assert "skills unavailable" in result.stderr and "retry with bl setup" in result.stderr, result.stderr
    assert len(result.stderr.splitlines()) == 1, result.stderr
    install.run()
    # A 503 is retried twice within the one attempt; later commands do not retry.
    assert install.downloads() == 3, install.downloads()
    assert install.marker().exists() and not install.installed_skills()
    print("PASS download failure is nonblocking and not repeated", flush=True)

    install = installation("explicit-first-run")
    install.fail_downloads()
    result = install.run(("skills", "install"), check=False)
    assert result.returncode != 0, result
    assert install.downloads() == 3
    assert result.stderr.count("503") == 1, result.stderr
    assert not install.marker().exists()
    install.run()
    assert install.downloads() == 6
    assert install.marker().exists()
    print("PASS explicit skills install leaves the automatic MCP refresh pending", flush=True)

    install = installation("agents")
    (install.home / ".claude").mkdir()
    result = install.run(("skills", "install"), direct=True)
    assert "for Claude Code (blaxel-cli, blaxel-sdk)" in result.stderr, result.stderr
    for skill in SKILLS:
        link = install.home / ".claude/skills" / skill
        assert link.is_symlink() and (link / "SKILL.md").is_file(), link
    print("PASS detected agents link to the shared skills", flush=True)

    install = installation("nonbrew")
    install.run(direct=True)
    assert not install.downloads() and not install.marker().exists()
    print("PASS non-Homebrew binary skips installation", flush=True)

    install = installation("explicit")
    install.run(("skills", "install"), direct=True,
                extra_env={"CI": "true", "BL_INSTALL_SKILLS": "false"})
    install.assert_installs(1)
    assert not install.marker().exists()
    print("PASS explicit skills install works outside Homebrew without authentication", flush=True)


FAKE_CURL = """#!/usr/bin/env python3
# Serves a local release for install.sh: the latest-release redirect, the
# archive and its checksums. Any other URL fails, so nothing reaches the network.
import os, shutil, sys
args, out, url, head = sys.argv[1:], None, None, False
for i, arg in enumerate(args):
    if arg == "-o":
        out = args[i + 1]
    elif arg == "--head":
        head = True
    elif arg.startswith("https://"):
        url = arg
release = os.environ["FAKE_RELEASE"]
if head and url.endswith("/releases/latest"):
    print("HTTP/2 302")
    print("location: https://github.com/blaxel-ai/toolkit/releases/tag/v9.9.9")
    sys.exit(0)
name = url.rsplit("/", 1)[-1] if url else ""
if "/releases/download/v9.9.9/" in (url or "") and os.path.exists(os.path.join(release, name)):
    shutil.copy(os.path.join(release, name), out)
    sys.exit(0)
print(f"curl: (22) no fixture for {url}", file=sys.stderr)
sys.exit(22)
"""


def fake_release(root, binary, corrupt=False):
    import hashlib
    release = root / "release"
    release.mkdir()
    stage = root / "stage"
    stage.mkdir()
    shutil.copy2(binary, stage / "blaxel")
    system = {"darwin": "Darwin", "linux": "Linux"}[sys.platform if sys.platform != "darwin" else "darwin"]
    machine = subprocess.run(["uname", "-m"], capture_output=True, text=True).stdout.strip()
    arch = {"x86_64": "x86_64", "amd64": "x86_64", "arm64": "arm64", "aarch64": "arm64"}[machine]
    archive = release / f"blaxel_{system}_{arch}.tar.gz"
    subprocess.run(["tar", "-czf", str(archive), "-C", str(stage), "blaxel"], check=True)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if corrupt:
        digest = "0" * 64
    (release / "blaxel_9.9.9_checksums.txt").write_text(f"{digest}  {archive.name}\n")
    return release


def run_in_terminal(argv, cwd, env, timeout=60):
    """Runs argv on a pseudo-terminal, as at a person's terminal, and returns
    its exit code and output. The code is None when it is still running after
    the timeout, for example because it is waiting at a prompt."""
    import os, pty, select, signal, time
    pid, fd = pty.fork()
    if pid == 0:
        try:
            os.chdir(cwd)
            os.execve(argv[0], argv, env)
        finally:
            os._exit(127)
    chunks, deadline = [], time.monotonic() + timeout
    while True:
        left = deadline - time.monotonic()
        if left <= 0:
            os.killpg(pid, signal.SIGKILL)
            os.waitpid(pid, 0)
            os.close(fd)
            return None, b"".join(chunks).decode(errors="replace")
        if not select.select([fd], [], [], left)[0]:
            continue
        try:
            data = os.read(fd, 65536)
        except OSError:  # EIO once the terminal closes
            data = b""
        if not data:
            break
        chunks.append(data)
    _, status = os.waitpid(pid, 0)
    os.close(fd)
    return os.waitstatus_to_exitcode(status), b"".join(chunks).decode(errors="replace")


def install_script_tests(root, binary, server):
    """Runs install.sh end to end against a local release, with no network."""
    script = Path(__file__).resolve().parents[2] / "install.sh"
    for name, corrupt, extra in (
        ("defaults", False, {}),
        ("bash", False, {"SHELL": "/bin/bash"}),
        ("no-path", False, {"BL_INSTALL_PATH": "false"}),
        ("corrupt", True, {}),
        ("no-sha256", False, {}),
        # bl upgrade before v0.1.119 runs the latest install.sh at the user's
        # terminal with only these variables.
        ("old-upgrade", False, {"BL_INSTALL_SKILLS": "false"}),
        # The one-liner at a person's terminal.
        ("terminal", False, {}),
    ):
        install = Installation(root / f"script-{name}", binary, server)
        release = fake_release(install.root, binary, corrupt)
        (install.tools / "curl").write_text(FAKE_CURL.replace("#!/usr/bin/env python3", f"#!{sys.executable}", 1))
        (install.tools / "curl").chmod(0o755)
        for tool in ("sh", "uname", "tr", "sed", "grep", "cut", "head", "mkdir", "mktemp", "tar", "gzip", "install",
                     "basename", "dirname", "awk", "id", "rm", "mv", "cat", "chmod", "shasum", "sha256sum", "env", "sleep"):
            found = shutil.which(tool, path="/usr/bin:/bin:/usr/sbin:/sbin")
            if name == "no-sha256" and tool in ("shasum", "sha256sum"):
                continue
            if found and not (install.tools / tool).exists():
                (install.tools / tool).symlink_to(found)
        (install.home / ".claude").mkdir()
        env = {**install.env, "FAKE_RELEASE": str(release), "SHELL": "/bin/zsh", "BL_INSTALL_SETUP": "true",
               "BL_INSTALL_TRACKING": "false", **extra}
        if name == "terminal":
            del env["BL_INSTALL_SETUP"]
            code, output = run_in_terminal(["/bin/sh", str(script)], install.cwd, env, timeout=15)
            assert code is None, "it hands the terminal to bl setup, which waits for the person:\n" + output
            assert "Blaxel CLI" in output and "Install" in output, output
            print("PASS install.sh at a terminal opens bl setup", flush=True)
            continue
        if name == "old-upgrade":
            del env["BL_INSTALL_SETUP"]
            env["BINDIR"] = str(install.home / ".local/bin")
            code, output = run_in_terminal(["/bin/sh", str(script)], install.cwd, env)
            assert code is not None, "an old bl upgrade must not stop at bl setup:\n" + output
            assert code == 0, output
            assert (install.home / ".local/bin/bl").is_file(), output
            assert "Blaxel setup" not in output and not install.installed_skills(), output
            assert "bl setup" in output, "it says how to finish setting up:\n" + output
            print("PASS an old bl upgrade at a terminal installs and only suggests bl setup", flush=True)
            continue
        result = subprocess.run(["/bin/sh", str(script)], cwd=install.cwd, env=env, stdin=subprocess.DEVNULL,
                                capture_output=True, text=True, timeout=120)
        output = result.stdout + result.stderr
        bindir = install.home / ".local/bin"
        if name == "corrupt":
            assert result.returncode != 0, output
            assert "does not match the release checksums" in output, output
            assert not (bindir / "bl").exists(), "a corrupt download is not installed"
            print("PASS install.sh refuses a download that fails its checksum", flush=True)
            continue
        if name == "no-sha256":
            assert result.returncode != 0, output
            assert "cannot verify the download" in output, output
            assert not (bindir / "bl").exists(), "nothing is installed unverified"
            print("PASS install.sh installs nothing it cannot verify", flush=True)
            continue
        assert result.returncode == 0, output
        assert (bindir / "bl").is_file() and (bindir / "blaxel").is_file(), output
        assert "Blaxel CLI" in output and "v9.9.9" in output and "verified" in output, output
        zshrc = (install.home / ".zshrc").read_text() if (install.home / ".zshrc").exists() else ""
        completion = install.home / ".zsh/completions/_bl"
        if name == "defaults":
            assert f'export PATH="{bindir}:$PATH"' in zshrc, zshrc
            assert completion.is_file() and "compdef" in completion.read_text(), "zsh completions are installed"
            assert "fpath=(" in zshrc
            # Without a terminal, BL_INSTALL_SETUP=true runs bl setup with the defaults.
            assert "Blaxel setup" in output and "Claude Code" in output, output
            assert install.installed_skills() == SKILLS, output
            assert (install.home / ".claude/skills/blaxel-cli").is_symlink()
            assert "error reports off" in output, output
            assert output.count("✦ 1 step left: bl login") == 1, output
            assert "Blaxel is ready" not in output and "enter close" not in output, output
            # Running it again does not add PATH twice.
            again = subprocess.run(["/bin/sh", str(script)], cwd=install.cwd, env=env, stdin=subprocess.DEVNULL,
                                   capture_output=True, text=True, timeout=120)
            assert again.returncode == 0, again.stdout + again.stderr
            assert (install.home / ".zshrc").read_text().count("export PATH=") == 1
            print("PASS install.sh installs, verifies, sets up the shell and hands off to bl setup", flush=True)
        elif name == "bash":
            # No ~/.bash_profile: PATH goes to ~/.bashrc, then bl setup runs.
            bashrc = (install.home / ".bashrc").read_text()
            assert f'export PATH="{bindir}:$PATH"' in bashrc, output
            assert (install.home / ".local/share/bash-completion/completions/bl").is_file(), output
            assert output.count("✦ 1 step left: bl login") == 1, output
            assert output.count("source ~/.bashrc") == 1, output
            print("PASS install.sh sets up bash through ~/.bashrc", flush=True)
        else:
            assert "export PATH" not in zshrc, zshrc
            hint = f'export PATH="{bindir}:$PATH"'
            assert hint in output, output
            # Run the printed hint in the user's shell: a quoted ~ stays literal in zsh.
            shell = shutil.which("zsh") or "/bin/sh"
            resolved = subprocess.run([shell, "-c", hint + "; command -v bl"], env=env,
                                      capture_output=True, text=True, check=True)
            assert resolved.stdout.strip() == str(bindir / "bl"), resolved.stdout
            print("PASS install.sh leaves the shell alone with BL_INSTALL_PATH=false", flush=True)


def real_test(root, binary, server):
    install = Installation(root / "real", binary, server)
    del install.env["BL_SKILLS_ARCHIVE_URL"]
    install.seed_project()
    first = install.run()
    assert "Blaxel setup refresh:" in first.stderr, first.stderr
    skills = install.installed_skills()
    assert SKILLS <= skills, skills
    lock = json.loads((install.home / ".agents/.skill-lock.json").read_text())
    assert SKILLS <= set(lock["skills"]), lock
    assert install.marker().exists()
    second = install.run()
    assert "Installing Blaxel skills" not in second.stderr, second.stderr
    assert first.stdout == second.stdout
    install.assert_project_unchanged()
    print(f"PASS real installation from GitHub with an empty PATH: {len(skills)} skills; second invocation skips setup", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="Prebuilt CLI binary outside a Homebrew keg")
    parser.add_argument("--real-network", action="store_true", help="Also install the real skills in an isolated home")
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    server = ArchiveServer()
    try:
        with tempfile.TemporaryDirectory(prefix="bl-homebrew-skills-") as temporary:
            root = Path(temporary).resolve()
            fake_tests(root, binary, server)
            install_script_tests(root, binary, server)
            if args.real_network:
                real_test(root, binary, server)
    finally:
        server.close()


if __name__ == "__main__":
    main()
