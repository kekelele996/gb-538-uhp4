import { defineStore } from 'pinia'
import { attributionRunApi } from '../api/attribution-run-api'
import { errorMessage } from '../api/client'
import type { AttributionComparison, AttributionRun, CreateAttributionRun } from '../types/attribution-run'

export const useAttributionRunStore = defineStore('attribution-runs', {
  state: () => ({
    items: [] as AttributionRun[],
    selected: null as AttributionRun | null,
    loading: false,
    error: '',
    comparison: null as AttributionComparison | null,
    comparisonOtherId: null as number | null,
    comparisonLoading: false,
    comparisonError: '',
  }),
  actions: {
    async load() {
      this.loading = true; this.error = ''
      try { this.items = await attributionRunApi.list() } catch (error) { this.error = errorMessage(error) } finally { this.loading = false }
    },
    async select(id: number) {
      this.selected = await attributionRunApi.get(id)
      this.clearComparison()
      return this.selected
    },
    async compare(otherId: number) {
      if (!this.selected) return null
      this.comparisonLoading = true; this.comparisonError = ''; this.comparisonOtherId = otherId
      try {
        this.comparison = await attributionRunApi.compare(this.selected.id, otherId)
      } catch (error) {
        this.comparison = null
        this.comparisonError = errorMessage(error)
      } finally {
        this.comparisonLoading = false
      }
      return this.comparison
    },
    clearComparison() {
      this.comparison = null; this.comparisonOtherId = null; this.comparisonError = ''; this.comparisonLoading = false
    },
    async create(value: CreateAttributionRun) {
      const key = crypto.randomUUID(); const created = await attributionRunApi.create(value, key); await this.load(); this.selected = created; this.clearComparison(); return created
    },
    async review(item: AttributionRun, note: string) { const updated = await attributionRunApi.review(item.id, item.version, note); await this.load(); this.selected = updated; return updated },
    async confirm(item: AttributionRun) { const updated = await attributionRunApi.confirm(item.id, item.version); await this.load(); this.selected = updated; return updated },
    async void(item: AttributionRun) { const updated = await attributionRunApi.void(item.id, item.version); await this.load(); this.selected = updated; return updated },
  },
})
