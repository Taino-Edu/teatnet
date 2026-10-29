CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    email      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name       TEXT    NOT NULL DEFAULT '',
    slug       TEXT    UNIQUE,
    kind       TEXT    NOT NULL DEFAULT 'artista' CHECK (kind IN ('artista', 'espaco')),
    role       TEXT    NOT NULL DEFAULT 'membro' CHECK (role IN ('membro', 'moderador', 'admin')),
    bio        TEXT    NOT NULL DEFAULT '',
    instagram  TEXT    NOT NULL DEFAULT '',
    website    TEXT    NOT NULL DEFAULT '',
    avatar     TEXT    NOT NULL DEFAULT '',
    verified   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Tokens do link mágico: guardamos só o hash, nunca o token.
CREATE TABLE login_tokens (
    token_hash TEXT    PRIMARY KEY,
    email      TEXT    NOT NULL,
    expires_at INTEGER NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

CREATE TABLE venues (
    id            INTEGER PRIMARY KEY,
    slug          TEXT    NOT NULL UNIQUE,
    name          TEXT    NOT NULL,
    address       TEXT    NOT NULL DEFAULT '',
    neighborhood  TEXT    NOT NULL DEFAULT '',
    lat           REAL    NOT NULL,
    lng           REAL    NOT NULL,
    accessibility TEXT    NOT NULL DEFAULT '',
    website       TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- O local é sempre copiado para o evento (mesmo vindo de um espaço cadastrado),
-- assim a consulta do mapa não precisa de JOIN e o evento não muda se o espaço for editado.
CREATE TABLE events (
    id              INTEGER PRIMARY KEY,
    slug            TEXT    NOT NULL UNIQUE,
    title           TEXT    NOT NULL,
    category        TEXT    NOT NULL,
    description     TEXT    NOT NULL DEFAULT '',
    venue_id        INTEGER REFERENCES venues (id) ON DELETE SET NULL,
    place_name      TEXT    NOT NULL DEFAULT '',
    address         TEXT    NOT NULL DEFAULT '',
    lat             REAL    NOT NULL,
    lng             REAL    NOT NULL,
    price_type      TEXT    NOT NULL DEFAULT 'gratuito' CHECK (price_type IN ('gratuito', 'pago', 'contribuicao')),
    price_info      TEXT    NOT NULL DEFAULT '',
    ticket_url      TEXT    NOT NULL DEFAULT '',
    age_rating      TEXT    NOT NULL DEFAULT 'L',
    accessibility   TEXT    NOT NULL DEFAULT '',
    poster          TEXT    NOT NULL DEFAULT '',
    source_url      TEXT    NOT NULL DEFAULT '',
    status          TEXT    NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'publicado', 'recusado')),
    review_note     TEXT    NOT NULL DEFAULT '',
    author_id       INTEGER REFERENCES users (id) ON DELETE SET NULL,
    suggested_name  TEXT    NOT NULL DEFAULT '',
    suggested_email TEXT    NOT NULL DEFAULT '',
    views           INTEGER NOT NULL DEFAULT 0,
    directions      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX events_status ON events (status);
CREATE INDEX events_author ON events (author_id);
CREATE INDEX events_venue ON events (venue_id);

-- Uma linha por apresentação. starts_at no horário de Brasília, "2006-01-02T15:04".
CREATE TABLE event_sessions (
    id        INTEGER PRIMARY KEY,
    event_id  INTEGER NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    starts_at TEXT    NOT NULL,
    UNIQUE (event_id, starts_at)
);
CREATE INDEX event_sessions_starts ON event_sessions (starts_at);

-- Contadores diários para as métricas da apresentação (visitas, rotas, compartilhamentos...).
CREATE TABLE daily_stats (
    day   TEXT    NOT NULL,
    key   TEXT    NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, key)
);
