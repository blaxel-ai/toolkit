"""Exercise a built CLI and concurrent MCP bridges using an isolated home.

All HTTPS downloads terminate at a local TLS fixture via HTTPS_PROXY. The CLI's
fixed GitHub origins and certificate checks remain active; no live API is used.
"""

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import selectors
import socketserver
import ssl
import subprocess
import tarfile
import tempfile
import threading
import time


def bundle(revision):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w") as archive:
        for name in ("blaxel-cli", "blaxel-sdk"):
            content = f"---\nname: {name}\ndescription: Fixture revision {revision}\n---\n\nRevision {revision}\n".encode()
            entry = tarfile.TarInfo(f"agent-skills-{revision}/skills/{name}/SKILL.md")
            entry.size, entry.mode = len(content), 0o644
            archive.addfile(entry, io.BytesIO(content))
    data = gzip.compress(output.getvalue(), mtime=0)
    return {
        "revision": revision,
        "bundleUrl": f"https://github.com/blaxel-ai/agent-skills/releases/download/skills-{revision}/skills.tar.gz",
        "sha256": hashlib.sha256(data).hexdigest(),
        "minimumCli": "0.1.121",
    }, data


def wait_for(predicate, timeout=10):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.02)
    raise AssertionError("timed out waiting for updater")


class Fixture(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


class Tunnel(socketserver.StreamRequestHandler):
    def handle(self):
        self.connection.settimeout(15)
        method, address, _ = self.rfile.readline().decode().strip().split()
        while self.rfile.readline().strip():
            pass
        if method != "CONNECT" or address not in ("raw.githubusercontent.com:443", "github.com:443"):
            self.wfile.write(b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n")
            return
        self.wfile.write(b"HTTP/1.1 200 Connection Established\r\n\r\n")
        self.wfile.flush()
        with self.server.tls.wrap_socket(self.connection, server_side=True) as secure:
            with secure.makefile("rb") as request:
                method, path, _ = request.readline().decode().strip().split()
                headers = {}
                while line := request.readline().decode().strip():
                    key, value = line.split(":", 1)
                    headers[key.lower()] = value.strip()
                assert "authorization" not in headers and "cookie" not in headers, headers
                with self.server.lock:
                    manifest, archive = self.server.manifest, self.server.archive
                    if path.endswith("/skills-manifest.json"):
                        self.server.checks += 1
                        content = json.dumps(manifest).encode()
                    elif path.endswith("/skills.tar.gz"):
                        self.server.downloads += 1
                        content = archive
                    else:
                        raise AssertionError(f"unexpected fixture request {address} {path}")
                secure.sendall(b"HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: " + str(len(content)).encode() + b"\r\n\r\n" + content)


def read_rpc(process, identifier):
    with selectors.DefaultSelector() as selector:
        selector.register(process.stdout, selectors.EVENT_READ)
        assert selector.select(3), "MCP response timed out"
        answer = json.loads(process.stdout.readline())
        assert answer["jsonrpc"] == "2.0" and answer["id"] == identifier, answer
        return answer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--release-dir", type=Path, help="Use the release generator's real bundle for the first install")
    parser.add_argument("--test-entrypoint", action="store_true", help="Use a Go test binary with injected fixture CA on macOS")
    args = parser.parse_args()
    executable = str(args.binary.resolve())
    with tempfile.TemporaryDirectory(prefix="blaxel-skills-integration-") as directory:
        home = Path(directory)
        certificate, key = home / "fixture.crt", home / "fixture.key"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=skills-fixture", "-addext", "subjectAltName=DNS:raw.githubusercontent.com,DNS:github.com", "-keyout", str(key), "-out", str(certificate)], check=True, capture_output=True)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.load_cert_chain(certificate, key)
        with Fixture(("127.0.0.1", 0), Tunnel) as server:
            server.tls, server.lock = tls, threading.Lock()
            server.checks, server.downloads = 0, 0
            server.manifest, server.archive = bundle("a" * 40)
            if args.release_dir:
                server.manifest = json.loads((args.release_dir / "skills-manifest.json").read_text())
                server.archive = (args.release_dir / "skills.tar.gz").read_bytes()
            serving = threading.Thread(target=server.serve_forever, daemon=True)
            serving.start()
            environment = {name: value for name, value in os.environ.items() if not name.startswith(("BL_", "XDG_", "CLAUDE_", "CODEX_")) and name.upper() not in ("HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "SSL_CERT_FILE", "SSL_CERT_DIR")}
            environment.update(HOME=directory, USERPROFILE=directory, BL_INSTALL_SKILLS="true", CI="true", HTTPS_PROXY=f"http://127.0.0.1:{server.server_address[1]}", SSL_CERT_FILE=str(certificate))
            if args.test_entrypoint:
                environment["BL_TEST_SKILLS_ENTRYPOINT"] = "1"
            state_path = home / ".blaxel/skills/state.json"
            def cli(*arguments):
                result = subprocess.run([executable, "--skip-version-warning", *arguments], env=environment, capture_output=True, text=True, timeout=15)
                assert result.returncode == 0, (arguments, result.stdout, result.stderr)
                return result.stdout
            def state():
                return json.loads(state_path.read_text())
            def due():
                data = state()
                data["lastAttempt"] = "2000-01-01T00:00:00Z"
                state_path.write_text(json.dumps(data))

            # Explicit initialization uses the same verified bundle installer.
            cli("skills", "update")
            assert state()["installedRevisions"]["blaxel-cli"] == server.manifest["revision"]
            cli("skills", "autoupdate", "off")
            assert json.loads(cli("skills", "status"))["automaticUpdates"] is False
            with server.lock:
                server.manifest, server.archive = bundle("b" * 40)
            due()
            checks_before = server.checks
            cli("version")
            time.sleep(0.2)
            assert server.checks == checks_before, "saved opt-out ignored"
            cli("skills", "autoupdate", "on")

            # Leading global flags and concurrent bridges share one update.
            bridges = []
            try:
                for _ in range(4):
                    bridge = subprocess.Popen([executable, "--skip-version-warning", "mcp"], env=environment, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                    bridges.append(bridge)
                    bridge.stdin.write(json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "fixture", "version": "1"}}}) + "\n")
                    bridge.stdin.flush()
                    answer = read_rpc(bridge, 1)
                    assert "bl login" in answer["result"]["instructions"], answer
                wait_for(lambda: state().get("verifiedRevision") == "b" * 40)
                assert server.checks == checks_before + 1, "bridges did not coordinate"
                assert server.downloads == 2
                for bridge in bridges:
                    bridge.stdin.write('{"jsonrpc":"2.0","id":2,"method":"ping"}\n')
                    bridge.stdin.flush()
                    assert read_rpc(bridge, 2)["result"] == {}

                # A finite worker survives an ordinary CLI command's exit and
                # protects edits while a real MCP connection remains alive.
                edit = home / ".agents/skills/blaxel-cli/custom.txt"
                edit.write_text("keep local changes")
                with server.lock:
                    server.manifest, server.archive = bundle("c" * 40)
                due()
                started = time.monotonic()
                cli("version")
                assert time.monotonic() - started < 3, "ordinary command waited for update"
                wait_for(lambda: state().get("verifiedRevision") == "c" * 40)
                assert state()["skippedSkills"] == ["blaxel-cli"]
                assert edit.read_text() == "keep local changes"
                assert "c" * 40 in (home / ".agents/skills/blaxel-sdk/SKILL.md").read_text()
                assert server.checks == checks_before + 2
                for bridge in bridges:
                    bridge.stdin.write('{"jsonrpc":"2.0","id":3,"method":"ping"}\n')
                    bridge.stdin.flush()
                    assert read_rpc(bridge, 3)["result"] == {}
            finally:
                for bridge in bridges:
                    bridge.terminate()
                    bridge.communicate(timeout=5)
                server.shutdown()
            print("PASS: verified bundles, persisted opt-out, four concurrent MCP bridges, JSON-only stdio, detached CLI update, and local edits in isolated home")


if __name__ == "__main__":
    main()
