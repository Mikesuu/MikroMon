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
	store   *store.Store
	client  *http.Client
	mu      sync.RWMutex
	status  Status
	onExit  func()
}

func New(currentVersion string, st *store.Store, onExit func()) *Manager {
	if currentVersion == "" {
		currentVersion = "dev"
	}
	auto := st.GetSetting("auto_update") == "true"
	mirror := st.GetSetting("gh_mirror")
	return &Manager{
		store:  st,
		client: &http.Client{Timeout: 25 * time.Second},
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

	ticker := time.NewTicker(30 * time.Minute)
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
	if st.HasUpdate && st.AutoUpdate && st.DownloadURL != "" && !st.Updating {
		log.Printf("auto-updater: found %s (current %s), applying...", st.LatestVersion, st.CurrentVersion)
		if err := m.Apply(ctx); err != nil {
			log.Printf("auto-updater apply err: %v", err)
		}
	}
}

func (m *Manager) Check(ctx context.Context) Status {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return m.setErr(err.Error())
	}
	req.Header.Set("User-Agent", "MikroMon-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := m.client.Do(req)
	if err != nil {
		return m.setErr(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return m.setErr(fmt.Sprintf("GitHub API status %d", resp.StatusCode))
	}

	var rel struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return m.setErr(err.Error())
	}

	targetAsset := fmt.Sprintf("mikromon-%s-%s", runtime.GOOS, runtime.GOARCH)
	var dlURL string
	for _, a := range rel.Assets {
		if a.Name == targetAsset {
			dlURL = a.BrowserDownloadURL
			break
		}
	}

	m.mu.Lock()
	m.status.LatestVersion = rel.TagName
	m.status.ReleaseNotes = rel.Body
	m.status.PublishedAt = rel.PublishedAt
	m.status.DownloadURL = dlURL
	m.status.LastChecked = time.Now().Unix()
	m.status.LastError = ""
	m.status.HasUpdate = m.status.CurrentVersion != "dev" && compareSemver(rel.TagName, m.status.CurrentVersion) > 0 && dlURL != ""
	out := m.status
	m.mu.Unlock()
	return out
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
		return fmt.Errorf("no downloadable binary asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	m.status.Updating = true
	m.status.LastError = ""
	m.mu.Unlock()

	if mirror != "" {
		dlURL = mirror + dlURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
	if err != nil {
		m.finishErr(err)
		return err
	}
	req.Header.Set("User-Agent", "MikroMon-Updater")

	dlClient := &http.Client{Timeout: 3 * time.Minute}
	resp, err := dlClient.Do(req)
	if err != nil {
		m.finishErr(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err := fmt.Errorf("download status %d", resp.StatusCode)
		m.finishErr(err)
		return err
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 80*1024*1024))
	if err != nil {
		m.finishErr(err)
		return err
	}
	if len(data) < 1500000 || !bytes.HasPrefix(data, []byte{0x7f, 'E', 'L', 'F'}) {
		err := fmt.Errorf("invalid binary file downloaded (size=%d)", len(data))
		m.finishErr(err)
		return err
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
			m.onExit()
		}
		os.Exit(0)
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
