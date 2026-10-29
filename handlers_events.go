package main

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// eventForm guarda o que foi digitado, para reexibir o formulário com os erros.
type eventForm struct {
	ID          int64
	Title       string
	Category    string
	Description string
	VenueID     string
	PlaceName   string
	Address     string
	Lat, Lng    string
	PriceType   string
	PriceInfo   string
	TicketURL   string
	AgeRating   string
	Access      string
	SourceURL   string
	Poster      string
	Sessions    []sessionInput
	Name, Email string // só para sugestões sem conta
	Errors      map[string]string
}

type sessionInput struct{ Date, Time string }

func formFromEvent(e *Event) *eventForm {
	f := &eventForm{
		ID: e.ID, Title: e.Title, Category: e.Category, Description: e.Description,
		PlaceName: e.PlaceName, Address: e.Address,
		Lat: strconv.FormatFloat(e.Lat, 'f', 6, 64), Lng: strconv.FormatFloat(e.Lng, 'f', 6, 64),
		PriceType: e.PriceType, PriceInfo: e.PriceInfo, TicketURL: e.TicketURL, AgeRating: e.AgeRating,
		Access: e.Accessibility, SourceURL: e.SourceURL, Poster: e.Poster,
	}
	if e.VenueID != 0 {
		f.VenueID = strconv.FormatInt(e.VenueID, 10)
	}
	for _, s := range e.Sessions {
		f.Sessions = append(f.Sessions, sessionInput{Date: s[:10], Time: s[11:]})
	}
	return f
}

// field lê um campo de texto do formulário, sem espaços nas pontas e só com UTF-8 válido.
func field(r *http.Request, name string) string {
	return strings.TrimSpace(strings.ToValidUTF8(r.FormValue(name), ""))
}

func parseEventForm(r *http.Request) *eventForm {
	f := &eventForm{
		Title:       field(r, "title"),
		Category:    r.FormValue("category"),
		Description: field(r, "description"),
		VenueID:     r.FormValue("venue_id"),
		PlaceName:   field(r, "place_name"),
		Address:     field(r, "address"),
		Lat:         r.FormValue("lat"),
		Lng:         r.FormValue("lng"),
		PriceType:   r.FormValue("price_type"),
		PriceInfo:   field(r, "price_info"),
		TicketURL:   field(r, "ticket_url"),
		AgeRating:   r.FormValue("age_rating"),
		SourceURL:   field(r, "source_url"),
		Name:        field(r, "name"),
		Email:       strings.ToLower(field(r, "email")),
		Errors:      map[string]string{},
	}
	var access []string
	for _, v := range r.Form["access"] {
		if validOpt(accessOptions, v) {
			access = append(access, v)
		}
	}
	f.Access = strings.Join(access, ",")
	dates, times := r.Form["s_date"], r.Form["s_time"]
	for i := range dates {
		t := ""
		if i < len(times) {
			t = times[i]
		}
		if dates[i] == "" && t == "" {
			continue
		}
		f.Sessions = append(f.Sessions, sessionInput{Date: dates[i], Time: t})
	}
	return f
}

func validURL(s string) bool {
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// toEvent valida o formulário e preenche o evento. Devolve false se houver erros em f.Errors.
func (a *App) toEvent(f *eventForm, e *Event, editing bool) bool {
	errs := f.Errors
	if n := len([]rune(f.Title)); n < 3 || n > 120 {
		errs["title"] = "Dê um título entre 3 e 120 caracteres."
	}
	if !validOpt(categories, f.Category) {
		errs["category"] = "Escolha uma categoria."
	}
	if len([]rune(f.Description)) > 3000 {
		errs["description"] = "A descrição pode ter até 3000 caracteres."
	}
	if !validOpt(priceTypes, f.PriceType) {
		f.PriceType = "gratuito"
	}
	if len([]rune(f.PriceInfo)) > 120 {
		errs["price_info"] = "Use até 120 caracteres."
	}
	if !validOpt(ageRatings, f.AgeRating) {
		f.AgeRating = ""
	}
	if !validURL(f.TicketURL) {
		errs["ticket_url"] = "Use um link completo, começando com https://"
	}
	if !validURL(f.SourceURL) {
		errs["source_url"] = "Use um link completo, começando com https://"
	}

	// Local: espaço cadastrado ou um ponto marcado no mapa.
	e.VenueID = 0
	if id, _ := strconv.ParseInt(f.VenueID, 10, 64); id != 0 {
		v, err := a.store.VenueByID(id)
		if err != nil {
			errs["venue"] = "Espaço não encontrado."
		} else {
			e.VenueID, e.PlaceName, e.Address, e.Lat, e.Lng = v.ID, v.Name, v.Address, v.Lat, v.Lng
		}
	} else {
		lat, err1 := strconv.ParseFloat(f.Lat, 64)
		lng, err2 := strconv.ParseFloat(f.Lng, 64)
		switch {
		case f.PlaceName == "":
			errs["venue"] = "Diga o nome do local (ex.: Praça Rui Barbosa)."
		case err1 != nil || err2 != nil:
			errs["venue"] = "Marque o local no mapa (busque o endereço ou clique no mapa)."
		case !inCity(lat, lng):
			errs["venue"] = "O ponto marcado fica fora de São José do Rio Preto."
		default:
			e.PlaceName, e.Address, e.Lat, e.Lng = f.PlaceName, f.Address, lat, lng
		}
	}

	// Sessões: datas válidas, sem repetição, até 90. Na criação, não aceitamos só datas passadas.
	seen := map[string]bool{}
	var sessions []string
	for _, s := range f.Sessions {
		t, err := time.ParseInLocation("2006-01-02 15:04", s.Date+" "+s.Time, loc)
		if err != nil {
			errs["sessions"] = "Confira as datas e horários: todas as sessões precisam de dia e hora."
			continue
		}
		v := t.Format(sessLayout)
		if !seen[v] {
			seen[v] = true
			sessions = append(sessions, v)
		}
	}
	sort.Strings(sessions)
	switch {
	case len(sessions) == 0 && errs["sessions"] == "":
		errs["sessions"] = "Adicione pelo menos uma data."
	case len(sessions) > 90:
		errs["sessions"] = "Máximo de 90 sessões por evento."
	case len(sessions) > 0 && !editing && sessions[len(sessions)-1] < nowLocal().Format(sessLayout):
		errs["sessions"] = "Todas as datas já passaram."
	}

	e.Title, e.Category, e.Description = f.Title, f.Category, f.Description
	e.PriceType, e.PriceInfo, e.TicketURL, e.AgeRating = f.PriceType, f.PriceInfo, f.TicketURL, f.AgeRating
	e.Accessibility, e.SourceURL, e.Sessions = f.Access, f.SourceURL, sessions
	return len(errs) == 0
}

func (a *App) renderEventForm(w http.ResponseWriter, r *http.Request, status int, f *eventForm) {
	venues, err := a.store.ListVenues()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	title := "Divulgar evento"
	if f.ID != 0 {
		title = "Editar evento"
	}
	if len(f.Sessions) == 0 {
		f.Sessions = []sessionInput{{}}
	}
	a.render(w, r, status, "evento_form", Page{Title: title, Data: map[string]any{
		"Form":      f,
		"Venues":    venues,
		"CenterLat": cityLat,
		"CenterLng": cityLng,
	}})
}

func (a *App) eventNew(w http.ResponseWriter, r *http.Request) {
	a.renderEventForm(w, r, http.StatusOK, &eventForm{PriceType: "gratuito", Errors: map[string]string{}})
}

func (a *App) eventCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	if err := r.ParseMultipartForm(maxUpload + 1<<20); err != nil {
		a.renderEventForm(w, r, http.StatusRequestEntityTooLarge, &eventForm{Errors: map[string]string{"poster": "Arquivo grande demais (máximo 8 MB)."}})
		return
	}
	u := currentUser(r)
	f := parseEventForm(r)
	e := &Event{}

	if u == nil {
		// Sugestão sem conta: pedimos contato, captcha e limitamos envios por IP.
		if f.Name == "" {
			f.Errors["name"] = "Diga seu nome."
		}
		if !validEmail(f.Email) {
			f.Errors["email"] = "Digite um e-mail válido (só a moderação vê)."
		}
		if !a.limiter.allow("sugestao:"+a.clientIP(r), 5, time.Hour) {
			f.Errors["form"] = "Você enviou muitas sugestões seguidas. Tente de novo mais tarde."
		} else if !a.checkTurnstile(r) {
			f.Errors["form"] = "Não conseguimos confirmar que você não é um robô."
		}
		e.SuggestedName, e.SuggestedEmail = f.Name, f.Email
	} else {
		e.AuthorID = u.ID
	}

	ok := a.toEvent(f, e, false)
	if ok {
		poster, err := a.saveImage(r, "poster", 1000)
		if err != nil {
			f.Errors["poster"] = err.Error()
			ok = false
		}
		e.Poster = poster
	}
	if !ok {
		a.renderEventForm(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	e.Status = "pendente"
	if u.Trusted() {
		e.Status = "publicado"
	}
	if err := a.store.SaveEvent(e); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.store.Bump("evento_enviado")

	switch {
	case e.Status == "publicado":
		setFlash(w, "Evento publicado! Já está no mapa.")
		http.Redirect(w, r, "/evento/"+e.Slug, http.StatusSeeOther)
	case u != nil:
		setFlash(w, "Evento enviado! Assim que a moderação aprovar, ele aparece no mapa.")
		http.Redirect(w, r, "/painel", http.StatusSeeOther)
	default:
		setFlash(w, "Obrigado pela sugestão! Ela vai passar pela moderação antes de aparecer no mapa.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// editableEvent carrega o evento se o usuário for o autor ou moderador.
func (a *App) editableEvent(w http.ResponseWriter, r *http.Request) (*Event, bool) {
	e, err := a.store.EventByID(pathID(r))
	u := currentUser(r)
	if errors.Is(err, errNotFound) || (err == nil && !u.IsMod() && e.AuthorID != u.ID) {
		a.notFound(w, r)
		return nil, false
	}
	if err != nil {
		a.serverError(w, r, err)
		return nil, false
	}
	return e, true
}

func (a *App) eventEdit(w http.ResponseWriter, r *http.Request) {
	e, ok := a.editableEvent(w, r)
	if !ok {
		return
	}
	f := formFromEvent(e)
	f.Errors = map[string]string{}
	a.renderEventForm(w, r, http.StatusOK, f)
}

func (a *App) eventUpdate(w http.ResponseWriter, r *http.Request) {
	e, ok := a.editableEvent(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	if err := r.ParseMultipartForm(maxUpload + 1<<20); err != nil {
		f := formFromEvent(e)
		f.Errors = map[string]string{"poster": "Arquivo grande demais (máximo 8 MB)."}
		a.renderEventForm(w, r, http.StatusRequestEntityTooLarge, f)
		return
	}
	f := parseEventForm(r)
	f.ID, f.Poster = e.ID, e.Poster
	valid := a.toEvent(f, e, true)
	if valid {
		poster, err := a.saveImage(r, "poster", 1000)
		if err != nil {
			f.Errors["poster"] = err.Error()
			valid = false
		} else if poster != "" {
			e.Poster = poster
		}
	}
	if !valid {
		a.renderEventForm(w, r, http.StatusUnprocessableEntity, f)
		return
	}
	// Quem ainda não é verificado volta para a fila ao editar um evento publicado.
	u := currentUser(r)
	if !u.Trusted() && e.Status == "publicado" {
		e.Status = "pendente"
	}
	if e.Status == "recusado" && !u.IsMod() {
		e.Status = "pendente"
	}
	if err := a.store.SaveEvent(e); err != nil {
		a.serverError(w, r, err)
		return
	}
	if e.Status == "pendente" {
		setFlash(w, "Alterações salvas. O evento volta a aparecer no mapa depois da moderação.")
	} else {
		setFlash(w, "Alterações salvas.")
	}
	http.Redirect(w, r, "/evento/"+e.Slug, http.StatusSeeOther)
}

// ---------- painel e perfil ----------

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	rows, err := a.store.EventsByAuthor(u.ID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	views, directions := 0, 0
	for _, e := range rows {
		views += e.Views
		directions += e.Directions
	}
	a.render(w, r, http.StatusOK, "painel", Page{Title: "Meu painel", Data: map[string]any{
		"Events": rows, "Views": views, "Directions": directions,
	}})
}

func (a *App) profileForm(w http.ResponseWriter, r *http.Request) {
	u := *currentUser(r)
	a.render(w, r, http.StatusOK, "perfil_form", Page{Title: "Meu perfil", Data: map[string]any{"Profile": &u, "Errors": map[string]string{}}})
}

func (a *App) profileSave(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	if err := r.ParseMultipartForm(maxUpload + 1<<20); err != nil {
		http.Error(w, "arquivo grande demais", http.StatusRequestEntityTooLarge)
		return
	}
	p := *currentUser(r)
	p.Name = strings.TrimSpace(r.FormValue("name"))
	p.Kind = r.FormValue("kind")
	p.Bio = strings.TrimSpace(r.FormValue("bio"))
	p.Instagram = strings.TrimPrefix(strings.TrimSpace(r.FormValue("instagram")), "@")
	p.Website = strings.TrimSpace(r.FormValue("website"))

	errs := map[string]string{}
	if n := len([]rune(p.Name)); n < 2 || n > 80 {
		errs["name"] = "Seu nome ou o nome do grupo (2 a 80 caracteres)."
	}
	if p.Kind != "artista" && p.Kind != "espaco" {
		p.Kind = "artista"
	}
	if len([]rune(p.Bio)) > 1500 {
		errs["bio"] = "A bio pode ter até 1500 caracteres."
	}
	if len(p.Instagram) > 40 || strings.ContainsAny(p.Instagram, " /") {
		errs["instagram"] = "Só o @ do perfil, sem link."
	}
	if !validURL(p.Website) {
		errs["website"] = "Use um link completo, começando com https://"
	}
	if len(errs) == 0 {
		avatar, err := a.saveImage(r, "avatar", 400)
		if err != nil {
			errs["avatar"] = err.Error()
		} else if avatar != "" {
			p.Avatar = avatar
		}
	}
	if len(errs) > 0 {
		a.render(w, r, http.StatusUnprocessableEntity, "perfil_form", Page{Title: "Meu perfil", Data: map[string]any{"Profile": &p, "Errors": errs}})
		return
	}
	if err := a.store.UpdateProfile(&p); err != nil {
		a.serverError(w, r, err)
		return
	}
	setFlash(w, "Perfil salvo.")
	http.Redirect(w, r, "/painel", http.StatusSeeOther)
}
