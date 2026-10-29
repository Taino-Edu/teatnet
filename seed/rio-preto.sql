-- Dados reais para o MVP, conferidos nas fontes em 29/10/2026 (cada evento tem o link em source_url).
-- Carregar com:  teatnet -seed seed/rio-preto.sql   (pode rodar mais de uma vez)
--
-- Coordenadas vieram do OpenStreetMap (Nominatim) pela rua, então o pino pode estar
-- a alguns metros da entrada: um moderador ajusta em /moderacao/espacos.

INSERT OR IGNORE INTO venues (slug, name, address, neighborhood, lat, lng, website) VALUES
  ('centro-cultural-sesi-rio-preto', 'Centro Cultural Sesi Rio Preto',
   'Av. Duque de Caxias, 4656', 'Vila Elvira', -20.8100765, -49.3703802, 'https://riopreto.sesisp.org.br'),
  ('cat-sesi-rio-preto', 'CAT Sesi Rio Preto',
   'Rua Caio Nogueira Bertozzi, 183', 'Vila Elvira', -20.8094997, -49.3618700, 'https://riopreto.sesisp.org.br');

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'tropicadelia-alucinacao-belchior', 'Tropicadelia toca “Alucinação” – 50 anos do clássico de Belchior', 'musica',
    'A banda Tropicadelia apresenta o álbum “Alucinação”, de Belchior, do começo ao fim, além de outros sucessos do compositor. O show é em formato power trio (guitarra, baixo e bateria) e acontece no Foyer do Centro Cultural.',
    v.id, v.name || ' · Foyer', v.address, v.lat, v.lng,
    'gratuito', 'Ingressos limitados, reserva pelo Meu Sesi', 'https://riopreto.sesisp.org.br', 'L',
    'https://riopreto.sesisp.org.br/noticia/banda-tropicadelia-toca-alucinacao---50-anos-do-classico-de-belchior-no-sesi-rio-preto',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'centro-cultural-sesi-rio-preto';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'polako-e-os-indieabrados', 'Polako & os Indieabrados – Autêntico & Visceral', 'musica',
    'Show de rock com músicas autorais, muita energia no palco e interação com o público, no Espaço Flamboyant do Centro Cultural Sesi.',
    v.id, v.name || ' · Espaço Flamboyant', v.address, v.lat, v.lng,
    'gratuito', 'Reserva pelo Meu Sesi', 'https://riopreto.sesisp.org.br', 'L',
    'https://riopreto.sesisp.org.br/noticia/polako--os-indieabrados-se-apresentam-no-centro-cultural-sesi-rio-preto-neste-sabado-31-de-outubro',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'centro-cultural-sesi-rio-preto';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'bruna-black-juventudes-antimisoginia', 'Bruna Black – Juventudes Antimisoginia', 'musica',
    'A cantora e compositora Bruna Black mistura MPB, R&B e ritmos regionais em um show que usa a música para falar do enfrentamento à misoginia.',
    v.id, v.name || ' · Ginásio', v.address, v.lat, v.lng,
    'gratuito', 'Ingressos limitados, reserva pelo Meu Sesi', 'https://riopreto.sesisp.org.br', 'L',
    'https://riopreto.sesisp.org.br/noticia/bruna-black-traz-seu-novo-show-para-o-centro-cultural-sesi-rio-preto-no-dia-06-de-novembro',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'cat-sesi-rio-preto';

INSERT OR IGNORE INTO event_sessions (event_id, starts_at)
SELECT id, '2026-10-30T19:00' FROM events WHERE slug = 'tropicadelia-alucinacao-belchior';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at)
SELECT id, '2026-10-31T19:00' FROM events WHERE slug = 'polako-e-os-indieabrados';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at)
SELECT id, '2026-11-06T19:00' FROM events WHERE slug = 'bruna-black-juventudes-antimisoginia';

-- ---------------------------------------------------------------------------
-- Segunda leva. Fontes: Rio Preto Cultural, RP Indica, Leve Ingresso e agenda da Prefeitura.
-- Classificação vazia = a fonte não informou.

INSERT OR IGNORE INTO venues (slug, name, address, neighborhood, lat, lng, website) VALUES
  ('sesc-rio-preto', 'Sesc Rio Preto', 'Av. Francisco das Chagas de Oliveira, 1333', 'Higienópolis',
   -20.8337103, -49.3937366, 'https://www.sescsp.org.br/unidades/rio-preto/'),
  ('teatro-municipal-paulo-moura', 'Teatro Municipal Paulo Moura', 'Prédio da Swift', 'Região Central',
   -20.8108795, -49.3691242, ''),
  ('vila-dionisio', 'Vila Dionísio', 'Av. Bady Bassitt, 3961', 'Vila Imperial',
   -20.8142175, -49.3905995, 'https://viladionisio.com.br'),
  ('recinto-de-exposicoes', 'Recinto de Exposições Alberto Bertelli Lucatto', 'Rua Daniel Antônio de Freitas', 'Distrito Industrial',
   -20.8202799, -49.4220971, ''),
  ('parque-municipal-olinda-tarraf', 'Parque Municipal Olinda Tarraf', 'Rua Olívia Adas Dib, s/n', 'Residencial SetSul II',
   -20.8647794, -49.3532527, '');

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'rodox-vila-dionisio', 'Rodox', 'musica',
    'A banda de rock Rodox volta aos palcos com a formação original: Rodolfo Abrantes, Fernando Schaefer, Patrick Laplan e Pedro Nogueira, com Victor Pradella como convidado. Abertura da casa às 19h.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', 'Meia R$ 120 · inteira R$ 240 (+ taxa)', 'https://leveingresso.com/comprar/541/05-11-rodox-sao-jose-do-rio-preto-sp', '18',
    'https://leveingresso.com/comprar/541/05-11-rodox-sao-jose-do-rio-preto-sp', 'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'vila-dionisio';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'chegada-do-papai-noel-olinda-tarraf', 'Chegada do Papai Noel', 'infantil',
    'Festa gratuita que abre a programação de Natal da cidade, com personagens, apresentações e a estreia da decoração natalina do parque. Pensada para crianças e famílias.',
    v.id, v.name, v.address, v.lat, v.lng,
    'gratuito', '', '', '',
    'https://www.riopretocultural.com.br/noticia/primeira-chegada-do-papai-noel-abre-a-magia-do-natal-em-rio-preto-no-dia-06-de-novembro-com-festa-gratuita-no-parque-municipal-olinda-tarraf',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'parque-municipal-olinda-tarraf';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'amae-festival-2026', 'Amaê Festival', 'musica',
    'Mais de 12 horas de música em dois palcos, misturando rap, trap, MPB e eletrônica. No line-up: Matuê, Marina Sena, Baco Exu do Blues, BK, Veigh, KayBlack, Vini Vici e outros. Abertura às 14h30.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', 'Setores pista, frontstage, lounge e backstage', '', '',
    'https://www.riopretocultural.com.br/noticia/amae-festival-reune-matue-marina-sena-baco-exu-do-blues-e-mais-atracoes-em-rio-preto-no-dia-07-de-novembro-no-recinto-de-exposicoes',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'recinto-de-exposicoes';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'bruno-sutter-50-anos-iron-maiden', 'Bruno Sutter celebra 50 anos de Iron Maiden', 'musica',
    'Bruno Sutter, conhecido pelo Massacration, revisita clássicos de várias fases do Iron Maiden em um show de heavy metal. O horário indicado é o da abertura da casa.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', '', 'https://articket.com.br', '18',
    'https://www.riopretocultural.com.br/noticia/bruno-sutter-celebra-50-anos-de-iron-maiden-em-show-no-palco-do-vila-dionisio-rio-preto-no-dia-15-de-novembro',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'vila-dionisio';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'silva-canta-canta-silva', 'Silva – Silva Canta / Canta Silva', 'musica',
    'Show intimista que celebra os 15 anos de carreira do cantor, com canções de diferentes fases e músicas dele gravadas por outros artistas. Ingressos online a partir de 10/11, às 17h, e nas unidades do Sesc a partir de 11/11.',
    v.id, v.name || ' · Ginásio de Esportes', v.address, v.lat, v.lng,
    'pago', 'A partir de R$ 18', 'https://centralrelacionamento.sescsp.org.br', 'L',
    'https://www.rpindica.com.br/2026/10/cantor-silva-traz-rio-preto-turne-com.html',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'sesc-rio-preto';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'mundo-torajo-viagem-ao-mundo-de-frutas', 'Mundo Torajo – Uma Viagem ao Mundo de Frutas', 'infantil',
    'O Torajo e seus personagens saem da internet e sobem ao palco numa aventura colorida, feita para as crianças e para quem acompanha o canal.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', '', 'https://ingressodigital.com', '',
    'https://www.riopretocultural.com.br/noticia/o-universo-do-mundo-torajo-vai-sair-das-telas-e-ganhar-o-palco-em-rio-preto-no-dia-22-de-novembro-as-15h-no-teatro-municipal-paulo-moura',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'teatro-municipal-paulo-moura';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'evinha-sesc-rio-preto', 'Evinha', 'musica',
    'A cantora percorre diferentes fases da carreira, com sucessos conhecidos e faixas do álbum Cartão Postal, acompanhada por uma banda de sete músicos. Ingressos online a partir de 17/11, às 17h, e presencialmente a partir de 18/11.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', '', 'https://centralrelacionamento.sescsp.org.br', '',
    'https://www.riopretocultural.com.br/noticia/evinha-traz-sucessos-que-atravessam-geracoes-em-show-no-sesc-rio-preto-no-dia-28-de-novembro',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'sesc-rio-preto';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'rodrigo-barba-bloco-do-eu-sozinho-25-anos', 'Rodrigo Barba – 25 anos de “Bloco do Eu Sozinho”', 'musica',
    'O baterista do Los Hermanos toca na íntegra, e na ordem original, o segundo disco da banda, além de versões que o grupo fazia ao vivo na época. Abertura da casa às 18h.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', 'Ingressos na Sympla', '', '',
    'https://www.riopretocultural.com.br/noticia/rodrigo-barba-celebra-25-anos-de-bloco-do-eu-sozinho-em-show-no-vila-dionisio-no-dia-29-de-novembro',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'vila-dionisio';

INSERT OR IGNORE INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
    price_type, price_info, ticket_url, age_rating, source_url, status, suggested_name)
SELECT 'padre-patrick-pecado-e-nao-rir', 'Padre Patrick – Pecado É Não Rir', 'humor',
    'Show de humor do Padre Patrick no Teatro Municipal Paulo Moura.',
    v.id, v.name, v.address, v.lat, v.lng,
    'pago', '', '', '',
    'https://eventos.riopreto.sp.gov.br/agenda',
    'publicado', 'Curadoria TeatNet'
FROM venues v WHERE v.slug = 'teatro-municipal-paulo-moura';

INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-05T20:00' FROM events WHERE slug = 'rodox-vila-dionisio';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-06T19:00' FROM events WHERE slug = 'chegada-do-papai-noel-olinda-tarraf';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-07T14:30' FROM events WHERE slug = 'amae-festival-2026';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-15T19:00' FROM events WHERE slug = 'bruno-sutter-50-anos-iron-maiden';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-18T21:00' FROM events WHERE slug = 'silva-canta-canta-silva';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-22T15:00' FROM events WHERE slug = 'mundo-torajo-viagem-ao-mundo-de-frutas';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-28T20:00' FROM events WHERE slug = 'evinha-sesc-rio-preto';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-11-29T19:30' FROM events WHERE slug = 'rodrigo-barba-bloco-do-eu-sozinho-25-anos';
INSERT OR IGNORE INTO event_sessions (event_id, starts_at) SELECT id, '2026-12-12T18:00' FROM events WHERE slug = 'padre-patrick-pecado-e-nao-rir';
