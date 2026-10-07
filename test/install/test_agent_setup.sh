#!/bin/sh
# Exercise when install.sh sets up the coding agents and starts the browser
# login, against a local release whose bl only records how it was called. No
# network, no login, and no real configuration is touched.
#
#   sh test/install/test_agent_setup.sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT HUP INT TERM
mkdir -p "$WORK/bin" "$WORK/rel" "$WORK/stage"

# The release: bl logs its arguments and whether DO_NOT_TRACK was set. Its
# token command fails unless FAKE_LOGGED_IN is set, as when nobody has logged in.
cat > "$WORK/stage/blaxel" <<'BL'
#!/bin/sh
echo "$* DO_NOT_TRACK=${DO_NOT_TRACK-unset}" >> "$CALLS"
[ "$1" != token ] || [ -n "${FAKE_LOGGED_IN:-}" ]
BL
chmod +x "$WORK/stage/blaxel"
case "$(uname -s)" in Darwin) os=Darwin ;; *) os=Linux ;; esac
case "$(uname -m)" in x86_64|amd64) arch=x86_64 ;; *) arch=arm64 ;; esac
tar -czf "$WORK/rel/blaxel_${os}_$arch.tar.gz" -C "$WORK/stage" blaxel
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
(cd "$WORK/rel" && $sum "blaxel_${os}_$arch.tar.gz" > blaxel_9.9.9_checksums.txt)
cat > "$WORK/bin/curl" <<'CURL'
#!/bin/sh
url="" out="" head="" prev=""
for a in "$@"; do
  case "$a" in https://*) url=$a ;; --head) head=1 ;; esac
  [ "$prev" = "-o" ] && out=$a
  prev=$a
done
case "$url" in
  */releases/latest) [ -n "$head" ] && { printf 'HTTP/2 302\r\nlocation: https://github.com/blaxel-ai/toolkit/releases/tag/v9.9.9\r\n'; exit 0; } ;;
  */releases/download/v9.9.9/*) cp "$REL/${url##*/}" "$out"; exit $? ;;
esac
echo "curl: no fixture for $url" >&2; exit 22
CURL
chmod +x "$WORK/bin/curl"

# run [VAR=value...] runs install.sh with no terminal and only those variables.
n=0
run() {
  n=$((n + 1))
  : > "$WORK/calls"
  mkdir -p "$WORK/home$n"
  env -i HOME="$WORK/home$n" PATH="$WORK/bin:/usr/bin:/bin" SHELL=/bin/sh TMPDIR="$WORK" \
    CALLS="$WORK/calls" REL="$WORK/rel" BL_INSTALL_PATH=false BL_INSTALL_COMPLETION=false "$@" \
    sh "$ROOT/install.sh" > "$WORK/out" 2>&1 < /dev/null || { cat "$WORK/out"; exit 1; }
}
has() { grep -q -- "$1" "$WORK/calls" || { echo "FAIL $name: bl was not called with '$1'"; cat "$WORK/calls"; exit 1; }; }
lacks() { ! grep -q -- "$1" "$WORK/calls" || { echo "FAIL $name: bl was called with '$1'"; cat "$WORK/calls"; exit 1; }; }
# Nothing new ran: no setup, no login.
untouched() { lacks 'setup --yes'; lacks '^login'; lacks '^token'; }
pass() { echo "PASS $name"; }

for marker in CLAUDECODE=1 CURSOR_AGENT=1 GEMINI_CLI=1 CODEX_THREAD_ID=t CODEX_SANDBOX=seatbelt OPENCODE=1 GOOSE_TERMINAL=1 AGENT=amp AI_AGENT=x; do
  name="$marker sets up the agents, then starts the login"
  run "$marker"
  has '^setup --yes'; has '^token'; has '^login'
  pass
done

name="no coding agent, no terminal: nothing new runs"
run; untouched; pass

name="CI wins over a coding agent"
run CLAUDECODE=1 CI=true; untouched; pass
run CURSOR_AGENT=1 GITHUB_ACTIONS=true; untouched; pass

name="BL_INSTALL_SETUP=true without an agent runs setup only, as before"
run BL_INSTALL_SETUP=true; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="BL_INSTALL_SETUP=false opts out"
run CLAUDECODE=1 BL_INSTALL_SETUP=false; untouched; pass

name="BL_INSTALL_SKILLS=false leaves the agents alone"
run CLAUDECODE=1 BL_INSTALL_SKILLS=false; untouched; pass

name="BL_INSTALL_LOGIN=false sets up without the login"
run CLAUDECODE=1 BL_INSTALL_LOGIN=false; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="an API key is the login"
run CLAUDECODE=1 BL_API_KEY=key; has '^setup --yes'; lacks '^login'; pass

name="a logged-in user is not logged in again"
run CLAUDECODE=1 FAKE_LOGGED_IN=1; has '^setup --yes'; has '^token'; lacks '^login'; pass

name="error reports are left for the first bl in a terminal"
run CLAUDECODE=1; has '^setup --yes DO_NOT_TRACK=1'; pass
run CLAUDECODE=1 BL_INSTALL_TRACKING=true; has '^setup --yes DO_NOT_TRACK=unset'; pass
run CLAUDECODE=1 DO_NOT_TRACK=0; has '^setup --yes DO_NOT_TRACK=0'; pass
