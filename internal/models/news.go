package models

type NewsItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
	PublishedAt string `json:"published_at"`
	Source      string `json:"source"`
}
