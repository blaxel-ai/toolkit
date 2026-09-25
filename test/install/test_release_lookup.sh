#!/bin/sh
# Exercise the real installer without network access or user configuration changes.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT HUP INT TERM
mkdir -p "$WORK/bin" "$WORK/home"
cat > "$WORK/bin/curl" <<'MOCK'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS"
case "$*" in
  *api.github.com*)
    case "$SCENARIO" in
      forbidden) echo 'curl: HTTP 403' >&2; exit 22 ;;
      network) echo 'curl: connection failed' >&2; exit 7 ;;
      empty) echo '[]' ;;
      preview) echo '"tag_name": "v1.0.0-preview"' ;;
      success) printf '%s\n' '"tag_name": "v1.0.0-preview"' '"tag_name": "v0.9.0"' ;;
      *) echo 'Unexpected API lookup' >&2; exit 99 ;;
    esac
    ;;
  *) exit 0 ;;
esac
MOCK
# Stop at extraction: success cases verify the selected download URL only.
cat > "$WORK/bin/tar" <<'MOCK'
#!/bin/sh
exit 42
MOCK
chmod +x "$WORK/bin/curl" "$WORK/bin/tar"
export PATH="$WORK/bin:$PATH" HOME="$WORK/home" CALLS="$WORK/calls"
export BL_INSTALL_PATH=false BL_INSTALL_COMPLETION=false BL_INSTALL_TRACKING=false
for SCENARIO in forbidden network empty preview success explicit; do
  export SCENARIO
  : > "$CALLS"
  VERSION=''
  [ "$SCENARIO" != explicit ] || VERSION=v0.8.0
  export VERSION
  status=0
  TMPDIR="$WORK" BINDIR="$WORK/install" sh "$ROOT/install.sh" > "$WORK/stdout" 2> "$WORK/stderr" || status=$?
  case "$SCENARIO" in
    success|explicit)
      [ "$status" -eq 42 ]
      ! grep -q 'unable to determine' "$WORK/stderr"
      tag=v0.9.0
      if [ "$SCENARIO" = explicit ]; then
        tag=v0.8.0
        ! grep -q api.github.com "$CALLS"
      fi
      grep -q "/releases/download/$tag/" "$CALLS"
      ;;
    *)
      [ "$status" -eq 1 ]
      grep -q 'unable to determine the latest stable version' "$WORK/stderr"
      grep -q 'VERSION="<release-tag>"' "$WORK/stderr"
      grep -q 'https://github.com/blaxel-ai/toolkit/releases' "$WORK/stderr"
      ! grep -q '/releases/download/' "$CALLS"
      [ "$(wc -l < "$CALLS" | tr -d ' ')" -eq 1 ]
      ;;
  esac
  [ ! -d "$WORK/install" ]
  printf 'PASS: %s\n' "$SCENARIO"
done
