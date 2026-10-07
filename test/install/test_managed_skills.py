#!/usr/bin/env python3
"""Run skill installer regressions in disposable homes with a local archive.

Usage: python3 test/install/test_managed_skills.py /tmp/bl [--expect-legacy]
Optional --pi-loader points to Pi's installed core/skills.js for real discovery.
--evidence saves command transcripts and before/after state as JSON.
"""

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from test_homebrew_skills import ArchiveServer, FAKE_CURL, Installation, SKILLS, fake_release


def manifest(name):
    return f"---\nname: {name}\ndescription: Local {name}.\n---\n\nLocal fork, not upstream.\n"


def write(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


def seed_projection(install):
    root = install.home / ".agents/skills"
    for name in sorted(SKILLS):
        write(root / "blaxel" / name / "SKILL.md", manifest(name))
        (root / name).symlink_to(Path("blaxel") / name)
    (install.home / ".pi/agent").mkdir(parents=True)
    (install.home / ".pi/agent/skills").symlink_to(root)
    (install.home / ".claude").mkdir()
    lock = '{"version":3,"skills":{"blaxel-cli":{"source":"blaxel-ai/agent-skills","updatedAt":"old"}},"dismissed":{"mine":true}}\n'
    write(install.home / ".agents/.skill-lock.json", lock)
    return lock


def discovery(install, loader):
    root = install.home / ".agents/skills"
    copies = {name: sorted({str(p.parent.resolve()) for p in root.rglob("SKILL.md")
                            if p.parent.name == name}) for name in sorted(SKILLS)}
    state = {"real_copies": {name: len(paths) for name, paths in copies.items()},
             "projections_are_links": {name: (root / name).is_symlink() for name in sorted(SKILLS)}}
    if loader:
        script = f"""
import {{loadSkills}} from {json.dumps(str(loader))};
const r=loadSkills({{cwd:{json.dumps(str(install.home))},agentDir:{json.dumps(str(install.home / '.pi/agent'))},skillPaths:[],includeDefaults:true}});
console.log(JSON.stringify({{skills:r.skills.filter(s=>['blaxel-cli','blaxel-sdk'].includes(s.name)).map(s=>s.name),collisions:r.diagnostics.filter(d=>d.type==='collision')}}));
"""
        run = subprocess.run([shutil.which("node"), "--input-type=module", "-e", script],
                             capture_output=True, text=True, check=True, timeout=30)
        state["pi"] = json.loads(run.stdout)
    return state


def run_one_liner(install):
    # Same install.sh -> bl setup path as curl | sh, with every release download
    # served locally. No login, real agent config, or public network is touched.
    release = fake_release(install.root, install.binary)
    (install.tools / "curl").write_text(FAKE_CURL.replace("#!/usr/bin/env python3", f"#!{sys.executable}", 1))
    (install.tools / "curl").chmod(0o755)
    for tool in ("sh", "uname", "tr", "sed", "grep", "cut", "head", "mkdir", "mktemp", "tar", "gzip", "install",
                 "basename", "dirname", "awk", "id", "rm", "mv", "cat", "chmod", "shasum", "sha256sum", "env", "sleep"):
        found = shutil.which(tool, path="/usr/bin:/bin:/usr/sbin:/sbin")
        if found:
            (install.tools / tool).symlink_to(found)
    env = {**install.env, "FAKE_RELEASE": str(release), "SHELL": "/bin/zsh", "BL_INSTALL_SETUP": "true",
           "BL_INSTALL_MCP": "false", "BL_INSTALL_LOGIN": "false", "BL_INSTALL_TRACKING": "false"}
    script = Path(__file__).resolve().parents[2] / "install.sh"
    return subprocess.run(["/bin/sh", str(script)], cwd=install.cwd, env=env, stdin=subprocess.DEVNULL,
                          capture_output=True, text=True, timeout=120)


def tests(root, binary, server, legacy, loader):
    evidence = []
    for method in ("skills-install", "one-liner"):
        install = Installation(root / f"managed-{method}", binary, server)
        lock = seed_projection(install)
        install.seed_project()
        before = discovery(install, loader)
        run = run_one_liner(install) if method == "one-liner" else install.run(("skills", "install"), direct=True)
        assert run.returncode == 0, run.stdout + run.stderr
        # A repeat must not convert kept projections into upstream copies.
        repeat = install.run(("skills", "install"), direct=True)
        after = discovery(install, loader)
        expected = 2 if legacy else 1
        assert set(after["real_copies"].values()) == {expected}, after
        assert all(after["projections_are_links"].values()) == (not legacy), after
        if loader:
            assert len(after["pi"]["collisions"]) == (2 if legacy else 0), after
        for name in sorted(SKILLS):
            assert (install.home / ".agents/skills/blaxel" / name / "SKILL.md").read_text() == manifest(name)
        if not legacy:
            assert (install.home / ".agents/.skill-lock.json").read_text() == lock
            assert "kept externally managed" in (run.stdout + run.stderr).lower()
            for name in sorted(SKILLS):
                assert (install.home / ".claude/skills" / name / "SKILL.md").read_text() == manifest(name)
        install.assert_project_unchanged()
        evidence.append({"scenario": method, "before": before, "after": after,
                         "output": run.stdout + run.stderr, "repeat": repeat.stdout + repeat.stderr})
        print(f"PASS {method}: {after['real_copies']}; managed links preserved={not legacy}", flush=True)

    for scenario in ("nested-only", "agent-link", "existing-duplicate", "one-liner-duplicate"):
        install = Installation(root / scenario, binary, server)
        shared = install.home / ".agents/skills"
        if scenario == "agent-link":
            target = install.home / "dotfiles/sdk"
            write(target / "SKILL.md", manifest("blaxel-sdk"))
            link = install.home / ".claude/skills/blaxel-sdk"
            link.parent.mkdir(parents=True)
            link.symlink_to(target)
        else:
            write(shared / "blaxel/blaxel-sdk/SKILL.md", manifest("blaxel-sdk"))
            if scenario in ("existing-duplicate", "one-liner-duplicate"):
                write(shared / "blaxel-sdk/SKILL.md", manifest("blaxel-sdk") + "Flat customization.\n")
                write(shared / "blaxel-sdk/custom.txt", "keep me")
        run = (run_one_liner(install) if scenario == "one-liner-duplicate"
               else install.run(("skills", "install"), direct=True, check=False))
        if legacy:
            assert run.returncode == 0, run.stderr
            assert (shared / "blaxel-cli/SKILL.md").exists()
        else:
            assert run.returncode == 0, run.stdout + run.stderr
            assert (shared / "blaxel-cli/SKILL.md").exists()
            assert (install.home / ".agents/.skill-lock.json").exists()
            if scenario == "agent-link":
                assert link.is_symlink() and (link / "SKILL.md").read_text() == manifest("blaxel-sdk")
            else:
                assert (shared / "blaxel-sdk").is_symlink()
                assert (shared / "blaxel-sdk").resolve() == (shared / "blaxel/blaxel-sdk").resolve()
                assert (shared / "blaxel/blaxel-sdk/SKILL.md").read_text() == manifest("blaxel-sdk")
                if scenario in ("existing-duplicate", "one-liner-duplicate"):
                    backups = list((install.home / ".agents").glob(".blaxel-skills-backup-*/blaxel-sdk"))
                    assert len(backups) == 1, backups
                    assert (backups[0] / "custom.txt").read_text() == "keep me"
                    assert "backed up" in (run.stdout + run.stderr).lower() or "Backup" in run.stdout
                repeat = install.run(("skills", "install"), direct=True)
                assert repeat.returncode == 0, repeat.stdout + repeat.stderr
                state = discovery(install, loader)
                assert state["real_copies"]["blaxel-sdk"] == 1, state
                if loader:
                    assert not state["pi"]["collisions"], state
        evidence.append({"scenario": scenario, "exit_code": run.returncode, "output": run.stdout + run.stderr})
        print(f"PASS {scenario}: exit={run.returncode}; existing copies reused={not legacy}", flush=True)

    install = Installation(root / "clean", binary, server)
    run = install.run(("skills", "install"), direct=True)
    assert install.installed_skills() == SKILLS
    assert (install.home / ".agents/.skill-lock.json").exists()
    evidence.append({"scenario": "clean", "output": run.stdout + run.stderr})
    print("PASS clean install: both skills and lock installed normally", flush=True)
    return evidence


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--expect-legacy", action="store_true")
    parser.add_argument("--pi-loader", type=Path)
    parser.add_argument("--evidence", type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    server = ArchiveServer()
    try:
        with tempfile.TemporaryDirectory(prefix="bl-managed-skills-") as temporary:
            evidence = tests(Path(temporary).resolve(), binary, server, args.expect_legacy, args.pi_loader)
    finally:
        server.close()
    if args.evidence:
        args.evidence.parent.mkdir(parents=True, exist_ok=True)
        args.evidence.write_text(json.dumps(evidence, indent=2) + "\n")


if __name__ == "__main__":
    main()
