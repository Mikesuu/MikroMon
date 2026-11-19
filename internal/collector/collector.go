package collector

import (
	"context"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/go-routeros/routeros/v3"

	"mikromon/internal/dpi"
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
	Protocol   string `json:"protocol"`
	ClientIP   string `json:"client_ip"`
	HostName   string `json:"host_name"`
	ClientPort int    `json:"client_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort int    `json:"remote_port"`
	Domain     string `json:"domain"`
	App        string `json:"app"`
	RxBps      int64  `json:"rx_bps"`
	TxBps      int64  `json:"tx_bps"`
	RxBytes    int64  `json:"rx_bytes"`
	TxBytes    int64  `json:"tx_bytes"`
	TCPState   string `json:"tcp_state"`
	Timeout    string `json:"timeout"`
	FastTrack  bool   `json:"fasttrack"`
}

type LiveIP struct {
	IP          string `json:"ip"`
	HostName    string `json:"host_name"`
	MAC         string `json:"mac"`
	RxBps       int64  `json:"rx_bps"`
	TxBps       int64  `json:"tx_bps"`
	ActiveConns int    `json:"active_conns"`
	TopApp      string `json:"top_app"`
}

type RouterSnapshot struct {
	RouterID    int64                   `json:"router_id"`
	Online      bool                    `json:"online"`
	LastError   string                  `json:"last_error,omitempty"`
	UpdatedAt   int64                   `json:"updated_at"`
	System      mikrotik.SystemResource `json:"system"`
	Interfaces  []LiveInterface         `json:"interfaces"`
	LiveIPs     []LiveIP                `json:"live_ips"`
	Connections []LiveConnection        `json:"connections"`
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
	leases        map[string]mikrotik.DHCPLease
	dnsCache      map[string]string
	lastMetaAt    time.Time
	lastConnAt    time.Time
}

type Collector struct {
	store     *store.Store
	mu        sync.RWMutex
	snapshots map[int64]*RouterSnapshot
	workers   map[int64]*routerWorker
}

func New(st *store.Store) *Collector {
	return &Collector{
		store:     st,
		snapshots: make(map[int64]*RouterSnapshot),
		workers:   make(map[int64]*routerWorker),
	}
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
	return &RouterSnapshot{Online: false, LastError: "尚未配置或启用路由器"}
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
	ticker := time.NewTicker(2 * time.Second)
	pruneTicker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	defer pruneTicker.Stop()

	c.pollAll()
	for {
		select {
		case <-ctx.Done():
			c.closeAll()
			return
		case <-pruneTicker.C:
			c.store.PruneOldData()
		case <-ticker.C:
			c.pollAll()
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
				leases:        make(map[string]mikrotik.DHCPLease),
				dnsCache:      make(map[string]string),
			}
			c.workers[cfg.ID] = w
		} else if w.cfg.Host != cfg.Host || w.cfg.Port != cfg.Port || w.cfg.Username != cfg.Username || (cfg.Password != "" && w.cfg.Password != cfg.Password) || w.cfg.UseTLS != cfg.UseTLS {
			if w.client != nil {
				w.client.Close()
				w.client = nil
			}
			w.cfg = cfg
			w.connFirstPoll = true
			w.prevConns = make(map[string]prevConnCounter)
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

	for _, inf := range ifaces {
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

	if now.Sub(w.lastMetaAt) >= 15*time.Second {
		if leases, err := mikrotik.GetDHCPLeases(w.client); err == nil {
			w.leases = leases
		}
		if dns, err := mikrotik.GetDNSCache(w.client); err == nil {
			w.dnsCache = dns
		}
		w.lastMetaAt = now
	}

	c.mu.RLock()
	prevSnap := c.snapshots[w.cfg.ID]
	c.mu.RUnlock()

	var liveConns []LiveConnection
	var liveIPs []LiveIP
	if prevSnap != nil {
		liveConns = prevSnap.Connections
		liveIPs = prevSnap.LiveIPs
	}

	if now.Sub(w.lastConnAt) >= 4*time.Second {
		w.lastConnAt = now
		if rawConns, err := mikrotik.GetConnections(w.client); err == nil {
			liveConns, liveIPs = c.processConnections(w, nowUnix, rawConns)
		}
	}

	snap := &RouterSnapshot{
		RouterID:    w.cfg.ID,
		Online:      true,
		UpdatedAt:   nowUnix,
		System:      sys,
		Interfaces:  liveIfaces,
		LiveIPs:     liveIPs,
		Connections: liveConns,
	}

	c.mu.Lock()
	c.snapshots[w.cfg.ID] = snap
	c.mu.Unlock()
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

func (c *Collector) processConnections(w *routerWorker, nowUnix int64, conns []mikrotik.RawConnection) ([]LiveConnection, []LiveIP) {
	newPrev := make(map[string]prevConnCounter, len(conns))
	ipDeltaMap := make(map[string]*store.IPTrafficDelta)
	appDeltaMap := make(map[string]*store.AppTrafficDelta)
	liveIPMap := make(map[string]*LiveIP)
	ipAppBytes := make(map[string]map[string]int64)

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

		if !isPrivateIP(rc.SrcIP) && isPrivateIP(rc.DstIP) {
			clientIP, clientPort = rc.DstIP, rc.DstPort
			remoteIP, remotePort = rc.SrcIP, rc.SrcPort
			rxBps, txBps = rc.OrigRate, rc.ReplRate
			rxBytes, txBytes = rc.OrigBytes, rc.ReplBytes
			dRx, dTx = dOrig, dRepl
		}

		lease := w.leases[clientIP]
		domain := w.dnsCache[remoteIP]
		app := dpi.Classify(rc.Protocol, remotePort, domain)

		liveConns = append(liveConns, LiveConnection{
			ID:         rc.ID,
			Protocol:   rc.Protocol,
			ClientIP:   clientIP,
			HostName:   lease.HostName,
			ClientPort: clientPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			Domain:     domain,
			App:        app,
			RxBps:      rxBps,
			TxBps:      txBps,
			RxBytes:    rxBytes,
			TxBytes:    txBytes,
			TCPState:   rc.TCPState,
			Timeout:    rc.Timeout,
			FastTrack:  rc.FastTrack,
		})

		if clientIP != "" {
			lip, ok := liveIPMap[clientIP]
			if !ok {
				lip = &LiveIP{
					IP:       clientIP,
					HostName: lease.HostName,
					MAC:      lease.MAC,
				}
				liveIPMap[clientIP] = lip
				ipAppBytes[clientIP] = make(map[string]int64)
			}
			lip.RxBps += rxBps
			lip.TxBps += txBps
			lip.ActiveConns++
			ipAppBytes[clientIP][app] += rxBps + txBps + 1

			if dRx > 0 || dTx > 0 {
				d, ok := ipDeltaMap[clientIP]
				if !ok {
					d = &store.IPTrafficDelta{
						IP:       clientIP,
						HostName: lease.HostName,
						MAC:      lease.MAC,
					}
					ipDeltaMap[clientIP] = d
				}
				d.RxBytes += dRx
				d.TxBytes += dTx
			}
		}

		if dRx > 0 || dTx > 0 {
			ad, ok := appDeltaMap[app]
			if !ok {
				ad = &store.AppTrafficDelta{App: app}
				appDeltaMap[app] = ad
			}
			ad.RxBytes += dRx
			ad.TxBytes += dTx
		}
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

	sort.Slice(liveConns, func(i, j int) bool {
		si := liveConns[i].RxBps + liveConns[i].TxBps
		sj := liveConns[j].RxBps + liveConns[j].TxBps
		if si != sj {
			return si > sj
		}
		return (liveConns[i].RxBytes + liveConns[i].TxBytes) > (liveConns[j].RxBytes + liveConns[j].TxBytes)
	})
	if len(liveConns) > 400 {
		liveConns = liveConns[:400]
	}

	return liveConns, liveIPs
}
