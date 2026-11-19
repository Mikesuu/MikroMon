package adguard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type QueryEntry struct {
	TS       int64
	ClientIP string
	Domain   string
	Answers  []string
}

var httpClient = &http.Client{Timeout: 4 * time.Second}

func FetchQueryLog(baseURL, user, pass string, limit int) ([]QueryEntry, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, nil
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	url := fmt.Sprintf("%s/control/querylog?limit=%d", baseURL, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("adguard status %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			Time     string `json:"time"`
			Client   string `json:"client"`
			Question struct {
				Name string `json:"name"`
			} `json:"question"`
			Answer []struct {
				Type  string `json:"type"`
				Value any    `json:"value"`
			} `json:"answer"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]QueryEntry, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Question.Name == "" {
			continue
		}
		ts := time.Now().Unix()
		if t, err := time.Parse(time.RFC3339Nano, item.Time); err == nil {
			ts = t.Unix()
		}
		var ans []string
		for _, a := range item.Answer {
			if a.Type == "A" || a.Type == "AAAA" {
				if s, ok := a.Value.(string); ok && s != "" {
					ans = append(ans, s)
				}
			}
		}
		out = append(out, QueryEntry{
			TS:       ts,
			ClientIP: item.Client,
			Domain:   strings.TrimSuffix(item.Question.Name, "."),
			Answers:  ans,
		})
	}
	return out, nil
}
