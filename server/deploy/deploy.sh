#!/usr/bin/env bash
# Ngajio deploy. DRY RUN by default; --apply does it; --first-time also installs
# nginx site, snippet, systemd unit and /etc/ngajio/env (only files that do not exist yet).
# Touches ONLY ngajio paths on the shared server. Never runs certbot.
set -euo pipefail

HOST=43.173.9.126; RUSER=ubuntu
[[ "$HOST" == "43.173.9.126" && "$RUSER" == "ubuntu" ]] || { echo "refuse: wrong target"; exit 1; }
DOMAIN=ngajio.transbara.my.id
WEB=/var/www/html/ngajio; OPT=/opt/ngajio; ETC=/etc/ngajio; STAGE=/opt/ngajio/stage
APPLY=0; FIRST=0
for a in "$@"; do case "$a" in --apply) APPLY=1;; --first-time) FIRST=1;; *) echo "usage: $0 [--apply] [--first-time]"; exit 1;; esac; done

ROOT=$(cd "$(dirname "$0")/../.." && pwd); cd "$ROOT"
D=server/deploy
SSH="ssh -i $HOME/.ssh/id_ed25519 -o IdentitiesOnly=yes"
# macOS rsync is old: no --chmod. Run rsync as root remotely, fix owner/mode with sudo after.
RS="rsync -rlz -e '$SSH' --rsync-path='sudo rsync'"
step() { printf '\n== %s\n' "$*"; }
# run LOCALCMD...: print, execute only with --apply
run() { echo "  + $*"; if [[ $APPLY == 1 ]]; then eval "$@"; fi; }
remote() { run "$SSH $RUSER@$HOST \"$1\""; }

echo "Ngajio deploy -> $RUSER@$HOST ($DOMAIN)  mode: $([[ $APPLY == 1 ]] && echo APPLY || echo 'DRY RUN')$([[ $FIRST == 1 ]] && echo ' +first-time')"

step "1. local pre-checks (no network)"
if [[ -n "$(git status --porcelain -- server src)" ]]; then
  git status --short -- server src
  if [[ $APPLY == 1 ]]; then echo "ABORT: commit or stash changes in server/ and src/ first"; exit 1; fi
  echo "WARN: working tree not clean (fatal with --apply)"
fi
npm run build >/dev/null && echo "npm run build ok"
[[ -s build/index.html ]] || { echo "ABORT: build/index.html missing"; exit 1; }
make -C server vet && make -C server test >/dev/null && echo "go vet + go test ok"
make -C server build
FILEINFO=$(file server/bin/ngajio); echo "$FILEINFO"
[[ "$FILEINFO" == *"ELF 64-bit"*"x86-64"* ]] || { echo "ABORT: not linux amd64 ELF"; exit 1; }

step "2. remote commands ($([[ $APPLY == 1 ]] && echo running || echo 'only printed'))"
remote "sudo mkdir -p $WEB $OPT $STAGE"
run "$RS --delete build/ $RUSER@$HOST:$WEB/"
remote "sudo chown -R root:root $WEB && sudo find $WEB -type d -exec chmod 755 {} + && sudo find $WEB -type f -exec chmod 644 {} +"
run "$RS server/bin/ngajio $RUSER@$HOST:$OPT/ngajio.new"
remote "cd $OPT && sudo chown root:root ngajio.new && sudo chmod 755 ngajio.new && { [ ! -f ngajio ] || sudo cp -p ngajio ngajio.prev; } && sudo mv -f ngajio.new ngajio"

if [[ $FIRST == 1 ]]; then
  step "2b. first-time installs (skip files that already exist)"
  run "$RS $D/nginx/ngajio.transbara.my.id.conf $D/nginx/ngajio-headers.conf $D/ngajio.service $D/env.example $RUSER@$HOST:$STAGE/"
  remote "cd $STAGE && sudo install -d -m 700 -o root -g root $ETC && { [ -e $ETC/env ] || sudo install -m 600 -o root -g root env.example $ETC/env; } && { [ -e /etc/nginx/snippets/ngajio-headers.conf ] || sudo install -m 644 -o root -g root ngajio-headers.conf /etc/nginx/snippets/ngajio-headers.conf; } && { [ -e /etc/nginx/sites-available/$DOMAIN ] || sudo install -m 644 -o root -g root $DOMAIN.conf /etc/nginx/sites-available/$DOMAIN; } && sudo ln -sfn /etc/nginx/sites-available/$DOMAIN /etc/nginx/sites-enabled/$DOMAIN && { [ -e /etc/systemd/system/ngajio.service ] || sudo install -m 644 -o root -g root ngajio.service /etc/systemd/system/ngajio.service; } && sudo systemctl daemon-reload && sudo systemctl enable ngajio"
fi

step "3. nginx (test first, reload only if test passes)"
if [[ $FIRST == 1 ]]; then
  remote "sudo nginx -t || { sudo rm -f /etc/nginx/sites-enabled/$DOMAIN; echo 'nginx -t failed: removed ngajio symlink from sites-enabled, nothing reloaded' >&2; exit 1; } && sudo systemctl reload nginx"
else
  remote "sudo nginx -t && sudo systemctl reload nginx"
fi

step "4. service (never starts with empty GOOGLE_CLIENT_ID)"
GATE="sudo grep -Eq '^GOOGLE_CLIENT_ID=.+' $ETC/env"
if [[ $APPLY == 1 ]] && ! $SSH $RUSER@$HOST "$GATE"; then
  cat <<MSG
STOP: GOOGLE_CLIENT_ID is empty in $ETC/env on the server. Service NOT started.
  1. $SSH $RUSER@$HOST
  2. sudo nano $ETC/env      (paste the Client ID after GOOGLE_CLIENT_ID=)
  3. sudo systemctl restart ngajio   (or run: $0 --apply)
MSG
  exit 0
fi
remote "$GATE && sudo systemctl restart ngajio"

step "5. verify"
remote "sleep 2; curl -fsS http://127.0.0.1:8787/api/health; echo; curl -s -o /dev/null -w 'nginx / -> %{http_code}\n' -H 'Host: $DOMAIN' http://127.0.0.1/; curl -s -o /dev/null -w 'nginx /api/health -> %{http_code}\n' -H 'Host: $DOMAIN' http://127.0.0.1/api/health"
run "curl -s -o /dev/null -w 'http://$DOMAIN/ -> %{http_code} (000 = DNS not ready; 301 = after certbot)\n' --max-time 10 http://$DOMAIN/ || true"

step "summary"
echo "frontend -> $WEB | binary -> $OPT/ngajio (previous: ngajio.prev) | unit -> ngajio.service"
[[ $APPLY == 1 ]] && echo "Done. Next (first time): sudo certbot --nginx -d $DOMAIN --redirect" || echo "DRY RUN: nothing was sent. Re-run with --apply (add --first-time on first deploy)."
