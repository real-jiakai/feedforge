// FeedForge — a self-hosted recreation of Feed43: turn any web page into an
// RSS feed using simple search patterns.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/real-jiakai/feedforge/internal/fetch"
	"github.com/real-jiakai/feedforge/internal/server"
	"github.com/real-jiakai/feedforge/internal/store"
)

// Only the served files are embedded — not the TypeScript source in
// web/src, which is compiled to web/app.js by `npm run build` and
// committed, so building the binary needs no Node toolchain.
//
//go:embed web/index.html web/style.css web/app.js
var webFiles embed.FS

func main() {
	var (
		addr         = flag.String("addr", envStr("FEEDFORGE_ADDR", ":8080"), "listen address")
		dataDir      = flag.String("data", envStr("FEEDFORGE_DATA", "./data"), "data directory")
		baseURL      = flag.String("base-url", envStr("FEEDFORGE_BASE_URL", ""), "public origin used in generated feed URLs, e.g. https://feeds.example.com (empty = derive from request)")
		allowPrivate = flag.Bool("allow-private", envBool("FEEDFORGE_ALLOW_PRIVATE", false), "allow fetching from private/internal network addresses")
		maxFetchMB   = flag.Int("max-fetch-mb", envInt("FEEDFORGE_MAX_FETCH_MB", 5), "max source page size in MB")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Error("opening data dir", "err", err)
		os.Exit(1)
	}

	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Error("embedded web files", "err", err)
		os.Exit(1)
	}

	handler := server.New(server.Config{
		Store:   st,
		Fetcher: fetch.New(*allowPrivate, int64(*maxFetchMB)*1024*1024, 20*time.Second),
		BaseURL: *baseURL,
		WebFS:   webFS,
		Logger:  log,
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("FeedForge listening", "addr", *addr, "data", *dataDir,
			"users", st.CountUsers(), "allowPrivate", *allowPrivate)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	// SIGTERM as well as SIGINT: `docker stop` sends SIGTERM, and without
	// this the process is killed outright — in-flight feed builds are cut
	// off and the container reports an unclean exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("shut down cleanly")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return def
}
