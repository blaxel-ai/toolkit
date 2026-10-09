#!/bin/sh
# Exercise the installer's release lookup without network access or user
# configuration changes: the latest-release redirect first, the GitHub API when
# the redirect fails or points at a preview, and VERSION to skip both.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT HUP INT TERM
mkdir -p "$WORK/bin" "$WORK/home"
cat > "$WORK/bin/curl" <<'MOCK'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS"
case "$*" in
  *--head*/releases/latest*)
    case "$SCENARIO" in
      redirect) printf 'HTTP/2 302\r\nlocation: https://github.com/blaxel-ai/toolkit/releases/tag/v0.9.0\r\n' ;;
      preview) printf 'HTTP/2 302\r\nlocation: https://github.com/blaxel-ai/toolkit/releases/tag/v1.0.0-preview\r\n' ;;
      *) echo 'curl: connection failed' >&2; exit 7 ;;
    esac
    ;;
  *api.github.com*)
    out=""
    while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out=$2; shift; done
    case "$SCENARIO" in
      forbidden) echo 'curl: HTTP 403' >&2; exit 22 ;;
      network) echo 'curl: connection failed' >&2; exit 7 ;;
      empty) echo '[]' > "$out" ;;
      only-preview) echo '"tag_name": "v1.0.0-preview"' > "$out" ;;
      api|preview) printf '%s\n' '"tag_name": "v1.0.0-preview"' '"tag_name": "v0.9.0"' > "$out" ;;
      *) echo 'Unexpected API lookup' >&2; exit 99 ;;
    esac
    ;;
  # Stop at the download: these cases check the release it picked.
  */releases/download/*) exit 42 ;;
  *) exit 0 ;;
esac
MOCK
chmod +x "$WORK/bin/curl"
export PATH="$WORK/bin:$PATH" HOME="$WORK/home" CALLS="$WORK/calls"
export BL_INSTALL_PATH=false BL_INSTALL_COMPLETION=false BL_INSTALL_SETUP=false
for SCENARIO in redirect preview api forbidden network empty only-preview explicit; do
  export SCENARIO
  : > "$CALLS"
  VERSION=''
  [ "$SCENARIO" != explicit ] || VERSION=v0.8.0
  export VERSION
  status=0
  TMPDIR="$WORK" BINDIR="$WORK/install" sh "$ROOT/install.sh" > "$WORK/stdout" 2> "$WORK/stderr" || status=$?
  [ "$status" -eq 1 ]
  case "$SCENARIO" in
    redirect|preview|api|explicit)
      grep -q 'could not download' "$WORK/stderr"
      tag=v0.9.0
      [ "$SCENARIO" != explicit ] || tag=v0.8.0
      grep -q "/releases/download/$tag/" "$CALLS"
      case "$SCENARIO" in
        redirect) ! grep -q api.github.com "$CALLS" ;;
        explicit) ! grep -q -e api.github.com -e /releases/latest "$CALLS" ;;
      esac
      ;;
    *)
      grep -q 'could not find the latest release; set VERSION' "$WORK/stderr"
      grep -q 'https://github.com/blaxel-ai/toolkit/releases' "$WORK/stderr"
      ! grep -q '/releases/download/' "$CALLS"
      ;;
  esac
  [ ! -d "$WORK/install" ]
  [ -z "$(ls "$WORK" | grep -v -e '^bin$' -e '^home$' -e '^calls$' -e '^stdout$' -e '^stderr$')" ]
  printf 'PASS: %s\n' "$SCENARIO"
done
