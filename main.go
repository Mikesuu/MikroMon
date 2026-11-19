package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"mikromon/internal/collector"
	"mikromon/internal/store"
	"mikromon/internal/updater"
	"mikromon/internal/web"
)

var Version = "dev"

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	listenAddr := flag.String("listen", envOrDefault("MIKROMON_LISTEN", ":8080"), "HTTP listen address")
	dataDir := flag.String("data", envOrDefault("MIKROMON_DATA", "./data"), "Data directory for SQLite")
	showVer := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVer {
		fmt.Println(Version)
		return
	}

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir %s: %v", *dataDir, err)
	}

	dbPath := filepath.Join(*dataDir, "mikromon.db")
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to init sqlite: %v", err)
	}
	defer st.Close()

	seedRouterFromEnv(st)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	col := collector.New(st)
	go col.Run(ctx)

	upd := updater.New(Version, st, func(newBin string) {
		cancel()
		time.Sleep(600 * time.Millisecond)
		_ = st.Close()
		args := append([]string{newBin}, os.Args[1:]...)
		if err := syscall.Exec(newBin, args, os.Environ()); err != nil {
			log.Printf("exec restart err: %v", err)
			os.Exit(1)
		}
	})
	go upd.RunLoop(ctx)

	srv := web.NewServer(st, col, upd)
	log.Printf("MikroMon %s starting on %s (data: %s)", Version, *listenAddr, dbPath)
	if err := srv.ListenAndServe(ctx, *listenAddr); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func seedRouterFromEnv(st *store.Store) {
	host := os.Getenv("MIKRO_HOST")
	if host == "" {
		return
	}
	routers, err := st.ListRouters()
	if err != nil || len(routers) > 0 {
		return
	}
	port, _ := strconv.Atoi(envOrDefault("MIKRO_PORT", "8728"))
	useTLS := os.Getenv("MIKRO_TLS") == "true" || port == 8729
	skipVerify := envOrDefault("MIKRO_TLS_SKIP_VERIFY", "true") == "true"
	_, err = st.SaveRouter(store.RouterConfig{
		Name:          envOrDefault("MIKRO_NAME", "Main MikroTik"),
		Host:          host,
		Port:          port,
		Username:      envOrDefault("MIKRO_USER", "admin"),
		Password:      os.Getenv("MIKRO_PASS"),
		UseTLS:        useTLS,
		TLSSkipVerify: skipVerify,
		Enabled:       true,
	})
	if err != nil {
		log.Printf("seed router warning: %v", err)
	}
}
