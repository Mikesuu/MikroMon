package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/go-routeros/routeros/v3"

	"mikromon/internal/collector"
	"mikromon/internal/dpi"
	"mikromon/internal/mikrotik"
	"mikromon/internal/store"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	store *store.Store
	col   *collector.Collector
}

func NewServer(st *store.Store, col *collector.Collector) *Server {
	return &Server{store: st, col: col}
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/routers", s.handleRouters)
	mux.HandleFunc("/api/routers/test", s.handleTestRouter)
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/history/interface", s.handleIfaceHistory)
	mux.HandleFunc("/api/history/ips", s.handleIPTop)
	mux.HandleFunc("/api/history/ip", s.handleIPDetailHistory)
	mux.HandleFunc("/api/history/apps", s.handleAppTop)
	mux.HandleFunc("/api/audit/domains", s.handleDomainAudit)
	mux.HandleFunc("/api/action/interface", s.handleToggleIface)
	mux.HandleFunc("/api/queues", s.handleQueues)
	mux.HandleFunc("/api/blocked", s.handleBlocked)
	mux.HandleFunc("/api/tools/torch", s.handleTorch)
	mux.HandleFunc("/api/tools/ping", s.handlePing)

	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func queryRouterID(r *http.Request) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get("router_id"), 10, 64)
	return v
}

func (s *Server) resolveRouterID(r *http.Request) int64 {
	id := queryRouterID(r)
	if id > 0 {
		return id
	}
	list, err := s.store.ListRouters()
	if err == nil && len(list) > 0 {
		return list[0].ID
	}
	return 0
}

func (s *Server) handleRouters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListRouters()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		for i := range list {
			list[i].Password = ""
			list[i].AdgPass = ""
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var cfg store.RouterConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if cfg.Port == 0 {
			cfg.Port = 8728
		}
		if cfg.Port == 8729 {
			cfg.UseTLS = true
		}
		id, err := s.store.SaveRouter(cfg)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"id": id})
	case http.MethodDelete:
		id := queryRouterID(r)
		if err := s.store.DeleteRouter(id); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(405)
	}
}

func (s *Server) handleTestRouter(w http.ResponseWriter, r *http.Request) {
	var cfg store.RouterConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if cfg.ID > 0 && cfg.Password == "" {
		if old, err := s.store.GetRouter(cfg.ID); err == nil {
			cfg.Password = old.Password
		}
	}
	cl, err := mikrotik.Dial(mikrotik.ConnConfig{
		Host:          cfg.Host,
		Port:          cfg.Port,
		Username:      cfg.Username,
		Password:      cfg.Password,
		UseTLS:        cfg.UseTLS || cfg.Port == 8729,
		TLSSkipVerify: cfg.TLSSkipVerify,
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer cl.Close()
	sys, err := mikrotik.GetSystemResource(cl)
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "system": sys})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	writeJSON(w, 200, s.col.GetSnapshot(id))
}

func (s *Server) handleIfaceHistory(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	iface := r.URL.Query().Get("iface")
	rng := r.URL.Query().Get("range")
	pts, err := s.store.QueryIfaceHistory(id, iface, rng)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, pts)
}

func (s *Server) handleIPTop(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	rng := r.URL.Query().Get("range")
	list, err := s.store.QueryTopIPs(id, rng, 200)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleIPDetailHistory(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	ip := r.URL.Query().Get("ip")
	rng := r.URL.Query().Get("range")
	pts, err := s.store.QueryIPHistory(id, ip, rng)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, pts)
}

func (s *Server) handleAppTop(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	rng := r.URL.Query().Get("range")
	list, err := s.store.QueryTopApps(id, rng, 30)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	for i := range list {
		list[i].Category = dpi.CategoryOf(list[i].App)
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleDomainAudit(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	q := r.URL.Query().Get("q")
	list, err := s.store.QueryDomainAudit(id, q, 200)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if len(list) == 0 {
		snap := s.col.GetSnapshot(id)
		if snap != nil && len(snap.RecentDomains) > 0 {
			list = snap.RecentDomains
		}
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleToggleIface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var req struct {
		RouterID int64  `json:"router_id"`
		Name     string `json:"name"`
		Enable   bool   `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	err := s.col.WithClient(req.RouterID, func(c *routeros.Client) error {
		return mikrotik.ToggleInterface(c, req.Name, req.Enable)
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleQueues(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	switch r.Method {
	case http.MethodGet:
		var list []mikrotik.SimpleQueue
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			var e error
			list, e = mikrotik.GetSimpleQueues(c)
			return e
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Target   string `json:"target"`
			MaxLimit string `json:"max_limit"`
			Comment  string `json:"comment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			return mikrotik.AddSimpleQueue(c, req.Name, req.Target, req.MaxLimit, req.Comment)
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	case http.MethodDelete:
		qid := r.URL.Query().Get("id")
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			return mikrotik.RemoveSimpleQueue(c, qid)
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleBlocked(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	switch r.Method {
	case http.MethodGet:
		var list []mikrotik.BlockedIP
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			var e error
			list, e = mikrotik.GetBlockedIPs(c)
			return e
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var req struct {
			IP      string `json:"ip"`
			Comment string `json:"comment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			return mikrotik.BlockIP(c, req.IP, req.Comment)
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	case http.MethodDelete:
		bid := r.URL.Query().Get("id")
		err := s.col.WithClient(id, func(c *routeros.Client) error {
			return mikrotik.UnblockIP(c, bid)
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleTorch(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	iface := r.URL.Query().Get("iface")
	if iface == "" {
		writeJSON(w, 400, map[string]string{"error": "iface required"})
		return
	}
	var entries []mikrotik.TorchEntry
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		entries, e = mikrotik.TorchSnapshot(r.Context(), c, iface)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, entries)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	target := r.URL.Query().Get("target")
	if target == "" {
		writeJSON(w, 400, map[string]string{"error": "target required"})
		return
	}
	var rows []map[string]string
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		rows, e = mikrotik.RunPing(c, target, 4)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rows)
}
