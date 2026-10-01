#!/usr/bin/env python3
"""Exercise a prebuilt CLI in temporary Homebrew kegs without invoking brew.

Usage: python3 test/install/test_homebrew_skills.py /tmp/blaxel [--real-npm]
The optional network test installs real skills into a disposable home directory.
"""

import argparse
import base64
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import textwrap


# The fixture home has no agent directories, so only the shared directory is targeted.
INSTALL_ARGS = ["add", "blaxel-ai/agent-skills", "-g", "-y", "--skill", "*", "--json", "--agent", "universal"]
NPM_ARGS = ["ci", "--ignore-scripts", "--no-audit", "--no-fund",
            "--engine-strict", "--registry=https://registry.npmjs.org"]
LOCK = json.loads((Path(__file__).resolve().parents[2] / "cli/skillsinstaller/package-lock.json").read_text())
PROJECT_CANARIES = {
    ".npmrc": "registry=https://invalid.example.invalid\nignore-scripts=false\n",
    "package.json": '{"name":"untouched-user-project","private":true}\n',
    "package-lock.json": '{"name":"untouched-user-project","lockfileVersion":3}\n',
    ".tool-versions": "nodejs 22.20.0\n",
}


class Installation:
    def __init__(self, root, binary):
        self.root = root
        self.binary = binary
        self.home = root / "home"
        self.prefix = root / "prefix"
        self.tools = root / "tools"
        self.cwd = root / "work"
        for directory in (self.home, self.tools, self.cwd, self.prefix / "bin", self.prefix / "opt"):
            directory.mkdir(parents=True)
        self.env = {
            "HOME": str(self.home),
            "USERPROFILE": str(self.home),
            "XDG_CONFIG_HOME": str(self.home / ".config"),
            "XDG_CACHE_HOME": str(self.home / ".cache"),
            "XDG_DATA_HOME": str(self.home / ".local/share"),
            "CODEX_HOME": str(self.home / ".codex"),
            "CLAUDE_CONFIG_DIR": str(self.home / ".claude"),
            "npm_config_cache": str(self.home / ".npm"),
            "npm_config_userconfig": str(self.home / ".npmrc"),
            "npm_config_globalconfig": str(self.home / "npm-globalrc"),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": str(self.home / ".gitconfig"),
            "GIT_TERMINAL_PROMPT": "0",
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

    def fake_npm(self):
        # Run only Python stubs. No shell expands paths or test fixtures.
        npm = self.tools / "npm-runner"
        (self.tools / "npm").symlink_to(npm)
        node = self.tools / "node"
        script = (
            f"#!{sys.executable}\n"
            "import json, os, pathlib, sys\n"
            f"expected_lock = {LOCK!r}\n"
            f"npm_args = {NPM_ARGS!r}\n"
            f"install_args = {INSTALL_ARGS!r}\n" + textwrap.dedent("""\
                phase = 'skills' if pathlib.Path(sys.argv[0]).name == 'node' else 'npm'
                if phase == 'skills' and sys.argv[1:] == ['--version']:
                    print(os.environ.get('FAKE_NODE_VERSION', 'v22.20.0'))
                    sys.exit(0)
                args = sys.argv[2:] if phase == 'skills' else sys.argv[1:]
                if phase == 'npm':
                    prefix = next(arg for arg in args if arg.startswith('--prefix='))
                    root = pathlib.Path(prefix.split('=', 1)[1])
                else:
                    root = pathlib.Path(sys.argv[1]).parents[3]
                assert root.is_relative_to(pathlib.Path(os.environ['TMPDIR'])), root
                assert pathlib.Path.cwd() == pathlib.Path(os.environ['TMPDIR']) / 'work'
                with (pathlib.Path(os.environ['HOME']) / 'calls.jsonl').open('a') as log:
                    log.write(json.dumps({'tool': phase, 'args': args}) + '\\n')
                if phase == 'npm':
                    expected_args = npm_args + [
                        '--userconfig=' + str(root / 'npmrc'),
                        '--globalconfig=' + str(root / 'global-npmrc'),
                        '--cache=' + str(root / 'npm-cache'),
                        '--prefix=' + str(root),
                    ]
                    assert args == expected_args, sys.argv
                    lock = json.loads((root / 'package-lock.json').read_text())
                    assert lock == expected_lock, lock
                    manifest = json.loads((root / 'package.json').read_text())
                    assert manifest['dependencies'] == {'skills': '1.7.0'}
                    for name, package in lock['packages'].items():
                        if name:
                            assert package['integrity'].startswith('sha512-')
                            assert package['resolved'].startswith('https://registry.npmjs.org/')
                    print('FAKE_NPM_STDOUT')
                    print('FAKE_NPM_STDERR', file=sys.stderr)
                    if os.environ.get('FAKE_NPM_EXIT'):
                        sys.exit(int(os.environ['FAKE_NPM_EXIT']))
                    target = root / 'node_modules/skills/bin/cli.mjs'
                    target.parent.mkdir(parents=True)
                    target.write_text('// fake locked skills installer\\n')
                else:
                    entry = pathlib.Path(sys.argv[1])
                    assert entry.is_absolute() and entry.is_file(), entry
                    assert entry == root / 'node_modules/skills/bin/cli.mjs', entry
                    expected = install_args + (['claude-code'] if os.environ.get('FAKE_INSTALL_ARGS') == 'claude' else [])
                    assert sys.argv[2:] == expected, sys.argv
                    print(json.dumps([{'name': 'blaxel-cli', 'status': 'installed'},
                                      {'name': 'blaxel-sdk', 'status': 'installed'}]))
                    print('FAKE_NODE_STDERR', file=sys.stderr)
            """)
        )
        for executable in (node, npm):
            executable.write_text(script)
            executable.chmod(0o755)
        git = self.tools / "git"
        git.write_text(f"#!{sys.executable}\n")
        git.chmod(0o755)

    def calls(self):
        log = self.home / "calls.jsonl"
        return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

    def assert_installs(self, count):
        calls = self.calls()
        assert len(calls) == count * 2, calls
        for npm, node in zip(calls[::2], calls[1::2]):
            assert npm["tool"] == "npm" and npm["args"][:len(NPM_ARGS)] == NPM_ARGS, npm
            assert node == {'tool': 'skills', 'args': INSTALL_ARGS}, node

    def marker(self, version="1.0.0"):
        return self.home / ".blaxel/skills/homebrew" / version

    def seed_project(self):
        for name, content in PROJECT_CANARIES.items():
            (self.cwd / name).write_text(content)

    def assert_project_unchanged(self):
        for name, content in PROJECT_CANARIES.items():
            assert (self.cwd / name).read_text() == content, name
        assert not (self.cwd / "node_modules").exists()

    def run(self, args=("--help",), extra_env=None, direct=False):
        binary = self.binary if direct else self.prefix / "bin/bl"
        result = subprocess.run(
            [str(binary), *args], cwd=self.cwd,
            env={**self.env, **(extra_env or {})}, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=150,
        )
        assert result.returncode == 0, (args, result.returncode, result.stdout, result.stderr)
        assert "FAKE_" not in result.stdout, result.stdout
        assert "Installing Blaxel skills" not in result.stdout, result.stdout
        return result


def fake_tests(root, binary):
    install = Installation(root / "npm-shell-shim", binary)
    install.fake_npm()
    (install.tools / "npm").unlink()
    (install.tools / "npm").write_text(
        '#!/bin/sh\n'
        'printf "executed\\n" > "$HOME/npm-shim-executed"\n'
        'exec "$TMPDIR/tools/npm-runner" "$@"\n'
    )
    (install.tools / "npm").chmod(0o755)
    install.seed_project()
    install.run()
    install.assert_installs(1)
    assert (install.home / "npm-shim-executed").read_text() == "executed\n"
    install.assert_project_unchanged()
    print("PASS npm shell shim executes with caller cwd and leaves project files untouched", flush=True)

    for index, args in enumerate((
        ("__complete", ""), ("__completeNoDesc", ""),
        ("--verbose", "__complete", ""), ("--verbose", "__completeNoDesc", ""),
    )):
        install = Installation(root / f"completion-{index}", binary)
        install.fake_npm()
        result = install.run(args)
        assert not install.calls() and not install.marker().exists(), result
        install.run()
        install.assert_installs(1)
        assert install.marker().exists()
        print(f"PASS completion skips setup: bl {' '.join(args)}", flush=True)

    for index, args in enumerate((("--help",), ("--version",), ("help",), ("version",), ())):
        install = Installation(root / f"command-{index}", binary)
        install.fake_npm()
        first = install.run(args)
        install.assert_installs(1)
        assert install.marker().exists()
        # Installer progress is captured; users see one concise summary.
        assert "FAKE_" not in first.stderr, first.stderr
        assert "Blaxel skills installed to ~/.agents/skills (blaxel-cli, blaxel-sdk)" in first.stderr, first.stderr
        second = install.run(args)
        install.assert_installs(1)
        assert first.stdout == second.stdout
        assert "Installing Blaxel skills" not in second.stderr
        print(f"PASS first invocation and repeat: bl {' '.join(args)}", flush=True)

    install = Installation(root / "upgrade", binary)
    install.fake_npm()
    install.run()
    install.use_keg("1.0.1")
    install.run()
    install.run()
    install.assert_installs(2)
    assert install.marker("1.0.1").exists()
    print("PASS new keg refresh", flush=True)

    install = Installation(root / "upgrade-command", binary)
    install.fake_npm()
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
    assert install.marker().exists() and install.marker("1.0.1").exists()
    install.run()
    install.assert_installs(1)
    print("PASS first bl upgrade installs once and marks the new keg", flush=True)

    for name, skipped, enabled in (
        ("optout", {"BL_INSTALL_SKILLS": "false"}, {}),
        ("ci", {"CI": "true"}, {"CI": "true", "BL_INSTALL_SKILLS": "true"}),
    ):
        install = Installation(root / name, binary)
        install.fake_npm()
        install.run(extra_env=skipped)
        assert not install.calls() and not install.marker().exists()
        install.run(extra_env=enabled)
        install.assert_installs(1)
        assert install.marker().exists()
        print(f"PASS {name} skip then enable", flush=True)

    install = Installation(root / "failure", binary)
    install.fake_npm()
    result = install.run(extra_env={"FAKE_NPM_EXIT": "1"})
    assert "Could not install" in result.stderr
    assert "FAKE_NPM_STDERR" in result.stderr, "failure shows the installer output"
    install.run()
    assert len(install.calls()) == 1 and install.calls()[0]["tool"] == "npm", install.calls()
    assert install.marker().exists()
    print("PASS installer failure is nonblocking and not repeated", flush=True)

    install = Installation(root / "no-node", binary)
    result = install.run()
    assert "requires Node.js 22.20.0 or later and npm" in result.stderr, result.stderr
    install.fake_npm()
    install.run()
    assert not install.calls() and install.marker().exists()
    print("PASS missing Node.js is nonblocking and not repeated", flush=True)

    install = Installation(root / "no-npm", binary)
    install.fake_npm()
    (install.tools / "npm").unlink()
    result = install.run()
    assert "requires npm" in result.stderr, result.stderr
    (install.tools / "npm").symlink_to(install.tools / "npm-runner")
    install.run()
    assert not install.calls() and install.marker().exists()
    print("PASS missing npm is nonblocking and not repeated", flush=True)

    install = Installation(root / "old-node", binary)
    install.fake_npm()
    result = install.run(extra_env={"FAKE_NODE_VERSION": "v20.11.0"})
    assert "requires Node.js 22.20.0 or later (found v20.11.0)" in result.stderr, result.stderr
    assert not install.calls()
    print("PASS old Node.js is reported before running npm", flush=True)

    install = Installation(root / "agents", binary)
    install.fake_npm()
    (install.home / ".claude").mkdir()
    install.run(("skills", "install"), direct=True, extra_env={"FAKE_INSTALL_ARGS": "claude"})
    node = install.calls()[1]
    assert node["args"] == INSTALL_ARGS + ["claude-code"], node
    print("PASS detected agents are targeted explicitly", flush=True)

    install = Installation(root / "nonbrew", binary)
    install.fake_npm()
    install.run(direct=True)
    assert not install.calls() and not install.marker().exists()
    print("PASS non-Homebrew binary skips installation", flush=True)

    install = Installation(root / "explicit", binary)
    install.fake_npm()
    install.run(("skills", "install"), direct=True,
                extra_env={"CI": "true", "BL_INSTALL_SKILLS": "false"})
    install.assert_installs(1)
    assert not install.marker().exists()
    print("PASS explicit skills install works outside Homebrew without authentication", flush=True)


def shell_tests(root, binary):
    source = (Path(__file__).resolve().parents[2] / "install.sh").read_text()
    function = "setup_skills() {" + source.split("setup_skills() {", 1)[1].split("\n}", 1)[0] + "\n}\n"
    for name, extra_env in (("success", {}), ("failure", {"FAKE_NPM_EXIT": "1"})):
        install = Installation(root / f"shell-{name}", binary)
        install.fake_npm()
        (install.tools / "sh").symlink_to("/bin/sh")
        (install.tools / "sed").symlink_to("/usr/bin/sed")
        identity = install.tools / "id"
        identity.write_text(f"#!{sys.executable}\nprint(1000)\n")
        identity.chmod(0o755)
        # Metacharacters stay literal all the way through the shell installer's
        # argument passing. No environment or binary from the real home is used.
        directory = install.root / "installed cli ' $(not-a-command)"
        directory.mkdir()
        shutil.copy2(binary, directory / "blaxel")
        script = install.root / "setup-skills.sh"
        script.write_text("set -eu\n" + function + "\nsetup_skills\n")
        result = subprocess.run(
            ["/bin/sh", str(script)], cwd=install.cwd,
            env={**install.env, **extra_env, "BL_INSTALL_SKILLS": "true",
                 "ABSOLUTE_BINDIR": str(directory), "BINARY": "blaxel",
                 "SKILLS_INSTALL_CMD": "bl skills install"},
            stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=30,
        )
        assert result.returncode == 0, (result.stdout, result.stderr)
        if name == "success":
            install.assert_installs(1)
            assert "Blaxel skills installed" in result.stderr, (result.stdout, result.stderr)
        else:
            assert len(install.calls()) == 1 and install.calls()[0]["tool"] == "npm", install.calls()
            assert "Could not install the Blaxel skills" in result.stdout, result.stdout
        print(f"PASS shell setup_skills {name} with literal binary path", flush=True)


def integrity_test(install):
    directory = install.root / "integrity"
    directory.mkdir()
    source = Path(__file__).resolve().parents[2] / "cli/skillsinstaller"
    shutil.copy2(source / "package.json", directory / "package.json")
    lock = json.loads((source / "package-lock.json").read_text())
    lock["packages"]["node_modules/skills"]["integrity"] = (
        "sha512-" + base64.b64encode(bytes(64)).decode()
    )
    (directory / "package-lock.json").write_text(json.dumps(lock))
    # Use an empty, independent cache so npm must validate the downloaded bytes.
    result = subprocess.run(
        [str(install.tools / "npm"), *NPM_ARGS, "--fetch-retries=0", "--fetch-timeout=15000",
         "--cache=" + str(directory / "cache")],
        cwd=directory, env=install.env, stdin=subprocess.DEVNULL,
        capture_output=True, text=True, timeout=60,
    )
    assert result.returncode != 0 and "EINTEGRITY" in result.stderr, (
        result.returncode, result.stdout, result.stderr
    )
    print("PASS real npm rejects a modified package integrity checksum", flush=True)


def sudo_shell_tests(root, binary, source=None):
    if source is None:
        source = (Path(__file__).resolve().parents[2] / "install.sh").read_text()
    function = "setup_skills() {" + source.split("setup_skills() {", 1)[1].split("\n}", 1)[0] + "\n}\n"
    for name, extra_env in (("success", {}), ("failure", {"FAKE_NPM_EXIT": "1"}),
                            ("missing-tools", {})):
        install = Installation(root / f"sudo-shell-{name}", binary)
        if name != "missing-tools":
            install.fake_npm()
        (install.tools / "sh").symlink_to("/bin/sh")
        root_tools = install.root / "root-tools"
        root_tools.mkdir()
        root_home = install.root / "root-home"
        root_home.mkdir()
        for tool, actual in (("sh", "/bin/sh"), ("sed", "/usr/bin/sed")):
            (root_tools / tool).symlink_to(actual)
        identity = root_tools / "id"
        identity.write_text(f"#!{sys.executable}\nprint(0)\n")
        identity.chmod(0o755)
        sudo = root_tools / "sudo"
        sudo.write_text(
            f"#!{sys.executable}\n"
            "import json, os, pathlib, subprocess, sys\n"
            f"target_home = {str(install.home)!r}\n"
            f"login_path = {str(install.tools)!r}\n" + textwrap.dedent("""\
                assert sys.argv[1:5] == ['-u', 'fixture-user', '-H', '-i'], sys.argv
                args = sys.argv[5:]
                with (pathlib.Path(target_home) / 'sudo-calls.jsonl').open('a') as log:
                    log.write(json.dumps(args) + '\\n')
                # sudo -i joins and re-escapes argv for the login shell. It
                # deliberately leaves dollar signs unescaped, so '$1' is not
                # preserved by merely passing it as a separately quoted arg.
                def escape(argument):
                    return ''.join(c if c.isascii() and (c.isalnum() or c in '_-$')
                                   else chr(92) + c for c in argument)
                command = ' '.join(escape(argument) for argument in args)
                env = dict(os.environ, HOME=target_home, USERPROFILE=target_home,
                           PATH=login_path)
                result = subprocess.run(['/bin/sh', '-c', command], env=env)
                sys.exit(result.returncode)
            """)
        )
        sudo.chmod(0o755)
        directory = install.root / "installed cli ' $BL_SKILLS_PATH_CANARY $(printf injected > command-substitution-ran)"
        directory.mkdir()
        shutil.copy2(binary, directory / "blaxel")
        script = install.root / "setup-skills.sh"
        script.write_text(
            'set -eu\nis_command() { command -v "$1" >/dev/null 2>&1; }\n'
            + function + "\nsetup_skills\n"
        )
        result = subprocess.run(
            ["/bin/sh", str(script)], cwd=install.cwd,
            env={**install.env, **extra_env, "HOME": str(root_home), "PATH": str(root_tools),
                 "BL_INSTALL_SKILLS": "true", "SUDO_USER": "fixture-user",
                 "BL_SKILLS_PATH_CANARY": "must-not-expand",
                 "ABSOLUTE_BINDIR": str(directory), "BINARY": "blaxel",
                 "SKILLS_INSTALL_CMD": "bl skills install"},
            stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=30,
        )
        assert result.returncode == 0, (result.stdout, result.stderr)
        if name == "success":
            install.assert_installs(1)
            assert "Blaxel skills installed" in result.stderr, (result.stdout, result.stderr)
        elif name == "failure":
            assert len(install.calls()) == 1 and install.calls()[0]["tool"] == "npm", install.calls()
            assert "Could not install the Blaxel skills" in result.stdout, result.stdout
        else:
            assert not install.calls(), install.calls()
            assert "install Node.js 22.20+" in result.stdout, result.stdout
        sudo_calls = [json.loads(line) for line in (install.home / "sudo-calls.jsonl").read_text().splitlines()]
        assert sudo_calls[0] == ["sh", "-c", "command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1"], sudo_calls
        assert sudo_calls[1:] == ([] if name == "missing-tools" else [["sh", "-s"]]), sudo_calls
        assert not (root_home / "calls.jsonl").exists()
        assert not (install.cwd / "command-substitution-ran").exists()
        print(f"PASS sudo login setup_skills {name}: target user tools and literal binary path", flush=True)


def real_test(root, binary):
    install = Installation(root / "real", binary)
    install.seed_project()
    # Expose only the tools npm and the skills installer need. No inherited
    # credentials, npm config, agent config, or user PATH reaches the subprocess.
    for name in ("npm", "node", "git", "sh"):
        executable = shutil.which(name)
        if not executable:
            raise RuntimeError(f"--real-npm requires {name} on the invoking PATH")
        (install.tools / name).symlink_to(Path(executable).resolve())
    first = install.run()
    assert "Blaxel skills installed" in first.stderr, first.stderr
    skills = list((install.home / ".agents/skills").glob("*/SKILL.md"))
    assert {path.parent.name for path in skills} == {"blaxel-cli", "blaxel-sdk"}, skills
    assert install.marker().exists()
    second = install.run()
    assert "Installing Blaxel skills" not in second.stderr, second.stderr
    assert first.stdout == second.stdout
    install.assert_project_unchanged()
    print(f"PASS real locked npm installation: {len(skills)} skills; second invocation skips setup", flush=True)
    integrity_test(install)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="Prebuilt CLI binary outside a Homebrew keg")
    parser.add_argument("--real-npm", action="store_true", help="Also install actual skills in an isolated home")
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="bl-homebrew-skills-") as temporary:
        root = Path(temporary).resolve()
        fake_tests(root, binary)
        shell_tests(root, binary)
        sudo_shell_tests(root, binary)
        if args.real_npm:
            real_test(root, binary)


if __name__ == "__main__":
    main()
