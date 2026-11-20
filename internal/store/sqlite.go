package store

import (
	"database/sql"
	"fmt"
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
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_url TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_user TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE routers ADD COLUMN adg_pass TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE ip_isp_cache ADD COLUMN asn TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE ip_isp_cache ADD COLUMN location_en TEXT NOT NULL DEFAULT '';`)
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
	since := time.Now().Add(-30 * 24 * time.Hour).Unix()
	rows, err := s.db.Query(`SELECT ip, location, location_en, isp, org, asn, updated_at FROM ip_isp_cache WHERE updated_at >= ?`, since)
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

	for _, d := range ips {
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

func (s *Store) PruneOldData() {
	now := time.Now().Unix()
	s.db.Exec(`DELETE FROM iface_raw WHERE ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM ip_traffic_1m WHERE bucket_ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM app_traffic_1m WHERE bucket_ts < ?`, now-86400)
	s.db.Exec(`DELETE FROM iface_rollup WHERE gran='5m' AND bucket_ts < ?`, now-7*86400)
	s.db.Exec(`DELETE FROM iface_rollup WHERE gran='1h' AND bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM ip_traffic_5m WHERE bucket_ts < ?`, now-7*86400)
	s.db.Exec(`DELETE FROM ip_traffic_1h WHERE bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM app_traffic_5m WHERE bucket_ts < ?`, now-7*86400)
	s.db.Exec(`DELETE FROM app_traffic_1h WHERE bucket_ts < ?`, now-35*86400)
	s.db.Exec(`DELETE FROM domain_audit WHERE ts < ?`, now-7*86400)
	s.db.Exec(`DELETE FROM ip_isp_cache WHERE updated_at < ?`, now-30*86400)
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
