import { request } from './client'
import type { NewsItem } from './types'

export function getNews(init?: { signal?: AbortSignal }): Promise<NewsItem[]> {
  return request<NewsItem[]>('/api/news', init)
}
