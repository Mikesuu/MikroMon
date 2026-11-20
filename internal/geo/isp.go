package geo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
		cidr, loc, locEN, isp, org, asn string
	}{
		{"1.1.1.0/24", "Anycast", "Anycast", "Cloudflare", "Cloudflare DNS", "AS13335"},
		{"1.0.0.0/24", "Anycast", "Anycast", "Cloudflare", "Cloudflare DNS", "AS13335"},
		{"104.16.0.0/12", "Anycast", "Anycast", "Cloudflare", "Cloudflare CDN", "AS13335"},
		{"172.64.0.0/13", "Anycast", "Anycast", "Cloudflare", "Cloudflare CDN", "AS13335"},
		{"8.8.8.0/24", "美国", "United States", "Google", "Google DNS", "AS15169"},
		{"8.8.4.0/24", "美国", "United States", "Google", "Google DNS", "AS15169"},
		{"142.250.0.0/15", "美国", "United States", "Google", "Google LLC", "AS15169"},
		{"172.217.0.0/16", "美国", "United States", "Google", "Google LLC", "AS15169"},
		{"216.58.192.0/19", "美国", "United States", "Google", "Google LLC", "AS15169"},
		{"17.0.0.0/8", "美国", "United States", "Apple Inc.", "Apple", "AS714"},
		{"149.154.160.0/20", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"91.108.4.0/22", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"91.108.8.0/22", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"91.108.12.0/22", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"91.108.16.0/22", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"91.108.56.0/22", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"2001:b28:f23d::/48", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"2001:67c:4e8::/48", "海外", "Global", "Telegram", "Telegram", "AS62041"},
		{"223.5.5.0/24", "中国", "China", "阿里云 DNS", "Alibaba", "AS37963"},
		{"223.6.6.0/24", "中国", "China", "阿里云 DNS", "Alibaba", "AS37963"},
		{"120.24.0.0/14", "中国", "China", "阿里云", "Alibaba Cloud", "AS37963"},
		{"47.92.0.0/14", "中国", "China", "阿里云", "Alibaba Cloud", "AS37963"},
		{"39.96.0.0/13", "中国", "China", "阿里云", "Alibaba Cloud", "AS37963"},
		{"119.29.29.0/24", "中国", "China", "腾讯云 DNSPod", "Tencent", "AS45090"},
		{"1.12.12.0/24", "中国", "China", "腾讯云 DNSPod", "Tencent", "AS45090"},
		{"43.128.0.0/11", "中国", "China", "腾讯云", "Tencent Cloud", "AS45090"},
		{"62.234.0.0/16", "中国", "China", "腾讯云", "Tencent Cloud", "AS45090"},
		{"119.28.0.0/15", "中国", "China", "腾讯云", "Tencent Cloud", "AS45090"},
		{"114.114.114.0/24", "中国", "China", "114DNS", "114DNS", ""},
		{"110.242.0.0/16", "中国", "China", "百度云", "Baidu", "AS55967"},
		{"220.181.0.0/16", "中国", "China", "百度", "Baidu", "AS55967"},
		{"2408::/12", "中国", "China", "中国联通 IPv6", "China Unicom", "AS4837"},
		{"240e::/12", "中国", "China", "中国电信 IPv6", "China Telecom", "AS4134"},
		{"2409::/12", "中国", "China", "中国移动 IPv6", "China Mobile", "AS9808"},
		{"2001:da8::/32", "中国", "China", "教育网 IPv6", "CERNET", "AS4538"},
	}
	for _, d := range defs {
		if _, n, err := net.ParseCIDR(d.cidr); err == nil {
			staticRanges = append(staticRanges, staticCIDR{
				net: n,
				rec: store.ISPRecord{Location: d.loc, LocationEN: d.locEN, ISP: d.isp, Org: d.org, ASN: d.asn},
			})
		}
	}
}

func New(st *store.Store) *Enricher {
	cached, _ := st.LoadISPCache()
	if cached == nil {
		cached = make(map[string]store.ISPRecord)
	}
	pending := make(map[string]bool)
	for ip, rec := range cached {
		if rec.LocationEN == "" && IsPublicIP(ip) && len(pending) < 600 {
			pending[ip] = true
		}
	}
	return &Enricher{
		store:   st,
		cache:   cached,
		pending: pending,
		client:  &http.Client{Timeout: 10 * time.Second},
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
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 || ip4[0] == 255 || (ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127) {
			return false
		}
	}
	return true
}

func (e *Enricher) Lookup(ipStr string) (location, locationEN, isp, asnMeta string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", "", "", ""
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return "运营商内网", "Carrier NAT", "CGNAT (100.64)", ""
	}
	if !IsPublicIP(ipStr) {
		return "内网", "LAN", "LAN", ""
	}
	e.mu.RLock()
	rec, ok := e.cache[ipStr]
	e.mu.RUnlock()
	if ok {
		if rec.LocationEN == "" {
			e.enqueue(ipStr)
		}
		return rec.Location, rec.LocationEN, firstNonEmpty(rec.ISP, rec.Org), strings.TrimSpace(rec.ISP + " " + rec.Org + " " + rec.ASN)
	}
	for _, sr := range staticRanges {
		if sr.net.Contains(ip) {
			e.enqueue(ipStr)
			return sr.rec.Location, sr.rec.LocationEN, sr.rec.ISP, sr.rec.ISP + " " + sr.rec.Org + " " + sr.rec.ASN
		}
	}
	e.enqueue(ipStr)
	return "", "", "", ""
}

func (e *Enricher) LookupASN(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil || !IsPublicIP(ipStr) {
		return ""
	}
	e.mu.RLock()
	rec, ok := e.cache[ipStr]
	e.mu.RUnlock()
	if ok && rec.ASN != "" {
		return strings.Fields(rec.ASN)[0]
	}
	for _, sr := range staticRanges {
		if sr.net.Contains(ip) && sr.rec.ASN != "" {
			return strings.Fields(sr.rec.ASN)[0]
		}
	}
	return ""
}

func (e *Enricher) enqueue(ipStr string) {
	e.mu.Lock()
	if len(e.pending) < 600 {
		e.pending[ipStr] = true
	}
	e.mu.Unlock()
}

func (e *Enricher) RunWorker(stop <-chan struct{}) {
	time.Sleep(2 * time.Second)
	e.flushBatch()
	ticker := time.NewTicker(10 * time.Second)
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
	batch := make([]string, 0, 80)
	for ip := range e.pending {
		batch = append(batch, ip)
		delete(e.pending, ip)
		if len(batch) >= 80 {
			break
		}
	}
	e.mu.Unlock()

	if e.queryIPAPI(batch) {
		return
	}
	e.queryIPWhoFallback(batch)
}

type ipapiResult struct {
	Status     string `json:"status"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
	Org        string `json:"org"`
	AS         string `json:"as"`
	Query      string `json:"query"`
}

func (e *Enricher) fetchIPAPILang(batch []string, lang string) map[string]ipapiResult {
	body, _ := json.Marshal(batch)
	u := "http://ip-api.com/batch?fields=status,country,regionName,city,isp,org,as,query&lang=" + lang
	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := e.client.Do(req)
	if err != nil {
		log.Printf("geo ip-api (%s) err: %v", lang, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("geo ip-api (%s) status: %d", lang, resp.StatusCode)
		return nil
	}
	var results []ipapiResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Printf("geo ip-api (%s) decode err: %v", lang, err)
		return nil
	}
	out := make(map[string]ipapiResult, len(results))
	for _, r := range results {
		if r.Status == "success" && r.Query != "" {
			out[r.Query] = r
		}
	}
	return out
}

func joinLoc(country, region, city string) string {
	parts := make([]string, 0, 3)
	if country != "" {
		parts = append(parts, country)
	}
	if region != "" && region != country {
		parts = append(parts, region)
	}
	if city != "" && city != region {
		parts = append(parts, city)
	}
	return strings.Join(parts, " ")
}

func normalizeZHLoc(s string) string {
	repl := strings.NewReplacer(
		"澳大利亞", "澳大利亚",
		"紐西蘭", "新西兰",
		"荷蘭", "荷兰",
		"德國", "德国",
		"法國", "法国",
		"英國", "英国",
		"韓國", "韩国",
		"臺灣", "台湾",
		"香港特別行政區", "香港",
		"澳門特別行政區", "澳门",
	)
	return repl.Replace(s)
}

func (e *Enricher) queryIPAPI(batch []string) bool {
	enMap := e.fetchIPAPILang(batch, "en")
	zhMap := e.fetchIPAPILang(batch, "zh-CN")
	if len(enMap) == 0 && len(zhMap) == 0 {
		return false
	}

	now := time.Now().Unix()
	toSave := make([]store.ISPRecord, 0, len(batch))
	e.mu.Lock()
	for _, ip := range batch {
		rEN, okEN := enMap[ip]
		rZH, okZH := zhMap[ip]
		if !okEN && !okZH {
			continue
		}
		locEN := ""
		if okEN {
			locEN = joinLoc(rEN.Country, rEN.RegionName, rEN.City)
		}
		locZH := ""
		if okZH {
			locZH = normalizeZHLoc(joinLoc(rZH.Country, rZH.RegionName, rZH.City))
		}
		if locZH == "" {
			locZH = locEN
		}
		if locEN == "" {
			locEN = locZH
		}
		base := rEN
		if !okEN {
			base = rZH
		}
		ispName := firstNonEmpty(base.ISP, base.Org, base.AS)
		rec := store.ISPRecord{
			IP:         ip,
			Location:   locZH,
			LocationEN: locEN,
			ISP:        ispName,
			Org:        firstNonEmpty(base.Org, base.AS),
			ASN:        base.AS,
			UpdatedAt:  now,
		}
		e.cache[ip] = rec
		toSave = append(toSave, rec)
	}
	e.mu.Unlock()
	if err := e.store.SaveISPCache(toSave); err != nil {
		log.Printf("geo save sqlite err: %v", err)
	}
	return len(toSave) > 0
}

func (e *Enricher) queryIPWhoFallback(batch []string) {
	now := time.Now().Unix()
	var toSave []store.ISPRecord
	limit := 10
	if len(batch) < limit {
		limit = len(batch)
	}
	for _, ip := range batch[:limit] {
		req, err := http.NewRequest("GET", "https://ipwho.is/"+ip, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := e.client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		var r struct {
			Success    bool   `json:"success"`
			Country    string `json:"country"`
			Region     string `json:"region"`
			City       string `json:"city"`
			Connection struct {
				ASN int    `json:"asn"`
				Org string `json:"org"`
				ISP string `json:"isp"`
			} `json:"connection"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err == nil && r.Success {
			locEN := strings.TrimSpace(joinLoc(r.Country, r.Region, r.City))
			ispName := firstNonEmpty(r.Connection.ISP, r.Connection.Org)
			asnStr := ""
			if r.Connection.ASN > 0 {
				asnStr = fmt.Sprintf("AS%d", r.Connection.ASN)
			}
			rec := store.ISPRecord{
				IP:         ip,
				Location:   locEN,
				LocationEN: locEN,
				ISP:        ispName,
				Org:        r.Connection.Org,
				ASN:        asnStr,
				UpdatedAt:  now,
			}
			e.mu.Lock()
			e.cache[ip] = rec
			e.mu.Unlock()
			toSave = append(toSave, rec)
		}
		resp.Body.Close()
	}
	if len(toSave) > 0 {
		_ = e.store.SaveISPCache(toSave)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
