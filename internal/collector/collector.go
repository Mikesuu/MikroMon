package collector

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-routeros/routeros/v3"

	"mikromon/internal/adguard"
	"mikromon/internal/dpi"
	"mikromon/internal/geo"
	"mikromon/internal/mikrotik"
	"mikromon/internal/store"
)

type LiveInterface struct {
	mikrotik.InterfaceInfo
	RxBps int64 `json:"rx_bps"`
	TxBps int64 `json:"tx_bps"`
}

type LiveConnection struct {
	ID         string `json:"id"`
	IPVersion  int    `json:"ip_version"`
	Protocol   string `json:"protocol"`
	ClientIP   string `json:"client_ip"`
	HostName   string `json:"host_name"`
	ClientPort int    `json:"client_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Domain     string `json:"domain"`
	Location   string `json:"location"`
	LocationEN string `json:"location_en"`
	ISP        string `json:"isp"`
	App        string `json:"app"`
	Category   string `json:"category"`
	Color      string `json:"color"`

	IsRealApp bool   `json:"is_real_app"`
	RxBps     int64  `json:"rx_bps"`
	TxBps     int64  `json:"tx_bps"`
	RxBytes   int64  `json:"rx_bytes"`
	TxBytes   int64  `json:"tx_bytes"`
	TCPState  string `json:"tcp_state"`
	Timeout   string `json:"timeout"`
	FastTrack bool   `json:"fasttrack"`
}

type LiveIP struct {
	IP          string `json:"ip"`
	IPVersion   int    `json:"ip_version"`
	HostName    string `json:"host_name"`
	MAC         string `json:"mac"`
	Vendor      string `json:"vendor"`
	RxBps       int64  `json:"rx_bps"`
	TxBps       int64  `json:"tx_bps"`
	ActiveConns int    `json:"active_conns"`
	TopApp      string `json:"top_app"`
}

type ActiveAppSummary struct {
	App      string `json:"app"`
	Category string `json:"category"`
	Color    string `json:"color"`

	RxBps    int64 `json:"rx_bps"`
	TxBps    int64 `json:"tx_bps"`
	Conns    int   `json:"conns"`
	LastSeen int64 `json:"last_seen"`
}

type SecurityAlert struct {
	Level      string `json:"level"`
	Rule       string `json:"rule"`
	ClientIP   string `json:"client_ip"`
	HostName   string `json:"host_name"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Protocol   string `json:"protocol"`
	Bytes      int64  `json:"bytes"`
	Detail     string `json:"detail"`
}

type RouterSnapshot struct {
	RouterID         int64                    `json:"router_id"`
	Online           bool                     `json:"online"`
	LastError        string                   `json:"last_error,omitempty"`
	UpdatedAt        int64                    `json:"updated_at"`
	System           mikrotik.SystemResource  `json:"system"`
	WANIPs           []mikrotik.WANIPInfo     `json:"wan_ips"`
	Interfaces       []LiveInterface          `json:"interfaces"`
	LiveIPs          []LiveIP                 `json:"live_ips"`
	ActiveApps       []ActiveAppSummary       `json:"active_apps"`
	TotalConns       int                      `json:"total_conns"`
	OnlineDevices    int                      `json:"online_devices"`
	OnlineIPv4       int                      `json:"online_ipv4"`
	OnlineIPv6       int                      `json:"online_ipv6"`
	Connections      []LiveConnection         `json:"connections"`
	RecentDomains    []store.DomainAuditEntry `json:"recent_domains"`
	SecurityAlerts   []SecurityAlert          `json:"security_alerts"`
	WanLatencyCN     float64                  `json:"wan_latency_cn"`
	WanLatencyGlobal float64                  `json:"wan_latency_global"`
}

type prevIfaceCounter struct {
	rxBytes int64
	txBytes int64
	ts      time.Time
}

type prevConnCounter struct {
	origBytes int64
	replBytes int64
}

type routerWorker struct {
	cfg           store.RouterConfig
	client        *routeros.Client
	prevIfaces    map[string]prevIfaceCounter
	prevConns     map[string]prevConnCounter
	connFirstPoll bool
	leasesByIP    map[string]mikrotik.DHCPLease
	leasesByMAC   map[string]mikrotik.DHCPLease
	v6Neighbors   map[string]string
	v6Prefixes    []*net.IPNet
	etherRates    map[string]mikrotik.EthernetStatus
	wanIPs        []mikrotik.WANIPInfo
	dnsCache      map[string]string
	adgIPDomain   map[string]string
	recentAppHits map[string]ActiveAppSummary
	lastAdgTS     int64
	lastMetaAt    time.Time
	lastConnAt    time.Time
	lastHistWrite map[string]int64
	recentDomains []store.DomainAuditEntry
}

type Collector struct {
	store        *store.Store
	latMu        sync.RWMutex
	wanLatencies map[int64][2]float64
	eventMu      sync.Mutex
	lastEventAt  map[string]int64
	autoBlocked  map[string]bool
	enricher     *geo.Enricher
	mu           sync.RWMutex
	snapshots    map[int64]*RouterSnapshot
	workers      map[int64]*routerWorker
}

func ReloadCustomDPIRules(st *store.Store) {
	list, err := st.ListCustomDPIRules()
	if err != nil {
		return
	}
	var pack []struct{ Domain, App, Category, Color string }
	for _, r := range list {
		pack = append(pack, struct{ Domain, App, Category, Color string }{r.Domain, r.App, r.Category, r.Color})
	}
	dpi.SetCustomRules(pack)
}

func New(st *store.Store) *Collector {
	ReloadCustomDPIRules(st)
	return &Collector{
		store:        st,
		enricher:     geo.New(st),
		snapshots:    make(map[int64]*RouterSnapshot),
		workers:      make(map[int64]*routerWorker),
		lastEventAt:  make(map[string]int64),
		wanLatencies: make(map[int64][2]float64),
		autoBlocked:  make(map[string]bool),
	}
}

func (c *Collector) shouldEmitEvent(key string, nowUnix int64) bool {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	if last, ok := c.lastEventAt[key]; ok && nowUnix-last < 600 {
		return false
	}
	c.lastEventAt[key] = nowUnix
	if len(c.lastEventAt) > 2000 {
		c.lastEventAt = make(map[string]int64)
	}
	return true
}

func (c *Collector) ClearGeoCache() error {
	c.enricher.ClearMemory()
	return c.store.ClearISPCache()
}

func (c *Collector) GeoDBStatus() map[string]any {
	return c.enricher.DBStatus()
}

func (c *Collector) UpdateGeoDB() error {
	return c.enricher.DownloadLocalDB(true)
}

func (c *Collector) GetSnapshot(routerID int64) *RouterSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.snapshots[routerID]; ok {
		return s
	}
	for _, s := range c.snapshots {
		return s
	}
	return &RouterSnapshot{Online: false, LastError: "No router configured or enabled"}
}

func (c *Collector) WithClient(routerID int64, fn func(*routeros.Client) error) error {
	cfg, err := c.store.GetRouter(routerID)
	if err != nil {
		return err
	}
	cl, err := mikrotik.Dial(mikrotik.ConnConfig{
		Host:          cfg.Host,
		Port:          cfg.Port,
		Username:      cfg.Username,
		Password:      cfg.Password,
		UseTLS:        cfg.UseTLS,
		TLSSkipVerify: cfg.TLSSkipVerify,
	})
	if err != nil {
		return err
	}
	defer cl.Close()
	return fn(cl)
}

func (c *Collector) Run(ctx context.Context) {
	stopGeo := make(chan struct{})
	go c.enricher.RunWorker(stopGeo)
	go c.runLatencyProbe(ctx)

	ticker := time.NewTicker(2 * time.Second)
	pruneTicker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	defer pruneTicker.Stop()

	c.pollAll()
	for {
		select {
		case <-ctx.Done():
			close(stopGeo)
			c.closeAll()
			return
		case <-pruneTicker.C:
			c.store.PruneOldData()
		case <-ticker.C:
			c.pollAll()
		}
	}
}

func (c *Collector) runLatencyProbe(ctx context.Context) {
	latClients := make(map[int64]*routeros.Client)
	defer func() {
		for _, cl := range latClients {
			if cl != nil {
				cl.Close()
			}
		}
	}()

	probeOnce := func() {
		routers, err := c.store.ListRouters()
		if err != nil {
			return
		}
		for _, cfg := range routers {
			if !cfg.Enabled {
				continue
			}
			cl := latClients[cfg.ID]
			if cl == nil {
				var derr error
				cl, derr = mikrotik.Dial(mikrotik.ConnConfig{
					Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password,
					UseTLS: cfg.UseTLS, TLSSkipVerify: cfg.TLSSkipVerify,
				})
				if derr != nil {
					continue
				}
				latClients[cfg.ID] = cl
			}
			cn := mikrotik.PingOnceMs(cl, "223.5.5.5")
			gl := mikrotik.PingOnceMs(cl, "8.8.8.8")
			if cn < 0 && gl < 0 {
				cl.Close()
				delete(latClients, cfg.ID)
			}
			c.latMu.Lock()
			c.wanLatencies[cfg.ID] = [2]float64{cn, gl}
			c.latMu.Unlock()
		}
	}

	time.Sleep(1500 * time.Millisecond)
	probeOnce()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			probeOnce()
		}
	}
}

func (c *Collector) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.workers {
		if w.client != nil {
			w.client.Close()
		}
	}
}

func (c *Collector) pollAll() {
	routers, err := c.store.ListRouters()
	if err != nil {
		log.Printf("list routers err: %v", err)
		return
	}

	activeIDs := make(map[int64]bool)
	for _, cfg := range routers {
		if !cfg.Enabled {
			continue
		}
		activeIDs[cfg.ID] = true

		c.mu.Lock()
		w, ok := c.workers[cfg.ID]
		if !ok {
			w = &routerWorker{
				cfg:           cfg,
				prevIfaces:    make(map[string]prevIfaceCounter),
				prevConns:     make(map[string]prevConnCounter),
				connFirstPoll: true,
				leasesByIP:    make(map[string]mikrotik.DHCPLease),
				leasesByMAC:   make(map[string]mikrotik.DHCPLease),
				v6Neighbors:   make(map[string]string),
				dnsCache:      make(map[string]string),
				adgIPDomain:   make(map[string]string),
				recentAppHits: make(map[string]ActiveAppSummary),
				lastHistWrite: make(map[string]int64),
			}
			c.workers[cfg.ID] = w
		} else {
			if w.cfg.Host != cfg.Host || w.cfg.Port != cfg.Port || w.cfg.Username != cfg.Username || w.cfg.Password != cfg.Password || w.cfg.UseTLS != cfg.UseTLS {
				if w.client != nil {
					w.client.Close()
					w.client = nil
				}
				w.connFirstPoll = true
				w.prevConns = make(map[string]prevConnCounter)
			}
			w.cfg = cfg
		}
		c.mu.Unlock()

		c.pollWorker(w)
	}

	c.mu.Lock()
	for id, w := range c.workers {
		if !activeIDs[id] {
			if w.client != nil {
				w.client.Close()
			}
			delete(c.workers, id)
			delete(c.snapshots, id)
		}
	}
	c.mu.Unlock()
}

func (c *Collector) ensureConn(w *routerWorker) error {
	if w.client != nil {
		return nil
	}
	cl, err := mikrotik.Dial(mikrotik.ConnConfig{
		Host:          w.cfg.Host,
		Port:          w.cfg.Port,
		Username:      w.cfg.Username,
		Password:      w.cfg.Password,
		UseTLS:        w.cfg.UseTLS,
		TLSSkipVerify: w.cfg.TLSSkipVerify,
	})
	if err != nil {
		return err
	}
	w.client = cl
	return nil
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return false
}

func isLocalClient(ipStr string, v6Prefixes []*net.IPNet) bool {
	if isPrivateIP(ipStr) {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range v6Prefixes {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var ouiVendors = map[string]string{
	"00:0C:29": "VMware", "00:50:56": "VMware",
	"00:11:32": "Synology", "90:09:D0": "Synology",
	"D8:3A:DD": "Raspberry Pi", "B8:27:EB": "Raspberry Pi",
	"28:6C:07": "Xiaomi", "64:09:80": "Xiaomi", "78:11:DC": "Xiaomi", "F0:B4:29": "Xiaomi",
	"DC:4F:22": "Espressif IoT", "24:0A:C4": "Espressif IoT", "EC:FA:BC": "Espressif IoT",
	"A4:C1:38": "Tuya", "10:D5:61": "Tuya",
	"00:E0:4C": "Realtek", "00:1B:21": "Intel", "B0:6E:BF": "ASUS",
	"F4:F5:D8": "Google", "3C:22:FB": "Apple", "A4:83:E7": "Apple", "F0:18:98": "Apple",
	"E4:5E:37": "Intel", "48:2C:A0": "Xiaomi", "C8:5B:76": "Lenovo",
}

func macVendor(mac string) string {
	if len(mac) < 8 {
		return ""
	}
	prefix := strings.ToUpper(mac[:8])
	if v, ok := ouiVendors[prefix]; ok {
		return v
	}
	return ""
}

func (c *Collector) pollWorker(w *routerWorker) {
	now := time.Now()
	nowUnix := now.Unix()

	if err := c.ensureConn(w); err != nil {
		c.setOffline(w.cfg.ID, err.Error())
		return
	}

	sys, err := mikrotik.GetSystemResource(w.client)
	if err != nil {
		w.client.Close()
		w.client = nil
		c.setOffline(w.cfg.ID, err.Error())
		return
	}

	ifaces, err := mikrotik.GetInterfaces(w.client)
	if err != nil {
		w.client.Close()
		w.client = nil
		c.setOffline(w.cfg.ID, err.Error())
		return
	}

	liveIfaces := make([]LiveInterface, 0, len(ifaces))
	dbSamples := make([]store.IfaceSample, 0, len(ifaces))

	var etherNames []string
	for _, inf := range ifaces {
		if inf.Type == "ether" {
			etherNames = append(etherNames, inf.Name)
		}
	}
	if len(etherNames) > 0 && (w.etherRates == nil || now.Sub(w.lastMetaAt) >= 10*time.Second) {
		w.etherRates = mikrotik.GetEthernetRates(w.client, etherNames)
	}

	for _, inf := range ifaces {
		if es, ok := w.etherRates[inf.Name]; ok {
			inf.LinkRate = es.Rate
			inf.FullDuplex = es.FullDuplex
		}
		var rxBps, txBps int64
		if prev, ok := w.prevIfaces[inf.Name]; ok {
			dt := now.Sub(prev.ts).Seconds()
			if dt > 0.2 {
				dRx := inf.RxBytes - prev.rxBytes
				dTx := inf.TxBytes - prev.txBytes
				if dRx >= 0 {
					rxBps = int64(float64(dRx*8) / dt)
				}
				if dTx >= 0 {
					txBps = int64(float64(dTx*8) / dt)
				}
			}
		}
		w.prevIfaces[inf.Name] = prevIfaceCounter{
			rxBytes: inf.RxBytes,
			txBytes: inf.TxBytes,
			ts:      now,
		}
		liveIfaces = append(liveIfaces, LiveInterface{
			InterfaceInfo: inf,
			RxBps:         rxBps,
			TxBps:         txBps,
		})
		dbSamples = append(dbSamples, store.IfaceSample{
			RouterID: w.cfg.ID,
			TS:       nowUnix,
			Iface:    inf.Name,
			RxBps:    rxBps,
			TxBps:    txBps,
			RxBytes:  inf.RxBytes,
			TxBytes:  inf.TxBytes,
			Running:  inf.Running,
		})
	}
	if err := c.store.InsertIfaceSamples(dbSamples); err != nil {
		log.Printf("insert iface samples err: %v", err)
	}

	if now.Sub(w.lastMetaAt) >= 10*time.Second {
		if byIP, byMAC, _, err := mikrotik.GetDHCPLeases(w.client); err == nil {
			w.leasesByIP = byIP
			w.leasesByMAC = byMAC
		}
		if dns, err := mikrotik.GetDNSCache(w.client); err == nil {
			w.dnsCache = dns
			c.recordDNSApps(w, dns, nowUnix)
		}
		w.v6Neighbors = mikrotik.GetIPv6Neighbors(w.client)
		w.v6Prefixes = mikrotik.GetLocalIPv6Prefixes(w.client)
		wans := mikrotik.GetWANIPs(w.client)
		for i := range wans {
			wanIP := strings.Split(wans[i].Address, "/")[0]
			loc, locEN, isp, _ := c.enricher.Lookup(wanIP)
			wans[i].Location = loc
			wans[i].LocationEN = locEN
			wans[i].ISP = isp
			wans[i].ASN = c.enricher.LookupASN(wanIP)
		}
		w.wanIPs = wans
		c.syncAdGuard(w)
		var devList []store.KnownDevice
		for _, l := range w.leasesByMAC {
			if l.MAC != "" {
				devList = append(devList, store.KnownDevice{
					RouterID: w.cfg.ID, MAC: l.MAC, LastIP: l.Address, HostName: l.HostName,
				})
			}
		}
		if newDevs, err := c.store.UpsertKnownDevices(w.cfg.ID, nowUnix, devList); err == nil && len(newDevs) > 0 {
			var evs []store.SecurityEventRecord
			for _, nd := range newDevs {
				evs = append(evs, store.SecurityEventRecord{
					TS: nowUnix, RouterID: w.cfg.ID, Level: "info", Rule: "new_device",
					ClientIP: nd.LastIP, HostName: nd.HostName,
					Detail: fmt.Sprintf("New MAC joined: %s", nd.MAC),
				})
			}
			_ = c.store.InsertSecurityEvents(evs)
		}
		w.lastMetaAt = now
	}

	c.mu.RLock()
	prevSnap := c.snapshots[w.cfg.ID]
	c.mu.RUnlock()

	var totalConns, onlineDevs, onlineV4, onlineV6 int
	var liveConns []LiveConnection
	var secAlerts []SecurityAlert
	var liveIPs []LiveIP
	var activeApps []ActiveAppSummary
	if prevSnap != nil {
		totalConns = prevSnap.TotalConns
		onlineDevs = prevSnap.OnlineDevices
		onlineV4 = prevSnap.OnlineIPv4
		onlineV6 = prevSnap.OnlineIPv6
		liveConns = prevSnap.Connections
		secAlerts = prevSnap.SecurityAlerts
		liveIPs = prevSnap.LiveIPs
		activeApps = prevSnap.ActiveApps
	}

	if now.Sub(w.lastConnAt) >= 4*time.Second {
		w.lastConnAt = now
		if rawConns, err := mikrotik.GetConnections(w.client); err == nil {
			totalConns = len(rawConns)
			liveConns, liveIPs, activeApps, secAlerts = c.processConnections(w, nowUnix, rawConns)
			onlineDevs, onlineV4, onlineV6 = countUniqueDevices(liveIPs)
		}
	}

	c.latMu.RLock()
	lats := c.wanLatencies[w.cfg.ID]
	c.latMu.RUnlock()
	latCN, latGL := lats[0], lats[1]
	snap := &RouterSnapshot{
		RouterID:         w.cfg.ID,
		Online:           true,
		UpdatedAt:        nowUnix,
		System:           sys,
		WANIPs:           w.wanIPs,
		Interfaces:       liveIfaces,
		LiveIPs:          liveIPs,
		ActiveApps:       activeApps,
		TotalConns:       totalConns,
		OnlineDevices:    onlineDevs,
		OnlineIPv4:       onlineV4,
		OnlineIPv6:       onlineV6,
		Connections:      liveConns,
		RecentDomains:    w.recentDomains,
		SecurityAlerts:   secAlerts,
		WanLatencyCN:     latCN,
		WanLatencyGlobal: latGL,
	}

	c.mu.Lock()
	c.snapshots[w.cfg.ID] = snap
	c.mu.Unlock()
}

func (c *Collector) recordDNSApps(w *routerWorker, dns map[string]string, nowUnix int64) {
	for _, domain := range dns {
		info := dpi.Classify("tcp", 443, domain, "", "")
		if info.IsRealApp {
			prev := w.recentAppHits[info.App]
			prev.App = info.App
			prev.Category = info.Category
			prev.Color = info.Color

			prev.LastSeen = nowUnix
			w.recentAppHits[info.App] = prev
		}
	}
}

func (c *Collector) resolveClientMeta(w *routerWorker, clientIP string) (hostName, mac string) {
	if l, ok := w.leasesByIP[clientIP]; ok {
		return l.HostName, l.MAC
	}
	if m, ok := w.v6Neighbors[clientIP]; ok {
		mac = m
		if l, ok := w.leasesByMAC[m]; ok {
			hostName = l.HostName
		}
	}
	return hostName, mac
}

func (c *Collector) syncAdGuard(w *routerWorker) {
	if w.cfg.AdgURL == "" {
		return
	}
	entries, err := adguard.FetchQueryLog(w.cfg.AdgURL, w.cfg.AdgUser, w.cfg.AdgPass, 400)
	if err != nil {
		return
	}
	nowUnix := time.Now().Unix()
	var maxTS int64 = w.lastAdgTS
	var audits []store.DomainAuditEntry
	recent := make([]store.DomainAuditEntry, 0, 50)
	seen := make(map[string]bool)

	dedupAudit := make(map[string]bool)
	for idx := len(entries) - 1; idx >= 0; idx-- {
		e := entries[idx]
		for _, ip := range e.Answers {
			w.adgIPDomain[ip] = e.Domain
		}
		if len(w.adgIPDomain) > 8000 {
			w.adgIPDomain = make(map[string]string)
		}

		host, _ := c.resolveClientMeta(w, e.ClientIP)
		appInfo := dpi.Classify("tcp", 443, e.Domain, "", "")
		if appInfo.IsRealApp && nowUnix-e.TS <= 900 {
			prev := w.recentAppHits[appInfo.App]
			prev.App = appInfo.App
			prev.Category = appInfo.Category
			prev.Color = appInfo.Color

			if e.TS > prev.LastSeen {
				prev.LastSeen = e.TS
			}
			w.recentAppHits[appInfo.App] = prev
		}

		item := store.DomainAuditEntry{
			TS:       e.TS,
			RouterID: w.cfg.ID,
			ClientIP: e.ClientIP,
			HostName: host,
			Domain:   e.Domain,
			App:      appInfo.App,
			Category: appInfo.Category,
			Source:   "AdGuard",
		}
		if len(recent) < 40 {
			key := e.ClientIP + "|" + e.Domain
			if !seen[key] && !strings.HasSuffix(e.Domain, ".arpa") {
				seen[key] = true
				recent = append(recent, item)
			}
		}
		if e.TS > w.lastAdgTS && !strings.HasSuffix(e.Domain, ".arpa") {
			akey := fmt.Sprintf("%d|%s|%s", e.TS, e.ClientIP, e.Domain)
			if !dedupAudit[akey] {
				dedupAudit[akey] = true
				audits = append(audits, item)
				if tRule, tLvl, tDetail := dpi.ClassifyThreat(e.Domain, 0); tRule != "" && c.shouldEmitEvent("dom|"+e.ClientIP+"|"+e.Domain, nowUnix) {
					_ = c.store.InsertSecurityEvents([]store.SecurityEventRecord{{
						TS: nowUnix, RouterID: w.cfg.ID, Level: tLvl, Rule: tRule,
						ClientIP: e.ClientIP, HostName: host, Domain: e.Domain, Detail: tDetail,
					}})
				}
			}
			if e.TS > maxTS {
				maxTS = e.TS
			}
		}
	}
	if len(recent) > 0 {
		w.recentDomains = recent
	}
	if len(audits) > 0 {
		w.lastAdgTS = maxTS
		_ = c.store.InsertDomainAudits(audits)
	}
}

func (c *Collector) setOffline(routerID int64, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.snapshots[routerID]
	snap := &RouterSnapshot{
		RouterID:  routerID,
		Online:    false,
		LastError: errMsg,
		UpdatedAt: time.Now().Unix(),
	}
	if prev != nil {
		snap.Interfaces = prev.Interfaces
	}
	c.snapshots[routerID] = snap
}

func (c *Collector) processConnections(w *routerWorker, nowUnix int64, conns []mikrotik.RawConnection) ([]LiveConnection, []LiveIP, []ActiveAppSummary, []SecurityAlert) {
	newPrev := make(map[string]prevConnCounter, len(conns))
	ipDeltaMap := make(map[string]*store.IPTrafficDelta)
	appDeltaMap := make(map[string]*store.AppTrafficDelta)
	liveIPMap := make(map[string]*LiveIP)
	ipAppBytes := make(map[string]map[string]int64)
	realAppMap := make(map[string]*ActiveAppSummary)
	var fallbackDomains []store.DomainAuditEntry
	seenDomain := make(map[string]bool)

	wanSet := make(map[string]bool, len(w.wanIPs))
	for _, wi := range w.wanIPs {
		wanSet[strings.Split(wi.Address, "/")[0]] = true
	}
	liveConns := make([]LiveConnection, 0, len(conns))

	for _, rc := range conns {
		newPrev[rc.ID] = prevConnCounter{
			origBytes: rc.OrigBytes,
			replBytes: rc.ReplBytes,
		}

		var dOrig, dRepl int64
		if !w.connFirstPoll {
			if prev, ok := w.prevConns[rc.ID]; ok {
				if rc.OrigBytes >= prev.origBytes {
					dOrig = rc.OrigBytes - prev.origBytes
				}
				if rc.ReplBytes >= prev.replBytes {
					dRepl = rc.ReplBytes - prev.replBytes
				}
			} else {
				dOrig = rc.OrigBytes
				dRepl = rc.ReplBytes
			}
		}

		clientIP, clientPort := rc.SrcIP, rc.SrcPort
		remoteIP, remotePort := rc.DstIP, rc.DstPort
		rxBps, txBps := rc.ReplRate, rc.OrigRate
		rxBytes, txBytes := rc.ReplBytes, rc.OrigBytes
		dRx, dTx := dRepl, dOrig

		if !isLocalClient(rc.SrcIP, w.v6Prefixes) && isLocalClient(rc.DstIP, w.v6Prefixes) {
			clientIP, clientPort = rc.DstIP, rc.DstPort
			remoteIP, remotePort = rc.SrcIP, rc.SrcPort
			rxBps, txBps = rc.OrigRate, rc.ReplRate
			rxBytes, txBytes = rc.OrigBytes, rc.ReplBytes
			dRx, dTx = dOrig, dRepl
		}

		hostName, mac := c.resolveClientMeta(w, clientIP)
		domain := w.adgIPDomain[remoteIP]
		if domain == "" {
			domain = w.dnsCache[remoteIP]
		}
		loc, locEN, isp, asnMeta := c.enricher.Lookup(remoteIP)
		appInfo := dpi.Classify(rc.Protocol, remotePort, domain, remoteIP, asnMeta)

		if domain != "" && len(fallbackDomains) < 40 {
			dk := clientIP + "|" + domain
			if !seenDomain[dk] {
				seenDomain[dk] = true
				fallbackDomains = append(fallbackDomains, store.DomainAuditEntry{
					TS:       nowUnix,
					RouterID: w.cfg.ID,
					ClientIP: clientIP,
					HostName: hostName,
					Domain:   domain,
					App:      appInfo.App,
					Category: appInfo.Category,
					Source:   "RouterOS",
				})
			}
		}

		liveConns = append(liveConns, LiveConnection{
			ID:         rc.ID,
			IPVersion:  rc.IPVersion,
			Protocol:   rc.Protocol,
			ClientIP:   clientIP,
			HostName:   hostName,
			ClientPort: clientPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			Domain:     domain,
			Location:   loc,
			LocationEN: locEN,
			ISP:        isp,
			App:        appInfo.App,
			Category:   appInfo.Category,
			Color:      appInfo.Color,

			IsRealApp: appInfo.IsRealApp,
			RxBps:     rxBps,
			TxBps:     txBps,
			RxBytes:   rxBytes,
			TxBytes:   txBytes,
			TCPState:  rc.TCPState,
			Timeout:   rc.Timeout,
			FastTrack: rc.FastTrack,
		})

		if appInfo.IsRealApp {
			aa, ok := realAppMap[appInfo.App]
			if !ok {
				aa = &ActiveAppSummary{
					App:      appInfo.App,
					Category: appInfo.Category,
					Color:    appInfo.Color,

					LastSeen: nowUnix,
				}
				realAppMap[appInfo.App] = aa
			}
			aa.RxBps += rxBps
			aa.TxBps += txBps
			aa.Conns++
			w.recentAppHits[appInfo.App] = *aa
		}

		if clientIP != "" && clientIP != "0.0.0.0" && !wanSet[clientIP] && isLocalClient(clientIP, w.v6Prefixes) {
			lip, ok := liveIPMap[clientIP]
			if !ok {
				lip = &LiveIP{
					IP:        clientIP,
					IPVersion: rc.IPVersion,
					HostName:  hostName,
					MAC:       mac,
					Vendor:    macVendor(mac),
				}
				liveIPMap[clientIP] = lip
				ipAppBytes[clientIP] = make(map[string]int64)
			}
			lip.RxBps += rxBps
			lip.TxBps += txBps
			lip.ActiveConns++
			weight := rxBps + txBps + 1
			if appInfo.IsRealApp {
				weight += 1000
			}
			ipAppBytes[clientIP][appInfo.App] += weight

			if dRx > 0 || dTx > 0 {
				statIP := clientIP
				statHost := hostName
				if !store.IsPrivateLANIP(statIP) && mac != "" {
					if lease, ok := w.leasesByMAC[mac]; ok && store.IsPrivateLANIP(lease.Address) {
						statIP = lease.Address
						if statHost == "" {
							statHost = lease.HostName
						}
					}
				}
				if store.IsPrivateLANIP(statIP) {
					d, ok := ipDeltaMap[statIP]
					if !ok {
						d = &store.IPTrafficDelta{
							IP:       statIP,
							HostName: statHost,
							MAC:      mac,
						}
						ipDeltaMap[statIP] = d
					}
					d.RxBytes += dRx
					d.TxBytes += dTx
				}
			}
		}

		if dRx > 0 || dTx > 0 {
			appKey := appInfo.App
			if !appInfo.IsRealApp {
				if strings.HasPrefix(appKey, "TCP:") {
					appKey = "Other TCP"
				} else if strings.HasPrefix(appKey, "UDP:") {
					appKey = "Other UDP"
				}
			}
			ad, ok := appDeltaMap[appKey]
			if !ok {
				ad = &store.AppTrafficDelta{App: appKey}
				appDeltaMap[appKey] = ad
			}
			ad.RxBytes += dRx
			ad.TxBytes += dTx
		}
	}

	// 合并近 15 分钟通过 DNS/ADG/DPI 识别到的真实活跃 APP
	for k, hist := range w.recentAppHits {
		if nowUnix-hist.LastSeen > 900 {
			delete(w.recentAppHits, k)
			continue
		}
		if _, ok := realAppMap[k]; !ok {
			hist.RxBps = 0
			hist.TxBps = 0
			hist.Conns = 0
			realAppMap[k] = &hist
		}
	}

	if w.cfg.AdgURL == "" && len(fallbackDomains) > 0 {
		w.recentDomains = fallbackDomains
	}

	w.prevConns = newPrev
	w.connFirstPoll = false

	if len(ipDeltaMap) > 0 || len(appDeltaMap) > 0 {
		ipList := make([]store.IPTrafficDelta, 0, len(ipDeltaMap))
		for _, v := range ipDeltaMap {
			ipList = append(ipList, *v)
		}
		appList := make([]store.AppTrafficDelta, 0, len(appDeltaMap))
		for _, v := range appDeltaMap {
			appList = append(appList, *v)
		}
		if err := c.store.AddTrafficDeltas(w.cfg.ID, nowUnix, ipList, appList); err != nil {
			log.Printf("add traffic deltas err: %v", err)
		}
	}

	liveIPs := make([]LiveIP, 0, len(liveIPMap))
	for ip, v := range liveIPMap {
		var bestApp string
		var bestVal int64
		for a, val := range ipAppBytes[ip] {
			if val > bestVal {
				bestVal = val
				bestApp = a
			}
		}
		v.TopApp = bestApp
		liveIPs = append(liveIPs, *v)
	}
	sort.Slice(liveIPs, func(i, j int) bool {
		return (liveIPs[i].RxBps + liveIPs[i].TxBps) > (liveIPs[j].RxBps + liveIPs[j].TxBps)
	})

	activeApps := make([]ActiveAppSummary, 0, len(realAppMap))
	for _, v := range realAppMap {
		activeApps = append(activeApps, *v)
	}
	sort.Slice(activeApps, func(i, j int) bool {
		si := activeApps[i].RxBps + activeApps[i].TxBps
		sj := activeApps[j].RxBps + activeApps[j].TxBps
		if si != sj {
			return si > sj
		}
		if activeApps[i].Conns != activeApps[j].Conns {
			return activeApps[i].Conns > activeApps[j].Conns
		}
		return activeApps[i].LastSeen > activeApps[j].LastSeen
	})

	sort.Slice(liveConns, func(i, j int) bool {
		si := liveConns[i].RxBps + liveConns[i].TxBps
		sj := liveConns[j].RxBps + liveConns[j].TxBps
		if si != sj {
			return si > sj
		}
		return (liveConns[i].RxBytes + liveConns[i].TxBytes) > (liveConns[j].RxBytes + liveConns[j].TxBytes)
	})
	// 写入 14 天去重连接访问历史 (同一五元组+域名每 5 分钟最多刷新一次)
	if w.lastHistWrite == nil {
		w.lastHistWrite = make(map[string]int64)
	}
	var histBatch []store.ConnHistoryEntry
	for _, lc := range liveConns {
		if lc.ClientIP == "" || lc.RemoteIP == "" {
			continue
		}
		hKey := fmt.Sprintf("%s|%s|%d|%s", lc.ClientIP, lc.RemoteIP, lc.RemotePort, lc.Domain)
		if lastW, ok := w.lastHistWrite[hKey]; !ok || nowUnix-lastW >= 300 {
			w.lastHistWrite[hKey] = nowUnix
			histBatch = append(histBatch, store.ConnHistoryEntry{
				RouterID:   w.cfg.ID,
				ClientIP:   lc.ClientIP,
				HostName:   lc.HostName,
				RemoteIP:   lc.RemoteIP,
				RemotePort: lc.RemotePort,
				Protocol:   lc.Protocol,
				Domain:     lc.Domain,
				Location:   lc.Location,
				LocationEN: lc.LocationEN,
				ISP:        lc.ISP,
				App:        lc.App,
				Category:   lc.Category,
				BytesTotal: lc.RxBytes + lc.TxBytes,
				TS:         nowUnix,
			})
		}
	}
	if len(w.lastHistWrite) > 10000 {
		w.lastHistWrite = make(map[string]int64)
	}
	if len(histBatch) > 0 {
		_ = c.store.UpsertConnHistory(w.cfg.ID, histBatch)
	}

	if len(liveConns) > 1000 {
		liveConns = liveConns[:1000]
	}

	// === 安全态势与异常连接检测 (支持威胁域名、高危端口、Geo 关注列表、持久化事件与自动封禁) ===
	var secAlerts []SecurityAlert
	var secEventsToSave []store.SecurityEventRecord
	autoBlock := c.store.GetSetting("sec_auto_block") == "true"
	geoWatchRaw := strings.TrimSpace(c.store.GetSetting("sec_geo_watch"))
	var geoWatches []string
	if geoWatchRaw != "" {
		for _, g := range strings.Split(geoWatchRaw, ",") {
			g = strings.ToLower(strings.TrimSpace(g))
			if g != "" {
				geoWatches = append(geoWatches, g)
			}
		}
	}

	seenAlert := make(map[string]bool)
	clientPorts := make(map[string]map[int]bool)
	clientConns := make(map[string]int)
	clientHost := make(map[string]string)

	for _, lc := range liveConns {
		clientConns[lc.ClientIP]++
		if lc.HostName != "" {
			clientHost[lc.ClientIP] = lc.HostName
		}
		if clientPorts[lc.ClientIP] == nil {
			clientPorts[lc.ClientIP] = make(map[int]bool)
		}
		if lc.RemotePort > 0 {
			clientPorts[lc.ClientIP][lc.RemotePort] = true
		}
		if !isPrivateIP(lc.RemoteIP) {
			tRule, tLvl, tDetail := dpi.ClassifyThreat(lc.Domain, lc.RemotePort)
			if tRule == "" && len(geoWatches) > 0 {
				geoAll := strings.ToLower(lc.Location + " " + lc.LocationEN)
				for _, gw := range geoWatches {
					if strings.Contains(geoAll, gw) {
						tRule, tLvl, tDetail = "geo_watch", "medium", "Geo Watchlist Match: "+firstNonEmpty(lc.LocationEN, lc.Location)
						break
					}
				}
			}
			if tRule != "" {
				k := fmt.Sprintf("%s|%s|%s|%d", tRule, lc.ClientIP, lc.RemoteIP, lc.RemotePort)
				if !seenAlert[k] && len(secAlerts) < 100 {
					seenAlert[k] = true
					secAlerts = append(secAlerts, SecurityAlert{
						Level: tLvl, Rule: tRule,
						ClientIP: lc.ClientIP, HostName: lc.HostName,
						RemoteIP: lc.RemoteIP, RemotePort: lc.RemotePort,
						Protocol: lc.Protocol, Bytes: lc.RxBytes + lc.TxBytes,
						Detail: tDetail,
					})
					act := ""
					if autoBlock && tLvl == "high" && lc.RemoteIP != "" {
						c.eventMu.Lock()
						alreadyBlk := c.autoBlocked[lc.RemoteIP]
						c.autoBlocked[lc.RemoteIP] = true
						c.eventMu.Unlock()
						if !alreadyBlk && w.client != nil {
							if err := mikrotik.BlockIP(w.client, lc.RemoteIP, "AutoBlock:"+tRule); err == nil {
								act = "auto_blocked"
							}
						}
					}
					if c.shouldEmitEvent(k, nowUnix) {
						secEventsToSave = append(secEventsToSave, store.SecurityEventRecord{
							TS: nowUnix, RouterID: w.cfg.ID, Level: tLvl, Rule: tRule,
							ClientIP: lc.ClientIP, HostName: lc.HostName,
							RemoteIP: lc.RemoteIP, RemotePort: lc.RemotePort, Domain: lc.Domain,
							Detail: tDetail, ActionTaken: act,
						})
					}
				}
			} else if lc.Domain == "" && (lc.RxBytes+lc.TxBytes) >= 20*1024*1024 {
				k := fmt.Sprintf("nodns|%s|%s", lc.ClientIP, lc.RemoteIP)
				if !seenAlert[k] && len(secAlerts) < 100 {
					seenAlert[k] = true
					secAlerts = append(secAlerts, SecurityAlert{
						Level: "info", Rule: "direct_ip_heavy",
						ClientIP: lc.ClientIP, HostName: lc.HostName,
						RemoteIP: lc.RemoteIP, RemotePort: lc.RemotePort,
						Protocol: lc.Protocol, Bytes: lc.RxBytes + lc.TxBytes,
						Detail: "Direct IP (>20MB, No DNS)",
					})
				}
			}
		}
	}
	for ip, cnt := range clientConns {
		pCnt := len(clientPorts[ip])
		if cnt >= 80 || pCnt >= 25 {
			det := fmt.Sprintf("%d sessions / %d ports", cnt, pCnt)
			secAlerts = append(secAlerts, SecurityAlert{
				Level: "medium", Rule: "conn_flood",
				ClientIP: ip, HostName: clientHost[ip],
				Detail: det,
			})
			if c.shouldEmitEvent("flood|"+ip, nowUnix) {
				secEventsToSave = append(secEventsToSave, store.SecurityEventRecord{
					TS: nowUnix, RouterID: w.cfg.ID, Level: "medium", Rule: "conn_flood",
					ClientIP: ip, HostName: clientHost[ip], Detail: det,
				})
			}
		}
	}
	if len(secEventsToSave) > 0 {
		_ = c.store.InsertSecurityEvents(secEventsToSave)
	}

	return liveConns, liveIPs, activeApps, secAlerts
}

func countUniqueDevices(ips []LiveIP) (devs, v4, v6 int) {
	devSet := make(map[string]bool)
	for _, ip := range ips {
		if ip.IPVersion == 6 {
			v6++
		} else {
			v4++
		}
		key := ip.MAC
		if key == "" {
			key = ip.IP
		}
		devSet[key] = true
	}
	return len(devSet), v4, v6
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
