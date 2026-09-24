import { defineStore } from 'pinia'
import { attributionRunApi } from '../api/attribution-run-api'
import { errorMessage } from '../api/client'
import type { AttributionComparison, AttributionRun, CreateAttributionRun } from '../types/attribution-run'

interface AttributionRunState {
  items: AttributionRun[]
  selected: AttributionRun | null
  comparison: AttributionComparison | null
  comparisonLoading: boolean
  comparisonError: string
  loading: boolean
  error: string
}

export const useAttributionRunStore = defineStore('attribution-runs', {
  state: (): AttributionRunState => ({
    items: [], selected: null, comparison: null, comparisonLoading: false, comparisonError: '', loading: false, error: '',
  }),
  actions: {
    async load() {
      this.loading = true; this.error = ''
      try { this.items = await attributionRunApi.list() } catch (error) { this.error = errorMessage(error) } finally { this.loading = false }
    },
    async select(id: number) {
      this.selected = await attributionRunApi.get(id)
      this.comparison = null; this.comparisonError = ''
      return this.selected
    },
    async compare(otherId: number) {
      if (!this.selected) return null
      this.comparisonLoading = true; this.comparisonError = ''
      try {
        this.comparison = await attributionRunApi.compare(this.selected.id, otherId)
        return this.comparison
      } catch (error) {
        this.comparison = null
        this.comparisonError = errorMessage(error)
        throw error
      } finally {
        this.comparisonLoading = false
      }
    },
    clearComparison() { this.comparison = null; this.comparisonError = '' },
    async create(value: CreateAttributionRun) {
      const key = crypto.randomUUID(); const created = await attributionRunApi.create(value, key); await this.load(); this.selected = created; return created
    },
    async review(item: AttributionRun, note: string) { const updated = await attributionRunApi.review(item.id, item.version, note); await this.load(); this.selected = updated; return updated },
    async confirm(item: AttributionRun) { const updated = await attributionRunApi.confirm(item.id, item.version); await this.load(); this.selected = updated; return updated },
    async void(item: AttributionRun) { const updated = await attributionRunApi.void(item.id, item.version); await this.load(); this.selected = updated; return updated },
  },
})
