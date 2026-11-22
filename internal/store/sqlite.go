package store

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type RouterConfig struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password,omitempty"`
	UseTLS        bool   `json:"use_tls"`
	TLSSkipVerify bool   `json:"tls_skip_verify"`
	Enabled       bool   `json:"enabled"`
	AdgURL        string `json:"adg_url"`
	AdgUser       string `json:"adg_user"`
	AdgPass       string `json:"adg_pass,omitempty"`
}

type IfaceSample struct {
	RouterID int64
	TS       int64
	Iface    string
	RxBps    int64
	TxBps    int64
	RxBytes  int64
	TxBytes  int64
	Running  bool
}

type IfaceSeriesPoint struct {
	TS    int64 `json:"ts"`
	RxBps int64 `json:"rx_bps"`
	TxBps int64 `json:"tx_bps"`
}

type ClientTotalStat struct {
	IP         string `json:"ip"`
	HostName   string `json:"host_name"`
	MAC        string `json:"mac"`
	TodayRx    int64  `json:"today_rx"`
	TodayTx    int64  `json:"today_tx"`
	TodayTotal int64  `json:"today_total"`
	AllRx      int64  `json:"all_rx"`
	AllTx      int64  `json:"all_tx"`
	AllTotal   int64  `json:"all_total"`
	UpdatedAt  int64  `json:"updated_at"`
}

type IPStat struct {
	IP       string `json:"ip"`
	HostName string `json:"host_name"`
	MAC      string `json:"mac"`
	RxBytes  int64  `json:"rx_bytes"`
	TxBytes  int64  `json:"tx_bytes"`
	Total    int64  `json:"total_bytes"`
}

type AppStat struct {
	App      string `json:"app"`
	Category string `json:"category"`
	RxBytes  int64  `json:"rx_bytes"`
	TxBytes  int64  `json:"tx_bytes"`
	Total    int64  `json:"total_bytes"`
}

type IPTrafficDelta struct {
	IP       string
	HostName string
	MAC      string
	RxBytes  int64
	TxBytes  int64
}

type AppTrafficDelta struct {
	App     string
	RxBytes int64
	TxBytes int64
}

type ISPRecord struct {
	IP         string `json:"ip"`
	Location   string `json:"location"`
	LocationEN string `json:"location_en"`
	ISP        string `json:"isp"`
	Org        string `json:"org"`
	ASN        string `json:"asn"`
	UpdatedAt  int64  `json:"updated_at"`
}

type DomainAuditEntry struct {
	TS       int64  `json:"ts"`
	RouterID int64  `json:"router_id"`
	ClientIP string `json:"client_ip"`
	HostName string `json:"host_name"`
	Domain   string `json:"domain"`
	App      string `json:"app"`
	Category string `json:"category"`
	Source   string `json:"source"`
}

type SecurityEventRecord struct {
	ID          int64  `json:"id"`
	TS          int64  `json:"ts"`
	RouterID    int64  `json:"router_id"`
	Level       string `json:"level"`
	Rule        string `json:"rule"`
	ClientIP    string `json:"client_ip"`
	HostName    string `json:"host_name"`
	RemoteIP    string `json:"remote_ip"`
	RemotePort  int    `json:"remote_port"`
	Domain      string `json:"domain"`
	Detail      string `json:"detail"`
	ActionTaken string `json:"action_taken"`
}

type KnownDevice struct {
	RouterID  int64  `json:"router_id"`
	MAC       string `json:"mac"`
	LastIP    string `json:"last_ip"`
	HostName  string `json:"host_name"`
	FirstSeen int64  `json:"first_seen"`
	LastSeen  int64  `json:"last_seen"`
	Trusted   bool   `json:"trusted"`
}

type DBStats struct {
	GeoCachedIPs   int64 `json:"geo_cached_ips"`
	AuditRecords   int64 `json:"audit_records"`
	HistorySamples int64 `json:"history_samples"`
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA temp_store=MEMORY;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %s: %w", p, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS routers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 8728,
		username TEXT NOT NULL,
		password TEXT NOT NULL,
		use_tls INTEGER NOT NULL DEFAULT 0,
		tls_skip_verify INTEGER NOT NULL DEFAULT 1,
		enabled INTEGER NOT NULL DEFAULT 1,
		adg_url TEXT NOT NULL DEFAULT '',
		adg_user TEXT NOT NULL DEFAULT '',
		adg_pass TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS iface_raw (
		ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		iface TEXT NOT NULL,
		rx_bps INTEGER NOT NULL,
		tx_bps INTEGER NOT NULL,
		rx_bytes INTEGER NOT NULL,
		tx_bytes INTEGER NOT NULL,
		running INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_iface_raw_q ON iface_raw(router_id, iface, ts);
	CREATE INDEX IF NOT EXISTS idx_iface_raw_ts ON iface_raw(ts);

	CREATE TABLE IF NOT EXISTS iface_rollup (
		bucket_ts INTEGER NOT NULL,
		gran TEXT NOT NULL,
		router_id INTEGER NOT NULL,
		iface TEXT NOT NULL,
		rx_bps_avg INTEGER NOT NULL,
		tx_bps_avg INTEGER NOT NULL,
		rx_bps_max INTEGER NOT NULL,
		tx_bps_max INTEGER NOT NULL,
		samples INTEGER NOT NULL,
		PRIMARY KEY (bucket_ts, gran, router_id, iface)
	);

	CREATE TABLE IF NOT EXISTS ip_traffic_1m (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		mac TEXT NOT NULL DEFAULT '',
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, ip)
	);
	CREATE INDEX IF NOT EXISTS idx_ip_1m_q ON ip_traffic_1m(router_id, bucket_ts, ip);

	CREATE TABLE IF NOT EXISTS app_traffic_1m (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		app TEXT NOT NULL,
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, app)
	);

	CREATE TABLE IF NOT EXISTS ip_traffic_5m (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		mac TEXT NOT NULL DEFAULT '',
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, ip)
	);
	CREATE INDEX IF NOT EXISTS idx_ip_5m_q ON ip_traffic_5m(router_id, bucket_ts, ip);

	CREATE TABLE IF NOT EXISTS ip_traffic_1h (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		mac TEXT NOT NULL DEFAULT '',
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, ip)
	);
	CREATE INDEX IF NOT EXISTS idx_ip_1h_q ON ip_traffic_1h(router_id, bucket_ts, ip);

	CREATE TABLE IF NOT EXISTS app_traffic_5m (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		app TEXT NOT NULL,
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, app)
	);

	CREATE TABLE IF NOT EXISTS app_traffic_1h (
		bucket_ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		app TEXT NOT NULL,
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_ts, router_id, app)
	);

	CREATE TABLE IF NOT EXISTS ip_isp_cache (
		ip TEXT PRIMARY KEY,
		location TEXT NOT NULL DEFAULT '',
		location_en TEXT NOT NULL DEFAULT '',
		isp TEXT NOT NULL DEFAULT '',
		org TEXT NOT NULL DEFAULT '',
		asn TEXT NOT NULL DEFAULT '',
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS app_settings (
		k TEXT PRIMARY KEY,
		v TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS domain_audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		client_ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		domain TEXT NOT NULL,
		app TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_domain_audit_q ON domain_audit(router_id, ts DESC);

	CREATE TABLE IF NOT EXISTS security_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts INTEGER NOT NULL,
		router_id INTEGER NOT NULL,
		level TEXT NOT NULL,
		rule TEXT NOT NULL,
		client_ip TEXT NOT NULL DEFAULT '',
		host_name TEXT NOT NULL DEFAULT '',
		remote_ip TEXT NOT NULL DEFAULT '',
		remote_port INTEGER NOT NULL DEFAULT 0,
		domain TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		action_taken TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_sec_events_q ON security_events(router_id, ts DESC);

	CREATE TABLE IF NOT EXISTS known_devices (
		router_id INTEGER NOT NULL,
		mac TEXT NOT NULL,
		last_ip TEXT NOT NULL DEFAULT '',
		host_name TEXT NOT NULL DEFAULT '',
		first_seen INTEGER NOT NULL,
		last_seen INTEGER NOT NULL,
		trusted INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (router_id, mac)
	);

	CREATE TABLE IF NOT EXISTS custom_dpi_rules (
		domain TEXT PRIMARY KEY,
		app TEXT NOT NULL,
		category TEXT NOT NULL,
		color TEXT NOT NULL DEFAULT '#0099cc'
	);

	CREATE TABLE IF NOT EXISTS ip_traffic_total (
		router_id INTEGER NOT NULL,
		ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		mac TEXT NOT NULL DEFAULT '',
		rx_bytes INTEGER NOT NULL DEFAULT 0,
		tx_bytes INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (router_id, ip)
	);

	CREATE TABLE IF NOT EXISTS conn_history (
		router_id INTEGER NOT NULL,
		client_ip TEXT NOT NULL,
		host_name TEXT NOT NULL DEFAULT '',
		remote_ip TEXT NOT NULL,
		remote_port INTEGER NOT NULL DEFAULT 0,
		protocol TEXT NOT NULL DEFAULT '',
		domain TEXT NOT NULL DEFAULT '',
		location TEXT NOT NULL DEFAULT '',
		location_en TEXT NOT NULL DEFAULT '',
		isp TEXT NOT NULL DEFAULT '',
		app TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		bytes_total INTEGER NOT NULL DEFAULT 0,
		first_ts INTEGER NOT NULL,
		last_ts INTEGER NOT NULL,
		PRIMARY KEY (router_id, client_ip, remote_ip, remote_port, domain)
	);
	CREATE INDEX IF NOT EXISTS idx_conn_hist_q ON conn_history(router_id, last_ts DESC);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_url TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_user TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_pass TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE ip_isp_cache ADD COLUMN asn TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE ip_isp_cache ADD COLUMN location_en TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`
		INSERT OR IGNORE INTO ip_traffic_total (router_id, ip, host_name, mac, rx_bytes, tx_bytes, updated_at)
		SELECT router_id, ip, MAX(host_name), MAX(mac), SUM(rx_bytes), SUM(tx_bytes), MAX(bucket_ts)
		FROM ip_traffic_1h
		WHERE NOT EXISTS (SELECT 1 FROM ip_traffic_total LIMIT 1)
		GROUP BY router_id, ip;
	`)
	if rows, err := s.db.Query(`SELECT DISTINCT ip FROM ip_traffic_total`); err == nil {
		var toDel []string
		for rows.Next() {
			var ip string
			if rows.Scan(&ip) == nil && !IsPrivateLANIP(ip) {
				toDel = append(toDel, ip)
			}
		}
		rows.Close()
		for _, ip := range toDel {
			_, _ = s.db.Exec(`DELETE FROM ip_traffic_total WHERE ip=?`, ip)
		}
	}
	return nil
}

func (s *Store) ListRouters() ([]RouterConfig, error) {
	rows, err := s.db.Query(`SELECT id, name, host, port, username, password, use_tls, tls_skip_verify, enabled, adg_url, adg_user, adg_pass FROM routers ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RouterConfig
	for rows.Next() {
		var r RouterConfig
		var tlsInt, skipInt, enInt int
		if err := rows.Scan(&r.ID, &r.Name, &r.Host, &r.Port, &r.Username, &r.Password, &tlsInt, &skipInt, &enInt, &r.AdgURL, &r.AdgUser, &r.AdgPass); err != nil {
			return nil, err
		}
		r.UseTLS = tlsInt == 1
		r.TLSSkipVerify = skipInt == 1
		r.Enabled = enInt == 1
		list = append(list, r)
	}
	return list, nil
}

func (s *Store) GetRouter(id int64) (*RouterConfig, error) {
	list, err := s.ListRouters()
	if err != nil {
		return nil, err
	}
	for _, r := range list {
		if r.ID == id {
			return &r, nil
		}
	}
	if len(list) > 0 && id == 0 {
		return &list[0], nil
	}
	return nil, sql.ErrNoRows
}

func (s *Store) SaveRouter(cfg RouterConfig) (int64, error) {
	useTLS := 0
	if cfg.UseTLS {
		useTLS = 1
	}
	skip := 0
	if cfg.TLSSkipVerify {
		skip = 1
	}
	en := 1
	if !cfg.Enabled {
		en = 0
	}
	if cfg.ID > 0 {
		old, err := s.GetRouter(cfg.ID)
		if err == nil {
			if cfg.Password == "" {
				cfg.Password = old.Password
			}
			if cfg.AdgPass == "" {
				cfg.AdgPass = old.AdgPass
			}
		}
		_, err = s.db.Exec(`UPDATE routers SET name=?, host=?, port=?, username=?, password=?, use_tls=?, tls_skip_verify=?, enabled=?, adg_url=?, adg_user=?, adg_pass=? WHERE id=?`,
			cfg.Name, cfg.Host, cfg.Port, cfg.Username, cfg.Password, useTLS, skip, en, cfg.AdgURL, cfg.AdgUser, cfg.AdgPass, cfg.ID)
		return cfg.ID, err
	}
	res, err := s.db.Exec(`INSERT INTO routers (name, host, port, username, password, use_tls, tls_skip_verify, enabled, adg_url, adg_user, adg_pass, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		cfg.Name, cfg.Host, cfg.Port, cfg.Username, cfg.Password, useTLS, skip, en, cfg.AdgURL, cfg.AdgUser, cfg.AdgPass, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteRouter(id int64) error {
	_, err := s.db.Exec(`DELETE FROM routers WHERE id=?`, id)
	return err
}

func (s *Store) LoadISPCache() (map[string]ISPRecord, error) {
	rows, err := s.db.Query(`SELECT ip, location, location_en, isp, org, asn, updated_at FROM ip_isp_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]ISPRecord)
	for rows.Next() {
		var r ISPRecord
		if err := rows.Scan(&r.IP, &r.Location, &r.LocationEN, &r.ISP, &r.Org, &r.ASN, &r.UpdatedAt); err == nil {
			out[r.IP] = r
		}
	}
	return out, nil
}

func (s *Store) SaveISPCache(items []ISPRecord) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO ip_isp_cache (ip, location, location_en, isp, org, asn, updated_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(ip) DO UPDATE SET location=excluded.location, location_en=excluded.location_en, isp=excluded.isp, org=excluded.org, asn=excluded.asn, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(it.IP, it.Location, it.LocationEN, it.ISP, it.Org, it.ASN, it.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ClearISPCache() error {
	_, err := s.db.Exec(`DELETE FROM ip_isp_cache`)
	return err
}

func (s *Store) GetDBStats() DBStats {
	var st DBStats
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM ip_isp_cache`).Scan(&st.GeoCachedIPs)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM domain_audit`).Scan(&st.AuditRecords)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iface_raw`).Scan(&st.HistorySamples)
	return st
}

func (s *Store) InsertDomainAudits(items []DomainAuditEntry) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO domain_audit (ts, router_id, client_ip, host_name, domain, app, category, source) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(it.TS, it.RouterID, it.ClientIP, it.HostName, it.Domain, it.App, it.Category, it.Source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) QueryDomainAudit(routerID int64, q string, limit int) ([]DomainAuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows *sql.Rows
	var err error
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		rows, err = s.db.Query(`SELECT ts, router_id, client_ip, host_name, domain, app, category, source FROM domain_audit WHERE router_id=? AND (LOWER(client_ip) LIKE ? OR LOWER(host_name) LIKE ? OR LOWER(domain) LIKE ? OR LOWER(app) LIKE ?) ORDER BY ts DESC, id DESC LIMIT ?`,
			routerID, like, like, like, like, limit)
	} else {
		rows, err = s.db.Query(`SELECT ts, router_id, client_ip, host_name, domain, app, category, source FROM domain_audit WHERE router_id=? ORDER BY ts DESC, id DESC LIMIT ?`,
			routerID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DomainAuditEntry, 0)
	for rows.Next() {
		var it DomainAuditEntry
		if err := rows.Scan(&it.TS, &it.RouterID, &it.ClientIP, &it.HostName, &it.Domain, &it.App, &it.Category, &it.Source); err == nil {
			out = append(out, it)
		}
	}
	return out, nil
}

func (s *Store) QueryDomainAuditPaged(routerID int64, q string, limit, offset int) ([]DomainAuditEntry, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	var rows *sql.Rows
	var err error
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM domain_audit WHERE router_id=? AND (LOWER(client_ip) LIKE ? OR LOWER(host_name) LIKE ? OR LOWER(domain) LIKE ? OR LOWER(app) LIKE ?)`,
			routerID, like, like, like, like).Scan(&total)
		rows, err = s.db.Query(`SELECT ts, router_id, client_ip, host_name, domain, app, category, source FROM domain_audit WHERE router_id=? AND (LOWER(client_ip) LIKE ? OR LOWER(host_name) LIKE ? OR LOWER(domain) LIKE ? OR LOWER(app) LIKE ?) ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`,
			routerID, like, like, like, like, limit, offset)
	} else {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM domain_audit WHERE router_id=?`, routerID).Scan(&total)
		rows, err = s.db.Query(`SELECT ts, router_id, client_ip, host_name, domain, app, category, source FROM domain_audit WHERE router_id=? ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`,
			routerID, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]DomainAuditEntry, 0)
	for rows.Next() {
		var it DomainAuditEntry
		if err := rows.Scan(&it.TS, &it.RouterID, &it.ClientIP, &it.HostName, &it.Domain, &it.App, &it.Category, &it.Source); err == nil {
			out = append(out, it)
		}
	}
	return out, total, nil
}

func (s *Store) InsertIfaceSamples(samples []IfaceSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtRaw, err := tx.Prepare(`INSERT INTO iface_raw (ts, router_id, iface, rx_bps, tx_bps, rx_bytes, tx_bytes, running) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmtRaw.Close()

	stmtRollup, err := tx.Prepare(`
		INSERT INTO iface_rollup (bucket_ts, gran, router_id, iface, rx_bps_avg, tx_bps_avg, rx_bps_max, tx_bps_max, samples)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(bucket_ts, gran, router_id, iface) DO UPDATE SET
			rx_bps_avg = (rx_bps_avg * samples + excluded.rx_bps_avg) / (samples + 1),
			tx_bps_avg = (tx_bps_avg * samples + excluded.tx_bps_avg) / (samples + 1),
			rx_bps_max = MAX(rx_bps_max, excluded.rx_bps_max),
			tx_bps_max = MAX(tx_bps_max, excluded.tx_bps_max),
			samples = samples + 1
	`)
	if err != nil {
		return err
	}
	defer stmtRollup.Close()

	for _, sm := range samples {
		run := 0
		if sm.Running {
			run = 1
		}
		if _, err := stmtRaw.Exec(sm.TS, sm.RouterID, sm.Iface, sm.RxBps, sm.TxBps, sm.RxBytes, sm.TxBytes, run); err != nil {
			return err
		}
		b5m := (sm.TS / 300) * 300
		if _, err := stmtRollup.Exec(b5m, "5m", sm.RouterID, sm.Iface, sm.RxBps, sm.TxBps, sm.RxBps, sm.TxBps); err != nil {
			return err
		}
		b1h := (sm.TS / 3600) * 3600
		if _, err := stmtRollup.Exec(b1h, "1h", sm.RouterID, sm.Iface, sm.RxBps, sm.TxBps, sm.RxBps, sm.TxBps); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AddTrafficDeltas(routerID, ts int64, ips []IPTrafficDelta, apps []AppTrafficDelta) error {
	if len(ips) == 0 && len(apps) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	b1m := (ts / 60) * 60
	b5m := (ts / 300) * 300
	b1h := (ts / 3600) * 3600

	stmtIP5m, err := tx.Prepare(`
		INSERT INTO ip_traffic_5m (bucket_ts, router_id, ip, host_name, mac, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, ip) DO UPDATE SET
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE mac END,
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtIP5m.Close()

	stmtIP1h, err := tx.Prepare(`
		INSERT INTO ip_traffic_1h (bucket_ts, router_id, ip, host_name, mac, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, ip) DO UPDATE SET
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE mac END,
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtIP1h.Close()

	stmtIP1m, err := tx.Prepare(`
		INSERT INTO ip_traffic_1m (bucket_ts, router_id, ip, host_name, mac, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, ip) DO UPDATE SET
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE mac END,
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtIP1m.Close()

	stmtIPTotal, err := tx.Prepare(`
		INSERT INTO ip_traffic_total (router_id, ip, host_name, mac, rx_bytes, tx_bytes, updated_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(router_id, ip) DO UPDATE SET
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE mac END,
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return err
	}
	defer stmtIPTotal.Close()

	for _, d := range ips {
		if IsPrivateLANIP(d.IP) {
			if _, err := stmtIPTotal.Exec(routerID, d.IP, d.HostName, d.MAC, d.RxBytes, d.TxBytes, ts); err != nil {
				return err
			}
		}
		if _, err := stmtIP1m.Exec(b1m, routerID, d.IP, d.HostName, d.MAC, d.RxBytes, d.TxBytes); err != nil {
			return err
		}
		if _, err := stmtIP5m.Exec(b5m, routerID, d.IP, d.HostName, d.MAC, d.RxBytes, d.TxBytes); err != nil {
			return err
		}
		if _, err := stmtIP1h.Exec(b1h, routerID, d.IP, d.HostName, d.MAC, d.RxBytes, d.TxBytes); err != nil {
			return err
		}
	}

	stmtApp5m, err := tx.Prepare(`
		INSERT INTO app_traffic_5m (bucket_ts, router_id, app, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, app) DO UPDATE SET
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtApp5m.Close()

	stmtApp1h, err := tx.Prepare(`
		INSERT INTO app_traffic_1h (bucket_ts, router_id, app, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, app) DO UPDATE SET
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtApp1h.Close()

	stmtApp1m, err := tx.Prepare(`
		INSERT INTO app_traffic_1m (bucket_ts, router_id, app, rx_bytes, tx_bytes)
		VALUES (?,?,?,?,?)
		ON CONFLICT(bucket_ts, router_id, app) DO UPDATE SET
			rx_bytes = rx_bytes + excluded.rx_bytes,
			tx_bytes = tx_bytes + excluded.tx_bytes
	`)
	if err != nil {
		return err
	}
	defer stmtApp1m.Close()

	for _, a := range apps {
		if _, err := stmtApp1m.Exec(b1m, routerID, a.App, a.RxBytes, a.TxBytes); err != nil {
			return err
		}
		if _, err := stmtApp5m.Exec(b5m, routerID, a.App, a.RxBytes, a.TxBytes); err != nil {
			return err
		}
		if _, err := stmtApp1h.Exec(b1h, routerID, a.App, a.RxBytes, a.TxBytes); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func RangeDuration(rangeKey string) (time.Duration, string) {
	switch rangeKey {
	case "1m":
		return time.Minute, "raw"
	case "5m":
		return 5 * time.Minute, "raw"
	case "15m":
		return 15 * time.Minute, "raw"
	case "30m":
		return 30 * time.Minute, "raw"
	case "1h":
		return time.Hour, "raw"
	case "6h":
		return 6 * time.Hour, "raw60"
	case "12h":
		return 12 * time.Hour, "raw120"
	case "24h":
		return 24 * time.Hour, "5m"
	case "7d":
		return 7 * 24 * time.Hour, "5m"
	case "15d":
		return 15 * 24 * time.Hour, "1h"
	case "30d":
		return 30 * 24 * time.Hour, "1h"
	default:
		return time.Hour, "raw"
	}
}

func (s *Store) QueryIfaceHistory(routerID int64, iface, rangeKey string) ([]IfaceSeriesPoint, error) {
	dur, mode := RangeDuration(rangeKey)
	since := time.Now().Add(-dur).Unix()

	var rows *sql.Rows
	var err error

	switch mode {
	case "raw":
		rows, err = s.db.Query(`SELECT ts, rx_bps, tx_bps FROM iface_raw WHERE router_id=? AND iface=? AND ts>=? ORDER BY ts ASC`, routerID, iface, since)
	case "raw60":
		rows, err = s.db.Query(`SELECT (ts/60)*60 as b, CAST(AVG(rx_bps) AS INTEGER), CAST(AVG(tx_bps) AS INTEGER) FROM iface_raw WHERE router_id=? AND iface=? AND ts>=? GROUP BY b ORDER BY b ASC`, routerID, iface, since)
	case "raw120":
		rows, err = s.db.Query(`SELECT (ts/120)*120 as b, CAST(AVG(rx_bps) AS INTEGER), CAST(AVG(tx_bps) AS INTEGER) FROM iface_raw WHERE router_id=? AND iface=? AND ts>=? GROUP BY b ORDER BY b ASC`, routerID, iface, since)
	case "5m":
		rows, err = s.db.Query(`SELECT bucket_ts, rx_bps_avg, tx_bps_avg FROM iface_rollup WHERE router_id=? AND iface=? AND gran='5m' AND bucket_ts>=? ORDER BY bucket_ts ASC`, routerID, iface, since)
	case "1h":
		rows, err = s.db.Query(`SELECT bucket_ts, rx_bps_avg, tx_bps_avg FROM iface_rollup WHERE router_id=? AND iface=? AND gran='1h' AND bucket_ts>=? ORDER BY bucket_ts ASC`, routerID, iface, since)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pts := make([]IfaceSeriesPoint, 0)
	for rows.Next() {
		var p IfaceSeriesPoint
		if err := rows.Scan(&p.TS, &p.RxBps, &p.TxBps); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, nil
}

func (s *Store) QueryIfaceVolume(routerID int64, iface, rangeKey string) (int64, int64, error) {
	dur, mode := RangeDuration(rangeKey)
	since := time.Now().Add(-dur).Unix()

	switch mode {
	case "raw", "raw60", "raw120":
		rows, err := s.db.Query(`SELECT rx_bytes, tx_bytes FROM iface_raw WHERE router_id=? AND iface=? AND ts>=? ORDER BY ts ASC`, routerID, iface, since)
		if err != nil {
			return 0, 0, err
		}
		defer rows.Close()
		var totRx, totTx, prevRx, prevTx int64
		first := true
		for rows.Next() {
			var rx, tx int64
			if err := rows.Scan(&rx, &tx); err != nil {
				return 0, 0, err
			}
			if first {
				prevRx, prevTx = rx, tx
				first = false
				continue
			}
			if rx >= prevRx {
				totRx += rx - prevRx
			}
			if tx >= prevTx {
				totTx += tx - prevTx
			}
			prevRx, prevTx = rx, tx
		}
		return totRx, totTx, nil
	case "5m":
		var rx, tx int64
		err := s.db.QueryRow(`SELECT COALESCE(SUM(rx_bps_avg),0)*300/8, COALESCE(SUM(tx_bps_avg),0)*300/8 FROM iface_rollup WHERE router_id=? AND iface=? AND gran='5m' AND bucket_ts>=?`, routerID, iface, since).Scan(&rx, &tx)
		return rx, tx, err
	case "1h":
		var rx, tx int64
		err := s.db.QueryRow(`SELECT COALESCE(SUM(rx_bps_avg),0)*3600/8, COALESCE(SUM(tx_bps_avg),0)*3600/8 FROM iface_rollup WHERE router_id=? AND iface=? AND gran='1h' AND bucket_ts>=?`, routerID, iface, since).Scan(&rx, &tx)
		return rx, tx, err
	}
	return 0, 0, nil
}

func (s *Store) QueryTopIPs(routerID int64, rangeKey string, limit int) ([]IPStat, error) {
	dur, mode := RangeDuration(rangeKey)
	since := time.Now().Add(-dur).Unix()
	table := "ip_traffic_5m"
	since = (since / 300) * 300
	if mode == "raw" {
		table = "ip_traffic_1m"
		since = (time.Now().Add(-dur).Unix() / 60) * 60
	} else if mode == "1h" {
		table = "ip_traffic_1h"
		since = (time.Now().Add(-dur).Unix() / 3600) * 3600
	}
	if limit <= 0 {
		limit = 200
	}
	q := fmt.Sprintf(`
		SELECT ip, MAX(host_name), MAX(mac), SUM(rx_bytes) as rx, SUM(tx_bytes) as tx
		FROM %s
		WHERE router_id=? AND bucket_ts>=?
		GROUP BY ip
		ORDER BY (rx + tx) DESC
		LIMIT ?
	`, table)
	rows, err := s.db.Query(q, routerID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]IPStat, 0)
	for rows.Next() {
		var st IPStat
		if err := rows.Scan(&st.IP, &st.HostName, &st.MAC, &st.RxBytes, &st.TxBytes); err != nil {
			return nil, err
		}
		st.Total = st.RxBytes + st.TxBytes
		out = append(out, st)
	}
	return out, nil
}

func (s *Store) QueryIPHistory(routerID int64, ip, rangeKey string) ([]IfaceSeriesPoint, error) {
	dur, mode := RangeDuration(rangeKey)
	since := time.Now().Add(-dur).Unix()
	table := "ip_traffic_5m"
	stepSec := int64(300)
	since = (since / 300) * 300
	if mode == "raw" {
		table = "ip_traffic_1m"
		stepSec = 60
		since = (time.Now().Add(-dur).Unix() / 60) * 60
	} else if mode == "1h" {
		table = "ip_traffic_1h"
		stepSec = 3600
		since = (time.Now().Add(-dur).Unix() / 3600) * 3600
	}
	q := fmt.Sprintf(`SELECT bucket_ts, rx_bytes, tx_bytes FROM %s WHERE router_id=? AND ip=? AND bucket_ts>=? ORDER BY bucket_ts ASC`, table)
	rows, err := s.db.Query(q, routerID, ip, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]IfaceSeriesPoint, 0)
	for rows.Next() {
		var ts, rxBytes, txBytes int64
		if err := rows.Scan(&ts, &rxBytes, &txBytes); err != nil {
			return nil, err
		}
		out = append(out, IfaceSeriesPoint{
			TS:    ts,
			RxBps: (rxBytes * 8) / stepSec,
			TxBps: (txBytes * 8) / stepSec,
		})
	}
	return out, nil
}

type ConnHistoryEntry struct {
	RouterID   int64  `json:"router_id"`
	ClientIP   string `json:"client_ip"`
	HostName   string `json:"host_name"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Protocol   string `json:"protocol"`
	Domain     string `json:"domain"`
	Location   string `json:"location"`
	LocationEN string `json:"location_en"`
	ISP        string `json:"isp"`
	App        string `json:"app"`
	Category   string `json:"category"`
	BytesTotal int64  `json:"bytes_total"`
	TS         int64  `json:"ts"`
	Source     string `json:"source"`
}

func IsPrivateLANIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 || (ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || (ip4[0] == 192 && ip4[1] == 168)
	}
	return len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc
}

func (s *Store) UpsertConnHistory(routerID int64, items []ConnHistoryEntry) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO conn_history (router_id, client_ip, host_name, remote_ip, remote_port, protocol, domain, location, location_en, isp, app, category, bytes_total, first_ts, last_ts)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(router_id, client_ip, remote_ip, remote_port, domain) DO UPDATE SET
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			location = CASE WHEN excluded.location != '' THEN excluded.location ELSE location END,
			location_en = CASE WHEN excluded.location_en != '' THEN excluded.location_en ELSE location_en END,
			isp = CASE WHEN excluded.isp != '' THEN excluded.isp ELSE isp END,
			app = CASE WHEN excluded.app != '' THEN excluded.app ELSE app END,
			category = CASE WHEN excluded.category != '' THEN excluded.category ELSE category END,
			bytes_total = MAX(bytes_total, excluded.bytes_total),
			last_ts = MAX(last_ts, excluded.last_ts)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(routerID, it.ClientIP, it.HostName, it.RemoteIP, it.RemotePort, it.Protocol, it.Domain, it.Location, it.LocationEN, it.ISP, it.App, it.Category, it.BytesTotal, it.TS, it.TS); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) QueryHistorySearch(routerID int64, q string, days, limit, offset int) ([]ConnHistoryEntry, int, error) {
	if days <= 0 {
		days = 14
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	trimQ := strings.TrimSpace(q)

	baseCTE := `
		WITH merged AS (
			SELECT last_ts AS ts, client_ip, host_name, remote_ip, remote_port, protocol, domain, location, location_en, isp, app, category, bytes_total, 'Flow' AS source
			FROM conn_history
			WHERE router_id=? AND last_ts>=?
			UNION ALL
			SELECT ts, client_ip, host_name, '' AS remote_ip, 0 AS remote_port, 'DNS' AS protocol, domain, '' AS location, '' AS location_en, '' AS isp, app, category, 0 AS bytes_total, source
			FROM domain_audit
			WHERE router_id=? AND ts>=?
		)
	`
	whereClause := `
		WHERE (? = '' OR LOWER(client_ip) LIKE ? OR LOWER(host_name) LIKE ? OR LOWER(remote_ip) LIKE ? OR LOWER(domain) LIKE ? OR LOWER(app) LIKE ? OR LOWER(category) LIKE ? OR LOWER(location) LIKE ? OR LOWER(location_en) LIKE ? OR LOWER(isp) LIKE ?)
	`
	args := []any{routerID, since, routerID, since, trimQ, like, like, like, like, like, like, like, like, like}
	var total int
	_ = s.db.QueryRow(baseCTE+"SELECT COUNT(*) FROM merged "+whereClause, args...).Scan(&total)

	qArgs := append(args, limit, offset)
	rows, err := s.db.Query(baseCTE+"SELECT ts, client_ip, host_name, remote_ip, remote_port, protocol, domain, location, location_en, isp, app, category, bytes_total, source FROM merged "+whereClause+" ORDER BY ts DESC LIMIT ? OFFSET ?", qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ConnHistoryEntry, 0)
	for rows.Next() {
		var it ConnHistoryEntry
		if rows.Scan(&it.TS, &it.ClientIP, &it.HostName, &it.RemoteIP, &it.RemotePort, &it.Protocol, &it.Domain, &it.Location, &it.LocationEN, &it.ISP, &it.App, &it.Category, &it.BytesTotal, &it.Source) == nil {
			it.RouterID = routerID
			out = append(out, it)
		}
	}
	return out, total, nil
}

func (s *Store) QueryClientTotalTraffic(routerID int64) ([]ClientTotalStat, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todayMap := make(map[string][2]int64)
	if rows, err := s.db.Query(`SELECT ip, SUM(rx_bytes), SUM(tx_bytes) FROM ip_traffic_5m WHERE router_id=? AND bucket_ts>=? GROUP BY ip`, routerID, todayStart); err == nil {
		for rows.Next() {
			var ip string
			var rx, tx int64
			if rows.Scan(&ip, &rx, &tx) == nil {
				todayMap[ip] = [2]int64{rx, tx}
			}
		}
		rows.Close()
	}
	rows, err := s.db.Query(`SELECT ip, host_name, mac, rx_bytes, tx_bytes, updated_at FROM ip_traffic_total WHERE router_id=? ORDER BY (rx_bytes + tx_bytes) DESC LIMIT 500`, routerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ClientTotalStat, 0)
	for rows.Next() {
		var st ClientTotalStat
		if err := rows.Scan(&st.IP, &st.HostName, &st.MAC, &st.AllRx, &st.AllTx, &st.UpdatedAt); err == nil {
			st.AllTotal = st.AllRx + st.AllTx
			if td, ok := todayMap[st.IP]; ok {
				st.TodayRx = td[0]
				st.TodayTx = td[1]
				st.TodayTotal = td[0] + td[1]
			}
			if IsPrivateLANIP(st.IP) {
				out = append(out, st)
			}
		}
	}
	return out, nil
}

func (s *Store) ResetClientTotalTraffic(routerID int64) error {
	_, err := s.db.Exec(`DELETE FROM ip_traffic_total WHERE router_id=?`, routerID)
	return err
}

func (s *Store) QueryTopApps(routerID int64, rangeKey string, limit int) ([]AppStat, error) {
	dur, mode := RangeDuration(rangeKey)
	since := time.Now().Add(-dur).Unix()
	table := "app_traffic_5m"
	since = (since / 300) * 300
	if mode == "raw" {
		table = "app_traffic_1m"
		since = (time.Now().Add(-dur).Unix() / 60) * 60
	} else if mode == "1h" {
		table = "app_traffic_1h"
		since = (time.Now().Add(-dur).Unix() / 3600) * 3600
	}
	if limit <= 0 {
		limit = 30
	}
	q := fmt.Sprintf(`
		SELECT app, SUM(rx_bytes) as rx, SUM(tx_bytes) as tx
		FROM %s
		WHERE router_id=? AND bucket_ts>=?
		GROUP BY app
		ORDER BY (rx + tx) DESC
		LIMIT ?
	`, table)
	rows, err := s.db.Query(q, routerID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AppStat, 0)
	for rows.Next() {
		var st AppStat
		if err := rows.Scan(&st.App, &st.RxBytes, &st.TxBytes); err != nil {
			return nil, err
		}
		st.Total = st.RxBytes + st.TxBytes
		out = append(out, st)
	}
	return out, nil
}

func (s *Store) InsertSecurityEvents(items []SecurityEventRecord) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO security_events (ts, router_id, level, rule, client_ip, host_name, remote_ip, remote_port, domain, detail, action_taken) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(it.TS, it.RouterID, it.Level, it.Rule, it.ClientIP, it.HostName, it.RemoteIP, it.RemotePort, it.Domain, it.Detail, it.ActionTaken); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) QuerySecurityEvents(routerID int64, limit, offset int) ([]SecurityEventRecord, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM security_events WHERE router_id=?`, routerID).Scan(&total)
	rows, err := s.db.Query(`SELECT id, ts, router_id, level, rule, client_ip, host_name, remote_ip, remote_port, domain, detail, action_taken FROM security_events WHERE router_id=? ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`, routerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]SecurityEventRecord, 0)
	for rows.Next() {
		var it SecurityEventRecord
		if err := rows.Scan(&it.ID, &it.TS, &it.RouterID, &it.Level, &it.Rule, &it.ClientIP, &it.HostName, &it.RemoteIP, &it.RemotePort, &it.Domain, &it.Detail, &it.ActionTaken); err == nil {
			out = append(out, it)
		}
	}
	return out, total, nil
}

func (s *Store) UpsertKnownDevices(routerID, now int64, devs []KnownDevice) ([]KnownDevice, error) {
	if len(devs) == 0 {
		return nil, nil
	}
	var existingCnt int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM known_devices WHERE router_id=?`, routerID).Scan(&existingCnt)

	knownMap := make(map[string]bool)
	rows, err := s.db.Query(`SELECT mac FROM known_devices WHERE router_id=?`, routerID)
	if err == nil {
		for rows.Next() {
			var m string
			if rows.Scan(&m) == nil {
				knownMap[m] = true
			}
		}
		rows.Close()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO known_devices (router_id, mac, last_ip, host_name, first_seen, last_seen, trusted)
		VALUES (?,?,?,?,?,?,0)
		ON CONFLICT(router_id, mac) DO UPDATE SET
			last_ip = CASE WHEN excluded.last_ip != '' THEN excluded.last_ip ELSE last_ip END,
			host_name = CASE WHEN excluded.host_name != '' THEN excluded.host_name ELSE host_name END,
			last_seen = excluded.last_seen
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var newDevs []KnownDevice
	for _, d := range devs {
		if d.MAC == "" {
			continue
		}
		if existingCnt > 0 && !knownMap[d.MAC] {
			knownMap[d.MAC] = true
			newDevs = append(newDevs, d)
		}
		if _, err := stmt.Exec(routerID, d.MAC, d.LastIP, d.HostName, now, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return newDevs, nil
}

func (s *Store) ListKnownDevices(routerID int64) ([]KnownDevice, error) {
	rows, err := s.db.Query(`SELECT router_id, mac, last_ip, host_name, first_seen, last_seen, trusted FROM known_devices WHERE router_id=? ORDER BY last_seen DESC`, routerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]KnownDevice, 0)
	for rows.Next() {
		var d KnownDevice
		var tr int
		if err := rows.Scan(&d.RouterID, &d.MAC, &d.LastIP, &d.HostName, &d.FirstSeen, &d.LastSeen, &tr); err == nil {
			d.Trusted = tr == 1
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *Store) SetDeviceTrusted(routerID int64, mac string, trusted bool) error {
	tr := 0
	if trusted {
		tr = 1
	}
	_, err := s.db.Exec(`UPDATE known_devices SET trusted=? WHERE router_id=? AND mac=?`, tr, routerID, mac)
	return err
}

type CustomDPIRule struct {
	Domain   string `json:"domain"`
	App      string `json:"app"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

func (s *Store) ListCustomDPIRules() ([]CustomDPIRule, error) {
	rows, err := s.db.Query(`SELECT domain, app, category, color FROM custom_dpi_rules ORDER BY app ASC, domain ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CustomDPIRule, 0)
	for rows.Next() {
		var r CustomDPIRule
		if rows.Scan(&r.Domain, &r.App, &r.Category, &r.Color) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) SaveCustomDPIRules(items []CustomDPIRule) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO custom_dpi_rules (domain, app, category, color) VALUES (?,?,?,?) ON CONFLICT(domain) DO UPDATE SET app=excluded.app, category=excluded.category, color=excluded.color`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		d := strings.ToLower(strings.TrimSpace(it.Domain))
		if d == "" || it.App == "" {
			continue
		}
		col := it.Color
		if col == "" {
			col = "#0099cc"
		}
		cat := it.Category
		if cat == "" {
			cat = "其他协议"
		}
		if _, err := stmt.Exec(d, it.App, cat, col); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteCustomDPIRule(domain string) error {
	_, err := s.db.Exec(`DELETE FROM custom_dpi_rules WHERE domain=?`, strings.ToLower(strings.TrimSpace(domain)))
	return err
}

func (s *Store) PruneOldData() {
	now := time.Now().Unix()
	s.db.Exec(`DELETE FROM iface_raw WHERE ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM ip_traffic_1m WHERE bucket_ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM app_traffic_1m WHERE bucket_ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM iface_rollup WHERE gran='5m' AND bucket_ts < ?`, now-7*86400)
	s.db.Exec(`DELETE FROM iface_rollup WHERE gran='1h' AND bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM ip_traffic_5m WHERE bucket_ts < ?`, now-14*86400)
	s.db.Exec(`DELETE FROM ip_traffic_1h WHERE bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM app_traffic_5m WHERE bucket_ts < ?`, now-14*86400)
	s.db.Exec(`DELETE FROM app_traffic_1h WHERE bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM domain_audit WHERE ts < ?`, now-14*86400)
	s.db.Exec(`DELETE FROM conn_history WHERE last_ts < ?`, now-14*86400)
	s.db.Exec(`DELETE FROM security_events WHERE ts < ?`, now-30*86400)
}

func (s *Store) GetSetting(k string) string {
	var v string
	_ = s.db.QueryRow(`SELECT v FROM app_settings WHERE k=?`, k).Scan(&v)
	return v
}

func (s *Store) SetSetting(k, v string) error {
	_, err := s.db.Exec(`INSERT INTO app_settings (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, v)
	return err
}
