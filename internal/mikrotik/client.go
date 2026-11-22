package mikrotik

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
)

type ConnConfig struct {
	Host          string
	Port          int
	Username      string
	Password      string
	UseTLS        bool
	TLSSkipVerify bool
}

type SystemResource struct {
	Identity     string  `json:"identity"`
	Version      string  `json:"version"`
	BoardName    string  `json:"board_name"`
	Architecture string  `json:"architecture"`
	Uptime       string  `json:"uptime"`
	CPULoad      int     `json:"cpu_load"`
	CPUCount     int     `json:"cpu_count"`
	TotalMem     int64   `json:"total_mem"`
	FreeMem      int64   `json:"free_mem"`
	MemUsagePct  float64 `json:"mem_usage_pct"`
}

type WANIPInfo struct {
	Interface  string `json:"interface"`
	Address    string `json:"address"`
	Version    int    `json:"version"`
	Location   string `json:"location"`
	LocationEN string `json:"location_en"`
	ISP        string `json:"isp"`
	ASN        string `json:"asn"`
}

type InterfaceInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MacAddr    string `json:"mac_address"`
	MTU        string `json:"mtu"`
	Running    bool   `json:"running"`
	Disabled   bool   `json:"disabled"`
	LinkRate   string `json:"link_rate"`
	FullDuplex bool   `json:"full_duplex"`
	RxBytes    int64  `json:"rx_bytes"`
	TxBytes    int64  `json:"tx_bytes"`
	RxPackets  int64  `json:"rx_packets"`
	TxPackets  int64  `json:"tx_packets"`
	RxDrops    int64  `json:"rx_drops"`
	TxDrops    int64  `json:"tx_drops"`
	LinkDowns  int64  `json:"link_downs"`
	Comment    string `json:"comment"`
}

type DHCPLease struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	MAC      string `json:"mac"`
	HostName string `json:"host_name"`
	Status   string `json:"status"`
	Dynamic  bool   `json:"dynamic"`
	Comment  string `json:"comment"`
}

type ARPEntry struct {
	Address   string `json:"address"`
	MAC       string `json:"mac"`
	Interface string `json:"interface"`
	Status    string `json:"status"`
}

type RouteEntry struct {
	DstAddress string `json:"dst_address"`
	Gateway    string `json:"gateway"`
	Distance   string `json:"distance"`
	Active     bool   `json:"active"`
	RoutingTbl string `json:"routing_table"`
}

type RawConnection struct {
	ID        string
	IPVersion int
	Protocol  string
	SrcIP     string
	SrcPort   int
	DstIP     string
	DstPort   int
	TCPState  string
	Timeout   string
	OrigRate  int64
	ReplRate  int64
	OrigBytes int64
	ReplBytes int64
	FastTrack bool
}

type SimpleQueue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Target   string `json:"target"`
	MaxLimit string `json:"max_limit"`
	Bytes    string `json:"bytes"`
	Rate     string `json:"rate"`
	Disabled bool   `json:"disabled"`
	Comment  string `json:"comment"`
}

type BlockedIP struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	List     string `json:"list"`
	Comment  string `json:"comment"`
	Creation string `json:"creation_time"`
}

type TorchEntry struct {
	SrcAddress string `json:"src_address"`
	DstAddress string `json:"dst_address"`
	Protocol   string `json:"protocol"`
	SrcPort    string `json:"src_port"`
	DstPort    string `json:"dst_port"`
	TxBps      int64  `json:"tx_bps"`
	RxBps      int64  `json:"rx_bps"`
	TxPackets  int64  `json:"tx_packets"`
	RxPackets  int64  `json:"rx_packets"`
}

func Dial(cfg ConnConfig) (*routeros.Client, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	timeout := 8 * time.Second
	if cfg.UseTLS || cfg.Port == 8729 {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
		return routeros.DialTLSTimeout(addr, cfg.Username, cfg.Password, tlsCfg, timeout)
	}
	return routeros.DialTimeout(addr, cfg.Username, cfg.Password, timeout)
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func splitHostPort(addr string) (string, int) {
	if addr == "" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err == nil {
		p, _ := strconv.Atoi(portStr)
		return host, p
	}
	return strings.Trim(addr, "[]"), 0
}

func GetSystemResource(c *routeros.Client) (SystemResource, error) {
	var res SystemResource
	r1, err := c.Run("/system/resource/print")
	if err != nil {
		return res, err
	}
	if len(r1.Re) > 0 {
		m := r1.Re[0].Map
		res.Version = m["version"]
		res.BoardName = m["board-name"]
		res.Architecture = m["architecture-name"]
		res.Uptime = m["uptime"]
		res.CPULoad = int(parseInt64(m["cpu-load"]))
		res.CPUCount = int(parseInt64(m["cpu-count"]))
		res.TotalMem = parseInt64(m["total-memory"])
		res.FreeMem = parseInt64(m["free-memory"])
		if res.TotalMem > 0 {
			res.MemUsagePct = float64(res.TotalMem-res.FreeMem) * 100 / float64(res.TotalMem)
		}
	}
	r2, err := c.Run("/system/identity/print")
	if err == nil && len(r2.Re) > 0 {
		res.Identity = r2.Re[0].Map["name"]
	}
	return res, nil
}

func isPrivateIPv4(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return false
}

func GetWANIPs(c *routeros.Client) []WANIPInfo {
	var out []WANIPInfo
	if r4, err := c.Run("/ip/address/print"); err == nil {
		for _, re := range r4.Re {
			m := re.Map
			if m["disabled"] == "true" || m["invalid"] == "true" {
				continue
			}
			ipStr := strings.Split(m["address"], "/")[0]
			ip := net.ParseIP(ipStr)
			if ip == nil || ip.To4() == nil {
				continue
			}
			ifaceLower := strings.ToLower(m["interface"])
			isWANIf := strings.Contains(ifaceLower, "pppoe") || strings.Contains(ifaceLower, "wan")
			if !isPrivateIPv4(ip) || isWANIf {
				out = append(out, WANIPInfo{
					Interface: m["interface"],
					Address:   ipStr,
					Version:   4,
				})
			}
		}
	}
	if r6, err := c.Run("/ipv6/address/print"); err == nil {
		var fallbackV6 *WANIPInfo
		for _, re := range r6.Re {
			m := re.Map
			if m["disabled"] == "true" || m["invalid"] == "true" {
				continue
			}
			ipStr := strings.Split(m["address"], "/")[0]
			ip := net.ParseIP(ipStr)
			if ip == nil || ip.IsLinkLocalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
				continue
			}
			ifaceLower := strings.ToLower(m["interface"])
			isWANIf := strings.Contains(ifaceLower, "pppoe") || strings.Contains(ifaceLower, "wan") || strings.HasSuffix(m["address"], "/128")
			if isWANIf {
				out = append(out, WANIPInfo{
					Interface: m["interface"],
					Address:   ipStr,
					Version:   6,
				})
			} else if fallbackV6 == nil {
				fallbackV6 = &WANIPInfo{
					Interface: m["interface"],
					Address:   m["address"],
					Version:   6,
				}
			}
		}
		hasV6 := false
		for _, x := range out {
			if x.Version == 6 {
				hasV6 = true
				break
			}
		}
		if !hasV6 && fallbackV6 != nil {
			out = append(out, *fallbackV6)
		}
	}
	return out
}

func GetInterfaces(c *routeros.Client) ([]InterfaceInfo, error) {
	r, err := c.Run("/interface/print")
	if err != nil {
		return nil, err
	}
	out := make([]InterfaceInfo, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, InterfaceInfo{
			Name:      m["name"],
			Type:      m["type"],
			MacAddr:   m["mac-address"],
			MTU:       m["actual-mtu"],
			Running:   m["running"] == "true",
			Disabled:  m["disabled"] == "true",
			RxBytes:   parseInt64(m["rx-byte"]),
			TxBytes:   parseInt64(m["tx-byte"]),
			RxPackets: parseInt64(m["rx-packet"]),
			TxPackets: parseInt64(m["tx-packet"]),
			RxDrops:   parseInt64(m["rx-drop"]),
			TxDrops:   parseInt64(m["tx-drop"]),
			LinkDowns: parseInt64(m["link-downs"]),
			Comment:   m["comment"],
		})
	}
	return out, nil
}

type EthernetStatus struct {
	Rate       string
	FullDuplex bool
}

func GetEthernetRates(c *routeros.Client, names []string) map[string]EthernetStatus {
	out := make(map[string]EthernetStatus)
	if len(names) > 0 {
		if r, err := c.Run("/interface/ethernet/monitor", "=numbers="+strings.Join(names, ","), "=once="); err == nil {
			for _, re := range r.Re {
				m := re.Map
				name := m["name"]
				if name == "" {
					continue
				}
				rate := m["rate"]
				if rate == "" && m["status"] == "no-link" {
					rate = "no-link"
				}
				out[name] = EthernetStatus{
					Rate:       rate,
					FullDuplex: m["full-duplex"] == "true",
				}
			}
		}
	}
	if r, err := c.Run("/interface/ethernet/print"); err == nil {
		for _, re := range r.Re {
			m := re.Map
			name := m["name"]
			if cur, ok := out[name]; !ok || cur.Rate == "" {
				out[name] = EthernetStatus{
					Rate:       m["speed"],
					FullDuplex: m["full-duplex"] == "true",
				}
			}
		}
	}
	return out
}

func RunTraceroute(c *routeros.Client, target string) ([]map[string]string, error) {
	r, err := c.Run("/tool/traceroute", "=address="+target, "=count=1", "=max-hops=15")
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]string, 0, len(r.Re))
	for _, re := range r.Re {
		rows = append(rows, re.Map)
	}
	return rows, nil
}

func GetDHCPLeases(c *routeros.Client) (map[string]DHCPLease, map[string]DHCPLease, []DHCPLease, error) {
	r, err := c.Run("/ip/dhcp-server/lease/print")
	if err != nil {
		return nil, nil, nil, err
	}
	byIP := make(map[string]DHCPLease, len(r.Re))
	byMAC := make(map[string]DHCPLease, len(r.Re))
	list := make([]DHCPLease, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		ip := m["active-address"]
		if ip == "" {
			ip = m["address"]
		}
		mac := strings.ToUpper(m["active-mac-address"])
		if mac == "" {
			mac = strings.ToUpper(m["mac-address"])
		}
		lease := DHCPLease{
			ID:       m[".id"],
			Address:  ip,
			MAC:      mac,
			HostName: m["host-name"],
			Status:   m["status"],
			Dynamic:  m["dynamic"] == "true",
			Comment:  m["comment"],
		}
		list = append(list, lease)
		if ip != "" {
			byIP[ip] = lease
		}
		if mac != "" && lease.HostName != "" {
			byMAC[mac] = lease
		}
	}
	return byIP, byMAC, list, nil
}

func MakeDHCPStatic(c *routeros.Client, id string) error {
	_, err := c.Run("/ip/dhcp-server/lease/make-static", "=numbers="+id)
	return err
}

func GetARPTable(c *routeros.Client) ([]ARPEntry, error) {
	r, err := c.Run("/ip/arp/print")
	if err != nil {
		return nil, err
	}
	out := make([]ARPEntry, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, ARPEntry{
			Address:   m["address"],
			MAC:       strings.ToUpper(m["mac-address"]),
			Interface: m["interface"],
			Status:    m["status"],
		})
	}
	return out, nil
}

func GetRoutes(c *routeros.Client) ([]RouteEntry, error) {
	r, err := c.Run("/ip/route/print")
	if err != nil {
		return nil, err
	}
	out := make([]RouteEntry, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, RouteEntry{
			DstAddress: m["dst-address"],
			Gateway:    m["gateway"],
			Distance:   m["distance"],
			Active:     m["active"] == "true",
			RoutingTbl: m["routing-table"],
		})
	}
	return out, nil
}

func GetIPv6Neighbors(c *routeros.Client) map[string]string {
	r, err := c.Run("/ipv6/neighbor/print")
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		addr := m["address"]
		mac := strings.ToUpper(m["mac-address"])
		if addr != "" && mac != "" {
			out[addr] = mac
		}
	}
	return out
}

func GetDNSCache(c *routeros.Client) (map[string]string, error) {
	r, err := c.Run("/ip/dns/cache/print")
	if err != nil {
		return nil, err
	}
	ipToDomain := make(map[string]string, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		data := m["data"]
		name := m["name"]
		typ := m["type"]
		if name == "" || data == "" {
			continue
		}
		if typ == "A" || typ == "AAAA" || net.ParseIP(data) != nil {
			ipToDomain[data] = name
		}
	}
	return ipToDomain, nil
}

func GetLocalIPv6Prefixes(c *routeros.Client) []*net.IPNet {
	r, err := c.Run("/ipv6/address/print")
	if err != nil {
		return nil
	}
	var nets []*net.IPNet
	for _, re := range r.Re {
		m := re.Map
		if m["disabled"] == "true" {
			continue
		}
		ip, ipNet, err := net.ParseCIDR(m["address"])
		if err != nil || ip.IsLinkLocalUnicast() {
			continue
		}
		ones, bits := ipNet.Mask.Size()
		if bits == 128 && ones <= 64 {
			nets = append(nets, ipNet)
		}
	}
	return nets
}

func parseConnSentences(r *routeros.Reply, prefix string, ipVer int) []RawConnection {
	out := make([]RawConnection, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		srcIP, srcPort := splitHostPort(m["src-address"])
		if srcPort == 0 && m["src-port"] != "" {
			srcPort = int(parseInt64(m["src-port"]))
		}
		dstIP, dstPort := splitHostPort(m["dst-address"])
		if dstPort == 0 && m["dst-port"] != "" {
			dstPort = int(parseInt64(m["dst-port"]))
		}
		origBytes := parseInt64(m["orig-bytes"]) + parseInt64(m["orig-fasttrack-bytes"])
		replBytes := parseInt64(m["repl-bytes"]) + parseInt64(m["repl-fasttrack-bytes"])
		out = append(out, RawConnection{
			ID:        prefix + m[".id"],
			IPVersion: ipVer,
			Protocol:  m["protocol"],
			SrcIP:     srcIP,
			SrcPort:   srcPort,
			DstIP:     dstIP,
			DstPort:   dstPort,
			TCPState:  m["tcp-state"],
			Timeout:   m["timeout"],
			OrigRate:  parseInt64(m["orig-rate"]),
			ReplRate:  parseInt64(m["repl-rate"]),
			OrigBytes: origBytes,
			ReplBytes: replBytes,
			FastTrack: m["fasttrack"] == "true",
		})
	}
	return out
}

func GetConnections(c *routeros.Client) ([]RawConnection, error) {
	r4, err := c.Run("/ip/firewall/connection/print")
	if err != nil {
		return nil, err
	}
	out := parseConnSentences(r4, "v4:", 4)
	if r6, err := c.Run("/ipv6/firewall/connection/print"); err == nil {
		out = append(out, parseConnSentences(r6, "v6:", 6)...)
	}
	return out, nil
}

func ToggleInterface(c *routeros.Client, name string, enable bool) error {
	cmd := "/interface/disable"
	if enable {
		cmd = "/interface/enable"
	}
	_, err := c.Run(cmd, "=numbers="+name)
	return err
}

func GetSimpleQueues(c *routeros.Client) ([]SimpleQueue, error) {
	r, err := c.Run("/queue/simple/print")
	if err != nil {
		return nil, err
	}
	out := make([]SimpleQueue, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, SimpleQueue{
			ID:       m[".id"],
			Name:     m["name"],
			Target:   m["target"],
			MaxLimit: m["max-limit"],
			Bytes:    m["bytes"],
			Rate:     m["rate"],
			Disabled: m["disabled"] == "true",
			Comment:  m["comment"],
		})
	}
	return out, nil
}

func AddSimpleQueue(c *routeros.Client, name, target, maxLimit, comment string) error {
	if !strings.Contains(target, "/") {
		target = target + "/32"
	}
	_, err := c.Run(
		"/queue/simple/add",
		"=name="+name,
		"=target="+target,
		"=max-limit="+maxLimit,
		"=comment="+comment,
	)
	return err
}

func RemoveSimpleQueue(c *routeros.Client, id string) error {
	_, err := c.Run("/queue/simple/remove", "=.id="+id)
	return err
}

func GetBlockedIPs(c *routeros.Client) ([]BlockedIP, error) {
	r, err := c.Run("/ip/firewall/address-list/print", "?list=mikromon_blocked")
	if err != nil {
		return nil, err
	}
	out := make([]BlockedIP, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, BlockedIP{
			ID:       m[".id"],
			Address:  m["address"],
			List:     m["list"],
			Comment:  m["comment"],
			Creation: m["creation-time"],
		})
	}
	return out, nil
}

func BlockIP(c *routeros.Client, ip, comment string) error {
	_, err := c.Run(
		"/ip/firewall/address-list/add",
		"=list=mikromon_blocked",
		"=address="+ip,
		"=comment="+comment,
	)
	return err
}

func UnblockIP(c *routeros.Client, id string) error {
	_, err := c.Run("/ip/firewall/address-list/remove", "=.id="+id)
	return err
}

func RunPing(c *routeros.Client, target string, count int) ([]map[string]string, error) {
	if count <= 0 || count > 10 {
		count = 4
	}
	r, err := c.Run("/ping", "=address="+target, "=count="+strconv.Itoa(count))
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]string, 0, len(r.Re))
	for _, re := range r.Re {
		rows = append(rows, re.Map)
	}
	return rows, nil
}

func TorchSnapshot(ctx context.Context, c *routeros.Client, iface string) ([]TorchEntry, error) {
	tctx, cancel := context.WithTimeout(ctx, 1800*time.Millisecond)
	defer cancel()

	listen, err := c.ListenArgsContext(tctx, []string{
		"/tool/torch",
		"=interface=" + iface,
		"=src-address=0.0.0.0/0",
		"=dst-address=0.0.0.0/0",
		"=port=any",
		"=duration=2",
	})
	if err != nil {
		return nil, fmt.Errorf("torch start: %w", err)
	}

	dedup := make(map[string]TorchEntry)
	for {
		select {
		case <-tctx.Done():
			out := make([]TorchEntry, 0, len(dedup))
			for _, v := range dedup {
				out = append(out, v)
			}
			return out, nil
		case sen, ok := <-listen.Chan():
			if !ok {
				out := make([]TorchEntry, 0, len(dedup))
				for _, v := range dedup {
					out = append(out, v)
				}
				return out, nil
			}
			m := sen.Map
			if m["src-address"] == "" && m["dst-address"] == "" {
				continue
			}
			proto := m["ip-protocol"]
			if proto == "" {
				proto = m["protocol"]
			}
			key := fmt.Sprintf("%s|%s|%s|%s|%s", m["src-address"], m["src-port"], m["dst-address"], m["dst-port"], proto)
			dedup[key] = TorchEntry{
				SrcAddress: m["src-address"],
				DstAddress: m["dst-address"],
				Protocol:   proto,
				SrcPort:    m["src-port"],
				DstPort:    m["dst-port"],
				TxBps:      parseInt64(m["tx"]),
				RxBps:      parseInt64(m["rx"]),
				TxPackets:  parseInt64(m["tx-packets"]),
				RxPackets:  parseInt64(m["rx-packets"]),
			}
		}
	}
}

type LogEntry struct {
	Time    string `json:"time"`
	Topics  string `json:"topics"`
	Message string `json:"message"`
}

type FirewallRule struct {
	ID       string `json:"id"`
	Chain    string `json:"chain"`
	Action   string `json:"action"`
	Comment  string `json:"comment"`
	Packets  int64  `json:"packets"`
	Bytes    int64  `json:"bytes"`
	Disabled bool   `json:"disabled"`
}

func GetLogs(c *routeros.Client) ([]LogEntry, error) {
	r, err := c.Run("/log/print")
	if err != nil {
		return nil, err
	}
	n := len(r.Re)
	start := 0
	if n > 500 {
		start = n - 500
	}
	out := make([]LogEntry, 0, n-start)
	for i := n - 1; i >= start; i-- {
		m := r.Re[i].Map
		out = append(out, LogEntry{
			Time:    m["time"],
			Topics:  m["topics"],
			Message: m["message"],
		})
	}
	return out, nil
}

func GetFirewallFilters(c *routeros.Client) ([]FirewallRule, error) {
	r, err := c.Run("/ip/firewall/filter/print")
	if err != nil {
		return nil, err
	}
	out := make([]FirewallRule, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		out = append(out, FirewallRule{
			ID:       m[".id"],
			Chain:    m["chain"],
			Action:   m["action"],
			Comment:  m["comment"],
			Packets:  parseInt64(m["packets"]),
			Bytes:    parseInt64(m["bytes"]),
			Disabled: m["disabled"] == "true",
		})
	}
	return out, nil
}

func WakeOnLAN(c *routeros.Client, mac, iface string) error {
	args := []string{"/tool/wol", "=mac=" + mac}
	if iface != "" {
		args = append(args, "=interface="+iface)
	}
	_, err := c.RunArgs(args)
	return err
}

type BaselineCheck struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation,omitempty"`
}

func RunBaselineAudit(c *routeros.Client) ([]BaselineCheck, error) {
	var checks []BaselineCheck

	// 1. Check /ip/service/print
	if r, err := c.Run("/ip/service/print"); err == nil {
		var insecure []string
		var unrestricted []string
		for _, re := range r.Re {
			m := re.Map
			if m["disabled"] == "true" {
				continue
			}
			name := m["name"]
			if name == "telnet" || name == "ftp" || name == "www" {
				insecure = append(insecure, name+":"+m["port"])
			}
			if (name == "api" || name == "winbox" || name == "ssh") && strings.TrimSpace(m["address"]) == "" {
				unrestricted = append(unrestricted, name)
			}
		}
		if len(insecure) > 0 {
			checks = append(checks, BaselineCheck{
				ID: "insecure_services", Title: "Cleartext Mgmt Services (Telnet/FTP/HTTP)",
				Status: "warn", Detail: "Enabled: " + strings.Join(insecure, ", "),
				Remediation: "/ip service disable telnet,ftp,www",
			})
		} else {
			checks = append(checks, BaselineCheck{
				ID: "insecure_services", Title: "Cleartext Mgmt Services (Telnet/FTP/HTTP)",
				Status: "pass", Detail: "Telnet / FTP / HTTP disabled",
			})
		}
		if len(unrestricted) > 0 {
			checks = append(checks, BaselineCheck{
				ID: "mgmt_acl", Title: "Management Service Source IP ACL",
				Status: "warn", Detail: "No IP restriction on: " + strings.Join(unrestricted, ", "),
				Remediation: "Restrict Allowed Address to LAN subnet in /ip/service",
			})
		} else {
			checks = append(checks, BaselineCheck{
				ID: "mgmt_acl", Title: "Management Service Source IP ACL",
				Status: "pass", Detail: "Management services restricted by IP ACL",
			})
		}
	}

	// 2. Check /ip/dns/print (DNS Open Resolver risk)
	if r, err := c.Run("/ip/dns/print"); err == nil && len(r.Re) > 0 {
		allowRem := r.Re[0].Map["allow-remote-requests"] == "true"
		if allowRem {
			checks = append(checks, BaselineCheck{
				ID: "dns_remote", Title: "DNS Remote Requests",
				Status: "warn", Detail: "allow-remote-requests=yes (Ensure WAN UDP/53 is dropped in input chain)",
				Remediation: "Ensure firewall input drops UDP/TCP 53 from WAN interface",
			})
		} else {
			checks = append(checks, BaselineCheck{
				ID: "dns_remote", Title: "DNS Remote Requests",
				Status: "pass", Detail: "allow-remote-requests=no",
			})
		}
	}

	// 3. Check /ip/firewall/filter/print (Drop rules & mikromon_blocked enforcement)
	if r, err := c.Run("/ip/firewall/filter/print"); err == nil {
		hasDrop := false
		hasBlocklistRule := false
		for _, re := range r.Re {
			m := re.Map
			if m["disabled"] == "true" {
				continue
			}
			if m["action"] == "drop" || m["action"] == "reject" {
				hasDrop = true
				if m["dst-address-list"] == "mikromon_blocked" || m["src-address-list"] == "mikromon_blocked" {
					hasBlocklistRule = true
				}
			}
		}
		if hasDrop {
			checks = append(checks, BaselineCheck{
				ID: "fw_drop", Title: "Firewall Filter Drop Policy",
				Status: "pass", Detail: "Active drop/reject rules detected",
			})
		} else {
			checks = append(checks, BaselineCheck{
				ID: "fw_drop", Title: "Firewall Filter Drop Policy",
				Status: "fail", Detail: "No active drop/reject rules in /ip/firewall/filter",
				Remediation: "Add default drop rules on WAN input/forward chains",
			})
		}
		if hasBlocklistRule {
			checks = append(checks, BaselineCheck{
				ID: "mikromon_fw", Title: "MikroMon Blocklist Enforcement (mikromon_blocked)",
				Status: "pass", Detail: "Active firewall drop rule referencing mikromon_blocked",
			})
		} else {
			checks = append(checks, BaselineCheck{
				ID: "mikromon_fw", Title: "MikroMon Blocklist Enforcement (mikromon_blocked)",
				Status: "fail", Detail: "Blocked IPs won't drop packets until a filter rule uses mikromon_blocked",
				Remediation: "/ip firewall filter add chain=forward dst-address-list=mikromon_blocked action=drop",
			})
		}
	}

	return checks, nil
}

func EnsureBlocklistFirewallRule(c *routeros.Client) error {
	if r, err := c.Run("/ip/firewall/filter/print"); err == nil {
		for _, re := range r.Re {
			m := re.Map
			if m["disabled"] != "true" && (m["action"] == "drop" || m["action"] == "reject") &&
				(m["dst-address-list"] == "mikromon_blocked" || m["src-address-list"] == "mikromon_blocked") {
				return nil
			}
		}
	}
	_, err := c.Run(
		"/ip/firewall/filter/add",
		"=chain=forward",
		"=dst-address-list=mikromon_blocked",
		"=action=drop",
		"=place-before=0",
		"=comment=MikroMon Auto-Block",
	)
	return err
}

func PingOnceMs(c *routeros.Client, target string) float64 {
	r, err := c.Run("/ping", "=address="+target, "=count=1")
	if err != nil || len(r.Re) == 0 {
		return -1
	}
	m := r.Re[0].Map
	if m["status"] == "timeout" || m["time"] == "" {
		return -1
	}
	d, err := time.ParseDuration(m["time"])
	if err != nil {
		return -1
	}
	ms := float64(d.Microseconds()) / 1000.0
	if ms < 0.1 {
		ms = 0.1
	}
	return float64(int(ms*10+0.5)) / 10.0
}

func FlushDNSCache(c *routeros.Client) error {
	_, err := c.Run("/ip/dns/cache/flush")
	return err
}
