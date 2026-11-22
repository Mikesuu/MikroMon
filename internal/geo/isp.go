package geo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ipipdotnet/ipdb-go"

	"mikromon/internal/store"
)

const defaultIPIPDBURL = "https://jms-pkg.oss-cn-beijing.aliyuncs.com/ip/ipipfree.ipdb"

type Enricher struct {
	store          *store.Store
	dataDir        string
	dbPath         string
	mu             sync.RWMutex
	cityDB         *ipdb.City
	dbLoaded       bool
	dbDownloading  bool
	cache          map[string]store.ISPRecord
	pending        map[string]bool
	saveBuf        map[string]store.ISPRecord
	failCooldown   map[string]int64
	ipapiBlockedAt int64
	client         *http.Client
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

var zhToEnGeo = map[string]string{
	"中国": "China", "香港": "Hong Kong", "澳门": "Macao", "台湾": "Taiwan",
	"美国": "United States", "英国": "United Kingdom", "日本": "Japan", "韩国": "South Korea",
	"新加坡": "Singapore", "德国": "Germany", "法国": "France", "荷兰": "Netherlands",
	"俄罗斯": "Russia", "加拿大": "Canada", "澳大利亚": "Australia", "新西兰": "New Zealand",
	"以色列": "Israel", "印度": "India", "巴西": "Brazil", "爱尔兰": "Ireland",
	"瑞典": "Sweden", "瑞士": "Switzerland", "芬兰": "Finland", "挪威": "Norway",
	"意大利": "Italy", "西班牙": "Spain", "波兰": "Poland", "马来西亚": "Malaysia",
	"泰国": "Thailand", "越南": "Vietnam", "印度尼西亚": "Indonesia", "菲律宾": "Philippines",
	"土耳其": "Turkey", "乌克兰": "Ukraine", "南非": "South Africa", "阿联酋": "United Arab Emirates",
	"沙特阿拉伯": "Saudi Arabia", "墨西哥": "Mexico", "阿根廷": "Argentina", "罗马尼亚": "Romania",
	"保加利亚": "Bulgaria", "捷克": "Czech Republic", "奥地利": "Austria", "比利时": "Belgium",
	"丹麦": "Denmark", "葡萄牙": "Portugal", "哈萨克斯坦": "Kazakhstan", "卢森堡": "Luxembourg",
	"冰岛": "Iceland", "保留地址": "Reserved", "局域网": "LAN", "内网": "LAN", "本机地址": "Localhost",
	"北京": "Beijing", "上海": "Shanghai", "天津": "Tianjin", "重庆": "Chongqing",
	"河北": "Hebei", "山西": "Shanxi", "辽宁": "Liaoning", "吉林": "Jilin", "黑龙江": "Heilongjiang",
	"江苏": "Jiangsu", "浙江": "Zhejiang", "安徽": "Anhui", "福建": "Fujian", "江西": "Jiangxi",
	"山东": "Shandong", "河南": "Henan", "湖北": "Hubei", "湖南": "Hunan", "广东": "Guangdong",
	"海南": "Hainan", "四川": "Sichuan", "贵州": "Guizhou", "云南": "Yunnan", "陕西": "Shaanxi",
	"甘肃": "Gansu", "青海": "Qinghai", "内蒙古": "Inner Mongolia", "广西": "Guangxi",
	"西藏": "Tibet", "宁夏": "Ningxia", "新疆": "Xinjiang",
	"成都": "Chengdu", "深圳": "Shenzhen", "广州": "Guangzhou", "杭州": "Hangzhou", "南京": "Nanjing",
	"武汉": "Wuhan", "西安": "Xi'an", "苏州": "Suzhou", "郑州": "Zhengzhou", "长沙": "Changsha",
	"东莞": "Dongguan", "佛山": "Foshan", "沈阳": "Shenyang", "青岛": "Qingdao", "宁波": "Ningbo",
	"无锡": "Wuxi", "昆明": "Kunming", "大连": "Dalian", "厦门": "Xiamen", "合肥": "Hefei",
	"福州": "Fuzhou", "哈尔滨": "Harbin", "济南": "Jinan", "长春": "Changchun", "温州": "Wenzhou",
	"石家庄": "Shijiazhuang", "南宁": "Nanning", "常州": "Changzhou", "泉州": "Quanzhou", "南昌": "Nanchang",
	"贵阳": "Guiyang", "太原": "Taiyuan", "烟台": "Yantai", "嘉兴": "Jiaxing", "南通": "Nantong",
	"金华": "Jinhua", "珠海": "Zhuhai", "惠州": "Huizhou", "徐州": "Xuzhou", "海口": "Haikou",
	"乌鲁木齐": "Urumqi", "绍兴": "Shaoxing", "中山": "Zhongshan", "台州": "Taizhou", "兰州": "Lanzhou",
}

func translateZHToEN(locZH string) string {
	if locZH == "" {
		return ""
	}
	parts := strings.Fields(locZH)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		clean := strings.TrimSuffix(strings.TrimSuffix(p, "省"), "市")
		if en, ok := zhToEnGeo[p]; ok {
			out = append(out, en)
		} else if en, ok := zhToEnGeo[clean]; ok {
			out = append(out, en)
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func New(st *store.Store) *Enricher {
	dataDir := os.Getenv("MIKROMON_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}
	dbPath := filepath.Join(dataDir, "ipipfree.ipdb")

	cached, _ := st.LoadISPCache()
	if cached == nil {
		cached = make(map[string]store.ISPRecord)
	}
	// 本地内存直接补全历史记录里的英文地名（0 外网请求）
	for ip, rec := range cached {
		if rec.LocationEN == "" && rec.Location != "" {
			rec.LocationEN = translateZHToEN(rec.Location)
			cached[ip] = rec
		}
	}

	e := &Enricher{
		store:        st,
		dataDir:      dataDir,
		dbPath:       dbPath,
		cache:        cached,
		pending:      make(map[string]bool),
		saveBuf:      make(map[string]store.ISPRecord),
		failCooldown: make(map[string]int64),
		client:       &http.Client{Timeout: 6 * time.Second},
	}
	e.loadLocalIPDB()
	if !e.dbLoaded {
		go e.DownloadLocalDB(false)
	}
	return e
}

func (e *Enricher) loadLocalIPDB() {
	candidates := []string{
		e.dbPath,
		filepath.Join(e.dataDir, "city.ipdb"),
		filepath.Join(e.dataDir, "ipip.ipdb"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.Size() > 1024*100 {
			if db, err := ipdb.NewCity(p); err == nil {
				e.mu.Lock()
				e.cityDB = db
				e.dbLoaded = true
				e.dbPath = p
				e.mu.Unlock()
				log.Printf("geo: loaded local offline database %s (%d KB)", p, st.Size()/1024)
				return
			}
		}
	}
}

func (e *Enricher) DBStatus() map[string]any {
	e.mu.RLock()
	loaded := e.dbLoaded
	downloading := e.dbDownloading
	path := e.dbPath
	e.mu.RUnlock()
	var sizeKB int64
	var modAt int64
	if st, err := os.Stat(path); err == nil {
		sizeKB = st.Size() / 1024
		modAt = st.ModTime().Unix()
	}
	return map[string]any{
		"loaded":      loaded,
		"downloading": downloading,
		"path":        path,
		"size_kb":     sizeKB,
		"updated_at":  modAt,
	}
}

func (e *Enricher) DownloadLocalDB(force bool) error {
	e.mu.Lock()
	if e.dbDownloading {
		e.mu.Unlock()
		return nil
	}
	if !force && e.dbLoaded {
		e.mu.Unlock()
		return nil
	}
	e.dbDownloading = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.dbDownloading = false
		e.mu.Unlock()
	}()

	_ = os.MkdirAll(e.dataDir, 0755)
	dlClient := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", defaultIPIPDBURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := dlClient.Do(req)
	if err != nil {
		log.Printf("geo: download ipipfree.ipdb err: %v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = fmt.Errorf("download status %d", resp.StatusCode)
		log.Printf("geo: %v", err)
		return err
	}

	tmpPath := e.dbPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 80*1024*1024))
	_ = f.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}

	db, err := ipdb.NewCity(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, e.dbPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if reloaded, err := ipdb.NewCity(e.dbPath); err == nil {
		db = reloaded
	}
	e.mu.Lock()
	e.cityDB = db
	e.dbLoaded = true
	e.mu.Unlock()
	log.Printf("geo: local offline ipipfree.ipdb ready at %s", e.dbPath)
	return nil
}

func (e *Enricher) ClearMemory() {
	e.mu.Lock()
	e.cache = make(map[string]store.ISPRecord)
	e.failCooldown = make(map[string]int64)
	e.ipapiBlockedAt = 0
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

func dedupLocParts(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == p {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

func (e *Enricher) lookupOfflineIPDB(ipStr string) (store.ISPRecord, bool) {
	e.mu.RLock()
	db := e.cityDB
	e.mu.RUnlock()
	if db == nil {
		return store.ISPRecord{}, false
	}
	info, err := db.FindInfo(ipStr, "CN")
	if err != nil || info == nil || info.CountryName == "" {
		return store.ISPRecord{}, false
	}
	locZH := dedupLocParts(info.CountryName, info.RegionName, info.CityName)
	locEN := translateZHToEN(locZH)
	isp := firstNonEmpty(info.IspDomain, info.OwnerDomain)
	return store.ISPRecord{
		IP:         ipStr,
		Location:   locZH,
		LocationEN: locEN,
		ISP:        isp,
		Org:        info.OwnerDomain,
		UpdatedAt:  time.Now().Unix(),
	}, true
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
		locEN := rec.LocationEN
		if locEN == "" && rec.Location != "" {
			locEN = translateZHToEN(rec.Location)
		}
		return rec.Location, locEN, firstNonEmpty(rec.ISP, rec.Org), strings.TrimSpace(rec.ISP + " " + rec.Org + " " + rec.ASN)
	}

	// 1. 优先命中静态内置段
	for _, sr := range staticRanges {
		if sr.net.Contains(ip) {
			return sr.rec.Location, sr.rec.LocationEN, sr.rec.ISP, sr.rec.ISP + " " + sr.rec.Org + " " + sr.rec.ASN
		}
	}

	// 2. 命中本地离线 IPIP 数据库并异步批量落盘（0 阻塞、不刷外网）
	if rec, ok := e.lookupOfflineIPDB(ipStr); ok {
		e.mu.Lock()
		e.cache[ipStr] = rec
		e.saveBuf[ipStr] = rec
		e.mu.Unlock()
		return rec.Location, rec.LocationEN, firstNonEmpty(rec.ISP, rec.Org), ""
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
	now := time.Now().Unix()
	e.mu.Lock()
	if until, cooled := e.failCooldown[ipStr]; cooled && now < until {
		e.mu.Unlock()
		return
	}
	if len(e.pending) < 400 {
		e.pending[ipStr] = true
	}
	e.mu.Unlock()
}

func (e *Enricher) RunWorker(stop <-chan struct{}) {
	time.Sleep(3 * time.Second)
	e.flushBatch()
	ticker := time.NewTicker(15 * time.Second)
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
	var bufList []store.ISPRecord
	if len(e.saveBuf) > 0 {
		bufList = make([]store.ISPRecord, 0, len(e.saveBuf))
		for _, r := range e.saveBuf {
			bufList = append(bufList, r)
		}
		e.saveBuf = make(map[string]store.ISPRecord)
	}
	e.mu.Unlock()
	if len(bufList) > 0 {
		_ = e.store.SaveISPCache(bufList)
	}

	e.mu.Lock()
	if len(e.pending) == 0 {
		e.mu.Unlock()
		return
	}
	batch := make([]string, 0, 30)
	for ip := range e.pending {
		batch = append(batch, ip)
		delete(e.pending, ip)
		if len(batch) >= 30 {
			break
		}
	}
	e.mu.Unlock()

	ip138Token := strings.TrimSpace(e.store.GetSetting("geo_ip138_token"))
	remaining := make([]string, 0, len(batch))
	var saved []store.ISPRecord

	for _, ip := range batch {
		// 先再检查一次本地离线库（防止刚下载完成）
		if rec, ok := e.lookupOfflineIPDB(ip); ok && rec.ISP != "" {
			saved = append(saved, rec)
			continue
		}
		// 如果配置了 IP138 Token，优先使用 IP138
		if ip138Token != "" {
			if rec, ok := e.queryIP138(ip, ip138Token); ok {
				saved = append(saved, rec)
				continue
			}
		}
		// 其次使用国内直连友好的淘宝 IP 接口（限速轻量）
		if len(remaining) < 8 {
			if rec, ok := e.queryTaobao(ip); ok {
				saved = append(saved, rec)
				time.Sleep(150 * time.Millisecond)
				continue
			}
		}
		remaining = append(remaining, ip)
	}

	// 本地离线库已加载时，不再请求海外 ip-api.com（彻底杜绝超时与日志刷屏）
	e.mu.RLock()
	hasLocalDB := e.dbLoaded
	e.mu.RUnlock()
	if len(remaining) > 0 && !hasLocalDB {
		now := time.Now().Unix()
		e.mu.RLock()
		blocked := now-e.ipapiBlockedAt < 300
		e.mu.RUnlock()
		if !blocked {
			if ok := e.queryIPAPI(remaining); !ok {
				e.mu.Lock()
				e.ipapiBlockedAt = now
				for _, ip := range remaining {
					e.failCooldown[ip] = now + 600
				}
				e.mu.Unlock()
			}
		} else {
			e.mu.Lock()
			for _, ip := range remaining {
				e.failCooldown[ip] = now + 600
			}
			e.mu.Unlock()
		}
	}

	if len(saved) > 0 {
		e.mu.Lock()
		for _, rec := range saved {
			if old, ok := e.cache[rec.IP]; ok {
				if rec.Location == "" {
					rec.Location = old.Location
				}
				if rec.LocationEN == "" {
					rec.LocationEN = old.LocationEN
				}
				if rec.ISP == "" {
					rec.ISP = old.ISP
				}
			}
			e.cache[rec.IP] = rec
		}
		e.mu.Unlock()
		_ = e.store.SaveISPCache(saved)
	}
}

func (e *Enricher) queryIP138(ip, token string) (store.ISPRecord, bool) {
	u := fmt.Sprintf("https://api.ip138.com/ip/?ip=%s&datatype=json&token=%s", ip, token)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return store.ISPRecord{}, false
	}
	resp, err := e.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return store.ISPRecord{}, false
	}
	defer resp.Body.Close()
	var res struct {
		Ret  string   `json:"ret"`
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.Ret != "ok" || len(res.Data) < 5 {
		return store.ISPRecord{}, false
	}
	locZH := dedupLocParts(res.Data[0], res.Data[1], res.Data[2])
	return store.ISPRecord{
		IP:         ip,
		Location:   locZH,
		LocationEN: translateZHToEN(locZH),
		ISP:        strings.TrimSpace(res.Data[4]),
		UpdatedAt:  time.Now().Unix(),
	}, true
}

func (e *Enricher) queryTaobao(ip string) (store.ISPRecord, bool) {
	u := "https://ip.taobao.com/outGetIpInfo?accessKey=alibaba-inc&ip=" + ip
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return store.ISPRecord{}, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := e.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return store.ISPRecord{}, false
	}
	defer resp.Body.Close()
	var res struct {
		Code int `json:"code"`
		Data struct {
			Country string `json:"country"`
			Region  string `json:"region"`
			City    string `json:"city"`
			ISP     string `json:"isp"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.Code != 0 || res.Data.Country == "" {
		return store.ISPRecord{}, false
	}
	cleanField := func(v string) string {
		if v == "XX" {
			return ""
		}
		return v
	}
	locZH := dedupLocParts(cleanField(res.Data.Country), cleanField(res.Data.Region), cleanField(res.Data.City))
	isp := cleanField(res.Data.ISP)
	return store.ISPRecord{
		IP:         ip,
		Location:   locZH,
		LocationEN: translateZHToEN(locZH),
		ISP:        isp,
		UpdatedAt:  time.Now().Unix(),
	}, true
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
		log.Printf("geo ip-api (%s) unreachable (cooling down 5m): %v", lang, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var results []ipapiResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
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

func (e *Enricher) queryIPAPI(batch []string) bool {
	enMap := e.fetchIPAPILang(batch, "en")
	if len(enMap) == 0 {
		return false
	}
	zhMap := e.fetchIPAPILang(batch, "zh-CN")

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
			locEN = dedupLocParts(rEN.Country, rEN.RegionName, rEN.City)
		}
		locZH := ""
		if okZH {
			locZH = dedupLocParts(rZH.Country, rZH.RegionName, rZH.City)
		}
		if locZH == "" {
			locZH = locEN
		}
		if locEN == "" {
			locEN = translateZHToEN(locZH)
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
