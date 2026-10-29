package main

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

func randToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func validEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s, ".")
}

func (a *App) loginForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/painel", http.StatusSeeOther)
		return
	}
	a.render(w, r, http.StatusOK, "entrar", Page{Title: "Entrar", Data: map[string]any{"Volta": r.URL.Query().Get("volta")}})
}

// loginSend gera um link mágico de uso único, válido por 20 minutos, e manda por e-mail.
// A resposta é sempre a mesma, exista a conta ou não.
func (a *App) loginSend(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	volta := r.FormValue("volta")
	data := map[string]any{"Email": email, "Volta": volta}
	fail := func(msg string, status int) {
		data["Error"] = msg
		a.render(w, r, status, "entrar", Page{Title: "Entrar", Data: data})
	}
	if !validEmail(email) {
		fail("Digite um e-mail válido.", http.StatusUnprocessableEntity)
		return
	}
	if !a.limiter.allow("login-ip:"+a.clientIP(r), 10, 10*time.Minute) || !a.limiter.allow("login-email:"+email, 3, 10*time.Minute) {
		fail("Muitas tentativas. Espere alguns minutos e tente de novo.", http.StatusTooManyRequests)
		return
	}
	if !a.checkTurnstile(r) {
		fail("Não conseguimos confirmar que você não é um robô. Tente de novo.", http.StatusUnprocessableEntity)
		return
	}

	token := randToken()
	if err := a.store.CreateLoginToken(hashToken(token), email, time.Now().Add(20*time.Minute)); err != nil {
		a.serverError(w, r, err)
		return
	}
	link := a.cfg.BaseURL + "/entrar/" + token
	if volta != "" {
		link += "?volta=" + url.QueryEscape(safeRedirect(volta))
	}
	body := "Olá!\n\nPara entrar no TeatNet, abra o link abaixo (vale por 20 minutos e só funciona uma vez):\n\n" +
		link + "\n\nSe não foi você que pediu, é só ignorar este e-mail.\n\nTeatNet · cultura em Rio Preto"
	if err := a.mailer.Send(email, "Seu link de acesso ao TeatNet", body); err != nil {
		log.Printf("envio de e-mail para %s: %v", email, err)
		fail("Não conseguimos enviar o e-mail agora. Tente de novo em instantes.", http.StatusBadGateway)
		return
	}
	data["Sent"] = true
	if a.cfg.Dev {
		data["DevLink"] = link
	}
	a.render(w, r, http.StatusOK, "entrar", Page{Title: "Confira seu e-mail", Data: data})
}

// loginConfirm só mostra um botão. O login acontece no POST para que antivírus de e-mail,
// que abrem os links sozinhos, não gastem o token antes da pessoa.
func (a *App) loginConfirm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "entrar_confirmar", Page{Title: "Entrar", Data: map[string]any{
		"Token": r.PathValue("token"),
		"Volta": r.URL.Query().Get("volta"),
	}})
}

func (a *App) loginVerify(w http.ResponseWriter, r *http.Request) {
	email, err := a.store.ConsumeLoginToken(hashToken(r.PathValue("token")))
	if err != nil {
		a.render(w, r, http.StatusGone, "entrar", Page{Title: "Entrar", Data: map[string]any{
			"Error": "Esse link expirou ou já foi usado. Peça um novo abaixo.",
		}})
		return
	}
	u, created, err := a.store.FindOrCreateUser(email, a.cfg.AdminEmail)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	sess := randToken()
	if err := a.store.CreateSession(hashToken(sess), u.ID, time.Now().Add(sessionTTL)); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "tn_sess",
		Value:    sess,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   strings.HasPrefix(a.cfg.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
	if created || u.Name == "" {
		setFlash(w, "Bem-vindo(a) ao TeatNet! Complete seu perfil para começar a divulgar.")
		http.Redirect(w, r, "/painel/perfil", http.StatusSeeOther)
		return
	}
	dest := "/painel"
	if v := r.FormValue("volta"); v != "" {
		dest = safeRedirect(v)
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("tn_sess"); err == nil {
		a.store.DeleteSession(hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "tn_sess", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
