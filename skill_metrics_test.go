package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPopularityExactSourceAndMissingValues(t *testing.T) {
	data := []byte(`{"skills":[{"id":"vercel-labs/agent-skills/demo","source":"vercel-labs/agent-skills","skillId":"demo","installs":0},{"id":"fork/skills/demo","source":"fork/skills","skillId":"demo","installs":99999},{"id":"vercel-labs/agent-skills/negative","source":"vercel-labs/agent-skills","skillId":"negative","installs":-1},{"id":"vercel-labs/agent-skills/missing","source":"vercel-labs/agent-skills","skillId":"missing"},{"id":"bad/id","source":"vercel-labs/agent-skills","skillId":"different","installs":100}]}`)
	entries, err := parseSkillPopularity(data, "vercel-labs/agent-skills")
	if err != nil || len(entries) != 1 || entries[0].Name != "demo" || entries[0].Installs != 0 || entries[0].URL != "https://skills.sh/vercel-labs/agent-skills/demo" {
		t.Fatalf("stats: %+v %v", entries, err)
	}
	if _, err := parseSkillPopularity([]byte(`{"error":"unavailable"}`), "vercel-labs/agent-skills"); err == nil {
		t.Fatal("invalid stats treated as zero installs")
	}
}
func TestMetricsCoalescedAndCached(t *testing.T) {
	marketMetricsCache.Lock()
	previous := marketMetricsCache.Entries
	marketMetricsCache.Entries = map[string]metricsEntry{}
	marketMetricsCache.Unlock()
	old := http.DefaultTransport
	var calls atomic.Int64
	http.DefaultTransport = skillTestTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		body := `{"full_name":"vercel-labs/agent-skills","stargazers_count":123}`
		if r.URL.Host == "skills.sh" {
			body = `{"skills":[{"id":"vercel-labs/agent-skills/demo","source":"vercel-labs/agent-skills","skillId":"demo","installs":456}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = old
		marketMetricsCache.Lock()
		marketMetricsCache.Entries = previous
		marketMetricsCache.Unlock()
	})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats, err := NewApp().LoadSkillMarketMetrics("vercel")
			if err != nil || len(stats.Errors) != 0 || stats.RepositoryStars == nil || *stats.RepositoryStars != 123 || len(stats.Skills) != 1 || stats.Skills[0].Installs != 456 {
				t.Errorf("metrics: %+v, %v", stats, err)
			}
		}()
	}
	wg.Wait()
	if _, err := NewApp().LoadSkillMarketMetrics("vercel"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("duplicated stats requests: %d", calls.Load())
	}
	if _, err := NewApp().LoadSkillMarketMetrics("unknown"); err == nil {
		t.Fatal("unknown market accepted")
	}
}
func TestMarketHTTPHonorsDeadline(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = skillTestTransport(func(r *http.Request) (*http.Response, error) {
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(time.Second):
			return nil, errors.New("context not propagated")
		}
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := fetchMarketCatalogContext(ctx, skillMarkets[0])
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("deadline: %v after %s", err, time.Since(started))
	}
}
func TestCatalogConcurrentRequestsCoalesce(t *testing.T) {
	m, _ := findMarket("vercel")
	archive := skillArchive(t, map[string]string{"repo/skills/demo/SKILL.md": testSkillDoc})
	catalogCache.Lock()
	oldEntries := catalogCache.Entries
	catalogCache.Entries = map[string]cachedCatalog{}
	catalogCache.Unlock()
	old := http.DefaultTransport
	var requests atomic.Int64
	http.DefaultTransport = skillTestTransport(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		data := archive
		if r.URL.Host == "api.github.com" {
			data = []byte(`{"object":{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = old
		catalogCache.Lock()
		catalogCache.Entries = oldEntries
		catalogCache.Unlock()
	})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := fetchMarketCatalog(m)
			if err != nil || len(items) != 1 {
				t.Errorf("catalog: %v %v", items, err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 2 {
		t.Fatalf("duplicated catalog downloads: %d", requests.Load())
	}
}

func TestLiveSkillMarketMetrics(t *testing.T) {
	if os.Getenv("MODELSWITCHER_LIVE_TEST") != "1" {
		t.Skip("optional network integration")
	}
	for _, id := range []string{"openai", "anthropic", "vercel"} {
		t.Run(id, func(t *testing.T) {
			stats, err := NewApp().LoadSkillMarketMetrics(id)
			if err != nil || len(stats.Errors) > 0 || stats.RepositoryStars == nil || len(stats.Skills) == 0 {
				t.Fatalf("metrics unavailable: %+v %v", stats, err)
			}
			t.Logf("%s: %d matched skills; %d repository stars; sample %s = %d installs", stats.Repository, len(stats.Skills), *stats.RepositoryStars, stats.Skills[0].Name, stats.Skills[0].Installs)
		})
	}
}
