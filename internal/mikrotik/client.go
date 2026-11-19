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

type InterfaceInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	MacAddr   string `json:"mac_address"`
	MTU       string `json:"mtu"`
	Running   bool   `json:"running"`
	Disabled  bool   `json:"disabled"`
	RxBytes   int64  `json:"rx_bytes"`
	TxBytes   int64  `json:"tx_bytes"`
	RxPackets int64  `json:"rx_packets"`
	TxPackets int64  `json:"tx_packets"`
	RxDrops   int64  `json:"rx_drops"`
	TxDrops   int64  `json:"tx_drops"`
	LinkDowns int64  `json:"link_downs"`
	Comment   string `json:"comment"`
}

type DHCPLease struct {
	Address  string `json:"address"`
	MAC      string `json:"mac"`
	HostName string `json:"host_name"`
	Status   string `json:"status"`
	Comment  string `json:"comment"`
}

type RawConnection struct {
	ID        string
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

func GetDHCPLeases(c *routeros.Client) (map[string]DHCPLease, error) {
	r, err := c.Run("/ip/dhcp-server/lease/print")
	if err != nil {
		return nil, err
	}
	res := make(map[string]DHCPLease, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		ip := m["active-address"]
		if ip == "" {
			ip = m["address"]
		}
		if ip == "" {
			continue
		}
		mac := m["active-mac-address"]
		if mac == "" {
			mac = m["mac-address"]
		}
		res[ip] = DHCPLease{
			Address:  ip,
			MAC:      mac,
			HostName: m["host-name"],
			Status:   m["status"],
			Comment:  m["comment"],
		}
	}
	return res, nil
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

func GetConnections(c *routeros.Client) ([]RawConnection, error) {
	r, err := c.Run("/ip/firewall/connection/print")
	if err != nil {
		return nil, err
	}
	out := make([]RawConnection, 0, len(r.Re))
	for _, re := range r.Re {
		m := re.Map
		srcIP, srcPort := splitHostPort(m["src-address"])
		dstIP, dstPort := splitHostPort(m["dst-address"])
		origBytes := parseInt64(m["orig-bytes"]) + parseInt64(m["orig-fasttrack-bytes"])
		replBytes := parseInt64(m["repl-bytes"]) + parseInt64(m["repl-fasttrack-bytes"])
		out = append(out, RawConnection{
			ID:        m[".id"],
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
