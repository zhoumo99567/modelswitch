package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type SkillPopularity struct {
	Name     string `json:"name"`
	Installs int64  `json:"installs"`
	URL      string `json:"url"`
}
type SkillMarketMetrics struct {
	Market          string            `json:"market"`
	Repository      string            `json:"repository"`
	RepositoryStars *int64            `json:"repositoryStars"`
	Skills          []SkillPopularity `json:"skills"`
	UpdatedAt       string            `json:"updatedAt"`
	Errors          []string          `json:"errors"`
}
type metricsEntry struct {
	Value SkillMarketMetrics
	At    time.Time
}
type metricsFlight struct {
	Done  chan struct{}
	Value SkillMarketMetrics
}

var marketMetricsCache = struct {
	sync.Mutex
	Entries map[string]metricsEntry
	Jobs    map[string]*metricsFlight
}{Entries: map[string]metricsEntry{}, Jobs: map[string]*metricsFlight{}}

func (a *App) LoadSkillMarketMetrics(marketID string) (SkillMarketMetrics, error) {
	m, ok := findMarket(marketID)
	if !ok {
		return SkillMarketMetrics{}, errors.New("不支持的技能市场")
	}
	marketMetricsCache.Lock()
	if cached, ok := marketMetricsCache.Entries[m.ID]; ok && time.Since(cached.At) < time.Hour {
		marketMetricsCache.Unlock()
		return cached.Value, nil
	}
	if job, ok := marketMetricsCache.Jobs[m.ID]; ok {
		marketMetricsCache.Unlock()
		<-job.Done
		return job.Value, nil
	}
	job := &metricsFlight{Done: make(chan struct{})}
	marketMetricsCache.Jobs[m.ID] = job
	marketMetricsCache.Unlock()
	job.Value = fetchSkillMarketMetrics(m)
	marketMetricsCache.Lock()
	if len(job.Value.Errors) == 0 {
		marketMetricsCache.Entries[m.ID] = metricsEntry{job.Value, time.Now()}
	}
	delete(marketMetricsCache.Jobs, m.ID)
	close(job.Done)
	marketMetricsCache.Unlock()
	return job.Value, nil
}
func fetchSkillMarketMetrics(m SkillMarket) SkillMarketMetrics {
	result := SkillMarketMetrics{Market: m.ID, Repository: m.Repository, Skills: []SkillPopularity{}, Errors: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	type response struct {
		Kind string
		Data []byte
		Err  error
	}
	replies := make(chan response, 2)
	owner, _, _ := strings.Cut(m.Repository, "/")
	urls := map[string]string{"installs": "https://skills.sh/api/search?" + url.Values{"q": {m.Repository}, "owner": {owner}, "limit": {"200"}}.Encode(), "stars": "https://api.github.com/repos/" + m.Repository}
	for kind, rawURL := range urls {
		go func(kind, rawURL string) {
			data, err := fetchURLContext(ctx, rawURL, 2<<20)
			replies <- response{kind, data, err}
		}(kind, rawURL)
	}
	for range urls {
		r := <-replies
		if r.Err != nil {
			result.Errors = append(result.Errors, r.Kind+": "+r.Err.Error())
			continue
		}
		if r.Kind == "stars" {
			var repo struct {
				FullName string `json:"full_name"`
				Stars    *int64 `json:"stargazers_count"`
			}
			if json.Unmarshal(r.Data, &repo) != nil || repo.FullName != m.Repository || repo.Stars == nil || *repo.Stars < 0 {
				result.Errors = append(result.Errors, "GitHub 仓库热度数据无效")
				continue
			}
			result.RepositoryStars = repo.Stars
		} else {
			entries, err := parseSkillPopularity(r.Data, m.Repository)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			result.Skills = entries
		}
	}
	sort.Strings(result.Errors)
	result.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return result
}
func parseSkillPopularity(data []byte, repository string) ([]SkillPopularity, error) {
	var listing struct {
		Skills []struct {
			ID       string `json:"id"`
			Source   string `json:"source"`
			SkillID  string `json:"skillId"`
			Name     string `json:"name"`
			Installs *int64 `json:"installs"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(data, &listing); err != nil || listing.Skills == nil {
		return nil, errors.New("skills.sh 安装量数据无效")
	}
	result := []SkillPopularity{}
	seen := map[string]bool{}
	for _, item := range listing.Skills {
		if item.Source != repository || item.Installs == nil || *item.Installs < 0 {
			continue
		}
		name := item.SkillID
		if name == "" {
			name = item.Name
		}
		if !skillNamePattern.MatchString(name) || item.ID != repository+"/"+name || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, SkillPopularity{Name: name, Installs: *item.Installs, URL: "https://skills.sh/" + repository + "/" + name})
	}
	return result, nil
}
