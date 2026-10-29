#!/usr/bin/env bash
# Instala ou atualiza o TeatNet na VPS. Rodar como root, de dentro do clone:
#   git clone https://github.com/Taino-Edu/teatnet /opt/teatnet-src
#   TEATNET_ADMIN_EMAIL=voce@exemplo.com bash /opt/teatnet-src/deploy/install.sh
# Para atualizar depois, é só rodar o script de novo (ele faz git pull).
#
# Permissões: tudo é de root, exceto /opt/teatnet/data (banco e uploads), que é do usuário
# do serviço. Assim, mesmo com o processo do site comprometido, ele não consegue plantar
# arquivos ou links onde este script (root) escreve.
set -euo pipefail

SRC=/opt/teatnet-src
APP=/opt/teatnet
CONF=/etc/teatnet
ENV=$CONF/teatnet.env
export PATH="$PATH:/usr/local/go/bin"

cd "$SRC"
# Atualiza o código e reexecuta a versão nova deste script.
if [ -z "${TEATNET_PULLED:-}" ]; then
  git pull --ff-only
  TEATNET_PULLED=1 exec bash "$SRC/deploy/install.sh" "$@"
fi

id teatnet >/dev/null 2>&1 || useradd --system --home-dir "$APP" --shell /usr/sbin/nologin teatnet

install -d -o root -g root -m 755 "$APP"
install -d -o teatnet -g teatnet -m 750 "$APP/data" "$APP/data/uploads"
install -d -o root -g teatnet -m 750 "$CONF"

# Instalações antigas guardavam o .env dentro de /opt/teatnet: move para /etc/teatnet.
if [ ! -e "$ENV" ] && [ -f "$APP/teatnet.env" ] && [ ! -L "$APP/teatnet.env" ]; then
  mv "$APP/teatnet.env" "$ENV"
fi
if [ ! -e "$ENV" ]; then
  sed "s|^TEATNET_ADMIN_EMAIL=.*|TEATNET_ADMIN_EMAIL=${TEATNET_ADMIN_EMAIL:-}|" deploy/teatnet.env.example > "$ENV"
fi
[ -L "$ENV" ] && { echo "$ENV é um link simbólico; abortando"; exit 1; }
chown root:teatnet "$ENV"
chmod 640 "$ENV"

echo "==> compilando"
tmp=$(mktemp "$APP/.teatnet.XXXXXX")
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$tmp" .
chown root:root "$tmp"
chmod 755 "$tmp"
mv -f "$tmp" "$APP/teatnet"

echo "==> serviço"
install -m 644 deploy/teatnet.service /etc/systemd/system/teatnet.service
systemctl daemon-reload
systemctl enable --quiet teatnet
systemctl restart teatnet

echo "==> eventos iniciais"
sudo -u teatnet bash -c 'set -a; . "$1"; "$2" -seed "$3"' _ "$ENV" "$APP/teatnet" "$SRC/seed/rio-preto.sql"

if ! grep -q "teatnet.tainodev.com.br" /etc/caddy/Caddyfile; then
  echo "==> caddy"
  # monta a configuração nova ao lado e só troca o arquivo se ela for válida
  { cat /etc/caddy/Caddyfile; printf '\n'; cat deploy/Caddyfile; } > /etc/caddy/Caddyfile.teatnet
  caddy validate --config /etc/caddy/Caddyfile.teatnet --adapter caddyfile
  cp /etc/caddy/Caddyfile "/etc/caddy/Caddyfile.bak-$(date +%Y%m%d%H%M%S)"
  mv /etc/caddy/Caddyfile.teatnet /etc/caddy/Caddyfile
  systemctl reload caddy
fi

sleep 1
curl -fsS -o /dev/null -w "TeatNet respondeu HTTP %{http_code}\n" http://127.0.0.1:8091/
