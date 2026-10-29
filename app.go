package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type App struct {
	cfg     Config
	store   *Store
	pages   map[string]*template.Template
	mailer  Mailer
	limiter *rateLimiter
	imgSem  chan struct{} // no máximo 2 imagens sendo processadas ao mesmo tempo (2 núcleos)
	version string

	geoMu   sync.Mutex
	geoLast time.Time
}

func newApp(cfg Config, db *sql.DB) (*App, error) {
	a := &App{
		cfg:     cfg,
		store:   &Store{db: db},
		mailer:  newMailer(cfg),
		limiter: newRateLimiter(),
		imgSem:  make(chan struct{}, 2),
		version: strconv.FormatInt(time.Now().Unix(), 36),
	}
	pages, err := a.loadPages()
	if err != nil {
		return nil, err
	}
	a.pages = pages
	return a, nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	static, _ := fs.Sub(embedded, "static")
	mux.Handle("GET /static/", cacheFor(365*24*time.Hour, http.StripPrefix("/static/", http.FileServerFS(static))))
	mux.Handle("GET /uploads/", cacheFor(30*24*time.Hour, http.StripPrefix("/uploads/",
		http.FileServer(noDirListing{http.Dir(filepath.Join(a.cfg.DataDir, "uploads"))}))))

	mux.HandleFunc("GET /{$}", a.home)
	mux.HandleFunc("GET /api/eventos", a.apiEvents)
	mux.HandleFunc("GET /api/geocode", a.apiGeocode)
	mux.HandleFunc("GET /evento/{slug}", a.eventPage)
	mux.HandleFunc("GET /evento/{slug}/rota", a.eventDirections)
	mux.HandleFunc("GET /evento/{slug}/whatsapp", a.eventShare)
	mux.HandleFunc("GET /evento/{slug}/agenda.ics", a.eventICS)
	mux.HandleFunc("GET /artista/{slug}", a.artistPage)
	mux.HandleFunc("GET /espaco/{slug}", a.venuePage)

	mux.HandleFunc("GET /entrar", a.loginForm)
	mux.HandleFunc("POST /entrar", a.loginSend)
	mux.HandleFunc("GET /entrar/{token}", a.loginConfirm)
	mux.HandleFunc("POST /entrar/{token}", a.loginVerify)
	mux.HandleFunc("POST /sair", a.logout)

	mux.HandleFunc("GET /eventos/novo", a.eventNew)
	mux.HandleFunc("POST /eventos/novo", a.eventCreate)
	mux.HandleFunc("GET /eventos/{id}/editar", a.requireUser(a.eventEdit))
	mux.HandleFunc("POST /eventos/{id}/editar", a.requireUser(a.eventUpdate))

	mux.HandleFunc("GET /painel", a.requireUser(a.dashboard))
	mux.HandleFunc("GET /painel/perfil", a.requireUser(a.profileForm))
	mux.HandleFunc("POST /painel/perfil", a.requireUser(a.profileSave))

	mux.HandleFunc("GET /moderacao", a.requireMod(a.modQueue))
	mux.HandleFunc("POST /moderacao/eventos/{id}/aprovar", a.requireMod(a.modApprove))
	mux.HandleFunc("POST /moderacao/eventos/{id}/recusar", a.requireMod(a.modReject))
	mux.HandleFunc("GET /moderacao/espacos", a.requireMod(a.venueList))
	mux.HandleFunc("GET /moderacao/espacos/novo", a.requireMod(a.venueForm))
	mux.HandleFunc("POST /moderacao/espacos/novo", a.requireMod(a.venueSave))
	mux.HandleFunc("GET /moderacao/espacos/{id}", a.requireMod(a.venueForm))
	mux.HandleFunc("POST /moderacao/espacos/{id}", a.requireMod(a.venueSave))
	mux.HandleFunc("GET /moderacao/metricas", a.requireMod(a.metrics))

	mux.HandleFunc("GET /admin/usuarios", a.requireAdmin(a.userList))
	mux.HandleFunc("POST /admin/usuarios/{id}", a.requireAdmin(a.userUpdate))

	// CrossOriginProtection (Go 1.25+) barra POST vindo de outro site: é a nossa proteção contra CSRF.
	return http.NewCrossOriginProtection().Handler(securityHeaders(a.withUser(mux)))
}

// ---------- templates ----------

func (a *App) loadPages() (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"session":  fmtSession,
		"day":      fmtDay,
		"time":     fmtTime,
		"rangeOf":  fmtRange,
		"category": categoryLabel,
		"price":    priceLabel,
		"access":   accessLabels,
		"age":      func(v string) string { return optLabel(ageRatings, v) },
		"has":      func(csv, v string) bool { return containsCSV(csv, v) },
		"isPast":   func(s string) bool { return s < nowLocal().Format(sessLayout) },
		"excerpt":  excerpt,
		"initial": func(s string) string {
			for _, r := range strings.ToUpper(s) {
				return string(r)
			}
			return "?"
		},
		"json": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"paragraphs": func(s string) []string {
			var out []string
			for _, p := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		},
		"instagramURL": func(h string) string {
			return "https://instagram.com/" + strings.TrimPrefix(strings.TrimSpace(h), "@")
		},
		"categories":    func() []Option { return categories },
		"accessOptions": func() []Option { return accessOptions },
		"ageRatings":    func() []Option { return ageRatings },
		"priceTypes":    func() []Option { return priceTypes },
	}
	pages := map[string]*template.Template{}
	files, err := fs.Glob(embedded, "templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		t, err := template.New("").Funcs(funcs).ParseFS(embedded, "templates/layout.html", "templates/partials/*.html", f)
		if err != nil {
			return nil, err
		}
		pages[strings.TrimSuffix(path.Base(f), ".html")] = t
	}
	return pages, nil
}

type Page struct {
	Title         string
	Description   string
	OGImage       string
	Canonical     string
	Flash         string
	User          *User
	Data          any
	Version       string
	TurnstileSite string
	Dev           bool
}

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, name string, p Page) {
	t, ok := a.pages[name]
	if !ok {
		http.Error(w, "template não encontrado: "+name, http.StatusInternalServerError)
		return
	}
	p.User = currentUser(r)
	p.Version = a.version
	p.TurnstileSite = a.cfg.TurnstileSite
	p.Dev = a.cfg.Dev
	if p.Description == "" {
		p.Description = "Teatro, dança, circo, música e outras artes acontecendo agora em São José do Rio Preto."
	}
	if c, err := r.Cookie("tn_flash"); err == nil {
		if v, err := url.QueryUnescape(c.Value); err == nil {
			p.Flash = v
		}
		http.SetCookie(w, &http.Cookie{Name: "tn_flash", Path: "/", MaxAge: -1})
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "erro ao montar a página", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// renderPartial devolve só um pedaço de template (usado nas respostas do HTMX).
func (a *App) renderPartial(w http.ResponseWriter, page, name string, data any) {
	var buf bytes.Buffer
	if err := a.pages[page].ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("partial %s/%s: %v", page, name, err)
		http.Error(w, "erro", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusNotFound, "erro", Page{Title: "Não encontrado", Data: "Essa página não existe ou o evento saiu do ar."})
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	a.render(w, r, http.StatusInternalServerError, "erro", Page{Title: "Erro", Data: "Algo deu errado do nosso lado. Tente de novo em instantes."})
}

func setFlash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "tn_flash", Value: url.QueryEscape(msg), Path: "/", MaxAge: 60, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// ---------- usuário logado ----------

type ctxKey int

const userKey ctxKey = 0

func currentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userKey).(*User)
	return u
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func (a *App) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("tn_sess"); err == nil && c.Value != "" {
			u, err := a.store.UserBySession(hashToken(c.Value))
			if err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			} else if !errors.Is(err, errNotFound) {
				log.Printf("sessão: %v", err)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) requireUser(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			http.Redirect(w, r, "/entrar?volta="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

func (a *App) requireMod(h http.HandlerFunc) http.HandlerFunc {
	return a.requireUser(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsMod() {
			a.notFound(w, r)
			return
		}
		h(w, r)
	})
}

func (a *App) requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return a.requireUser(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin() {
			a.notFound(w, r)
			return
		}
		h(w, r)
	})
}

// ---------- utilidades HTTP ----------

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func cacheFor(d time.Duration, next http.Handler) http.Handler {
	v := "public, max-age=" + strconv.Itoa(int(d.Seconds()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", v)
		next.ServeHTTP(w, r)
	})
}

type noDirListing struct{ fs http.FileSystem }

func (n noDirListing) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err == nil && st.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

// clientIP devolve o IP usado nos limites de requisição. O CF-Connecting-IP só é confiável
// porque, na VPS, o app escuta em 127.0.0.1 atrás do Caddy e o ufw só libera a 443 para os
// IPs da Cloudflare, que sempre sobrescreve esse cabeçalho. Sem esse firewall, não ligue
// TEATNET_BEHIND_CF: qualquer um poderia inventar o cabeçalho e escapar dos limites.
func (a *App) clientIP(r *http.Request) string {
	if a.cfg.BehindCF {
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func containsCSV(csv, v string) bool {
	for _, p := range splitCSV(csv) {
		if p == v {
			return true
		}
	}
	return false
}

func excerpt(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// safeRedirect só aceita caminhos internos, para o ?volta= não virar redirecionamento aberto.
// Recusa barra invertida, espaços e caracteres de controle (navegadores tratam "/\x" e
// "/%09/x" como outro domínio) e qualquer coisa que o url.Parse leia com esquema ou host.
func safeRedirect(p string) string {
	const fallback = "/painel"
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || len(p) > 300 {
		return fallback
	}
	for _, r := range p {
		if r <= ' ' || r == '\\' || r == 0x7f {
			return fallback
		}
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return fallback
	}
	return p
}

func (a *App) housekeeping(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.store.DeleteExpired(); err != nil {
				log.Printf("limpeza: %v", err)
			}
			a.limiter.cleanup()
		}
	}
}
