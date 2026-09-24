import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import RunComparisonPanel from './RunComparisonPanel.vue'
import type { AttributionComparison, AttributionRun } from '../../types/attribution-run'

const run = (id: number, code: string): AttributionRun => ({
  id, run_code: code, measurement_ids: [1], source_profile_ids: [1, 2],
  algorithm_version: 'octave-nnls-v1.0.0', input_hash: 'abc', input_snapshot: {},
  normalized_bands: {}, contributions: [], evidence: {
    matrix_rows: 8, matrix_columns: 2, iterations: 1, converged: true, condition_hint: 0,
    unreliable_bands: [], warnings: [], objective: 0, elapsed_millis: 1,
  },
  residual_error: 0.1, attribution_state: 'completed', explanation: '',
  started_at: '2026-09-01T00:00:00Z', finished_at: '2026-09-01T00:01:00Z',
  created_by: 1, reviewed_by: null, review_note: '', version: 1,
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z',
})

const comparison = (overrides: Partial<AttributionComparison> = {}): AttributionComparison => ({
  base_run_id: 2, other_run_id: 1, base_run_code: 'AR-NEW', other_run_code: 'AR-OLD',
  base_algorithm_version: 'octave-nnls-v1.0.0', other_algorithm_version: 'octave-nnls-v1.0.0',
  base_measurement_ids: [1], other_measurement_ids: [1], base_point_ids: [1], other_point_ids: [1],
  comparable: true, incomparability_reasons: [], residual_delta: -0.05, top_source_changed: true,
  base_top_source: 'SRC-A', other_top_source: 'SRC-B',
  source_deltas: [
    { source_profile_id: 1, source_code: 'SRC-A', source_name: '压缩机', in_base: true, in_other: true, base_pct: 70, other_pct: 40, delta_pct: -30, base_overall_db: 80, other_overall_db: 76 },
    { source_profile_id: 2, source_code: 'SRC-B', source_name: '风机', in_base: false, in_other: true, base_pct: 0, other_pct: 60, delta_pct: 0, base_overall_db: 0, other_overall_db: 74 },
  ],
  band_deltas: [],
  top_band_deltas: [
    { band_hz: 250, base_observed_db: 80, other_observed_db: 85, observed_delta_db: 5, base_predicted_db: 79, other_predicted_db: 84, predicted_delta_db: 5 },
    { band_hz: 1000, base_observed_db: 78, other_observed_db: 76, observed_delta_db: -2, base_predicted_db: 77, other_predicted_db: 75, predicted_delta_db: -2 },
    { band_hz: 4000, base_observed_db: 70, other_observed_db: 71, observed_delta_db: 1, base_predicted_db: 70, other_predicted_db: 71, predicted_delta_db: 1 },
  ],
  explanation: '可直接比较',
  ...overrides,
})

const globalStubs = {
  global: {
    stubs: {
      'el-select': { template: '<select><slot /></select>' },
      'el-option': { template: '<option />' },
      'el-button': { template: '<button><slot /></button>' },
      'el-alert': { props: ['title'], template: '<div class="el-alert"><div class="el-alert-title">{{ title }}</div><slot /></div>' },
    },
  },
}

describe('RunComparisonPanel', () => {
  it('renders source percentage-point deltas and top three octave bands', () => {
    const wrapper = mount(RunComparisonPanel, {
      props: {
        currentRun: run(2, 'AR-NEW'), runs: [run(1, 'AR-OLD'), run(2, 'AR-NEW')],
        comparison: comparison(), loading: false,
      },
      ...globalStubs,
    })
    const text = wrapper.text()
    expect(text).toContain('压缩机')
    expect(text).toContain('-30.00')
    expect(text).toContain('仅历史')
    expect(text).toContain('250 Hz')
    expect(text).toContain('1000 Hz')
    expect(text).toContain('4000 Hz')
    expect(text).toContain('AR-NEW')
  })

  it('shows incomparability reasons while still listing itemized deltas', () => {
    const wrapper = mount(RunComparisonPanel, {
      props: {
        currentRun: run(2, 'AR-NEW'), runs: [run(1, 'AR-OLD'), run(2, 'AR-NEW')],
        comparison: comparison({
          comparable: false,
          incomparability_reasons: ['算法版本不同，贡献百分点不能直接比较。'],
        }),
        loading: false,
      },
      ...globalStubs,
    })
    const text = wrapper.text()
    expect(text).toContain('两次运行不能直接比较')
    expect(text).toContain('算法版本不同')
    expect(text).toContain('-30.00')
  })

  it('emits compare with the selected history run id', async () => {
    const wrapper = mount(RunComparisonPanel, {
      props: {
        currentRun: run(2, 'AR-NEW'), runs: [run(1, 'AR-OLD'), run(2, 'AR-NEW')],
        comparison: null, loading: false,
      },
      ...globalStubs,
    }) as any
    wrapper.vm.otherRunId = 1
    await wrapper.vm.$nextTick()
    wrapper.findAll('button').find((b: any) => b.text().includes('对比'))!.element.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('compare')?.[0]).toEqual([1])
  })
})
