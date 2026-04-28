import { request } from './client'
import type { NewsItem } from './types'

export function getNews(): Promise<NewsItem[]> {
  return request<NewsItem[]>('/api/news')
}
