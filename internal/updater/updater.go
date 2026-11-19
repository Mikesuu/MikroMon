package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"mikromon/internal/store"
)

const Repo = "Mikesuu/MikroMon"

type Status struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
	DownloadURL    string `json:"download_url"`
	AutoUpdate     bool   `json:"auto_update"`
	MirrorPrefix   string `json:"mirror_prefix"`
	Updating       bool   `json:"updating"`
	LastChecked    int64  `json:"last_checked"`
	LastError      string `json:"last_error,omitempty"`
}

type Manager struct {
	store  *store.Store
	client *http.Client
	mu     sync.RWMutex
	status Status
	onExit func(string)
}

func New(currentVersion string, st *store.Store, onExit func(string)) *Manager {
	if currentVersion == "" {
		currentVersion = "dev"
	}
	auto := st.GetSetting("auto_update") == "true"
	mirror := st.GetSetting("gh_mirror")
	return &Manager{
		store:  st,
		client: &http.Client{Timeout: 20 * time.Second},
		onExit: onExit,
		status: Status{
			CurrentVersion: currentVersion,
			AutoUpdate:     auto,
			MirrorPrefix:   mirror,
		},
	}
}

func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) SaveConfig(autoUpdate bool, mirrorPrefix string) {
	mirrorPrefix = strings.TrimSpace(mirrorPrefix)
	if mirrorPrefix != "" && !strings.HasSuffix(mirrorPrefix, "/") {
		mirrorPrefix += "/"
	}
	v := "false"
	if autoUpdate {
		v = "true"
	}
	_ = m.store.SetSetting("auto_update", v)
	_ = m.store.SetSetting("gh_mirror", mirrorPrefix)

	m.mu.Lock()
	m.status.AutoUpdate = autoUpdate
	m.status.MirrorPrefix = mirrorPrefix
	m.mu.Unlock()
}

func (m *Manager) RunLoop(ctx context.Context) {
	time.Sleep(15 * time.Second)
	m.CheckAndMaybeApply(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CheckAndMaybeApply(ctx)
		}
	}
}

func (m *Manager) CheckAndMaybeApply(ctx context.Context) {
	st := m.Check(ctx)
	if st.HasUpdate && st.AutoUpdate && st.DownloadURL != "" && !st.Updating && st.CurrentVersion != "dev" {
		log.Printf("auto-updater: found %s (current %s), applying...", st.LatestVersion, st.CurrentVersion)
		if err := m.Apply(ctx); err != nil {
			log.Printf("auto-updater apply err: %v", err)
		}
	}
}

func (m *Manager) Check(ctx context.Context) Status {
	tag, err := m.checkViaRedirect(ctx)
	if err != nil || tag == "" {
		var apiErr error
		tag, apiErr = m.checkViaAPI(ctx)
		if apiErr != nil || tag == "" {
			if apiErr != nil {
				return m.setErr(apiErr.Error())
			}
			return m.setErr(err.Error())
		}
	}

	assetName := fmt.Sprintf("mikromon-%s-%s", runtime.GOOS, runtime.GOARCH)
	dlURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", Repo, tag, assetName)

	m.mu.Lock()
	m.status.LatestVersion = tag
	m.status.DownloadURL = dlURL
	m.status.LastChecked = time.Now().Unix()
	m.status.LastError = ""
	if m.status.CurrentVersion == "dev" {
		m.status.HasUpdate = true
	} else {
		m.status.HasUpdate = compareSemver(tag, m.status.CurrentVersion) > 0
	}
	out := m.status
	m.mu.Unlock()
	return out
}

// 优先通过 GitHub releases/latest 重定向获取最新 Tag（无限流、永不 403）
func (m *Manager) checkViaRedirect(ctx context.Context) (string, error) {
	noRedir := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	u := fmt.Sprintf("https://github.com/%s/releases/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, "HEAD", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := noRedir.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	loc := resp.Header.Get("Location")
	if idx := strings.LastIndex(loc, "/releases/tag/"); idx >= 0 {
		tag := strings.TrimSpace(loc[idx+len("/releases/tag/"):])
		if tag != "" {
			return tag, nil
		}
	}
	return "", fmt.Errorf("redirect status %d", resp.StatusCode)
}

func (m *Manager) checkViaAPI(ctx context.Context) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MikroMon-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func (m *Manager) setErr(msg string) Status {
	m.mu.Lock()
	m.status.LastError = msg
	m.status.LastChecked = time.Now().Unix()
	out := m.status
	m.mu.Unlock()
	return out
}

func (m *Manager) Apply(ctx context.Context) error {
	m.mu.Lock()
	if m.status.Updating {
		m.mu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	dlURL := m.status.DownloadURL
	mirror := m.status.MirrorPrefix
	if dlURL == "" {
		m.mu.Unlock()
		return fmt.Errorf("no downloadable binary asset found")
	}
	m.status.Updating = true
	m.status.LastError = ""
	m.mu.Unlock()

	var urls []string
	if mirror != "" {
		urls = append(urls, mirror+dlURL)
	}
	urls = append(urls, dlURL)

	var data []byte
	var lastErr error
	dlClient := &http.Client{Timeout: 3 * time.Minute}
	for _, tryURL := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", tryURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := dlClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			lastErr = fmt.Errorf("download status %d from %s", resp.StatusCode, tryURL)
			continue
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 80*1024*1024))
		resp.Body.Close()
		if err != nil || len(b) < 1500000 || !bytes.HasPrefix(b, []byte{0x7f, 'E', 'L', 'F'}) {
			lastErr = fmt.Errorf("invalid binary (size=%d) from %s", len(b), tryURL)
			continue
		}
		data = b
		break
	}
	if len(data) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("download failed")
		}
		m.finishErr(lastErr)
		return lastErr
	}

	targetPath := os.Getenv("MIKROMON_OTA_PATH")
	if targetPath == "" {
		exe, err := os.Executable()
		if err != nil {
			m.finishErr(err)
			return err
		}
		targetPath = exe
	}
	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
		m.finishErr(err)
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		m.finishErr(err)
		return err
	}

	log.Printf("updater: installed %s to %s, restarting...", m.status.LatestVersion, targetPath)
	go func() {
		time.Sleep(500 * time.Millisecond)
		if m.onExit != nil {
			m.onExit(targetPath)
		} else {
			os.Exit(1)
		}
	}()
	return nil
}

func (m *Manager) finishErr(err error) {
	m.mu.Lock()
	m.status.Updating = false
	m.status.LastError = err.Error()
	m.mu.Unlock()
}

func compareSemver(a, b string) int {
	pa := parseVer(a)
	pb := parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func parseVer(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}
