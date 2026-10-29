package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// periodRange converte o filtro de período em um intervalo [from, to) de sessões.
// Eventos que começaram há menos de 2h ainda aparecem (quem chega atrasado ainda pega).
func periodRange(period string, now time.Time) (string, string) {
	from := now.Add(-2 * time.Hour)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var to time.Time
	switch period {
	case "hoje":
		to = midnight.AddDate(0, 0, 1)
	case "semana":
		to = midnight.AddDate(0, 0, 8)
	case "fds":
		// sexta-feira deste fim de semana (no domingo, a sexta que passou)
		offset := 5 - int(now.Weekday())
		if now.Weekday() == time.Sunday {
			offset = -2
		}
		fri := midnight.AddDate(0, 0, offset)
		if fri.After(from) {
			from = fri
		}
		to = fri.AddDate(0, 0, 3)
	default:
		return from.Format(sessLayout), "9999"
	}
	return from.Format(sessLayout), to.Format(sessLayout)
}

func (a *App) cardFilter(r *http.Request) CardFilter {
	q := r.URL.Query()
	from, to := periodRange(q.Get("periodo"), nowLocal())
	f := CardFilter{From: from, To: to, FreeOnly: q.Get("gratis") == "1"}
	if validOpt(categories, q.Get("categoria")) {
		f.Category = q.Get("categoria")
	}
	return f
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	cards, err := a.store.UpcomingCards(a.cardFilter(r))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.store.Bump("visita_mapa")
	q := r.URL.Query()
	a.render(w, r, http.StatusOK, "home", Page{
		Canonical: a.cfg.BaseURL + "/",
		Data: map[string]any{
			"Cards":     cards,
			"Period":    q.Get("periodo"),
			"Category":  q.Get("categoria"),
			"FreeOnly":  q.Get("gratis") == "1",
			"CenterLat": cityLat,
			"CenterLng": cityLng,
		},
	})
}

func (a *App) apiEvents(w http.ResponseWriter, r *http.Request) {
	cards, err := a.store.UpcomingCards(a.cardFilter(r))
	if err != nil {
		http.Error(w, "erro", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	json.NewEncoder(w).Encode(cards)
}

func (a *App) apiGeocode(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 3 || len(q) > 200 {
		http.Error(w, "busca curta demais", http.StatusBadRequest)
		return
	}
	if !a.limiter.allow("geo:"+a.clientIP(r), 30, 10*time.Minute) {
		http.Error(w, "muitas buscas, espere um pouco", http.StatusTooManyRequests)
		return
	}
	res, err := a.geocode(r, q)
	if err != nil {
		http.Error(w, "busca indisponível no momento", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// visibleEvent busca o evento e esconde pendentes/recusados de quem não é autor nem moderador.
func (a *App) visibleEvent(r *http.Request) (*Event, bool) {
	e, err := a.store.EventBySlug(r.PathValue("slug"))
	if err != nil {
		return nil, false
	}
	if e.Status == "publicado" {
		return e, true
	}
	u := currentUser(r)
	if u != nil && (u.IsMod() || u.ID == e.AuthorID) {
		return e, true
	}
	return nil, false
}

func (a *App) eventPage(w http.ResponseWriter, r *http.Request) {
	e, ok := a.visibleEvent(r)
	if !ok {
		a.notFound(w, r)
		return
	}
	if e.Status == "publicado" {
		a.store.IncEventCounter(e.ID, "views")
		a.store.Bump("visita_evento")
	}
	now := nowLocal().Format(sessLayout)
	var upcoming, past []string
	for _, s := range e.Sessions {
		if s >= now {
			upcoming = append(upcoming, s)
		} else {
			past = append(past, s)
		}
	}
	desc := excerpt(e.Description, 150)
	if len(upcoming) > 0 {
		desc = fmtSession(upcoming[0]) + " · " + e.PlaceName + ". " + desc
	}
	og := ""
	if e.Poster != "" {
		og = a.cfg.BaseURL + e.Poster
	}
	u := currentUser(r)
	a.render(w, r, http.StatusOK, "evento", Page{
		Title:       e.Title,
		Description: desc,
		OGImage:     og,
		Canonical:   a.cfg.BaseURL + "/evento/" + e.Slug,
		Data: map[string]any{
			"Event":    e,
			"Upcoming": upcoming,
			"Past":     past,
			"CanEdit":  u != nil && (u.IsMod() || u.ID == e.AuthorID),
		},
	})
}

func (a *App) eventDirections(w http.ResponseWriter, r *http.Request) {
	e, ok := a.visibleEvent(r)
	if !ok {
		a.notFound(w, r)
		return
	}
	a.store.IncEventCounter(e.ID, "directions")
	a.store.Bump("como_chegar")
	http.Redirect(w, r, fmt.Sprintf("https://www.google.com/maps/dir/?api=1&destination=%f,%f", e.Lat, e.Lng), http.StatusFound)
}

func (a *App) eventShare(w http.ResponseWriter, r *http.Request) {
	e, ok := a.visibleEvent(r)
	if !ok {
		a.notFound(w, r)
		return
	}
	a.store.Bump("compartilhar_whatsapp")
	text := e.Title + " — " + a.cfg.BaseURL + "/evento/" + e.Slug
	http.Redirect(w, r, "https://wa.me/?text="+url.QueryEscape(text), http.StatusFound)
}

// eventICS gera um arquivo de calendário com todas as sessões futuras (Google, Apple, Outlook abrem).
func (a *App) eventICS(w http.ResponseWriter, r *http.Request) {
	e, ok := a.visibleEvent(r)
	if !ok {
		a.notFound(w, r)
		return
	}
	a.store.Bump("adicionar_agenda")
	esc := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", "")
	link := a.cfg.BaseURL + "/evento/" + e.Slug
	now := nowLocal().Format(sessLayout)
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//TeatNet//PT-BR\r\nCALSCALE:GREGORIAN\r\n")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, s := range e.Sessions {
		if s < now {
			continue
		}
		start, _ := parseSession(s)
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:%d-%s@teatnet\r\nDTSTAMP:%s\r\nDTSTART:%s\r\nDTEND:%s\r\n",
			e.ID, start.UTC().Format("20060102T1504"), stamp,
			start.UTC().Format("20060102T150405Z"), start.Add(2*time.Hour).UTC().Format("20060102T150405Z"))
		fmt.Fprintf(&b, "SUMMARY:%s\r\nLOCATION:%s\r\nDESCRIPTION:%s\r\nURL:%s\r\nEND:VEVENT\r\n",
			esc.Replace(e.Title), esc.Replace(strings.TrimSpace(e.PlaceName+", "+e.Address)), esc.Replace(link), link)
	}
	b.WriteString("END:VCALENDAR\r\n")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+e.Slug+`.ics"`)
	w.Write([]byte(b.String()))
}

func (a *App) artistPage(w http.ResponseWriter, r *http.Request) {
	u, err := a.store.UserBySlug(r.PathValue("slug"))
	if errors.Is(err, errNotFound) {
		a.notFound(w, r)
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	from, to := periodRange("", nowLocal())
	cards, err := a.store.UpcomingCards(CardFilter{From: from, To: to, AuthorID: u.ID})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	og := ""
	if u.Avatar != "" {
		og = a.cfg.BaseURL + u.Avatar
	}
	a.render(w, r, http.StatusOK, "artista", Page{
		Title:       u.Name,
		Description: excerpt(u.Bio, 160),
		OGImage:     og,
		Data:        map[string]any{"Profile": u, "Cards": cards},
	})
}

func (a *App) venuePage(w http.ResponseWriter, r *http.Request) {
	v, err := a.store.VenueBySlug(r.PathValue("slug"))
	if errors.Is(err, errNotFound) {
		a.notFound(w, r)
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	from, to := periodRange("", nowLocal())
	cards, err := a.store.UpcomingCards(CardFilter{From: from, To: to, VenueID: v.ID})
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "espaco", Page{
		Title:       v.Name,
		Description: "Agenda de " + v.Name + " em São José do Rio Preto.",
		Data:        map[string]any{"Venue": v, "Cards": cards},
	})
}
