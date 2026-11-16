// freeturn-admin - веб-админка серверов FreeTurn: один бинарник с API и фронтом.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/config"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/httpapi"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/rpc"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/secret"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
	"github.com/huhhuh78787878-collab/freeturn-admin/web"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "показать версию")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("freeturn-admin", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := bootstrapAdmin(ctx, st, cfg); err != nil {
		return err
	}
	box, err := secret.New(cfg.MasterKey)
	if err != nil {
		return err
	}
	api := &httpapi.API{Store: st, Box: box, Web: web.Dist(), SecureCookies: cfg.SecureCookies, TrustProxy: cfg.TrustProxy, Log: log,
		Script: &rpc.Script{URL: rpc.ScriptURL(cfg.CoreVersion), File: os.Getenv("FTA_INSTALL_SCRIPT")}}
	srv := &fasthttp.Server{
		Handler:            api.Handler(),
		Name:               "freeturn-admin",
		MaxRequestBodySize: 1 << 20,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       60 * time.Second,
		IdleTimeout:        120 * time.Second,
	}
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				st.PurgeExpiredSessions(ctx)
			}
		}
	}()

	errc := make(chan error, 1)
	go func() {
		log.Info("слушаю", "addr", cfg.Listen, "db", cfg.DBDriver, "version", version)
		if cfg.TLSCert != "" {
			errc <- srv.ListenAndServeTLS(cfg.Listen, cfg.TLSCert, cfg.TLSKey)
		} else {
			errc <- srv.ListenAndServe(cfg.Listen)
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("останавливаюсь")
		return srv.ShutdownWithContext(context.Background())
	}
}

// bootstrapAdmin: при пустой БД первый администратор берётся из FTA_ADMIN_LOGIN / FTA_ADMIN_PASSWORD.
// Без них его создают мастером в браузере.
func bootstrapAdmin(ctx context.Context, st *store.Store, cfg config.Config) error {
	if cfg.AdminLogin == "" {
		return nil
	}
	n, err := st.CountAdmins(ctx)
	if err != nil || n > 0 {
		return err
	}
	if len(cfg.AdminPassword) < 10 {
		return errors.New("FTA_ADMIN_PASSWORD: от 10 символов")
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = st.CreateAdmin(ctx, cfg.AdminLogin, hash, "root")
	return err
}
