package icons

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type appLookup struct {
	id, term, country string
}

var appCatalog = map[string]appLookup{
	"微信":            {id: "414478124", country: "cn"},
	"QQ":            {id: "444934666", country: "cn"},
	"抖音":            {id: "1142110895", country: "cn"},
	"字节跳动":          {id: "1142110895", country: "cn"},
	"小红书":           {id: "741292507", country: "cn"},
	"支付宝":           {id: "333206289", country: "cn"},
	"淘宝":            {id: "387682726", country: "cn"},
	"高德地图":          {id: "461703208", country: "cn"},
	"微博":            {id: "350962117", country: "cn"},
	"哔哩哔哩":          {id: "736536022", country: "cn"},
	"网易云音乐":         {id: "590338362", country: "cn"},
	"百度":            {id: "382201985", country: "cn"},
	"腾讯视频":          {id: "458318329", country: "cn"},
	"优酷":            {id: "336141475", country: "cn"},
	"快手":            {id: "440948110", country: "cn"},
	"QQ音乐":          {id: "414603431", country: "cn"},
	"萤石云":           {id: "507161324", country: "cn"},
	"西瓜视频":          {id: "1134496215", country: "cn"},
	"今日头条":          {id: "529092160", country: "cn"},
	"美团":            {id: "423084029", country: "cn"},
	"飞书":            {id: "1401729613", country: "cn"},
	"阿里云":           {term: "阿里云", country: "cn"},
	"阿里云 DNS":       {term: "阿里云", country: "cn"},
	"腾讯云":           {term: "腾讯云IM", country: "cn"},
	"腾讯云 DNS":       {term: "腾讯云IM", country: "cn"},
	"GitHub":        {id: "1477376905", country: "us"},
	"Steam":         {id: "495369748", country: "us"},
	"RDP 远程桌面":      {id: "714464092", country: "us"},
	"WireGuard VPN": {id: "1441195209", country: "us"},
	"YouTube":       {id: "544007664", country: "us"},
	"HomeAssistant": {id: "1099568401", country: "us"},
	"OpenAI":        {term: "ChatGPT", country: "us"},
	"Claude":        {term: "Claude by Anthropic", country: "us"},
	"Google":        {term: "Google", country: "us"},
	"Telegram":      {term: "Telegram Messenger", country: "us"},
	"Discord":       {term: "Discord", country: "us"},
	"Netflix":       {term: "Netflix", country: "us"},
	"Spotify":       {term: "Spotify", country: "us"},
	"X (Twitter)":   {term: "X", country: "us"},
	"工商银行":          {term: "中国工商银行", country: "cn"},
	"建设银行":          {term: "中国建设银行", country: "cn"},
	"农业银行":          {term: "中国农业银行", country: "cn"},
	"中国银行":          {term: "中国银行", country: "cn"},
	"交通银行":          {term: "交通银行", country: "cn"},
	"铁路12306":       {term: "铁路12306", country: "cn"},
	"ToDesk":        {term: "ToDesk", country: "cn"},
	"向日葵远程":         {term: "向日葵远程控制", country: "cn"},
	"WPS":           {term: "WPS Office", country: "cn"},
	"群晖 Synology":   {term: "DS finder", country: "cn"},
}

var forceSVG = map[string]bool{
	"Cloudflare": true, "BitTorrent/P2P": true, "Apple": true,
	"AppStore": true, "iCloud": true, "Microsoft": true, "SSH": true, "Quad9 DNS": true, "Speedtest": true,
}

type Manager struct {
	cacheDir string
	client   *http.Client
	mu       sync.Mutex
	inflight map[string]bool
	failAt   map[string]int64
	queue    chan string
	ver      int64
}

func New(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "icons")
	_ = os.MkdirAll(dir, 0755)
	m := &Manager{
		cacheDir: dir,
		client:   &http.Client{Timeout: 8 * time.Second},
		inflight: make(map[string]bool),
		failAt:   make(map[string]int64),
		queue:    make(chan string, 256),
	}
	go m.batchWarm("cn")
	go m.batchWarm("us")
	for i := 0; i < 2; i++ {
		go m.worker()
	}
	return m
}

func (m *Manager) Version() int64 {
	return atomic.LoadInt64(&m.ver)
}

func cachePathFor(cacheDir, appName string) string {
	sum := md5.Sum([]byte(appName))
	return filepath.Join(cacheDir, "v2_"+hex.EncodeToString(sum[:])+".img")
}

// 启动时用 Apple 批量 Lookup 接口一次性预热所有已知 ID，彻底避免限流
func (m *Manager) batchWarm(country string) {
	idToApps := make(map[string][]string)
	var ids []string
	for app, spec := range appCatalog {
		if spec.id == "" || spec.country != country {
			continue
		}
		if b, err := os.ReadFile(cachePathFor(m.cacheDir, app)); err == nil && len(b) > 100 {
			continue
		}
		if _, seen := idToApps[spec.id]; !seen {
			ids = append(ids, spec.id)
		}
		idToApps[spec.id] = append(idToApps[spec.id], app)
	}
	if len(ids) == 0 {
		return
	}
	apiURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s", strings.Join(ids, ","), country)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := m.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	defer resp.Body.Close()
	var res struct {
		Results []struct {
			TrackId       int64  `json:"trackId"`
			ArtworkUrl100 string `json:"artworkUrl100"`
			ArtworkUrl512 string `json:"artworkUrl512"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return
	}
	for _, r := range res.Results {
		idStr := fmt.Sprintf("%d", r.TrackId)
		imgURL := r.ArtworkUrl100
		if imgURL == "" {
			imgURL = r.ArtworkUrl512
		}
		if imgURL == "" {
			continue
		}
		imgBytes := m.downloadBytes(imgURL)
		if len(imgBytes) > 100 && strings.HasPrefix(http.DetectContentType(imgBytes), "image/") {
			for _, appName := range idToApps[idStr] {
				_ = os.WriteFile(cachePathFor(m.cacheDir, appName), imgBytes, 0644)
			}
			atomic.AddInt64(&m.ver, 1)
		}
	}
}

func (m *Manager) GetIcon(appName string) ([]byte, string, bool) {
	if forceSVG[appName] || strings.Contains(appName, ":") {
		return []byte(fallbackSVG(appName)), "image/svg+xml; charset=utf-8", true
	}
	cp := cachePathFor(m.cacheDir, appName)
	if b, err := os.ReadFile(cp); err == nil && len(b) > 100 {
		ct := http.DetectContentType(b)
		if strings.HasPrefix(ct, "image/") {
			return b, ct, true
		}
		_ = os.Remove(cp)
	}
	m.mu.Lock()
	already := m.inflight[appName]
	failed := time.Now().Unix()-m.failAt[appName] < 120
	if !already && !failed {
		m.inflight[appName] = true
		select {
		case m.queue <- appName:
		default:
			delete(m.inflight, appName)
		}
	}
	m.mu.Unlock()
	return []byte(fallbackSVG(appName)), "image/svg+xml; charset=utf-8", false
}

func (m *Manager) worker() {
	for appName := range m.queue {
		time.Sleep(250 * time.Millisecond)
		var imgBytes []byte
		if spec, ok := appCatalog[appName]; ok {
			imgBytes = m.fetchFromApple(spec)
		} else {
			imgBytes = m.fetchFromApple(appLookup{term: appName, country: "cn"})
			if len(imgBytes) <= 100 {
				imgBytes = m.fetchFromApple(appLookup{term: appName, country: "us"})
			}
		}
		if len(imgBytes) > 100 && strings.HasPrefix(http.DetectContentType(imgBytes), "image/") {
			_ = os.WriteFile(cachePathFor(m.cacheDir, appName), imgBytes, 0644)
			atomic.AddInt64(&m.ver, 1)
		} else {
			m.mu.Lock()
			m.failAt[appName] = time.Now().Unix()
			m.mu.Unlock()
		}
		m.mu.Lock()
		delete(m.inflight, appName)
		m.mu.Unlock()
	}
}

func (m *Manager) fetchFromApple(spec appLookup) []byte {
	apiURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&country=%s&entity=software&limit=1", url.QueryEscape(spec.term), spec.country)
	if spec.id != "" {
		apiURL = fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s", spec.id, spec.country)
	}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := m.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var res struct {
		Results []struct {
			TrackName     string `json:"trackName"`
			ArtworkUrl100 string `json:"artworkUrl100"`
			ArtworkUrl512 string `json:"artworkUrl512"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Results) == 0 {
		return nil
	}
	if spec.id == "" && spec.term != "" {
		tn := strings.ToLower(res.Results[0].TrackName)
		tk := strings.ToLower(spec.term)
		if !strings.Contains(tn, tk) && !strings.Contains(tk, tn) {
			return nil
		}
	}
	imgURL := res.Results[0].ArtworkUrl100
	if imgURL == "" {
		imgURL = res.Results[0].ArtworkUrl512
	}
	if imgURL == "" {
		return nil
	}
	return m.downloadBytes(imgURL)
}

func (m *Manager) downloadBytes(imgURL string) []byte {
	imgResp, err := m.client.Get(imgURL)
	if err != nil || imgResp.StatusCode != 200 {
		if imgResp != nil {
			imgResp.Body.Close()
		}
		return nil
	}
	defer imgResp.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(imgResp.Body, 512*1024))
	return buf
}

func fallbackSVG(app string) string {
	r := []rune(app)
	lbl := "AP"
	if len(r) >= 2 {
		lbl = string(r[:2])
	} else if len(r) == 1 {
		lbl = string(r[0])
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><rect width="48" height="48" rx="11" fill="#0099cc"/><text x="24" y="30" text-anchor="middle" fill="#fff" font-family="sans-serif" font-size="16" font-weight="bold">%s</text></svg>`, lbl)
}
