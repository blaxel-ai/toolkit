#!/bin/sh
# Runs install.sh under sudo in a throwaway Ubuntu container, for a user whose
# login shell is zsh, against a local release of the given Linux binary.
#
#   sh test/install/test_sudo.sh /path/to/linux/blaxel
#
# Checks that root works only inside the user's real home (never through a
# symlink out of it), uses the user's own shell, and that the user's choices
# reach bl setup.
set -eu
binary=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
here=$(cd "$(dirname "$0")" && pwd)
# Inside the checkout, which Docker can mount on every setup (colima shares only $HOME).
work=$(mktemp -d "$here/.sudo-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/rel" "$work/stage"
cp "$binary" "$work/stage/blaxel"
case "$(uname -m)" in x86_64|amd64) arch=x86_64 ;; *) arch=arm64 ;; esac
tar -czf "$work/rel/blaxel_Linux_$arch.tar.gz" -C "$work/stage" blaxel
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
(cd "$work/rel" && $sum "blaxel_Linux_$arch.tar.gz" > blaxel_9.9.9_checksums.txt)
cp "${INSTALL_SH:-$here/../../install.sh}" "$work/install.sh"
cat > "$work/cases.sh" <<'EOF'
#!/bin/sh
apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq sudo zsh curl ca-certificates >/dev/null
useradd -m -s /bin/zsh dev && echo 'dev ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dev
# curl for install.sh: GitHub's latest release is v9.9.9, served from /work/rel.
cat > /usr/local/bin/curl <<'CURL'
#!/bin/sh
url="" out="" head="" prev=""
for a in "$@"; do
  case "$a" in https://*) url=$a ;; --head) head=1 ;; esac
  [ "$prev" = "-o" ] && out=$a
  prev=$a
done
case "$url" in
  */toolkit/releases/latest) [ -n "$head" ] && { printf 'HTTP/2 302\r\nlocation: https://github.com/blaxel-ai/toolkit/releases/tag/v9.9.9\r\n'; exit 0; } ;;
  */toolkit/releases/download/v9.9.9/*) cp "/work/rel/${url##*/}" "$out"; exit $? ;;
esac
echo "curl: no fixture for $url" >&2; exit 22
CURL
chmod +x /usr/local/bin/curl
mkdir -p /opt/victim
status=0
pass() { echo "PASS $*"; }
bad() { echo "FAIL $*"; status=1; }
reset_home() { rm -rf /home/dev /opt/victim/* && mkdir -p /home/dev && chown dev:dev /home/dev; }
run() { su dev -c "cd /home/dev && sudo $* sh /work/install.sh" > /tmp/run.log 2>&1 < /dev/null || { bad "install failed"; cat /tmp/run.log; }; }

reset_home
run BL_INSTALL_SETUP=false
grep -q 'local/bin:$PATH' /home/dev/.zshrc && [ -s /home/dev/.zsh/completions/_bl ] && [ "$(stat -c %U /home/dev/.local/bin/bl /home/dev/.zsh/completions/_bl /home/dev/.zsh | sort -u)" = dev ] \
  && pass "sudo uses dev's zsh: PATH and completions, all owned by dev" || bad "plain sudo: $(cat /tmp/run.log)"

reset_home; su dev -c 'ln -s /opt/victim /home/dev/.zsh'
run BL_INSTALL_SETUP=false
[ -z "$(ls -A /opt/victim)" ] && [ "$(stat -c %U /opt/victim)" = root ] && pass "~/.zsh -> /opt/victim: nothing written or chowned there" || bad "~/.zsh link: $(ls -la /opt/victim) owner=$(stat -c %U /opt/victim)"
grep -q 'local/bin:$PATH' /home/dev/.zshrc && pass "  PATH still set up" || bad "  PATH: $(cat /tmp/run.log)"

reset_home; : > /opt/victim/zshrc; su dev -c 'ln -s /opt/victim/zshrc /home/dev/.zshrc'
run BL_INSTALL_SETUP=false
[ ! -s /opt/victim/zshrc ] && [ "$(stat -c %U /opt/victim/zshrc)" = root ] && grep -q 'export PATH=' /tmp/run.log \
  && pass "~/.zshrc -> /opt/victim/zshrc: left alone, the PATH line is printed instead" || bad "~/.zshrc link: $(cat /opt/victim/zshrc /tmp/run.log)"

reset_home; su dev -c 'mkdir /home/dev/dotfiles && : > /home/dev/dotfiles/zshrc && ln -s dotfiles/zshrc /home/dev/.zshrc'
run BL_INSTALL_SETUP=false
grep -q 'local/bin:$PATH' /home/dev/dotfiles/zshrc && [ "$(stat -c %U /home/dev/dotfiles/zshrc)" = dev ] && [ -L /home/dev/.zshrc ] \
  && pass "a dotfiles link inside the home is followed" || bad "dotfiles: $(cat /tmp/run.log)"

reset_home; su dev -c 'mkdir /home/dev/.claude'
run BL_INSTALL_SETUP=true BL_INSTALL_SKILLS=false BL_INSTALL_MCP=false DO_NOT_TRACK=1
grep -q 'Blaxel setup' /tmp/run.log && [ ! -e /home/dev/.agents/skills ] && ! grep -q blaxel /home/dev/.claude.json 2>/dev/null \
  && ! grep -q 'tracking: true' /home/dev/.blaxel/config.yaml 2>/dev/null \
  && pass "the user's choices reach bl setup under sudo" || bad "choices lost: $(cat /tmp/run.log /home/dev/.claude.json /home/dev/.blaxel/config.yaml 2>&1)"

reset_home; su dev -c 'mkdir /home/dev/.claude'
run BL_INSTALL_SETUP=true BL_INSTALL_SKILLS=false BL_INSTALL_TRACKING=false
grep -q '"blaxel"' /home/dev/.claude.json && [ "$(stat -c %U /home/dev/.claude.json)" = dev ] \
  && pass "bl setup runs as dev and adds the MCP servers" || bad "setup as dev: $(cat /tmp/run.log)"
exit $status
EOF
docker run --rm -v "$work:/work" ubuntu:22.04 sh /work/cases.sh
