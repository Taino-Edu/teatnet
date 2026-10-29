package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errNotFound = errors.New("não encontrado")

type scanner interface{ Scan(dest ...any) error }

type Store struct{ db *sql.DB }

// ---------- usuários ----------

type User struct {
	ID        int64
	Email     string
	Name      string
	Slug      string
	Kind      string
	Role      string
	Bio       string
	Instagram string
	Website   string
	Avatar    string
	Verified  bool
	CreatedAt string
}

func (u *User) IsMod() bool   { return u != nil && (u.Role == "moderador" || u.Role == "admin") }
func (u *User) IsAdmin() bool { return u != nil && u.Role == "admin" }

// Moderadores e artistas verificados publicam direto, sem passar pela fila.
func (u *User) Trusted() bool { return u != nil && (u.Verified || u.IsMod()) }

const userCols = `id, email, name, COALESCE(slug, ''), kind, role, bio, instagram, website, avatar, verified, created_at`

func scanUser(sc scanner) (*User, error) {
	u := &User{}
	err := sc.Scan(&u.ID, &u.Email, &u.Name, &u.Slug, &u.Kind, &u.Role, &u.Bio, &u.Instagram, &u.Website, &u.Avatar, &u.Verified, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return u, err
}

func (s *Store) UserByID(id int64) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *Store) UserBySlug(slug string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE slug = ?`, slug))
}

// FindOrCreateUser devolve o usuário do e-mail, criando a conta no primeiro acesso.
func (s *Store) FindOrCreateUser(email, adminEmail string) (u *User, created bool, err error) {
	u, err = scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE email = ?`, email))
	if err == nil {
		if adminEmail != "" && strings.EqualFold(email, adminEmail) && u.Role != "admin" {
			_, err = s.db.Exec(`UPDATE users SET role = 'admin' WHERE id = ?`, u.ID)
			u.Role = "admin"
		}
		return u, false, err
	}
	if !errors.Is(err, errNotFound) {
		return nil, false, err
	}
	role := "membro"
	if adminEmail != "" && strings.EqualFold(email, adminEmail) {
		role = "admin"
	}
	res, err := s.db.Exec(`INSERT INTO users (email, role) VALUES (?, ?)`, email, role)
	if err != nil {
		return nil, false, err
	}
	id, _ := res.LastInsertId()
	u, err = s.UserByID(id)
	return u, true, err
}

func (s *Store) UpdateProfile(u *User) error {
	if u.Slug == "" {
		slug, err := s.uniqueSlug("users", slugify(u.Name))
		if err != nil {
			return err
		}
		u.Slug = slug
	}
	_, err := s.db.Exec(`UPDATE users SET name = ?, slug = ?, kind = ?, bio = ?, instagram = ?, website = ?, avatar = ? WHERE id = ?`,
		u.Name, u.Slug, u.Kind, u.Bio, u.Instagram, u.Website, u.Avatar, u.ID)
	return err
}

func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query(`SELECT ` + userCols + ` FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) SetUserAccess(id int64, role string, verified bool) error {
	_, err := s.db.Exec(`UPDATE users SET role = ?, verified = ? WHERE id = ?`, role, verified, id)
	return err
}

// uniqueSlug acrescenta -2, -3... até achar um slug livre na tabela.
func (s *Store) uniqueSlug(table, base string) (string, error) {
	slug := base
	for i := 2; ; i++ {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE slug = ?`, slug).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// ---------- login e sessões ----------

func (s *Store) CreateLoginToken(hash, email string, expires time.Time) error {
	_, err := s.db.Exec(`INSERT INTO login_tokens (token_hash, email, expires_at) VALUES (?, ?, ?)`, hash, email, expires.Unix())
	return err
}

// ConsumeLoginToken marca o token como usado e devolve o e-mail; cada link só funciona uma vez.
func (s *Store) ConsumeLoginToken(hash string) (string, error) {
	var email string
	err := s.db.QueryRow(`UPDATE login_tokens SET used = 1 WHERE token_hash = ? AND used = 0 AND expires_at > ? RETURNING email`,
		hash, time.Now().Unix()).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNotFound
	}
	return email, err
}

func (s *Store) CreateSession(hash string, userID int64, expires time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`, hash, userID, expires.Unix())
	return err
}

func (s *Store) UserBySession(hash string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users
		WHERE id = (SELECT user_id FROM sessions WHERE token_hash = ? AND expires_at > ?)`, hash, time.Now().Unix()))
}

func (s *Store) DeleteSession(hash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hash)
	return err
}

func (s *Store) DeleteExpired() error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM login_tokens WHERE expires_at < ?`, now)
	return err
}

// ---------- espaços ----------

type Venue struct {
	ID            int64
	Slug          string
	Name          string
	Address       string
	Neighborhood  string
	Lat, Lng      float64
	Accessibility string
	Website       string
}

const venueCols = `id, slug, name, address, neighborhood, lat, lng, accessibility, website`

func scanVenue(sc scanner) (*Venue, error) {
	v := &Venue{}
	err := sc.Scan(&v.ID, &v.Slug, &v.Name, &v.Address, &v.Neighborhood, &v.Lat, &v.Lng, &v.Accessibility, &v.Website)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return v, err
}

func (s *Store) VenueByID(id int64) (*Venue, error) {
	return scanVenue(s.db.QueryRow(`SELECT `+venueCols+` FROM venues WHERE id = ?`, id))
}

func (s *Store) VenueBySlug(slug string) (*Venue, error) {
	return scanVenue(s.db.QueryRow(`SELECT `+venueCols+` FROM venues WHERE slug = ?`, slug))
}

func (s *Store) ListVenues() ([]*Venue, error) {
	rows, err := s.db.Query(`SELECT ` + venueCols + ` FROM venues ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Venue
	for rows.Next() {
		v, err := scanVenue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SaveVenue(v *Venue) error {
	if v.ID == 0 {
		slug, err := s.uniqueSlug("venues", slugify(v.Name))
		if err != nil {
			return err
		}
		v.Slug = slug
		res, err := s.db.Exec(`INSERT INTO venues (slug, name, address, neighborhood, lat, lng, accessibility, website) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Slug, v.Name, v.Address, v.Neighborhood, v.Lat, v.Lng, v.Accessibility, v.Website)
		if err != nil {
			return err
		}
		v.ID, _ = res.LastInsertId()
		return nil
	}
	_, err := s.db.Exec(`UPDATE venues SET name = ?, address = ?, neighborhood = ?, lat = ?, lng = ?, accessibility = ?, website = ? WHERE id = ?`,
		v.Name, v.Address, v.Neighborhood, v.Lat, v.Lng, v.Accessibility, v.Website, v.ID)
	return err
}

// ---------- eventos ----------

type Event struct {
	ID             int64
	Slug           string
	Title          string
	Category       string
	Description    string
	VenueID        int64
	PlaceName      string
	Address        string
	Lat, Lng       float64
	PriceType      string
	PriceInfo      string
	TicketURL      string
	AgeRating      string
	Accessibility  string
	Poster         string
	SourceURL      string
	Status         string
	ReviewNote     string
	AuthorID       int64
	SuggestedName  string
	SuggestedEmail string
	Views          int
	Directions     int
	CreatedAt      string
	UpdatedAt      string

	AuthorName string
	AuthorSlug string
	VenueSlug  string
	Sessions   []string
}

const eventSelect = `SELECT e.id, e.slug, e.title, e.category, e.description, COALESCE(e.venue_id, 0), e.place_name, e.address,
	e.lat, e.lng, e.price_type, e.price_info, e.ticket_url, e.age_rating, e.accessibility, e.poster, e.source_url,
	e.status, e.review_note, COALESCE(e.author_id, 0), e.suggested_name, e.suggested_email, e.views, e.directions,
	e.created_at, e.updated_at, COALESCE(u.name, ''), COALESCE(u.slug, ''), COALESCE(v.slug, '')
	FROM events e LEFT JOIN users u ON u.id = e.author_id LEFT JOIN venues v ON v.id = e.venue_id`

func (s *Store) scanEvent(sc scanner) (*Event, error) {
	e := &Event{}
	err := sc.Scan(&e.ID, &e.Slug, &e.Title, &e.Category, &e.Description, &e.VenueID, &e.PlaceName, &e.Address,
		&e.Lat, &e.Lng, &e.PriceType, &e.PriceInfo, &e.TicketURL, &e.AgeRating, &e.Accessibility, &e.Poster, &e.SourceURL,
		&e.Status, &e.ReviewNote, &e.AuthorID, &e.SuggestedName, &e.SuggestedEmail, &e.Views, &e.Directions,
		&e.CreatedAt, &e.UpdatedAt, &e.AuthorName, &e.AuthorSlug, &e.VenueSlug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT starts_at FROM event_sessions WHERE event_id = ? ORDER BY starts_at`, e.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return nil, err
		}
		e.Sessions = append(e.Sessions, st)
	}
	return e, rows.Err()
}

func (s *Store) EventBySlug(slug string) (*Event, error) {
	return s.scanEvent(s.db.QueryRow(eventSelect+` WHERE e.slug = ?`, slug))
}

func (s *Store) EventByID(id int64) (*Event, error) {
	return s.scanEvent(s.db.QueryRow(eventSelect+` WHERE e.id = ?`, id))
}

// SaveEvent cria ou atualiza o evento e substitui todas as sessões, numa transação só.
func (s *Store) SaveEvent(e *Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var venue any
	if e.VenueID != 0 {
		venue = e.VenueID
	}
	var author any
	if e.AuthorID != 0 {
		author = e.AuthorID
	}
	if e.ID == 0 {
		e.Slug = slugify(e.Title) + "-" + randHex(2)
		res, err := tx.Exec(`INSERT INTO events (slug, title, category, description, venue_id, place_name, address, lat, lng,
			price_type, price_info, ticket_url, age_rating, accessibility, poster, source_url, status, author_id, suggested_name, suggested_email)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Slug, e.Title, e.Category, e.Description, venue, e.PlaceName, e.Address, e.Lat, e.Lng,
			e.PriceType, e.PriceInfo, e.TicketURL, e.AgeRating, e.Accessibility, e.Poster, e.SourceURL, e.Status, author, e.SuggestedName, e.SuggestedEmail)
		if err != nil {
			return err
		}
		e.ID, _ = res.LastInsertId()
	} else {
		_, err := tx.Exec(`UPDATE events SET title = ?, category = ?, description = ?, venue_id = ?, place_name = ?, address = ?, lat = ?, lng = ?,
			price_type = ?, price_info = ?, ticket_url = ?, age_rating = ?, accessibility = ?, poster = ?, source_url = ?, status = ?,
			updated_at = datetime('now') WHERE id = ?`,
			e.Title, e.Category, e.Description, venue, e.PlaceName, e.Address, e.Lat, e.Lng,
			e.PriceType, e.PriceInfo, e.TicketURL, e.AgeRating, e.Accessibility, e.Poster, e.SourceURL, e.Status, e.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM event_sessions WHERE event_id = ?`, e.ID); err != nil {
			return err
		}
	}
	for _, st := range e.Sessions {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO event_sessions (event_id, starts_at) VALUES (?, ?)`, e.ID, st); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetEventStatus(id int64, status, note string) error {
	_, err := s.db.Exec(`UPDATE events SET status = ?, review_note = ?, updated_at = datetime('now') WHERE id = ?`, status, note, id)
	return err
}

// VerifyIfEarned dá o selo de verificado a quem já teve 3 eventos aprovados.
func (s *Store) VerifyIfEarned(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET verified = 1 WHERE id = ? AND verified = 0
		AND (SELECT COUNT(*) FROM events WHERE author_id = ? AND status = 'publicado') >= 3`, userID, userID)
	return err
}

func (s *Store) IncEventCounter(id int64, column string) {
	if column != "views" && column != "directions" {
		return
	}
	s.db.Exec(`UPDATE events SET `+column+` = `+column+` + 1 WHERE id = ?`, id)
}

// Card é o resumo de um evento usado no mapa, nas listas e nas páginas de perfil.
type Card struct {
	ID        int64   `json:"id"`
	Slug      string  `json:"slug"`
	Title     string  `json:"title"`
	Category  string  `json:"category"`
	CatLabel  string  `json:"categoryLabel"`
	Place     string  `json:"place"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Free      bool    `json:"free"`
	Price     string  `json:"price"`
	Poster    string  `json:"poster"`
	Next      string  `json:"next"`
	NextLabel string  `json:"nextLabel"`
	DayKey    string  `json:"dayKey"`
	DayLabel  string  `json:"dayLabel"`
	Time      string  `json:"time"`
	Weekday   string  `json:"weekday"`
	DayNum    int     `json:"dayNum"`
	Month     string  `json:"month"`
	Sessions  int     `json:"sessions"`
}

type CardFilter struct {
	From, To string
	Category string
	FreeOnly bool
	AuthorID int64
	VenueID  int64
	Limit    int
}

// UpcomingCards lista eventos publicados com sessão dentro de [From, To), pela próxima data.
func (s *Store) UpcomingCards(f CardFilter) ([]Card, error) {
	q := `SELECT e.id, e.slug, e.title, e.category, e.place_name, e.lat, e.lng, e.price_type, e.price_info, e.poster,
		MIN(s.starts_at), COUNT(s.id)
		FROM events e JOIN event_sessions s ON s.event_id = e.id
		WHERE e.status = 'publicado' AND s.starts_at >= ? AND s.starts_at < ?`
	args := []any{f.From, f.To}
	if f.Category != "" {
		q += ` AND e.category = ?`
		args = append(args, f.Category)
	}
	if f.FreeOnly {
		q += ` AND e.price_type = 'gratuito'`
	}
	if f.AuthorID != 0 {
		q += ` AND e.author_id = ?`
		args = append(args, f.AuthorID)
	}
	if f.VenueID != 0 {
		q += ` AND e.venue_id = ?`
		args = append(args, f.VenueID)
	}
	if f.Limit == 0 {
		f.Limit = 500
	}
	q += ` GROUP BY e.id ORDER BY MIN(s.starts_at) LIMIT ?`
	args = append(args, f.Limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		var c Card
		var priceType, priceInfo string
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Category, &c.Place, &c.Lat, &c.Lng, &priceType, &priceInfo, &c.Poster, &c.Next, &c.Sessions); err != nil {
			return nil, err
		}
		c.CatLabel = categoryLabel(c.Category)
		c.Free = priceType == "gratuito"
		c.Price = priceLabel(priceType, priceInfo)
		c.NextLabel = fmtSession(c.Next)
		c.DayLabel = relativeDay(c.Next)
		c.Time = fmtTime(c.Next)
		if t, ok := parseSession(c.Next); ok {
			c.DayKey = t.Format("2006-01-02")
			c.Weekday = weekdays[t.Weekday()]
			c.DayNum = t.Day()
			c.Month = months[t.Month()-1]
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Row é a linha de evento do painel do artista e da fila de moderação.
type Row struct {
	ID             int64
	Slug           string
	Title          string
	Category       string
	Description    string
	PlaceName      string
	Poster         string
	Status         string
	ReviewNote     string
	SourceURL      string
	AuthorName     string
	AuthorSlug     string
	AuthorVerified bool
	SuggestedName  string
	SuggestedEmail string
	Views          int
	Directions     int
	First, Last    string
	SessionCount   int
	CreatedAt      string
}

func (s *Store) rows(where string, args ...any) ([]Row, error) {
	rows, err := s.db.Query(`SELECT e.id, e.slug, e.title, e.category, e.description, e.place_name, e.poster, e.status, e.review_note,
		e.source_url, COALESCE(u.name, ''), COALESCE(u.slug, ''), COALESCE(u.verified, 0), e.suggested_name, e.suggested_email,
		e.views, e.directions,
		COALESCE((SELECT MIN(starts_at) FROM event_sessions WHERE event_id = e.id), ''),
		COALESCE((SELECT MAX(starts_at) FROM event_sessions WHERE event_id = e.id), ''),
		(SELECT COUNT(*) FROM event_sessions WHERE event_id = e.id),
		e.created_at
		FROM events e LEFT JOIN users u ON u.id = e.author_id WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.Category, &r.Description, &r.PlaceName, &r.Poster, &r.Status, &r.ReviewNote,
			&r.SourceURL, &r.AuthorName, &r.AuthorSlug, &r.AuthorVerified, &r.SuggestedName, &r.SuggestedEmail,
			&r.Views, &r.Directions, &r.First, &r.Last, &r.SessionCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) EventsByAuthor(userID int64) ([]Row, error) {
	return s.rows(`e.author_id = ? ORDER BY e.created_at DESC`, userID)
}

func (s *Store) PendingEvents() ([]Row, error) {
	return s.rows(`e.status = 'pendente' ORDER BY e.created_at`)
}

func (s *Store) TopEvents(limit int) ([]Row, error) {
	return s.rows(`e.status = 'publicado' ORDER BY e.views DESC LIMIT ?`, limit)
}

// ---------- métricas ----------

func (s *Store) Bump(key string) {
	s.db.Exec(`INSERT INTO daily_stats (day, key, count) VALUES (?, ?, 1)
		ON CONFLICT (day, key) DO UPDATE SET count = count + 1`, nowLocal().Format("2006-01-02"), key)
}

type Metrics struct {
	Published, Upcoming, Pending int
	Artists, Venues, Users       int
	Last30                       map[string]int
	ByCategory                   []struct {
		Label string
		Count int
	}
}

func (s *Store) Metrics() (*Metrics, error) {
	m := &Metrics{Last30: map[string]int{}}
	now := nowLocal().Format(sessLayout)
	counts := []struct {
		dst *int
		q   string
		arg []any
	}{
		{&m.Published, `SELECT COUNT(*) FROM events WHERE status = 'publicado'`, nil},
		{&m.Upcoming, `SELECT COUNT(DISTINCT e.id) FROM events e JOIN event_sessions s ON s.event_id = e.id WHERE e.status = 'publicado' AND s.starts_at >= ?`, []any{now}},
		{&m.Pending, `SELECT COUNT(*) FROM events WHERE status = 'pendente'`, nil},
		{&m.Artists, `SELECT COUNT(DISTINCT author_id) FROM events WHERE status = 'publicado' AND author_id IS NOT NULL`, nil},
		{&m.Venues, `SELECT COUNT(*) FROM venues`, nil},
		{&m.Users, `SELECT COUNT(*) FROM users`, nil},
	}
	for _, c := range counts {
		if err := s.db.QueryRow(c.q, c.arg...).Scan(c.dst); err != nil {
			return nil, err
		}
	}

	since := nowLocal().AddDate(0, 0, -30).Format("2006-01-02")
	rows, err := s.db.Query(`SELECT key, SUM(count) FROM daily_stats WHERE day >= ? GROUP BY key`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		m.Last30[k] = n
	}

	crows, err := s.db.Query(`SELECT category, COUNT(*) FROM events WHERE status = 'publicado' GROUP BY category ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var cat string
		var n int
		if err := crows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		m.ByCategory = append(m.ByCategory, struct {
			Label string
			Count int
		}{categoryLabel(cat), n})
	}
	return m, crows.Err()
}
