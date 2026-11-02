import { api } from './api'
import type { SiteInfo } from './types'

export type { SiteInfo }

let pending: Promise<SiteInfo> | null = null

export function loadSite(): Promise<SiteInfo> {
  if (!pending) {
    pending = api.get<SiteInfo>('/site').catch((error) => {
      pending = null
      throw error
    })
  }
  return pending
}
