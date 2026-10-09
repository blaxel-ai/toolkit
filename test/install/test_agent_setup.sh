#!/bin/sh
# Exercise when install.sh sets up the coding agents and starts the browser
# login, against a local release whose bl only records how it was called. No
# network, no login, and no real configuration is touched.
#
#   sh test/install/test_agent_setup.sh
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT HUP INT TERM
mkdir -p "$WORK/bin" "$WORK/rel" "$WORK/stage"

# The release: bl logs its arguments and whether DO_NOT_TRACK was set. Its
# token command fails unless FAKE_LOGGED_IN is set, as when nobody has logged in.
cat > "$WORK/stage/blaxel" <<'BL'
#!/bin/sh
echo "$* DO_NOT_TRACK=${DO_NOT_TRACK-unset} BL_INSTALL_SKILLS=${BL_INSTALL_SKILLS-unset} BL_INSTALL_MCP=${BL_INSTALL_MCP-unset} BL_INSTALL_REFRESH=${BL_INSTALL_REFRESH-unset}" >> "$CALLS"
case "$*" in
  'setup --help') [ -z "${FAKE_OLD_CLI:-}" ] ;;
  'setup --yes'|'setup --yes --skip-login') echo 'fixture: setup ran'; exit "${FAKE_SETUP_EXIT:-0}" ;;
  token) [ -n "${FAKE_LOGGED_IN:-}" ] ;;
  login) echo 'https://example.invalid/device?code=fixture'; exit "${FAKE_LOGIN_EXIT:-0}" ;;
esac
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
  for option in "$@"; do
    if [ "$option" = "FAKE_EXISTING_INSTALL=1" ]; then
      mkdir -p "$WORK/home$n/.local/bin"
      printf '#!/bin/sh\nexit 99\n' > "$WORK/home$n/.local/bin/bl"
      chmod +x "$WORK/home$n/.local/bin/bl"
    fi
  done
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
for marker in CI GITHUB_ACTIONS GITLAB_CI CIRCLECI TRAVIS JENKINS_URL BUILDKITE; do
  name="$marker wins over a coding agent"
  run CLAUDECODE=1 "$marker=true"; untouched; pass
done

name="explicit setup in CI does not start the agent login"
run CLAUDECODE=1 CI=true BL_INSTALL_SETUP=true; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="BL_INSTALL_SETUP=true without an agent runs setup only, as before"
run BL_INSTALL_SETUP=true; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="BL_INSTALL_SETUP=false opts out"
run CLAUDECODE=1 BL_INSTALL_SETUP=false; untouched; pass
run CLAUDECODE=1 BL_INSTALL_SETUP=false BL_INSTALL_SKILLS=true; untouched; pass

name="BL_INSTALL_SKILLS=false leaves the agents alone"
run CLAUDECODE=1 BL_INSTALL_SKILLS=false; untouched; pass

name="explicit setup passes along the skills opt-out"
run CLAUDECODE=1 BL_INSTALL_SETUP=true BL_INSTALL_SKILLS=false
has '^setup --yes .*BL_INSTALL_SKILLS=false'; has '^login'; pass

name="the MCP opt-out reaches setup"
run CLAUDECODE=1 BL_INSTALL_MCP=false; has '^setup --yes .*BL_INSTALL_MCP=false'; pass

name="BL_INSTALL_LOGIN=false sets up without the login"
run CLAUDECODE=1 BL_INSTALL_LOGIN=false; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="an API key is the login"
run CLAUDECODE=1 BL_API_KEY=key; has '^setup --yes'; lacks '^login'; pass

name="client credentials skip the browser login"
run CLAUDECODE=1 BL_CLIENT_CREDENTIALS=fixture; has '^setup --yes'; lacks '^login'; lacks '^token'; pass

name="a logged-in user is not logged in again"
run CLAUDECODE=1 FAKE_LOGGED_IN=1; has '^setup --yes'; has '^token'; lacks '^login'; pass

name="error reports are left for the first bl in a terminal"
run CLAUDECODE=1; has '^setup --yes DO_NOT_TRACK=1'; pass
run CLAUDECODE=1 BL_INSTALL_TRACKING=true; has '^setup --yes DO_NOT_TRACK=unset'; pass
run CLAUDECODE=1 DO_NOT_TRACK=0; has '^setup --yes DO_NOT_TRACK=0'; pass
run CLAUDECODE=1 BL_INSTALL_TRACKING=false; has '^setup --yes DO_NOT_TRACK=unset'; pass

name="setup precedes the token check and browser login"
run CLAUDECODE=1
test "$(cut -d ' ' -f 1 "$WORK/calls" | tr '\n' ' ')" = 'setup setup token login '
grep -q '^https://example.invalid/device?code=fixture$' "$WORK/out"; pass

name="failed setup keeps the CLI installed, reports a retry, and starts login"
run CLAUDECODE=1 FAKE_SETUP_EXIT=1
has '^login'; grep -q 'setup  .*to finish setting up' "$WORK/out"; pass

name="failed login keeps the CLI installed and reports a retry"
run CLAUDECODE=1 FAKE_LOGIN_EXIT=1 BINDIR="$WORK/bin with 'quote"
has '^login'; grep -q 'login  .*to log in' "$WORK/out"
if grep -Eq 'Successfully logged in|Blaxel is ready' "$WORK/out"; then
  echo "FAIL $name: misleading success output"; cat "$WORK/out"; exit 1
fi
pass

name="a release without setup only suggests login"
run CLAUDECODE=1 FAKE_OLD_CLI=1
untouched; grep -q 'login  .*log in to Blaxel' "$WORK/out"; pass

name="an agent reinstall refreshes with the new binary without logging in again"
run CLAUDECODE=1 FAKE_EXISTING_INSTALL=1
has '^setup --yes --skip-login .*BL_INSTALL_REFRESH=true'; lacks '^token'; lacks '^login'; pass

name="a non-agent reinstall without a terminal keeps install-only behavior"
run FAKE_EXISTING_INSTALL=1; untouched; pass

name="a pinned release keeps the previous setup and login hand-off"
run CLAUDECODE=1 FAKE_EXISTING_INSTALL=1 VERSION=v9.9.9
has '^setup --yes DO_NOT_TRACK'; has '^login'; lacks 'BL_INSTALL_REFRESH=true'; pass

name="a CI reinstall keeps install-only behavior"
run CLAUDECODE=1 CI=true FAKE_EXISTING_INSTALL=1; untouched; pass

name="forced setup in a CI reinstall keeps the previous hand-off"
run FAKE_EXISTING_INSTALL=1 CI=true BL_INSTALL_SETUP=true
has '^setup --yes DO_NOT_TRACK'; lacks 'BL_INSTALL_REFRESH=true'; lacks '^login'; pass

name="a reinstall preserves the setup opt-out"
run CLAUDECODE=1 FAKE_EXISTING_INSTALL=1 BL_INSTALL_SETUP=false; untouched; pass

name="a forced reinstall passes the skills and MCP opt-outs to refresh"
run FAKE_EXISTING_INSTALL=1 BL_INSTALL_SETUP=true BL_INSTALL_SKILLS=false BL_INSTALL_MCP=false
has '^setup --yes --skip-login .*BL_INSTALL_SKILLS=false BL_INSTALL_MCP=false BL_INSTALL_REFRESH=true'; lacks '^login'; pass

echo "PASS $n isolated installer runs"
