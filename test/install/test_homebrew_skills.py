#!/usr/bin/env python3
"""Exercise a prebuilt CLI in temporary Homebrew kegs without invoking brew.

Usage: python3 test/install/test_homebrew_skills.py /tmp/blaxel [--real-npx]
The optional network test installs real skills into a disposable home directory.
"""

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


INSTALL_ARGS = ["-y", "skills", "add", "blaxel-ai/agent-skills", "-g", "--all"]


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

    def fake_npx(self):
        script = self.tools / "npx"
        script.write_text(
            f"#!{sys.executable}\n"
            "import json, os, pathlib, sys\n"
            "with (pathlib.Path(os.environ['HOME']) / 'calls.jsonl').open('a') as log:\n"
            "    log.write(json.dumps(sys.argv[1:]) + '\\n')\n"
            "print('FAKE_NPX_STDOUT')\n"
            "print('FAKE_NPX_STDERR', file=sys.stderr)\n"
            "sys.exit(int(os.environ.get('FAKE_NPX_EXIT', '0')))\n"
        )
        script.chmod(0o755)

    def calls(self):
        log = self.home / "calls.jsonl"
        return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

    def marker(self, version="1.0.0"):
        return self.home / ".blaxel/skills/homebrew" / version

    def run(self, args=("--help",), extra_env=None, direct=False):
        binary = self.binary if direct else self.prefix / "bin/bl"
        result = subprocess.run(
            [str(binary), *args], cwd=self.cwd,
            env={**self.env, **(extra_env or {})}, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=150,
        )
        assert result.returncode == 0, (args, result.returncode, result.stdout, result.stderr)
        assert "FAKE_NPX" not in result.stdout, result.stdout
        assert "Installing Blaxel skills" not in result.stdout, result.stdout
        return result


def fake_tests(root, binary):
    for index, args in enumerate((("--help",), ("--version",), ("help",), ("version",), ())):
        install = Installation(root / f"command-{index}", binary)
        install.fake_npx()
        first = install.run(args)
        assert install.calls() == [INSTALL_ARGS]
        assert install.marker().exists()
        assert "FAKE_NPX_STDOUT" in first.stderr and "FAKE_NPX_STDERR" in first.stderr
        second = install.run(args)
        assert install.calls() == [INSTALL_ARGS]
        assert first.stdout == second.stdout
        assert "Installing Blaxel skills" not in second.stderr
        print(f"PASS first invocation and repeat: bl {' '.join(args)}", flush=True)

    install = Installation(root / "upgrade", binary)
    install.fake_npx()
    install.run()
    install.use_keg("1.0.1")
    install.run()
    install.run()
    assert install.calls() == [INSTALL_ARGS, INSTALL_ARGS]
    assert install.marker("1.0.1").exists()
    print("PASS new keg refresh", flush=True)

    install = Installation(root / "upgrade-command", binary)
    install.fake_npx()
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
    assert install.calls() == [INSTALL_ARGS]
    assert install.marker().exists() and install.marker("1.0.1").exists()
    install.run()
    assert install.calls() == [INSTALL_ARGS]
    print("PASS first bl upgrade installs once and marks the new keg", flush=True)

    for name, skipped, enabled in (
        ("optout", {"BL_INSTALL_SKILLS": "false"}, {}),
        ("ci", {"CI": "true"}, {"CI": "true", "BL_INSTALL_SKILLS": "true"}),
    ):
        install = Installation(root / name, binary)
        install.fake_npx()
        install.run(extra_env=skipped)
        assert not install.calls() and not install.marker().exists()
        install.run(extra_env=enabled)
        assert install.calls() == [INSTALL_ARGS] and install.marker().exists()
        print(f"PASS {name} skip then enable", flush=True)

    install = Installation(root / "failure", binary)
    install.fake_npx()
    result = install.run(extra_env={"FAKE_NPX_EXIT": "1"})
    assert "Could not install" in result.stderr
    install.run()
    assert install.calls() == [INSTALL_ARGS]
    print("PASS installer failure is nonblocking and not repeated", flush=True)

    install = Installation(root / "no-npx", binary)
    result = install.run()
    assert "npx (Node.js) was not found" in result.stderr
    install.fake_npx()
    install.run()
    assert not install.calls() and install.marker().exists()
    print("PASS missing npx is nonblocking and not repeated", flush=True)

    install = Installation(root / "nonbrew", binary)
    install.fake_npx()
    install.run(direct=True)
    assert not install.calls() and not install.marker().exists()
    print("PASS non-Homebrew binary skips installation", flush=True)


def real_test(root, binary):
    install = Installation(root / "real", binary)
    # Expose only the tools npm and the skills installer need. No inherited
    # credentials, npm config, agent config, or user PATH reaches the subprocess.
    for name in ("npx", "node", "git", "sh"):
        executable = shutil.which(name)
        if not executable:
            raise RuntimeError(f"--real-npx requires {name} on the invoking PATH")
        (install.tools / name).symlink_to(Path(executable).resolve())
    first = install.run()
    assert "Blaxel skills installed." in first.stderr, first.stderr
    skills = list((install.home / ".agents/skills").glob("*/SKILL.md"))
    assert skills, f"No installed SKILL.md files under {install.home}"
    assert install.marker().exists()
    second = install.run()
    assert "Installing Blaxel skills" not in second.stderr, second.stderr
    assert first.stdout == second.stdout
    print(f"PASS real npx installation: {len(skills)} skills; second invocation skips setup", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="Prebuilt CLI binary outside a Homebrew keg")
    parser.add_argument("--real-npx", action="store_true", help="Also install actual skills in an isolated home")
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="bl-homebrew-skills-") as temporary:
        root = Path(temporary).resolve()
        fake_tests(root, binary)
        if args.real_npx:
            real_test(root, binary)


if __name__ == "__main__":
    main()
