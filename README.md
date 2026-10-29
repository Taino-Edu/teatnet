# TeatNet

Mapa colaborativo de teatro, dança, circo, música e outras artes em São José do Rio Preto.

Go + SQLite + HTMX + Leaflet/OpenStreetMap. Tudo vira um binário só (templates, CSS e JS embutidos);
o banco é o arquivo `data/teatnet.db`.

## Rodar local

```bash
go run . -dev
```

Abra http://localhost:8080. Com `-dev`:

- o link de login aparece na própria tela (e no log), sem precisar de e-mail;
- `admin@teatnet.test` entra como admin.

Carregar os eventos reais iniciais (pode repetir, não duplica):

```bash
go run . -seed seed/rio-preto.sql
```

## Como funciona

- **Público** vê o mapa e os eventos, sem conta. Pode *sugerir* um evento (vai para a moderação).
- **Artistas, grupos e espaços** entram por link mágico no e-mail (sem senha), têm página própria
  (`/artista/nome`) e painel com visualizações e pedidos de rota.
- **Moderação** (`/moderacao`): aprova/recusa envios, cadastra espaços e vê métricas.
  Quem tem 3 eventos aprovados vira *verificado* e publica direto.
- **Admin** (`/admin/usuarios`): define moderadores e verificados.

## Produção (VPS)

Na VPS, como root (pelo console da Cloud Prime):

```bash
git clone https://github.com/Taino-Edu/teatnet /opt/teatnet-src   # só na primeira vez
TEATNET_ADMIN_EMAIL=voce@exemplo.com bash /opt/teatnet-src/deploy/install.sh
```

O script compila, instala o serviço systemd (`teatnet`, porta 8091), carrega os eventos iniciais e
acrescenta o subdomínio ao Caddy usando o certificado de origem da Cloudflare. Para atualizar, rode-o de novo.
As variáveis ficam em `/opt/teatnet/teatnet.env` (modelo em `deploy/teatnet.env.example`).
Sem SMTP configurado, os links de login aparecem em `journalctl -u teatnet`.

E-mail (Brevo) e captcha (Turnstile): `bash /opt/teatnet-src/deploy/configure.sh` pergunta as chaves,
grava no `.env`, reinicia o serviço e manda um e-mail de teste (`teatnet -testmail voce@exemplo.com`).
Backup: copiar `data/teatnet.db` com `sqlite3 data/teatnet.db ".backup backup.db"` e a pasta `data/uploads`.

| Variável | Para quê |
|---|---|
| `TEATNET_BASE_URL` | endereço público, usado nos links de e-mail e no WhatsApp |
| `TEATNET_ADMIN_EMAIL` | esse e-mail vira admin ao entrar |
| `TEATNET_SMTP_*`, `TEATNET_MAIL_FROM` | envio do link de login |
| `TEATNET_TURNSTILE_*` | captcha da Cloudflare no login e nas sugestões |
| `TEATNET_BEHIND_CF=1` | usar o IP real do visitante vindo da Cloudflare |
