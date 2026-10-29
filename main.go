package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

//go:embed templates static migrations
var embedded embed.FS

type Config struct {
	Addr       string
	BaseURL    string
	DataDir    string
	AdminEmail string
	Dev        bool
	BehindCF   bool // confia no cabeçalho CF-Connecting-IP (só ligar atrás da Cloudflare)

	SMTPHost, SMTPPort, SMTPUser, SMTPPass, MailFrom string

	TurnstileSite, TurnstileSecret string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	seed := flag.String("seed", "", "carrega um arquivo .sql de dados iniciais e sai")
	dev := flag.Bool("dev", false, "modo desenvolvimento: mostra o link de login na tela e admin@teatnet.test vira admin")
	flag.Parse()

	port := env("PORT", "8080")
	cfg := Config{
		Addr:            env("TEATNET_ADDR", "127.0.0.1:"+port),
		BaseURL:         strings.TrimSuffix(env("TEATNET_BASE_URL", "http://localhost:"+port), "/"),
		DataDir:         env("TEATNET_DATA", "data"),
		AdminEmail:      env("TEATNET_ADMIN_EMAIL", ""),
		Dev:             env("TEATNET_DEV", "") == "1",
		BehindCF:        env("TEATNET_BEHIND_CF", "") == "1",
		SMTPHost:        env("TEATNET_SMTP_HOST", ""),
		SMTPPort:        env("TEATNET_SMTP_PORT", "587"),
		SMTPUser:        env("TEATNET_SMTP_USER", ""),
		SMTPPass:        env("TEATNET_SMTP_PASS", ""),
		MailFrom:        env("TEATNET_MAIL_FROM", ""),
		TurnstileSite:   env("TEATNET_TURNSTILE_SITE", ""),
		TurnstileSecret: env("TEATNET_TURNSTILE_SECRET", ""),
	}
	if *dev {
		cfg.Dev = true
		if cfg.AdminEmail == "" {
			cfg.AdminEmail = "admin@teatnet.test"
		}
	}

	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads"), 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := openDB(filepath.Join(cfg.DataDir, "teatnet.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		log.Fatal("migração: ", err)
	}

	if *seed != "" {
		b, err := os.ReadFile(*seed)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			log.Fatal("seed: ", err)
		}
		log.Printf("seed %s carregado", *seed)
		return
	}

	app, err := newApp(cfg, db)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go app.housekeeping(ctx)

	go func() {
		log.Printf("TeatNet ouvindo em %s (%s)", cfg.Addr, cfg.BaseURL)
		if cfg.SMTPHost == "" {
			log.Printf("SMTP não configurado: os links de login vão aparecer aqui no log")
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
