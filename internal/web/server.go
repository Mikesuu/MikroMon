package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	srv := &Server{store: st, col: col, icons: icons.New(dataDir), upd: upd}
	srv.ensureAuthSeed()
	return srv
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/auth/setup", s.handleAuthSetup)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/auth/password", s.handleAuthPassword)
	mux.HandleFunc("/api/history/ips-total", s.handleClientTotalTraffic)
	mux.HandleFunc("/api/history/ips-total/reset", s.handleResetClientTotal)
	mux.HandleFunc("/api/search/history", s.handleHistorySearch)

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
	mux.HandleFunc("/api/appicon/clear", s.handleClearIcons)
	mux.HandleFunc("/api/update/status", s.handleUpdateStatus)
	mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("/api/update/config", s.handleUpdateConfig)
	mux.HandleFunc("/api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("/api/system/dbstats", s.handleDBStats)
	mux.HandleFunc("/api/system/clear-geo", s.handleClearGeo)
	mux.HandleFunc("/api/geo/db/update", s.handleGeoDBUpdate)
	mux.HandleFunc("/api/geo/config", s.handleGeoConfig)
	mux.HandleFunc("/api/action/interface", s.handleToggleIface)
	mux.HandleFunc("/api/queues", s.handleQueues)
	mux.HandleFunc("/api/blocked", s.handleBlocked)
	mux.HandleFunc("/api/tools/torch", s.handleTorch)
	mux.HandleFunc("/api/tools/ping", s.handlePing)
	mux.HandleFunc("/api/tools/traceroute", s.handleTraceroute)
	mux.HandleFunc("/api/tools/dnslookup", s.handleDNSLookup)
	mux.HandleFunc("/api/security/events", s.handleSecurityEvents)
	mux.HandleFunc("/api/security/devices", s.handleKnownDevices)
	mux.HandleFunc("/api/security/devices/trust", s.handleTrustDevice)
	mux.HandleFunc("/api/security/baseline", s.handleSecurityBaseline)
	mux.HandleFunc("/api/security/ensure-fw", s.handleEnsureBlockFW)
	mux.HandleFunc("/api/security/config", s.handleSecurityConfig)
	mux.HandleFunc("/api/dpi/rules", s.handleDPIRules)
	mux.HandleFunc("/api/tools/flush-dns", s.handleFlushDNS)

	sub, _ := fs.Sub(staticFS, "static")
	fileSrv := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fileSrv.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.authMiddleware(mux),
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
	list, err := s.store.QueryTopApps(id, rng, 1000)
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
		"geo_db":          s.col.GeoDBStatus(),
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

func (s *Server) handleTraceroute(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	target := r.URL.Query().Get("target")
	if target == "" {
		writeJSON(w, 400, map[string]string{"error": "target required"})
		return
	}
	var rows []map[string]string
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		rows, e = mikrotik.RunTraceroute(c, target)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) handleDNSLookup(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		writeJSON(w, 400, map[string]string{"error": "domain required"})
		return
	}
	ips, err := net.LookupHost(domain)
	if err != nil {
		writeJSON(w, 200, map[string]any{"domain": domain, "error": err.Error(), "ips": []string{}})
		return
	}
	writeJSON(w, 200, map[string]any{"domain": domain, "ips": ips})
}

func (s *Server) handleSecurityEvents(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	list, total, err := s.store.QuerySecurityEvents(id, pageSize, (page-1)*pageSize)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func (s *Server) handleKnownDevices(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	list, err := s.store.ListKnownDevices(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleTrustDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	var req struct {
		MAC     string `json:"mac"`
		Trusted bool   `json:"trusted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SetDeviceTrusted(id, req.MAC, req.Trusted); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleSecurityBaseline(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	var checks []mikrotik.BaselineCheck
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		var e error
		checks, e = mikrotik.RunBaselineAudit(c)
		return e
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, checks)
}

func (s *Server) handleEnsureBlockFW(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		return mikrotik.EnsureBlocklistFirewallRule(c)
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleSecurityConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			AutoBlock bool   `json:"auto_block"`
			GeoWatch  string `json:"geo_watch"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		ab := "false"
		if req.AutoBlock {
			ab = "true"
		}
		_ = s.store.SetSetting("sec_auto_block", ab)
		_ = s.store.SetSetting("sec_geo_watch", req.GeoWatch)
	}
	writeJSON(w, 200, map[string]any{
		"auto_block": s.store.GetSetting("sec_auto_block") == "true",
		"geo_watch":  s.store.GetSetting("sec_geo_watch"),
	})
}

func (s *Server) handleDPIRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListCustomDPIRules()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var items []store.CustomDPIRule
		bodyBytes := make([]byte, 0)
		_ = bodyBytes
		dec := json.NewDecoder(r.Body)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if len(raw) > 0 && raw[0] == '[' {
			if err := json.Unmarshal(raw, &items); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
		} else {
			var single store.CustomDPIRule
			if err := json.Unmarshal(raw, &single); err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			items = append(items, single)
		}
		if err := s.store.SaveCustomDPIRules(items); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		collector.ReloadCustomDPIRules(s.store)
		writeJSON(w, 200, map[string]any{"status": "ok", "count": len(items)})
	case http.MethodDelete:
		dom := r.URL.Query().Get("domain")
		if err := s.store.DeleteCustomDPIRule(dom); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		collector.ReloadCustomDPIRules(s.store)
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(405)
	}
}

func (s *Server) handleFlushDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	err := s.col.WithClient(id, func(c *routeros.Client) error {
		return mikrotik.FlushDNSCache(c)
	})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
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

func (s *Server) handleClearIcons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	s.icons.ClearCache()
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleClientTotalTraffic(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	list, err := s.store.QueryClientTotalTraffic(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) handleResetClientTotal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	id := s.resolveRouterID(r)
	if err := s.store.ResetClientTotalTraffic(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleHistorySearch(w http.ResponseWriter, r *http.Request) {
	id := s.resolveRouterID(r)
	q := r.URL.Query().Get("q")
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 14
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	items, total, err := s.store.QueryHistorySearch(id, q, days, pageSize, (page-1)*pageSize)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"days":      days,
	})
}

func (s *Server) handleGeoDBUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if err := s.col.UpdateGeoDB(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "geo_db": s.col.GeoDBStatus()})
}

func (s *Server) handleGeoConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			IP138Token string `json:"ip138_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		_ = s.store.SetSetting("geo_ip138_token", strings.TrimSpace(req.IP138Token))
	}
	writeJSON(w, 200, map[string]any{
		"ip138_token": s.store.GetSetting("geo_ip138_token"),
	})
}
