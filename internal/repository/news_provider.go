package repository

import (
	"context"

	"security-portal/internal/models"

	"github.com/mmcdole/gofeed"
)

type NewsProvider interface {
	FetchNews(ctx context.Context) ([]models.NewsItem, error)
}

type rssNewsProvider struct {
	feedURL string
	limit   int
}

func NewRSSNewsProvider(url string, limit int) NewsProvider {
	if limit <= 0 {
		limit = 5
	}
	return &rssNewsProvider{feedURL: url, limit: limit}
}

func (p *rssNewsProvider) FetchNews(ctx context.Context) ([]models.NewsItem, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(p.feedURL, ctx)
	if err != nil {
		return nil, err
	}

	n := len(feed.Items)
	if n > p.limit {
		n = p.limit
	}

	news := make([]models.NewsItem, 0, n)
	for i := 0; i < n; i++ {
		item := feed.Items[i]

		// PublishedParsed puede ser nil si el feed no trae fecha parseable.
		published := ""
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.UTC().Format("2006-01-02 15:04")
		} else if item.Published != "" {
			published = item.Published
		}

		news = append(news, models.NewsItem{
			Title:       item.Title,
			Description: item.Description,
			Link:        item.Link,
			PublishedAt: published,
			Source:      feed.Title,
		})
	}
	return news, nil
}
