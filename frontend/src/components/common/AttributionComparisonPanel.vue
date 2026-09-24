<script setup lang="ts">
import { CircleAlert, GitCompareArrows, TrendingDown, TrendingUp } from '@lucide/vue'
import { computed } from 'vue'
import type { AttributionComparison, AttributionRun } from '../../types/attribution-run'
import { fixed, formatDate, signedFixed } from '../../utils/format'

const props = defineProps<{
  base: AttributionRun
  items: AttributionRun[]
  otherRunId: number | null
  comparison: AttributionComparison | null
  loading: boolean
  error: string
}>()

const emit = defineEmits<{
  select: [id: number]
  clear: []
}>()

const otherOptions = computed(() => props.items.filter((item) => item.id !== props.base.id))

function deltaClass(value: number) {
  if (value > 0) return 'delta-up'
  if (value < 0) return 'delta-down'
  return 'delta-flat'
}

function presenceLabel(inBase: boolean, inOther: boolean) {
  if (inBase && inOther) return ''
  return inBase ? '仅当前' : '仅历史'
}
</script>

<template>
  <section class="compare-panel" aria-label="历史归因对比">
    <div class="compare-toolbar">
      <div class="compare-title">
        <span class="compare-icon"><GitCompareArrows :size="16" /></span>
        <div>
          <h3>与历史运行对比</h3>
          <p>当前运行 {{ base.run_code }} · 选择另一条冻结记录，查看声源贡献百分点与频带级差异。</p>
        </div>
      </div>
      <el-select
        :model-value="otherRunId ?? undefined"
        placeholder="选择历史运行"
        clearable
        style="width: min(320px, 100%)"
        @update:model-value="(value: number | undefined) => value ? emit('select', value) : emit('clear')"
      >
        <el-option v-for="item in otherOptions" :key="item.id" :value="item.id"
          :label="`${item.run_code} · ${formatDate(item.finished_at)}`">
          <span class="compare-option"><strong>{{ item.run_code }}</strong><small>{{ formatDate(item.finished_at) }} · {{ item.algorithm_version }}</small></span>
        </el-option>
      </el-select>
    </div>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-skeleton v-if="loading" :rows="5" animated />
    <p v-else-if="!comparison" class="compare-empty">从下拉中选择一条历史运行；差异按「历史 − 当前」计算，不会覆盖任何冻结结果。</p>

    <template v-else-if="comparison">
      <el-alert
        v-for="warning in comparison.comparability_warnings"
        :key="warning"
        :title="warning"
        type="warning"
        :closable="false"
        show-icon
        class="compare-warning"
      >
        <template #title>
          <span class="warning-title"><CircleAlert :size="14" /> 结果不能直接比较 —— 但下面仍保留逐项差异</span>
          <span class="warning-body">{{ warning }}</span>
        </template>
      </el-alert>
      <div v-if="comparison.comparable" class="compare-ok">
        两次运行算法版本与冻结监测点集合一致（{{ comparison.base_run.monitoring_points.join('、') || '—' }}），百分点差异可直接比较。
      </div>

      <div class="compare-runs">
        <div class="compare-run-card">
          <span>当前运行</span>
          <strong>{{ comparison.base_run.run_code }}</strong>
          <small>{{ comparison.base_run.algorithm_version }}</small>
          <small>{{ comparison.base_run.measurement_count }} 次测量 · {{ comparison.base_run.source_count }} 个候选声源</small>
          <small>监测点：{{ comparison.base_run.monitoring_points.join('、') || '—' }}</small>
        </div>
        <div class="compare-run-card">
          <span>历史运行</span>
          <strong>{{ comparison.other_run.run_code }}</strong>
          <small>{{ comparison.other_run.algorithm_version }}</small>
          <small>{{ comparison.other_run.measurement_count }} 次测量 · {{ comparison.other_run.source_count }} 个候选声源</small>
          <small>监测点：{{ comparison.other_run.monitoring_points.join('、') || '—' }}</small>
        </div>
      </div>

      <div class="compare-metrics">
        <div>
          <span>残差变化（历史 − 当前）</span>
          <strong :class="deltaClass(comparison.residual_delta)">{{ signedFixed(comparison.residual_delta, 4) }}</strong>
        </div>
        <div>
          <span>首要来源</span>
          <strong class="source-flow">
            <b>{{ comparison.base_top_source || '—' }}</b>
            <TrendingUp v-if="comparison.top_source_changed" :size="14" class="flow-icon" />
            <b>{{ comparison.other_top_source || '—' }}</b>
          </strong>
          <small v-if="comparison.top_source_changed">首要来源已切换</small>
          <small v-else>首要来源未变化</small>
        </div>
      </div>

      <div class="compare-tables">
        <div class="compare-table-wrap">
          <h4>按声源贡献百分点增减</h4>
          <div class="delta-table">
            <div class="delta-head"><span>声源</span><span>当前</span><span>历史</span><span>变化</span></div>
            <div v-for="delta in comparison.source_deltas" :key="delta.source_profile_id" class="delta-row">
              <span class="delta-source">
                <strong>{{ delta.source_name }}</strong>
                <small>{{ delta.source_code }}</small>
                <em v-if="presenceLabel(delta.in_base, delta.in_other)" class="presence-tag">{{ presenceLabel(delta.in_base, delta.in_other) }}</em>
              </span>
              <span>{{ delta.in_base ? fixed(delta.base_contribution_pct, 2) + '%' : '—' }}</span>
              <span>{{ delta.in_other ? fixed(delta.other_contribution_pct, 2) + '%' : '—' }}</span>
              <b :class="deltaClass(delta.delta_pct)">{{ signedFixed(delta.delta_pct, 2, ' pp') }}</b>
            </div>
          </div>
        </div>

        <div class="compare-table-wrap">
          <h4>差异最大的三个倍频程</h4>
          <div class="band-delta-list">
            <div v-for="band in comparison.top_band_deltas" :key="band.band_hz" class="band-delta-card">
              <span class="band-frequency">{{ band.band_hz >= 1000 ? `${band.band_hz / 1000} kHz` : `${band.band_hz} Hz` }}</span>
              <span class="band-levels">{{ fixed(band.base_level_db, 1) }} → {{ fixed(band.other_level_db, 1) }} dB</span>
              <b :class="deltaClass(band.delta_db)">
                <TrendingUp v-if="band.delta_db > 0" :size="13" />
                <TrendingDown v-else-if="band.delta_db < 0" :size="13" />
                {{ signedFixed(band.delta_db, 2, ' dB') }}
              </b>
            </div>
            <p v-if="!comparison.top_band_deltas.length" class="compare-empty">两次运行均无可用频带预测。</p>
          </div>
        </div>
      </div>

      <p class="compare-footnote">{{ comparison.explanation }}</p>
    </template>
  </section>
</template>

<style scoped>
.compare-panel { margin-top: 22px; padding-top: 18px; border-top: 2px solid var(--ink); }
.compare-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.compare-title { display: flex; align-items: center; gap: 11px; }
.compare-title h3 { margin: 0 0 3px; font-size: .82rem; }
.compare-title p { margin: 0; color: var(--muted); font-size: .64rem; }
.compare-icon { width: 34px; height: 34px; flex: 0 0 34px; display: grid; place-items: center; color: var(--blue); background: var(--blue-soft); border: 1px solid #9bbac8; }
.compare-option { display: grid; gap: 2px; }
.compare-option small { color: var(--muted); font-size: .6rem; }
.compare-warning { margin-top: 12px; }
.compare-warning :deep(.el-alert__content) { display: grid; gap: 3px; }
.warning-title { display: inline-flex; align-items: center; gap: 5px; font-size: .72rem; font-weight: 800; }
.warning-body { display: block; font-size: .68rem; line-height: 1.5; }
.compare-ok { margin-top: 12px; padding: 9px 12px; color: var(--green-dark); background: var(--green-soft); border-left: 3px solid var(--green); font-size: .68rem; }
.compare-empty { margin: 14px 0 0; color: var(--muted); font-size: .68rem; }
.compare-runs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }
.compare-run-card { display: grid; gap: 4px; min-width: 0; padding: 12px 14px; background: #f0f4f1; border: 1px solid var(--line); border-top: 3px solid var(--green); }
.compare-run-card:last-child { border-top-color: var(--blue); }
.compare-run-card span { color: var(--muted); font-size: .58rem; text-transform: uppercase; letter-spacing: .04em; }
.compare-run-card strong { font-size: .82rem; overflow-wrap: anywhere; }
.compare-run-card small { color: var(--muted); font-size: .62rem; overflow-wrap: anywhere; }
.compare-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
.compare-metrics > div { display: grid; gap: 5px; padding: 12px 14px; border: 1px solid var(--line); background: var(--paper); }
.compare-metrics span { color: var(--muted); font-size: .6rem; }
.compare-metrics strong { font-size: 1.05rem; font-variant-numeric: tabular-nums; }
.source-flow { display: flex; align-items: center; gap: 8px; font-size: .78rem !important; color: var(--blue); }
.flow-icon { color: var(--amber); }
.delta-up { color: var(--blue) !important; }
.delta-down { color: #8a5a17 !important; }
.delta-flat { color: var(--muted) !important; }
.compare-tables { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(260px, .75fr); gap: 18px; margin-top: 16px; }
.compare-table-wrap h4 { margin: 0 0 9px; padding-bottom: 8px; font-size: .72rem; border-bottom: 1px solid var(--line-strong); }
.delta-table { border-top: 1px solid var(--line-strong); }
.delta-head, .delta-row { display: grid; grid-template-columns: minmax(0, 1fr) 78px 78px 96px; align-items: center; gap: 8px; padding: 8px 10px; border-bottom: 1px solid var(--line); font-size: .68rem; }
.delta-head { color: var(--muted); background: #eef3ef; font-size: .59rem; font-weight: 780; text-transform: uppercase; }
.delta-row > span:not(.delta-source), .delta-row b { text-align: right; font-variant-numeric: tabular-nums; }
.delta-source { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 2px 7px; align-items: center; }
.delta-source strong { overflow: hidden; font-size: .7rem; text-overflow: ellipsis; white-space: nowrap; }
.delta-source small { grid-column: 1; color: var(--muted); font-size: .59rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.presence-tag { grid-row: 1 / 3; grid-column: 2; padding: 1px 6px; color: var(--amber); background: var(--amber-soft); border: 1px solid #d2b679; border-radius: 3px; font-size: .56rem; font-style: normal; white-space: nowrap; }
.band-delta-list { display: grid; gap: 8px; }
.band-delta-card { display: grid; grid-template-columns: 64px minmax(0, 1fr) auto; align-items: center; gap: 8px; padding: 10px 12px; background: #f0f4f1; border: 1px solid var(--line); border-left: 3px solid var(--blue); }
.band-frequency { font-weight: 800; font-size: .72rem; color: var(--blue); }
.band-levels { color: var(--muted); font-size: .62rem; font-variant-numeric: tabular-nums; }
.band-delta-card b { display: inline-flex; align-items: center; gap: 4px; font-size: .74rem; font-variant-numeric: tabular-nums; }
.compare-footnote { margin: 14px 0 0; color: var(--muted); font-size: .62rem; line-height: 1.5; }
@media (max-width: 1120px) {
  .compare-tables { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .compare-runs, .compare-metrics { grid-template-columns: 1fr; }
  .delta-head, .delta-row { grid-template-columns: minmax(0, 1fr) 64px 64px 84px; padding-inline: 4px; }
}
</style>
