package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ---------- limite de requisições ----------

// rateLimiter em memória, por chave (IP, e-mail...). Suficiente para um servidor só.
type rateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newRateLimiter() *rateLimiter { return &rateLimiter{hits: map[string][]time.Time{}} }

func (l *rateLimiter) allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func (l *rateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, ts := range l.hits {
		if len(ts) == 0 || time.Since(ts[len(ts)-1]) > 24*time.Hour {
			delete(l.hits, k)
		}
	}
}

// ---------- Cloudflare Turnstile (captcha) ----------

// checkTurnstile valida o captcha; sem chave configurada (desenvolvimento) sempre passa.
func (a *App) checkTurnstile(r *http.Request) bool {
	if a.cfg.TurnstileSecret == "" {
		return true
	}
	resp, err := httpClient.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {a.cfg.TurnstileSecret},
		"response": {r.FormValue("cf-turnstile-response")},
		"remoteip": {a.clientIP(r)},
	})
	if err != nil {
		log.Printf("turnstile: %v", err)
		return false
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Success
}

// ---------- e-mail ----------

type Mailer interface {
	Send(to, subject, body string) error
}

func newMailer(cfg Config) Mailer {
	if cfg.SMTPHost == "" {
		return logMailer{}
	}
	return smtpMailer{cfg}
}

// logMailer só escreve o e-mail no log: usado em desenvolvimento.
type logMailer struct{}

func (logMailer) Send(to, subject, body string) error {
	log.Printf("\n--- e-mail para %s ---\nAssunto: %s\n\n%s\n---------------------", to, subject, body)
	return nil
}

// smtpMailer envia por SMTP com STARTTLS (Brevo, Resend, Gmail...).
type smtpMailer struct{ cfg Config }

func (m smtpMailer) Send(to, subject, body string) error {
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: TeatNet <%s>\r\n", m.cfg.MailFrom)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	msg.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	return smtp.SendMail(m.cfg.SMTPHost+":"+m.cfg.SMTPPort, auth, m.cfg.MailFrom, []string{to}, msg.Bytes())
}

// ---------- imagens ----------

const maxUpload = 8 << 20

// saveImage processa a imagem enviada no campo: reduz para no máximo maxW de largura
// e grava como JPEG. É o trabalho mais pesado do sistema, por isso roda uma vez só, no envio.
func (a *App) saveImage(r *http.Request, field string, maxW int) (string, error) {
	f, _, err := r.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("não foi possível ler a imagem")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil {
		return "", errors.New("não foi possível ler a imagem")
	}
	if len(data) > maxUpload {
		return "", errors.New("imagem muito grande (máximo 8 MB)")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("formato não suportado (use JPG, PNG ou WebP)")
	}
	if cfg.Width*cfg.Height > 40_000_000 {
		return "", errors.New("imagem com resolução grande demais")
	}

	a.imgSem <- struct{}{}
	defer func() { <-a.imgSem }()

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("formato não suportado (use JPG, PNG ou WebP)")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src) // fundo branco para PNG transparente
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	name := randHex(12) + ".jpg"
	out, err := os.Create(filepath.Join(a.cfg.DataDir, "uploads", name))
	if err != nil {
		return "", err
	}
	defer out.Close()
	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

// ---------- busca de endereço (Nominatim / OpenStreetMap) ----------

type geoResult struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label string  `json:"label"`
}

// geocode consulta o Nominatim respeitando a política de uso: no máximo 1 requisição
// por segundo e User-Agent identificado. A busca fica presa ao território de Rio Preto.
func (a *App) geocode(r *http.Request, q string) ([]geoResult, error) {
	a.geoMu.Lock()
	if wait := time.Second - time.Since(a.geoLast); wait > 0 {
		time.Sleep(wait)
	}
	a.geoLast = time.Now()
	a.geoMu.Unlock()

	u := "https://nominatim.openstreetmap.org/search?" + url.Values{
		"q":            {q + ", São José do Rio Preto, SP"},
		"format":       {"jsonv2"},
		"limit":        {"5"},
		"countrycodes": {"br"},
		"viewbox":      {nominatimViewbox},
		"bounded":      {"1"},
	}.Encode()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TeatNet/0.1 (+"+a.cfg.BaseURL+")")
	req.Header.Set("Accept-Language", "pt-BR")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim: status %d", resp.StatusCode)
	}
	var raw []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := []geoResult{}
	for _, g := range raw {
		var res geoResult
		fmt.Sscan(g.Lat, &res.Lat)
		fmt.Sscan(g.Lon, &res.Lng)
		res.Label = shortAddress(g.DisplayName)
		out = append(out, res)
	}
	return out, nil
}

// shortAddress corta o "display_name" do Nominatim nas 3 primeiras partes.
func shortAddress(s string) string {
	parts := strings.Split(s, ", ")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, ", ")
}
