import type { AttributionState } from './enums/attribution-state'

export interface BandContribution {
  band_hz: number
  predicted_db: number
  energy_share: number
}

export interface SourceContribution {
  source_profile_id: number
  source_code: string
  source_name: string
  coefficient: number
  contribution_pct: number
  overall_db: number
  bands: BandContribution[]
}

export interface AttributionEvidence {
  matrix_rows: number
  matrix_columns: number
  iterations: number
  converged: boolean
  condition_hint: number
  unreliable_bands: string[]
  warnings: string[]
  objective: number
  elapsed_millis: number
}

export interface AttributionRun {
  id: number
  run_code: string
  measurement_ids: number[]
  source_profile_ids: number[]
  algorithm_version: string
  input_hash: string
  input_snapshot: unknown
  normalized_bands: unknown
  contributions: SourceContribution[]
  evidence: AttributionEvidence
  residual_error: number
  attribution_state: AttributionState
  explanation: string
  started_at: string
  finished_at: string | null
  created_by: number
  reviewed_by: number | null
  review_note: string
  version: number
  created_at: string
  updated_at: string
}

export interface CreateAttributionRun {
  measurement_ids: number[]
  source_profile_ids: number[]
}

export interface AttributionComparisonSource {
  source_profile_id: number
  source_code: string
  source_name: string
  in_base: boolean
  in_other: boolean
  base_pct: number
  other_pct: number
  delta_pct: number
  base_overall_db: number
  other_overall_db: number
}

export interface AttributionComparisonBand {
  band_hz: number
  base_observed_db: number
  other_observed_db: number
  observed_delta_db: number
  base_predicted_db: number
  other_predicted_db: number
  predicted_delta_db: number
}

export interface AttributionComparison {
  base_run_id: number
  other_run_id: number
  base_run_code: string
  other_run_code: string
  base_algorithm_version: string
  other_algorithm_version: string
  base_measurement_ids: number[]
  other_measurement_ids: number[]
  base_point_ids: number[]
  other_point_ids: number[]
  comparable: boolean
  incomparability_reasons: string[]
  residual_delta: number
  top_source_changed: boolean
  base_top_source: string
  other_top_source: string
  source_deltas: AttributionComparisonSource[]
  band_deltas: AttributionComparisonBand[]
  top_band_deltas: AttributionComparisonBand[]
  explanation: string
}
