#!/usr/bin/env bash
# Configura o e-mail (SMTP do Brevo) e o captcha (Cloudflare Turnstile) do TeatNet.
# Rodar como root na VPS:
#   bash /opt/teatnet-src/deploy/configure.sh
# Pergunta cada valor (Enter mantém o atual; as chaves secretas não aparecem na tela),
# grava em /opt/teatnet/teatnet.env, reinicia o serviço e manda um e-mail de teste.
set -euo pipefail

APP=/opt/teatnet
ENV=$APP/teatnet.env
[ -f "$ENV" ] || { echo "Rode antes o deploy/install.sh"; exit 1; }

current() { grep -m1 "^$1=" "$ENV" | cut -d= -f2- || true; }

set_var() {
  grep -v "^$1=" "$ENV" > "$ENV.tmp" || true
  printf '%s=%s\n' "$1" "$2" >> "$ENV.tmp"
  mv "$ENV.tmp" "$ENV"
}

ask() { # ask VAR "pergunta" [padrão]
  local cur def v
  cur=$(current "$1"); def=${cur:-${3:-}}
  read -r -p "$2${def:+ [$def]}: " v
  set_var "$1" "${v:-$def}"
}

ask_secret() { # ask_secret VAR "pergunta"
  local cur v hint=""
  cur=$(current "$1")
  [ -n "$cur" ] && hint=" [Enter mantém a atual]"
  read -r -s -p "$2$hint: " v; echo
  [ -n "$v" ] && set_var "$1" "$v"
  return 0
}

echo "== E-mail (Brevo: Configurações > SMTP & API > SMTP) =="
ask        TEATNET_SMTP_HOST "Servidor SMTP" "smtp-relay.brevo.com"
ask        TEATNET_SMTP_PORT "Porta" "587"
ask        TEATNET_SMTP_USER "Login SMTP (ex.: 1a2b3c001@smtp-brevo.com)"
ask_secret TEATNET_SMTP_PASS "Chave SMTP"
ask        TEATNET_MAIL_FROM "Remetente" "no-reply@tainodev.com.br"

echo
echo "== Captcha (Cloudflare > Turnstile > TeatNet) =="
ask        TEATNET_TURNSTILE_SITE "Site key"
ask_secret TEATNET_TURNSTILE_SECRET "Secret key"

chmod 600 "$ENV"
chown teatnet:teatnet "$ENV"
systemctl restart teatnet
sleep 1
systemctl is-active --quiet teatnet && echo "Serviço reiniciado."

echo
read -r -p "Enviar e-mail de teste para (Enter pula): " to
if [ -n "$to" ]; then
  sudo -u teatnet bash -c 'set -a; . "$1"; "$2" -testmail "$3"' _ "$ENV" "$APP/teatnet" "$to"
fi
