<script setup lang="ts">
import { GitCompareArrows, TriangleAlert } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import type { AttributionComparison, AttributionRun } from '../../types/attribution-run'
import { fixed, formatDate, signed } from '../../utils/format'

const props = defineProps<{
  currentRun: AttributionRun
  runs: AttributionRun[]
  comparison: AttributionComparison | null
  loading: boolean
}>()
const emit = defineEmits<{ compare: [otherId: number] }>()

const otherRunId = ref<number | null>(null)
const showAllBands = ref(false)
const otherRuns = computed(() => props.runs.filter((run) => run.id !== props.currentRun.id))

watch(() => props.currentRun.id, () => { otherRunId.value = null; showAllBands.value = false })

function runLabel(run: AttributionRun) {
  return `${run.run_code} · ${formatDate(run.finished_at)}`
}
</script>

<template>
  <section class="compare-panel">
    <div class="compare-heading">
      <div>
        <p class="eyebrow"><GitCompareArrows :size="12" /> RUN COMPARISON</p>
        <h3>与历史运行对比</h3>
        <p>差值统一为「历史运行 − 当前运行」，贡献以百分点（pp）表示。</p>
      </div>
      <div class="compare-picker">
        <el-select v-model="otherRunId" placeholder="选择另一条历史运行" filterable :disabled="loading || !otherRuns.length">
          <el-option v-for="run in otherRuns" :key="run.id" :label="runLabel(run)" :value="run.id" />
        </el-select>
        <el-button type="primary" plain :loading="loading" :disabled="!otherRunId" @click="otherRunId && emit('compare', otherRunId)">对比</el-button>
      </div>
    </div>

    <p v-if="!otherRuns.length" class="compare-hint">当前只有一条归因运行，再冻结一次计算后即可对比声源与频带变化。</p>

    <template v-else-if="comparison">
      <el-alert
        :title="comparison.comparable ? '算法版本与监测点集合一致，结果可直接比较。' : '两次运行不能直接比较'"
        :type="comparison.comparable ? 'success' : 'warning'"
        :closable="false" show-icon
      >
        <ul v-if="!comparison.comparable" class="compare-reasons">
          <li v-for="reason in comparison.incomparability_reasons" :key="reason"><TriangleAlert :size="12" />{{ reason }}</li>
        </ul>
        <p class="compare-explain">{{ comparison.explanation }}</p>
      </el-alert>

      <div class="compare-meta">
        <div><span>当前运行</span><strong>{{ comparison.base_run_code }}</strong><small>{{ comparison.base_algorithm_version }}</small></div>
        <div><span>历史运行</span><strong>{{ comparison.other_run_code }}</strong><small>{{ comparison.other_algorithm_version }}</small></div>
        <div><span>残差变化</span><strong :class="comparison.residual_delta > 0 ? 'delta-up' : comparison.residual_delta < 0 ? 'delta-down' : ''">{{ signed(comparison.residual_delta, 4) }}</strong></div>
        <div>
          <span>首要来源</span>
          <strong v-if="!comparison.top_source_changed" class="delta-same">{{ comparison.base_top_source || '—' }}（未变）</strong>
          <strong v-else class="delta-up">{{ comparison.base_top_source || '—' }} → {{ comparison.other_top_source || '—' }}</strong>
        </div>
      </div>

      <h4 class="compare-subtitle">按声源的贡献百分点增减</h4>
      <div class="compare-table">
        <div class="compare-row compare-row-head"><span>候选声源</span><span>当前 %</span><span>历史 %</span><span>增减 (pp)</span></div>
        <div v-for="source in comparison.source_deltas" :key="source.source_profile_id" class="compare-row">
          <span class="compare-source"><strong>{{ source.source_name }}</strong><small>{{ source.source_code }}<em v-if="!source.in_base" class="tag tag-history">仅历史</em><em v-if="!source.in_other" class="tag tag-current">仅当前</em></small></span>
          <span>{{ source.in_base ? fixed(source.base_pct, 2) : '—' }}</span>
          <span>{{ source.in_other ? fixed(source.other_pct, 2) : '—' }}</span>
          <b :class="source.delta_pct > 0 ? 'delta-up' : source.delta_pct < 0 ? 'delta-down' : 'delta-flat'">{{ source.in_base && source.in_other ? signed(source.delta_pct, 2) : '—' }}</b>
        </div>
      </div>

      <h4 class="compare-subtitle">差异最大的三个倍频程</h4>
      <div class="band-delta-grid">
        <div v-for="band in comparison.top_band_deltas" :key="band.band_hz" class="band-delta-card">
          <div class="band-delta-head"><strong>{{ band.band_hz }} Hz</strong><span :class="Math.abs(band.observed_delta_db) >= Math.abs(band.predicted_delta_db) ? 'delta-up' : 'delta-down'">{{ signed(Math.abs(band.observed_delta_db) >= Math.abs(band.predicted_delta_db) ? band.observed_delta_db : band.predicted_delta_db, 2, ' dB') }}</span></div>
          <dl>
            <div><dt>实测（归一化）</dt><dd>{{ fixed(band.base_observed_db, 2) }} → {{ fixed(band.other_observed_db, 2) }} <b :class="band.observed_delta_db > 0 ? 'delta-up' : band.observed_delta_db < 0 ? 'delta-down' : 'delta-flat'">{{ signed(band.observed_delta_db, 2) }}</b></dd></div>
            <div><dt>模型预测</dt><dd>{{ fixed(band.base_predicted_db, 2) }} → {{ fixed(band.other_predicted_db, 2) }} <b :class="band.predicted_delta_db > 0 ? 'delta-up' : band.predicted_delta_db < 0 ? 'delta-down' : 'delta-flat'">{{ signed(band.predicted_delta_db, 2) }}</b></dd></div>
          </dl>
        </div>
      </div>

      <button class="band-toggle" type="button" @click="showAllBands = !showAllBands">
        {{ showAllBands ? '收起全部八个频带' : '查看全部八个频带差异' }}
      </button>
      <div v-if="showAllBands" class="compare-table band-table">
        <div class="compare-row compare-row-head"><span>频带 Hz</span><span>当前实测</span><span>历史实测</span><span>实测 Δ</span><span>当前预测</span><span>历史预测</span><span>预测 Δ</span></div>
        <div v-for="band in comparison.band_deltas" :key="band.band_hz" class="compare-row">
          <span>{{ band.band_hz }}</span>
          <span>{{ fixed(band.base_observed_db, 2) }}</span>
          <span>{{ fixed(band.other_observed_db, 2) }}</span>
          <b :class="band.observed_delta_db > 0 ? 'delta-up' : band.observed_delta_db < 0 ? 'delta-down' : 'delta-flat'">{{ signed(band.observed_delta_db, 2) }}</b>
          <span>{{ fixed(band.base_predicted_db, 2) }}</span>
          <span>{{ fixed(band.other_predicted_db, 2) }}</span>
          <b :class="band.predicted_delta_db > 0 ? 'delta-up' : band.predicted_delta_db < 0 ? 'delta-down' : 'delta-flat'">{{ signed(band.predicted_delta_db, 2) }}</b>
        </div>
      </div>
    </template>
  </section>
</template>
