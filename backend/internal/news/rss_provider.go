package news

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"security-portal/internal/models"

	"github.com/mmcdole/gofeed"
)

// Provider abstrae cualquier fuente externa de noticias (RSS, API, etc.).
// Vive fuera de internal/repository porque NO es persistencia: es un gateway
// hacia un sistema externo, conceptualmente equivalente a internal/jira.
type Provider interface {
	FetchNews(ctx context.Context) ([]models.NewsItem, error)
}

type rssProvider struct {
	feedURLs       []string
	limitPerSource int
}

// NewRSSProvider consulta varios feeds RSS en paralelo. Por cada feed toma
// los `limitPerSource` items más recientes y los devuelve concatenados en
// el orden de feedURLs (NO mezclados por fecha global), de forma que la UI
// pueda agruparlos por fuente sin esfuerzo.
func NewRSSProvider(urls []string, limitPerSource int) Provider {
	if limitPerSource <= 0 {
		limitPerSource = 3
	}
	return &rssProvider{feedURLs: urls, limitPerSource: limitPerSource}
}

// rssItem mantiene el time.Time original al lado del NewsItem (que tiene
// la fecha como string ya formateada). Es interno: solo lo necesita el sort
// dentro de fetchOne.
type rssItem struct {
	item    models.NewsItem
	pubTime time.Time
}

func (p *rssProvider) FetchNews(ctx context.Context) ([]models.NewsItem, error) {
	if len(p.feedURLs) == 0 {
		return nil, fmt.Errorf("no hay feeds configurados")
	}

	type result struct {
		items []models.NewsItem
		err   error
	}
	results := make([]result, len(p.feedURLs))

	// Una goroutine por feed. gofeed.ParseURLWithContext respeta el ctx
	// del padre, así que al cancelar el ctx (timeout, shutdown) los fetches
	// abortan y la wg termina rápido.
	var wg sync.WaitGroup
	for i, url := range p.feedURLs {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			items, err := fetchOne(ctx, url, p.limitPerSource)
			results[i] = result{items: items, err: err}
		}(i, url)
	}
	wg.Wait()

	// Concatenamos preservando el orden de feedURLs: la UI agrupa por
	// `source` con un Map (que mantiene insertion order en JS), así el
	// orden visual coincide con el de configuración.
	out := make([]models.NewsItem, 0, len(p.feedURLs)*p.limitPerSource)
	failed := 0
	for i, r := range results {
		if r.err != nil {
			slog.Warn("rss: feed falló", "url", p.feedURLs[i], "err", r.err)
			failed++
			continue
		}
		out = append(out, r.items...)
	}

	// Si TODOS fallaron, propagar error: el cache stale del NewsService
	// puede hacerse cargo. Si solo algunos fallaron, devolvemos lo que tenemos.
	if failed == len(p.feedURLs) {
		return nil, fmt.Errorf("todos los feeds fallaron (%d/%d)", failed, len(p.feedURLs))
	}
	return out, nil
}

// fetchOne parsea un feed, ordena por fecha desc y devuelve los top `limit`.
func fetchOne(ctx context.Context, url string, limit int) ([]models.NewsItem, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, err
	}

	parsed := make([]rssItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		ri := rssItem{
			item: models.NewsItem{
				Title:  item.Title,
				Link:   item.Link,
				Source: feed.Title,
			},
		}
		// PublishedParsed puede ser nil si el feed no trae fecha parseable.
		// En ese caso conservamos el string crudo como fallback de display.
		if item.PublishedParsed != nil {
			ri.pubTime = *item.PublishedParsed
			ri.item.PublishedAt = item.PublishedParsed.UTC().Format("2006-01-02 15:04")
		} else if item.Published != "" {
			ri.item.PublishedAt = item.Published
		}
		parsed = append(parsed, ri)
	}

	// Sort por fecha desc dentro del feed; items sin fecha caen al final.
	sort.SliceStable(parsed, func(i, j int) bool {
		ti, tj := parsed[i].pubTime, parsed[j].pubTime
		if ti.IsZero() && !tj.IsZero() {
			return false
		}
		if !ti.IsZero() && tj.IsZero() {
			return true
		}
		return ti.After(tj)
	})

	if len(parsed) > limit {
		parsed = parsed[:limit]
	}

	out := make([]models.NewsItem, len(parsed))
	for i, ri := range parsed {
		out[i] = ri.item
	}
	return out, nil
}
