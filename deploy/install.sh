#!/usr/bin/env bash
# Instala ou atualiza o TeatNet na VPS. Rodar como root, de dentro do clone:
#   git clone https://github.com/Taino-Edu/teatnet /opt/teatnet-src
#   TEATNET_ADMIN_EMAIL=voce@exemplo.com bash /opt/teatnet-src/deploy/install.sh
# Para atualizar depois, é só rodar o script de novo (ele faz git pull).
set -euo pipefail

SRC=/opt/teatnet-src
APP=/opt/teatnet
export PATH="$PATH:/usr/local/go/bin"

cd "$SRC"
git pull --ff-only

id teatnet >/dev/null 2>&1 || useradd --system --home-dir "$APP" --shell /usr/sbin/nologin teatnet
mkdir -p "$APP/data/uploads"

echo "==> compilando"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$APP/teatnet.new" .
mv "$APP/teatnet.new" "$APP/teatnet"

if [ ! -f "$APP/teatnet.env" ]; then
  sed "s|^TEATNET_ADMIN_EMAIL=.*|TEATNET_ADMIN_EMAIL=${TEATNET_ADMIN_EMAIL:-}|" deploy/teatnet.env.example > "$APP/teatnet.env"
  chmod 600 "$APP/teatnet.env"
fi
chown -R teatnet:teatnet "$APP"

echo "==> serviço"
install -m 644 deploy/teatnet.service /etc/systemd/system/teatnet.service
systemctl daemon-reload
systemctl enable --quiet teatnet
systemctl restart teatnet

echo "==> eventos iniciais"
sudo -u teatnet bash -c "set -a; . $APP/teatnet.env; $APP/teatnet -seed $SRC/seed/rio-preto.sql"

if ! grep -q "teatnet.tainodev.com.br" /etc/caddy/Caddyfile; then
  echo "==> caddy"
  cp /etc/caddy/Caddyfile "/etc/caddy/Caddyfile.bak-$(date +%Y%m%d%H%M%S)"
  printf '\n' >> /etc/caddy/Caddyfile
  cat deploy/Caddyfile >> /etc/caddy/Caddyfile
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
  systemctl reload caddy
fi

sleep 1
curl -fsS -o /dev/null -w "TeatNet respondeu HTTP %{http_code}\n" http://127.0.0.1:8091/
