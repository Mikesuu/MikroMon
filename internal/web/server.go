package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-routeros/routeros/v3"

	"mikromon/internal/collector"
	"mikromon/internal/dpi"
	"mikromon/internal/icons"
	"mikromon/internal/mikrotik"
	"mikromon/internal/store"
	"mikromon/internal/updater"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	store *store.Store
	col   *collector.Collector
	icons *icons.Manager
	upd   *updater.Manager
}

func NewServer(st *store.Store, col *collector.Collector, upd *updater.Manager) *Server {
	dataDir := os.Getenv("MIKROMON_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}
	return &Server{store: st, col: col, icons: icons.New(dataDir), upd: upd}
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/routers", s.handleRouters)
	mux.HandleFunc("/api/routers/test", s.handleTestRouter)
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/history/interface", s.handleIfaceHistory)
	mux.HandleFunc("/api/history/iface-volume", s.handleIfaceVolume)
	mux.HandleFunc("/api/history/ips", s.handleIPTop)
	mux.HandleFunc("/api/history/ip", s.handleIPDetailHistory)
	mux.HandleFunc("/api/history/apps", s.handleAppTop)
	mux.HandleFunc("/api/audit/domains", s.handleDomainAudit)
	mux.HandleFunc("/api/network/details", s.handleNetworkDetails)
	mux.HandleFunc("/api/dhcp/static", s.handleDHCPStatic)
	mux.HandleFunc("/api/system/logs", s.handleLogs)
	mux.HandleFunc("/api/firewall/rules", s.handleFirewallRules)
	mux.HandleFunc("/api/tools/wol", s.handleWoL)
	mux.HandleFunc("/api/appicon", s.handleAppIcon)
	mux.HandleFunc("/api/appicon/ver", s.handleAppIconVer)
	mux.HandleFunc("/api/update/status", s.handleUpdateStatus)
	mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("/api/update/config", s.handleUpdateConfig)
	mux.HandleFunc("/api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("/api/system/dbstats", s.handleDBStats)
	mux.HandleFunc("/api/system/clear-geo", s.handleClearGeo)
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

func (s *Server) handleAppIcon(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	if app == "" {
		w.WriteHeader(400)
		return
	}
	data, ct, cached := s.icons.GetIcon(app)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if cached {
		w.Header().Set("Cache-Control", "public, max-age=604800")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=5")
	}
	w.Write(data)
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

func (s *Server) handleIfaceVolume(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	iface := r.URL.Query().Get("iface")
	rng := r.URL.Query().Get("range")
	rx, tx, err := s.store.QueryIfaceVolume(id, iface, rng)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"iface":    iface,
		"range":    rng,
		"rx_bytes": rx,
		"tx_bytes": tx,
	})
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
	list, err := s.store.QueryTopApps(id, rng, 100)
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
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	list, total, err := s.store.QueryDomainAuditPaged(id, q, pageSize, offset)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if total == 0 && page == 1 {
		snap := s.col.GetSnapshot(id)
		if snap != nil && len(snap.RecentDomains) > 0 {
			list = snap.RecentDomains
			total = len(list)
		}
	}
	writeJSON(w, 200, map[string]any{
		"items":     list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) handleNetworkDetails(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	var leases []mikrotik.DHCPLease
	var arps []mikrotik.ARPEntry
	var routes []mikrotik.RouteEntry
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		_, _, l, err := mikrotik.GetDHCPLeases(c)
		if err == nil {
			leases = l
		}
		arps, _ = mikrotik.GetARPTable(c)
		routes, _ = mikrotik.GetRoutes(c)
		return nil
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"leases": leases,
		"arp":    arps,
		"routes": routes,
	})
}

func (s *Server) handleDHCPStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	leaseID := r.URL.Query().Get("id")
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		return mikrotik.MakeDHCPStatic(c, leaseID)
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	var logs []mikrotik.LogEntry
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		logs, e = mikrotik.GetLogs(c)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, logs)
}

func (s *Server) handleFirewallRules(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	var rules []mikrotik.FirewallRule
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		rules, e = mikrotik.GetFirewallFilters(c)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rules)
}

func (s *Server) handleWoL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	mac := r.URL.Query().Get("mac")
	iface := r.URL.Query().Get("iface")
	if mac == "" {
		writeJSON(w, 400, map[string]string{"error": "mac required"})
		return
	}
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		return mikrotik.WakeOnLAN(c, mac, iface)
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleDBStats(w http.ResponseWriter, r *http.Request) {
	st := s.store.GetDBStats()
	writeJSON(w, 200, map[string]any{
		"dpi_signatures":  dpi.SignatureCount(),
		"geo_cached_ips":  st.GeoCachedIPs,
		"audit_records":   st.AuditRecords,
		"history_samples": st.HistorySamples,
	})
}

func (s *Server) handleClearGeo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if err := s.col.ClearGeoCache(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
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

func (s *Server) handleAppIconVer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ver": s.icons.Version()})
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.upd.GetStatus())
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.upd.Check(r.Context()))
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoUpdate   bool   `json:"auto_update"`
		MirrorPrefix string `json:"mirror_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.upd.SaveConfig(req.AutoUpdate, req.MirrorPrefix)
	writeJSON(w, 200, s.upd.GetStatus())
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if err := s.upd.Apply(r.Context()); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "restarting"})
}
