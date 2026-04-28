package services

import (
	"context"
	"sync"
	"time"

	"security-portal/internal/models"
	"security-portal/internal/news"
)

type NewsService struct {
	provider   news.Provider
	cache      []models.NewsItem
	lastUpdate time.Time
	mu         sync.RWMutex
	cacheTTL   time.Duration
}

func NewNewsService(provider news.Provider, ttl time.Duration) *NewsService {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &NewsService{provider: provider, cacheTTL: ttl}
}

func (s *NewsService) GetNews(ctx context.Context) ([]models.NewsItem, error) {
	s.mu.RLock()
	if time.Since(s.lastUpdate) < s.cacheTTL && len(s.cache) > 0 {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Doble validación por si otra goroutine ya refrescó mientras esperábamos.
	if time.Since(s.lastUpdate) < s.cacheTTL && len(s.cache) > 0 {
		return s.cache, nil
	}

	items, err := s.provider.FetchNews(ctx)
	if err != nil {
		if len(s.cache) > 0 {
			return s.cache, nil
		}
		return nil, err
	}

	s.cache = items
	s.lastUpdate = time.Now()
	return s.cache, nil
}
