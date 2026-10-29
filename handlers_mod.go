package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) modQueue(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.PendingEvents()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "moderacao", Page{Title: "Moderação", Data: rows})
}

func (a *App) modApprove(w http.ResponseWriter, r *http.Request) {
	e, err := a.store.EventByID(pathID(r))
	if err != nil {
		a.notFound(w, r)
		return
	}
	if err := a.store.SetEventStatus(e.ID, "publicado", ""); err != nil {
		a.serverError(w, r, err)
		return
	}
	if e.AuthorID != 0 {
		a.store.VerifyIfEarned(e.AuthorID)
	}
	a.store.Bump("evento_aprovado")
	a.modDone(w, r, e, "Aprovado e publicado no mapa.")
}

func (a *App) modReject(w http.ResponseWriter, r *http.Request) {
	e, err := a.store.EventByID(pathID(r))
	if err != nil {
		a.notFound(w, r)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if len([]rune(note)) > 500 {
		note = string([]rune(note)[:500])
	}
	if err := a.store.SetEventStatus(e.ID, "recusado", note); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.modDone(w, r, e, "Recusado.")
}

func (a *App) modDone(w http.ResponseWriter, r *http.Request, e *Event, msg string) {
	if isHTMX(r) {
		a.renderPartial(w, "moderacao", "mod-done", map[string]any{"Event": e, "Message": msg})
		return
	}
	setFlash(w, e.Title+": "+msg)
	http.Redirect(w, r, "/moderacao", http.StatusSeeOther)
}

// ---------- espaços ----------

func (a *App) venueList(w http.ResponseWriter, r *http.Request) {
	venues, err := a.store.ListVenues()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "espacos", Page{Title: "Espaços culturais", Data: venues})
}

func (a *App) venueForm(w http.ResponseWriter, r *http.Request) {
	v := &Venue{Lat: cityLat, Lng: cityLng}
	if r.PathValue("id") != "" {
		var err error
		if v, err = a.store.VenueByID(pathID(r)); err != nil {
			a.notFound(w, r)
			return
		}
	}
	a.renderVenueForm(w, r, http.StatusOK, v, map[string]string{})
}

func (a *App) renderVenueForm(w http.ResponseWriter, r *http.Request, status int, v *Venue, errs map[string]string) {
	title := "Novo espaço"
	if v.ID != 0 {
		title = "Editar espaço"
	}
	a.render(w, r, status, "espaco_form", Page{Title: title, Data: map[string]any{"Venue": v, "Errors": errs}})
}

func (a *App) venueSave(w http.ResponseWriter, r *http.Request) {
	v := &Venue{}
	if r.PathValue("id") != "" {
		var err error
		if v, err = a.store.VenueByID(pathID(r)); errors.Is(err, errNotFound) {
			a.notFound(w, r)
			return
		} else if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	v.Name = strings.TrimSpace(r.FormValue("name"))
	v.Address = strings.TrimSpace(r.FormValue("address"))
	v.Neighborhood = strings.TrimSpace(r.FormValue("neighborhood"))
	v.Website = strings.TrimSpace(r.FormValue("website"))
	var access []string
	for _, x := range r.Form["access"] {
		if validOpt(accessOptions, x) {
			access = append(access, x)
		}
	}
	v.Accessibility = strings.Join(access, ",")

	errs := map[string]string{}
	if n := len([]rune(v.Name)); n < 2 || n > 120 {
		errs["name"] = "Nome entre 2 e 120 caracteres."
	}
	lat, err1 := strconv.ParseFloat(r.FormValue("lat"), 64)
	lng, err2 := strconv.ParseFloat(r.FormValue("lng"), 64)
	if err1 != nil || err2 != nil || !inCity(lat, lng) {
		errs["venue"] = "Marque o local no mapa, dentro de Rio Preto."
	} else {
		v.Lat, v.Lng = lat, lng
	}
	if !validURL(v.Website) {
		errs["website"] = "Use um link completo, começando com https://"
	}
	if len(errs) > 0 {
		a.renderVenueForm(w, r, http.StatusUnprocessableEntity, v, errs)
		return
	}
	if err := a.store.SaveVenue(v); err != nil {
		a.serverError(w, r, err)
		return
	}
	setFlash(w, "Espaço salvo.")
	http.Redirect(w, r, "/moderacao/espacos", http.StatusSeeOther)
}

// ---------- métricas e usuários ----------

func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	m, err := a.store.Metrics()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	top, err := a.store.TopEvents(10)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "metricas", Page{Title: "Métricas", Data: map[string]any{"M": m, "Top": top}})
}

func (a *App) userList(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers()
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "usuarios", Page{Title: "Usuários", Data: users})
}

func (a *App) userUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	role := r.FormValue("role")
	if role != "membro" && role != "moderador" && role != "admin" {
		http.Error(w, "papel inválido", http.StatusBadRequest)
		return
	}
	if id == currentUser(r).ID && role != "admin" {
		setFlash(w, "Você não pode tirar o seu próprio acesso de admin.")
		http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
		return
	}
	if err := a.store.SetUserAccess(id, role, r.FormValue("verified") == "1"); err != nil {
		a.serverError(w, r, err)
		return
	}
	setFlash(w, "Usuário atualizado.")
	http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
}
