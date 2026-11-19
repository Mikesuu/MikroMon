package geo

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mikromon/internal/store"
)

type Enricher struct {
	store   *store.Store
	mu      sync.RWMutex
	cache   map[string]store.ISPRecord
	pending map[string]bool
	client  *http.Client
}

type staticCIDR struct {
	net *net.IPNet
	rec store.ISPRecord
}

var staticRanges []staticCIDR

func init() {
	defs := []struct {
		cidr, loc, isp, org, asn string
	}{
		{"1.1.1.0/24", "Anycast", "Cloudflare", "Cloudflare DNS", "AS13335"},
		{"1.0.0.0/24", "Anycast", "Cloudflare", "Cloudflare DNS", "AS13335"},
		{"104.16.0.0/12", "Anycast", "Cloudflare", "Cloudflare CDN", "AS13335"},
		{"172.64.0.0/13", "Anycast", "Cloudflare", "Cloudflare CDN", "AS13335"},
		{"8.8.8.0/24", "Anycast", "Google", "Google DNS", "AS15169"},
		{"8.8.4.0/24", "Anycast", "Google", "Google DNS", "AS15169"},
		{"17.0.0.0/8", "美国", "Apple Inc.", "Apple", "AS714"},
		{"149.154.160.0/20", "海外", "Telegram", "Telegram", "AS62041"},
		{"91.108.4.0/22", "海外", "Telegram", "Telegram", "AS62041"},
		{"91.108.8.0/22", "海外", "Telegram", "Telegram", "AS62041"},
		{"91.108.12.0/22", "海外", "Telegram", "Telegram", "AS62041"},
		{"91.108.16.0/22", "海外", "Telegram", "Telegram", "AS62041"},
		{"91.108.56.0/22", "海外", "Telegram", "Telegram", "AS62041"},
		{"2001:b28:f23d::/48", "海外", "Telegram", "Telegram", "AS62041"},
		{"2001:67c:4e8::/48", "海外", "Telegram", "Telegram", "AS62041"},
		{"223.5.5.0/24", "中国", "阿里云 DNS", "Alibaba", "AS37963"},
		{"223.6.6.0/24", "中国", "阿里云 DNS", "Alibaba", "AS37963"},
		{"119.29.29.0/24", "中国", "腾讯云 DNSPod", "Tencent", "AS45090"},
		{"114.114.114.0/24", "中国", "114DNS", "114DNS", ""},
	}
	for _, d := range defs {
		if _, n, err := net.ParseCIDR(d.cidr); err == nil {
			staticRanges = append(staticRanges, staticCIDR{
				net: n,
				rec: store.ISPRecord{Location: d.loc, ISP: d.isp, Org: d.org, ASN: d.asn},
			})
		}
	}
}

func New(st *store.Store) *Enricher {
	cached, _ := st.LoadISPCache()
	if cached == nil {
		cached = make(map[string]store.ISPRecord)
	}
	return &Enricher{
		store:   st,
		cache:   cached,
		pending: make(map[string]bool),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (e *Enricher) ClearMemory() {
	e.mu.Lock()
	e.cache = make(map[string]store.ISPRecord)
	e.mu.Unlock()
}

func IsPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return false
	}
	return true
}

func (e *Enricher) Lookup(ipStr string) (location, isp, asnMeta string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", "", ""
	}
	if !IsPublicIP(ipStr) {
		return "内网", "局域网", ""
	}
	e.mu.RLock()
	rec, ok := e.cache[ipStr]
	e.mu.RUnlock()
	if ok {
		return rec.Location, firstNonEmpty(rec.ISP, rec.Org), strings.TrimSpace(rec.ISP + " " + rec.Org + " " + rec.ASN)
	}
	for _, sr := range staticRanges {
		if sr.net.Contains(ip) {
			return sr.rec.Location, sr.rec.ISP, sr.rec.ISP + " " + sr.rec.Org + " " + sr.rec.ASN
		}
	}
	e.mu.Lock()
	if len(e.pending) < 600 {
		e.pending[ipStr] = true
	}
	e.mu.Unlock()
	return "", "", ""
}

func (e *Enricher) RunWorker(stop <-chan struct{}) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			e.flushBatch()
		}
	}
}

func (e *Enricher) flushBatch() {
	e.mu.Lock()
	if len(e.pending) == 0 {
		e.mu.Unlock()
		return
	}
	batch := make([]string, 0, 90)
	for ip := range e.pending {
		batch = append(batch, ip)
		delete(e.pending, ip)
		if len(batch) >= 90 {
			break
		}
	}
	e.mu.Unlock()

	body, _ := json.Marshal(batch)
	req, err := http.NewRequest("POST", "http://ip-api.com/batch?fields=status,country,regionName,city,isp,org,as,asname,query&lang=zh-CN", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	var results []struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
		Org        string `json:"org"`
		AS         string `json:"as"`
		ASName     string `json:"asname"`
		Query      string `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return
	}

	now := time.Now().Unix()
	toSave := make([]store.ISPRecord, 0, len(results))
	e.mu.Lock()
	for _, r := range results {
		if r.Status != "success" || r.Query == "" {
			continue
		}
		locParts := make([]string, 0, 3)
		if r.Country != "" {
			locParts = append(locParts, r.Country)
		}
		if r.RegionName != "" && r.RegionName != r.Country {
			locParts = append(locParts, r.RegionName)
		}
		if r.City != "" && r.City != r.RegionName {
			locParts = append(locParts, r.City)
		}
		ispName := firstNonEmpty(r.ISP, r.Org, r.ASName)
		rec := store.ISPRecord{
			IP:        r.Query,
			Location:  strings.Join(locParts, " "),
			ISP:       ispName,
			Org:       firstNonEmpty(r.Org, r.ASName),
			ASN:       r.AS,
			UpdatedAt: now,
		}
		e.cache[r.Query] = rec
		toSave = append(toSave, rec)
	}
	e.mu.Unlock()
	_ = e.store.SaveISPCache(toSave)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
