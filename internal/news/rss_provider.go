package news

import (
	"context"

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
	feedURL string
	limit   int
}

func NewRSSProvider(url string, limit int) Provider {
	if limit <= 0 {
		limit = 5
	}
	return &rssProvider{feedURL: url, limit: limit}
}

func (p *rssProvider) FetchNews(ctx context.Context) ([]models.NewsItem, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(p.feedURL, ctx)
	if err != nil {
		return nil, err
	}

	n := len(feed.Items)
	if n > p.limit {
		n = p.limit
	}

	out := make([]models.NewsItem, 0, n)
	for i := 0; i < n; i++ {
		item := feed.Items[i]

		// PublishedParsed puede ser nil si el feed no trae fecha parseable.
		published := ""
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.UTC().Format("2006-01-02 15:04")
		} else if item.Published != "" {
			published = item.Published
		}

		out = append(out, models.NewsItem{
			Title:       item.Title,
			Description: item.Description,
			Link:        item.Link,
			PublishedAt: published,
			Source:      feed.Title,
		})
	}
	return out, nil
}
