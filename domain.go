package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Horário de Brasília (sem horário de verão desde 2019). O tzdata vai embutido no binário.
var loc = mustLoadLocation("America/Sao_Paulo")

const sessLayout = "2006-01-02T15:04"

// Limites aproximados do município, usados para validar pinos e a busca de endereço.
const (
	cityLat, cityLng = -20.8113, -49.3758
	minLat, maxLat   = -20.98, -20.68
	minLng, maxLng   = -49.58, -49.22
	nominatimViewbox = "-49.58,-20.68,-49.22,-20.98"
)

func mustLoadLocation(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

func nowLocal() time.Time { return time.Now().In(loc) }

type Option struct{ Value, Label string }

var categories = []Option{
	{"teatro", "Teatro"},
	{"danca", "Dança"},
	{"circo", "Circo"},
	{"musica", "Música"},
	{"literatura", "Literatura e sarau"},
	{"artes-visuais", "Artes visuais"},
	{"cinema", "Cinema"},
	{"infantil", "Infantil"},
	{"performance", "Performance"},
	{"humor", "Humor"},
	{"outro", "Outro"},
}

var accessOptions = []Option{
	{"libras", "Intérprete de Libras"},
	{"audiodescricao", "Audiodescrição"},
	{"cadeirante", "Acesso para cadeira de rodas"},
	{"legendas", "Legendas"},
}

var ageRatings = []Option{
	{"L", "Livre"}, {"10", "10 anos"}, {"12", "12 anos"}, {"14", "14 anos"}, {"16", "16 anos"}, {"18", "18 anos"},
	{"", "Não informada"},
}

var priceTypes = []Option{
	{"gratuito", "Gratuito"}, {"pago", "Pago"}, {"contribuicao", "Contribuição voluntária"},
}

func optLabel(opts []Option, v string) string {
	for _, o := range opts {
		if o.Value == v {
			return o.Label
		}
	}
	return v
}

func validOpt(opts []Option, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

func categoryLabel(v string) string { return optLabel(categories, v) }

func priceLabel(kind, info string) string {
	switch kind {
	case "gratuito":
		if info != "" {
			return "Gratuito · " + info
		}
		return "Gratuito"
	case "contribuicao":
		if info != "" {
			return "Contribuição voluntária · " + info
		}
		return "Contribuição voluntária"
	default:
		if info != "" {
			return info
		}
		return "Pago"
	}
}

func accessLabels(csv string) []string {
	var out []string
	for _, v := range splitCSV(csv) {
		out = append(out, optLabel(accessOptions, v))
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var weekdays = [...]string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}
var months = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

func parseSession(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(sessLayout, s, loc)
	return t, err == nil
}

// "sáb, 31 out"
func fmtDay(s string) string {
	t, ok := parseSession(s)
	if !ok {
		return s
	}
	return fmt.Sprintf("%s, %d %s", weekdays[t.Weekday()], t.Day(), months[t.Month()-1])
}

// "19h" ou "19h30"
func fmtTime(s string) string {
	t, ok := parseSession(s)
	if !ok {
		return ""
	}
	if t.Minute() == 0 {
		return fmt.Sprintf("%dh", t.Hour())
	}
	return fmt.Sprintf("%dh%02d", t.Hour(), t.Minute())
}

// "Hoje", "Amanhã" ou "sáb, 31 out": usado nos títulos que agrupam a lista por dia.
func relativeDay(s string) string {
	t, ok := parseSession(s)
	if !ok {
		return s
	}
	today := nowLocal().Format("2006-01-02")
	switch t.Format("2006-01-02") {
	case today:
		return "Hoje"
	case nowLocal().AddDate(0, 0, 1).Format("2006-01-02"):
		return "Amanhã"
	}
	return fmtDay(s)
}

func fmtSession(s string) string {
	if s == "" {
		return ""
	}
	return fmtDay(s) + " · " + fmtTime(s)
}

// "31 out – 25 nov" para temporadas, ou só o dia quando é uma data.
func fmtRange(first, last string) string {
	if first == "" {
		return "sem data"
	}
	if first[:10] == last[:10] {
		return fmtSession(first)
	}
	a, _ := parseSession(first)
	b, _ := parseSession(last)
	return fmt.Sprintf("%d %s – %d %s", a.Day(), months[a.Month()-1], b.Day(), months[b.Month()-1])
}

func inCity(lat, lng float64) bool {
	return lat >= minLat && lat <= maxLat && lng >= minLng && lng <= maxLng
}

var slugReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

func slugify(s string) string {
	s = slugReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if len(out) > 60 {
		out = strings.TrimSuffix(out[:60], "-")
	}
	if out == "" {
		out = "item"
	}
	return out
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
