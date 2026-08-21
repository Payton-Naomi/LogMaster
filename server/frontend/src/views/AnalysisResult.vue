<template>
  <div class="page analysis-page" v-loading="loading">
    <header class="page-heading">
      <div class="title"><el-button text circle :icon="ArrowLeft" title="返回任务详情" @click="router.push(`/task/${taskId}`)" /><div><h1>解析结果</h1><p>{{ task.original_name || taskId }}</p></div></div>
      <div class="heading-actions"><el-tag v-if="task.scenario_name" effect="plain">{{ task.scenario_name }}</el-tag><el-button :icon="Download" :disabled="!results.length" @click="exportResults">导出结果</el-button></div>
    </header>
    <el-alert v-if="loadError" class="page-error" :title="loadError" type="error" show-icon :closable="false" />
    <el-alert v-if="agentError" class="page-error" :title="agentError" type="warning" show-icon :closable="false" />

    <section v-if="false && task.task_id" class="summary">
      <div><span>日志总行数</span><strong>{{ Number(task.total_lines || 0).toLocaleString() }}</strong></div>
      <div><span>错误</span><strong class="error">{{ task.error_count || 0 }}</strong></div>
      <div><span>警告</span><strong class="warning">{{ task.warning_count || 0 }}</strong></div>
      <div><span>关联原因</span><strong>{{ relatedCauseCount }}</strong></div>
    </section>

    <section v-if="false" class="panel timeline-panel">
      <div class="panel-heading"><div><h2>严重异常时间线</h2><p>密集事件会自动聚合，圆内数字表示该时间段的异常数量</p></div><span>{{ timelineResults.length }} 个异常 · {{ timelineGroups.length }} 个事件组</span></div>
      <div v-if="timelineResults.length" ref="timelineRef" class="timeline-chart" />
      <el-empty v-else description="暂无严重异常时间点" :image-size="70" />
      <div v-if="clusterEvents.length > 1" class="cluster-detail">
        <div class="cluster-heading"><div><strong>{{ clusterTitle }}</strong><span>共 {{ clusterEvents.length }} 条，点击具体事件查看上下文</span></div><el-button text circle :icon="Close" title="收起事件组" @click="clusterEvents = []" /></div>
        <div class="cluster-list">
          <button v-for="(item, index) in clusterEvents" :key="`${item.file_path}-${item.line_number}-${item.rule_name}-${index}`" type="button" @click="openResult(item)">
            <span class="event-time">{{ shortTime(item.event_time) }}</span>
            <strong>{{ item.rule_name || item.matched_text }}</strong>
            <span class="event-line">第 {{ item.line_number }} 行</span>
            <p>{{ item.content }}</p>
            <ArrowRight />
          </button>
        </div>
      </div>
    </section>

    <section v-if="agentReady || agentLoading" class="panel ai-summary-panel">
      <div class="panel-heading"><div><h2>AI 总结</h2><p>基于日志命中行及前后文生成的分析结论</p></div><span v-if="agentLoading">AI 正在分析，请稍候...</span><span v-else-if="agentResults.length">{{ agentResults.length }} 个文件 · {{ agentStatusText }}</span><span v-else>暂无 AI 结果</span></div>
      <el-alert v-if="agentLoading" title="AI 正在分析，通常需要几秒到几十秒，请耐心等待，页面会自动更新。" type="info" :closable="false" show-icon />
      <el-alert v-else-if="!agentResults.length" title="AI 分析可能仍在后台处理中，请耐心等待后刷新页面；关键字分析结果不受影响。" type="info" :closable="false" show-icon />
      <div v-for="item in agentResults" :key="`${item.log_file_id || item.file_path}-${item.updated_at || item.created_at || item.status}`" class="ai-file-result">
        <div class="ai-file-heading"><strong>{{ item.file_path || '当前日志文件' }}</strong><div><el-button text size="small" :icon="CopyDocument" @click="copyText(item.summary || item.error_message || '')">复制</el-button><el-tag :type="item.status === 'completed' ? 'success' : 'danger'" effect="plain">{{ item.status === 'completed' ? '分析完成' : '分析失败' }}</el-tag></div></div>
        <p v-if="item.status === 'completed' && item.summary" class="ai-summary-copy">{{ item.summary }}</p>
        <p v-else-if="item.status === 'failed'" class="ai-summary-error">{{ item.error_message || 'AI 分析失败，请稍后重试' }}</p>
        <div v-if="item.findings?.length" class="ai-findings">
          <article v-for="(finding, index) in item.findings" :key="`${finding.line_number || index}-${finding.category || 'finding'}`" class="ai-finding">
            <div class="ai-finding-heading"><strong>{{ finding.root_cause || finding.category || '诊断结论' }}</strong><div><el-button text size="small" :icon="CopyDocument" @click="copyText(formatFinding(finding))">复制</el-button><el-tag v-if="finding.confidence != null" size="small" effect="plain">置信度 {{ Math.round(finding.confidence * 100) }}%</el-tag></div></div>
            <dl><div v-if="finding.evidence"><dt>证据</dt><dd>{{ finding.evidence }}</dd></div><div v-if="finding.impact"><dt>影响</dt><dd>{{ finding.impact }}</dd></div><div v-if="finding.suggestion"><dt>建议</dt><dd>{{ finding.suggestion }}</dd></div></dl>
          </article>
        </div>
      </div>
    </section>

    <section class="panel result-panel">
      <div class="filters">
        <el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索规则、文件或日志内容" />
        <el-select v-model="level" clearable placeholder="全部级别"><el-option label="错误" value="error" /><el-option label="警告" value="warning" /><el-option label="信息" value="info" /></el-select>
        <el-select v-model="category" clearable placeholder="全部场景"><el-option v-for="item in categories" :key="item" :label="item" :value="item" /></el-select><el-select v-model="groupBy" placeholder="分组方式"><el-option label="按时间排序" value="none" /><el-option label="按分类分组" value="category" /><el-option label="按级别分组" value="level" /></el-select>
        <span>共 {{ filtered.length }} 条</span>
      </div>
      <el-table :data="paged" @row-click="openResult">
        <el-table-column prop="level" label="级别" width="80"><template #default="scope"><el-tag :type="levelType(scope.row.level)" effect="plain">{{ levelLabel(scope.row.level) }}</el-tag></template></el-table-column>
        <el-table-column prop="event_time" label="事件时间" width="170"><template #default="scope">{{ formatTime(scope.row.event_time) }}</template></el-table-column>
        <el-table-column prop="rule_name" label="解析规则" min-width="150" show-overflow-tooltip />
        <el-table-column prop="category" label="分类" width="110" />
        <el-table-column prop="file_path" label="文件" min-width="190" show-overflow-tooltip />
        <el-table-column prop="line_number" label="行号" width="75" />
        <el-table-column prop="content" label="日志内容" min-width="320" show-overflow-tooltip />
        <el-table-column label="操作" width="210"><template #default="scope"><el-button type="primary" link @click.stop="copyResult(scope.row)">复制</el-button><el-button type="primary" link @click.stop="openResult(scope.row)">看上下文</el-button><el-button v-if="findAgentFinding(scope.row)" type="primary" link @click.stop="openAgentFinding(scope.row)">AI 解读</el-button></template></el-table-column>
        <template #empty><el-empty description="数据库中暂无解析结果" /></template>
      </el-table>
      <footer><span>点击结果行查看关键字前后各 50 行日志和可能原因</span><el-pagination v-model:current-page="page" :page-size="pageSize" :total="filtered.length" layout="prev, pager, next" /></footer>
    </section>

    <el-drawer v-model="drawer" class="analysis-context-drawer" title="错误上下文" size="680px">
      <template v-if="selected">
        <div class="detail-head"><div><el-tag :type="levelType(selected.level)" effect="plain">{{ levelLabel(selected.level) }}</el-tag><strong>{{ selected.rule_name || selected.matched_text }}</strong></div><span>{{ formatTime(selected.event_time) }}</span></div>
        <dl class="detail-meta"><div><dt>文件</dt><dd>{{ selected.file_path }}</dd></div><div><dt>错误行</dt><dd>{{ selected.line_number }}</dd></div><div><dt>分类</dt><dd>{{ selected.category || '未分类' }}</dd></div><div><dt>窗口</dt><dd>{{ formatTime(selected.context_start_time) }} 至 {{ formatTime(selected.context_end_time) }}</dd></div></dl>

        <section class="drawer-section"><div class="section-title"><h3>可能原因</h3><span>{{ selected.related_causes?.length || 0 }} 项</span></div><el-alert v-if="!selected.related_causes?.length" title="当前时间窗口内未识别到其他关联原因" type="info" :closable="false" /><div v-for="cause in selected.related_causes" :key="`${cause.kind}-${cause.line_number}`" class="cause-row"><div><el-tag type="warning" effect="plain">{{ Math.round((cause.confidence || 0) * 100) }}%</el-tag><strong>{{ cause.label }}</strong><span>第 {{ cause.line_number }} 行 · {{ formatTime(cause.timestamp) }}</span></div><p>{{ cause.reason }}</p><code>{{ cause.content }}</code></div></section>

        <section class="drawer-section"><div class="section-title"><h3>关键字前后各 50 行</h3><span>{{ selected.context_lines?.length || 0 }} 行</span></div><div class="context-window"><div v-for="line in contextLines" :key="`${line.line_number}-${line.content}`" :class="['context-line', { hit: line.is_hit, cause: isCauseLine(line) }]" class="context-line"><span class="line-number">{{ line.line_number }}</span><span class="line-time">{{ shortTime(line.timestamp) }}</span><span class="line-content">{{ line.content }}</span></div><div v-if="!contextLines.length" class="fallback-line"><span class="line-number">{{ selected.line_number }}</span><span class="line-content">{{ selected.content }}</span></div></div></section>
      </template>
    </el-drawer>

    <el-dialog v-model="agentFindingDialog" class="agent-finding-dialog" title="AI 解读" width="560px">
      <template v-if="selectedAgentFinding">
        <div class="agent-finding-meta">
          <el-tag :type="levelType(selectedAgentFinding.severity)" effect="plain">{{ levelLabel(selectedAgentFinding.severity) }}</el-tag>
          <span v-if="selectedAgentFinding.file_path || selectedAgentFinding._filePath">{{ selectedAgentFinding.file_path || selectedAgentFinding._filePath }}</span>
          <span v-if="selectedAgentFinding.line_number">第 {{ selectedAgentFinding.line_number }} 行</span>
        </div>
        <dl class="agent-finding-detail">
          <div v-if="selectedAgentFinding.root_cause"><dt>可能原因</dt><dd>{{ selectedAgentFinding.root_cause }}</dd></div>
          <div v-if="selectedAgentFinding.evidence"><dt>证据</dt><dd>{{ selectedAgentFinding.evidence }}</dd></div>
          <div v-if="selectedAgentFinding.impact"><dt>影响</dt><dd>{{ selectedAgentFinding.impact }}</dd></div>
          <div v-if="selectedAgentFinding.suggestion"><dt>建议</dt><dd>{{ selectedAgentFinding.suggestion }}</dd></div>
        </dl>
        <p v-if="selectedAgentFinding.confidence != null" class="agent-confidence">置信度 {{ Math.round(selectedAgentFinding.confidence * 100) }}%</p>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, Close, CopyDocument, Download, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { getAgentResults, getTaskDetail, getTaskResults } from '@/api/task'

const route = useRoute()
const router = useRouter()
const taskId = route.params.taskId
const loading = ref(false)
const loadError = ref('')
const agentError = ref('')
const agentLoading = ref(false)
const agentReady = ref(false)
const task = ref({})
const results = ref([])
const agentResults = ref([])
const search = ref('')
const level = ref('')
const category = ref('')
const groupBy = ref('none')
const page = ref(1)
const pageSize = 20
const drawer = ref(false)
const selected = ref(null)
const selectedAgentFinding = ref(null)
const agentFindingDialog = ref(false)
const timelineRef = ref(null)
const clusterEvents = ref([])
const clusterTitle = ref('')
let chart = null

const categories = computed(() => [...new Set(results.value.map(item => item.category).filter(Boolean))])
const agentFindings = computed(() => agentResults.value.flatMap(item => (item.findings || []).map(finding => ({ ...finding, _filePath: finding.file_path || item.file_path || '' }))))
const relatedCauseCount = computed(() => results.value.reduce((sum, item) => sum + (item.related_causes?.length || 0), 0))
const timelineResults = computed(() => results.value.filter(item => item.level === 'error' && item.event_time))
const timelineGroups = computed(() => {
  const items = timelineResults.value
    .map(item => ({ item, time: Date.parse(item.event_time), category: item.category || item.rule_name || '未分类' }))
    .filter(entry => Number.isFinite(entry.time))
    .sort((left, right) => left.time - right.time)
  if (!items.length) return []
  const minimum = items[0].time
  const maximum = items[items.length - 1].time
  const bucketSize = Math.max(100, Math.ceil(Math.max(1, maximum - minimum) / 24))
  const groups = new Map()
  items.forEach(entry => {
    const bucket = Math.floor((entry.time - minimum) / bucketSize)
    const key = `${entry.category}\u0000${bucket}`
    if (!groups.has(key)) groups.set(key, { category: entry.category, bucket, totalTime: 0, items: [] })
    const group = groups.get(key)
    group.totalTime += entry.time
    group.items.push(entry.item)
  })
  return [...groups.values()]
    .map(group => ({ ...group, time: Math.round(group.totalTime / group.items.length) }))
    .sort((left, right) => left.time - right.time || left.category.localeCompare(right.category))
})
const contextLines = computed(() => selected.value?.context_lines || [])
const filtered = computed(() => {
  const text = search.value.trim().toLowerCase()
  const items = results.value.filter(item => (!level.value || item.level === level.value) && (!category.value || item.category === category.value) && (!text || `${item.rule_name}${item.matched_text}${item.file_path}${item.content}`.toLowerCase().includes(text)))
  return [...items].sort((a, b) => groupBy.value === 'category' ? String(a.category || '').localeCompare(String(b.category || '')) : groupBy.value === 'level' ? String(a.level || '').localeCompare(String(b.level || '')) : (Date.parse(b.event_time) || 0) - (Date.parse(a.event_time) || 0))
})
const paged = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))

const levelLabel = value => ({ critical: '严重', error: '错误', warning: '警告', info: '信息' }[value] || value || '未知')
const levelType = value => ({ critical: 'danger', error: 'danger', warning: 'warning', info: 'info' }[value] || 'info')
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const shortTime = value => value ? new Date(value).toLocaleTimeString('zh-CN', { hour12: false, fractionalSecondDigits: 3 }) : '--:--:--'
const isCauseLine = line => selected.value?.related_causes?.some(item => item.line_number === line.line_number)
const errorMessage = (error, fallback) => error?.response?.data?.message || error?.message || fallback
const agentStatusText = computed(() => { const failed = agentResults.value.filter(item => item.status === 'failed').length; return failed ? `${failed} 个失败` : '全部完成' })
const formatFinding = finding => [finding.root_cause && `原因：${finding.root_cause}`, finding.evidence && `证据：${finding.evidence}`, finding.impact && `影响：${finding.impact}`, finding.suggestion && `建议：${finding.suggestion}`].filter(Boolean).join('\n')
async function copyText(value) { if (!value) return ElMessage.info('暂无可复制内容'); try { await navigator.clipboard.writeText(String(value)); ElMessage.success('内容已复制') } catch { ElMessage.warning('复制失败，请检查浏览器剪贴板权限') } }
function copyResult(item) { copyText([item.event_time, levelLabel(item.level), item.file_path && `${item.file_path}:${item.line_number}`, item.rule_name || item.matched_text, item.content].filter(Boolean).join(' | ')) }
const normalizePath = value => String(value || '').replaceAll('\\', '/').replace(/^\.\//, '').toLowerCase()
function findAgentFinding(result) {
  if (!result || !agentFindings.value.length) return null
  const lineNumber = Number(result.line_number)
  if (!Number.isFinite(lineNumber)) return null
  const resultPath = normalizePath(result.file_path)
  return agentFindings.value.find(finding => Number(finding.line_number) === lineNumber
    && normalizePath(finding.file_path || finding._filePath) === resultPath)
    || (agentFindings.value.filter(finding => Number(finding.line_number) === lineNumber).length === 1
      ? agentFindings.value.find(finding => Number(finding.line_number) === lineNumber)
      : null)
    || null
}
function openAgentFinding(result) {
  const finding = findAgentFinding(result)
  if (!finding) return
  selectedAgentFinding.value = finding
  agentFindingDialog.value = true
}

async function loadAllResults() {
  const items = []
  let currentPage = 1
  const chunkSize = 200
  while (currentPage <= 100) {
    const chunk = await getTaskResults(taskId, { page: currentPage, page_size: chunkSize })
    if (!Array.isArray(chunk) || !chunk.length) break
    items.push(...chunk)
    if (chunk.length < chunkSize) break
    currentPage += 1
  }
  return items
}

async function load() {
  loading.value = true
  loadError.value = ''
  agentError.value = ''
  try {
    const [detailResult, parsedResult, agentsResult] = await Promise.allSettled([getTaskDetail(taskId), loadAllResults(), getAgentResults(taskId)])
    if (detailResult.status === 'rejected') throw detailResult.reason
    if (parsedResult.status === 'rejected') throw parsedResult.reason
    task.value = detailResult.value.task
    results.value = parsedResult.value
    if (agentsResult.status === 'fulfilled') {
      agentResults.value = agentsResult.value || []
      if (!agentResults.value.length && task.value.status === 'completed') {
        agentLoading.value = true
        for (let attempt = 0; attempt < 2 && !agentResults.value.length; attempt += 1) {
          await new Promise(resolve => window.setTimeout(resolve, attempt ? 5000 : 3000))
          try { agentResults.value = await getAgentResults(taskId) || [] } catch { /* AI results are optional */ }
        }
        agentLoading.value = false
      }
      agentReady.value = true
    } else {
      agentResults.value = []
      agentError.value = `Agent 诊断结果加载失败：${errorMessage(agentsResult.reason, '请检查 agent-results 接口')}`
      agentReady.value = true
    }
    await nextTick()
    drawTimeline()
  } catch (error) {
    loadError.value = errorMessage(error, '加载解析结果失败')
    task.value = {}
    results.value = []
    agentResults.value = []
    agentLoading.value = false
    agentReady.value = false
  } finally {
    loading.value = false
  }
}

function drawTimeline() {
  if (!timelineRef.value || !timelineResults.value.length) {
    chart?.clear()
    return
  }
  if (!chart) chart = echarts.init(timelineRef.value)
  const groups = timelineGroups.value
  const names = [...new Set(groups.map(item => item.category))]
  const times = groups.map(item => item.time)
  const minimum = Math.min(...times)
  const maximum = Math.max(...times)
  const padding = Math.max(500, Math.round((maximum - minimum) * 0.04))
  chart.setOption({
    animationDuration: 250,
    grid: { left: 26, right: 32, top: 22, bottom: 48, containLabel: true },
    tooltip: {
      trigger: 'item',
	  renderMode: 'richText',
      formatter: params => {
        const group = groups[params.data.groupIndex]
        const preview = group.items.slice(0, 4).map(item => `${shortTime(item.event_time)}  ${item.rule_name || item.matched_text}  第 ${item.line_number} 行`).join('\n')
		return `${group.category} · ${group.items.length} 个严重异常\n${preview}${group.items.length > 4 ? `\n还有 ${group.items.length - 4} 条，点击展开` : ''}`
      }
    },
    xAxis: { type: 'time', min: minimum - padding, max: maximum + padding, axisLabel: { color: '#7a8493', hideOverlap: true }, axisTick: { show: false }, axisLine: { lineStyle: { color: '#dfe4ea' } }, splitLine: { show: true, lineStyle: { color: '#f0f2f5' } } },
    yAxis: { type: 'category', data: names, axisLabel: { color: '#5f6c80', fontWeight: 600, margin: 18 }, axisTick: { show: false }, axisLine: { show: false }, splitLine: { show: true, lineStyle: { color: '#edf0f4' } } },
    dataZoom: [{ type: 'inside' }, { type: 'slider', height: 16, bottom: 8 }],
    series: [{
      type: 'scatter',
      symbolSize: value => Math.min(40, 16 + Math.sqrt(value[2]) * 6),
      label: { show: true, color: '#fff', fontSize: 11, fontWeight: 700, formatter: params => params.data.count > 1 ? String(params.data.count) : '' },
      emphasis: { scale: 1.12 },
      data: groups.map((group, index) => ({
        value: [group.time, group.category, group.items.length],
        count: group.items.length,
        groupIndex: index,
        itemStyle: { color: '#d94f4f', borderColor: '#fff', borderWidth: 2, shadowBlur: 7, shadowColor: 'rgba(190, 48, 48, .2)' }
      }))
    }]
  }, true)
  chart.off('click')
  chart.on('click', params => {
    const group = groups[params.data.groupIndex]
    if (group.items.length === 1) {
      clusterEvents.value = []
      openResult(group.items[0])
      return
    }
    clusterEvents.value = group.items
    clusterTitle.value = `${group.category} · ${formatTime(new Date(group.time).toISOString())}`
  })
}

function openResult(item) { selected.value = item; drawer.value = true }

function exportResults() {
  const header = 'event_time,level,rule_name,category,file_path,line_number,content,related_causes'
  const rows = results.value.map(item => [item.event_time || '', item.level, item.rule_name || item.matched_text, item.category || '', item.file_path, item.line_number, item.content, (item.related_causes || []).map(cause => cause.label).join('|')].map(value => `"${String(value).replaceAll('"', '""')}"`).join(','))
  const url = URL.createObjectURL(new Blob([`\uFEFF${[header, ...rows].join('\n')}`], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = `${taskId}-analysis.csv`; link.click(); URL.revokeObjectURL(url)
}

watch([search, level, category], () => { page.value = 1 })
function resizeTimeline() { chart?.resize() }
onMounted(() => { window.addEventListener('resize', resizeTimeline); load() })
onBeforeUnmount(() => { window.removeEventListener('resize', resizeTimeline); chart?.dispose() })
</script>

<style scoped>
.page { height: 100%; overflow: auto; color: #1f2937; }
.page-heading, .title, .heading-actions, .filters, .panel-heading, .detail-head, .section-title { display: flex; align-items: center; }
.page-heading { justify-content: space-between; margin-bottom: 18px; }.title { gap: 8px; }.title h1 { margin: 0; font-size: 21px; }.title p { margin: 4px 0 0; color: #8a94a3; font: 11px Consolas, monospace; }.heading-actions { gap: 10px; }
.summary { display: grid; grid-template-columns: repeat(4, 1fr); margin-bottom: 16px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }.summary div { display: flex; align-items: center; flex-direction: column; gap: 6px; padding: 18px; border-right: 1px solid #edf0f3; }.summary div:last-child { border-right: 0; }.summary span { color: #8a94a3; font-size: 11px; }.summary strong { font-size: 22px; }.summary .error { color: #d95858; }.summary .warning { color: #c9861b; }
.panel { padding: 17px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }.timeline-panel { margin-bottom: 16px; }.panel-heading { justify-content: space-between; margin-bottom: 10px; }.panel-heading h2 { margin: 0; font-size: 15px; }.panel-heading p { margin: 4px 0 0; color: #8a94a3; font-size: 11px; }.panel-heading > span { color: #8a94a3; font-size: 12px; }.timeline-chart { height: 300px; }
.cluster-detail { margin: 6px -17px -17px; border-top: 1px solid #e7ebf0; background: #fafbfd; }.cluster-heading { display: flex; align-items: center; justify-content: space-between; padding: 13px 17px 9px; }.cluster-heading > div { display: flex; align-items: baseline; gap: 10px; }.cluster-heading strong { font-size: 13px; }.cluster-heading span { color: #8a94a3; font-size: 11px; }.cluster-list { display: grid; max-height: 190px; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1px; overflow-y: auto; border-top: 1px solid #edf0f3; background: #edf0f3; }.cluster-list button { position: relative; display: grid; min-width: 0; grid-template-columns: 92px minmax(110px, auto) 75px 18px; align-items: center; gap: 9px; padding: 10px 14px; border: 0; background: #fff; color: #344054; text-align: left; cursor: pointer; }.cluster-list button:hover { background: #f3f7fc; }.cluster-list .event-time { color: #c43e3e; font: 11px Consolas, monospace; }.cluster-list strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.cluster-list .event-line { color: #8993a2; font-size: 10px; }.cluster-list p { grid-column: 1 / 4; overflow: hidden; margin: 0; color: #687588; font: 10px Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }.cluster-list svg { grid-column: 4; grid-row: 1 / 3; width: 14px; color: #8b96a7; }
.filters { gap: 10px; margin-bottom: 14px; }.filters .el-input { width: min(380px, 42%); }.filters .el-select { width: 130px; }.filters > span { margin-left: auto; color: #8a94a3; font-size: 11px; }.result-panel :deep(.el-table__row) { cursor: pointer; }footer { display: flex; align-items: center; justify-content: space-between; padding-top: 15px; color: #8a94a3; font-size: 11px; }
.detail-head { justify-content: space-between; padding-bottom: 15px; border-bottom: 1px solid #edf0f3; }.detail-head > div { display: flex; align-items: center; gap: 9px; }.detail-head > span { color: #8a94a3; font-size: 11px; }.detail-meta { display: grid; grid-template-columns: 1fr 1fr; gap: 13px 18px; margin: 16px 0 22px; }.detail-meta div { min-width: 0; }.detail-meta dt { color: #8a94a3; font-size: 11px; }.detail-meta dd { overflow: hidden; margin: 4px 0 0; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }.drawer-section { margin-top: 22px; }.section-title { justify-content: space-between; margin-bottom: 10px; }.section-title h3 { margin: 0; font-size: 14px; }.section-title span { color: #8a94a3; font-size: 11px; }.cause-row { padding: 11px 0; border-bottom: 1px solid #edf0f3; }.cause-row > div { display: flex; align-items: center; gap: 8px; }.cause-row > div span { color: #8a94a3; font-size: 10px; }.cause-row p { margin: 7px 0; color: #4a5568; font-size: 12px; }.cause-row code { display: block; overflow: hidden; color: #536174; font: 11px/1.45 Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }
.context-window { overflow: auto; max-height: 430px; padding: 7px 0; border: 1px solid #dfe3e8; border-radius: 4px; background: #101827; }.context-line { display: grid; grid-template-columns: 58px 102px minmax(0, 1fr); gap: 8px; padding: 4px 10px; color: #cbd5e1; font: 11px/1.45 Consolas, monospace; }.context-line.hit { background: rgb(207 69 69 / 28%); color: #fff; }.context-line.cause { background: rgb(201 134 27 / 16%); }.line-number { color: #7f8ea5; text-align: right; }.line-time { color: #91a8c7; }.line-content { overflow-wrap: anywhere; }.fallback-line { padding: 12px; color: #fff; font: 11px/1.5 Consolas, monospace; }
@media (max-width: 900px) { .summary { grid-template-columns: repeat(2, 1fr); }.filters { flex-wrap: wrap; }.filters .el-input, .filters .el-select { width: 100%; }.filters > span { margin-left: 0; }.cluster-list { grid-template-columns: 1fr; }.cluster-heading > div { align-items: flex-start; flex-direction: column; gap: 3px; } }
.ai-summary-panel { margin-bottom: 16px; }.ai-summary-panel .el-alert { margin-bottom: 12px; }.ai-file-result { padding: 14px 0; border-bottom: 1px solid #edf0f3; }.ai-file-result:last-child { border-bottom: 0; padding-bottom: 0; }.ai-file-heading,.ai-finding-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; }.ai-file-heading strong { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:13px; }.ai-summary-copy,.ai-summary-error { margin:10px 0 0; color:#4a5568; font-size:13px; line-height:1.7; }.ai-summary-error { color:#c43e3e; }.ai-findings { display:grid; gap:9px; margin-top:12px; }.ai-finding { padding:11px 12px; border:1px solid #e5e9ef; border-radius:6px; background:#fafbfd; }.ai-finding-heading strong { font-size:12px; }.ai-finding dl { display:grid; gap:6px; margin:9px 0 0; }.ai-finding dl div { display:grid; grid-template-columns:42px minmax(0,1fr); gap:8px; }.ai-finding dt { color:#8a94a3; font-size:11px; }.ai-finding dd { margin:0; color:#536174; font-size:11px; line-height:1.55; }
</style>

<style scoped>
.ai-summary-panel { border:1px solid rgba(255,255,255,.16)!important; background:rgba(255,255,255,.085)!important; box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 18px 52px rgba(0,0,0,.28)!important; }
.ai-summary-panel .panel-heading p,.ai-summary-panel .panel-heading>span { color:#b7bec8; }
.ai-summary-panel .ai-file-result { border-bottom-color:rgba(255,255,255,.12); }
.ai-summary-panel .ai-summary-copy { color:#d5dde5; }
.ai-summary-panel .ai-summary-error { color:#fda4af; }
.ai-summary-panel .ai-finding { border-color:rgba(255,255,255,.13); background:rgba(0,0,0,.16); }
.ai-summary-panel .ai-finding dt { color:#9da9b6; }
.ai-summary-panel .ai-finding dd { color:#c9d2dc; }
</style>

<style scoped>
:global(.page){font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif!important;-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;background:radial-gradient(circle at 50% 7%,rgba(255,255,255,.1),transparent 32%),radial-gradient(circle at 50% 82%,rgba(56,189,248,.07),transparent 52%),linear-gradient(145deg,#14171b,#1b1f25 48%,#111418)!important;color:#f5f7fa!important}.page-heading,.result-panel{background:rgba(255,255,255,.085)!important;border:1px solid rgba(255,255,255,.16)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 18px 52px rgba(0,0,0,.28)!important;border-radius:14px!important}.page-heading{padding:18px 20px!important;margin-bottom:14px!important;animation:result-rise .55s cubic-bezier(.4,0,.2,1) both}.title h1,.filters>span,.result-panel :deep(td.el-table__cell){color:#f5f7fa!important}.title p,.filters>span,footer{color:#b7bec8!important}.title :deep(.el-button){width:36px;height:36px;border:1px solid rgba(255,255,255,.14);border-radius:9px;background:rgba(255,255,255,.07);color:#c5ced8}.heading-actions{gap:9px}.heading-actions :deep(.el-tag){border-color:rgba(167,139,250,.35);background:rgba(167,139,250,.12);color:#c4b5fd}.heading-actions :deep(.el-button){border:0;border-radius:9px;background:linear-gradient(135deg,#0891b2,#06b6d4);box-shadow:0 9px 24px rgba(6,182,212,.22);color:#001115}.title :deep(.el-button:active),.heading-actions :deep(.el-button:active){transform:scale(.94)}.result-panel{padding:0!important;overflow:hidden;animation:result-rise .55s .08s cubic-bezier(.4,0,.2,1) both}.filters{gap:9px!important;margin:0!important;padding:15px 16px!important;border-bottom:1px solid rgba(255,255,255,.12);background:rgba(255,255,255,.035)}.filters .el-input{width:min(460px,46%)!important}.filters :deep(.el-input__wrapper),.filters :deep(.el-select__wrapper){background:rgba(0,0,0,.22)!important;box-shadow:0 0 0 1px rgba(255,255,255,.17) inset!important}.filters :deep(input){color:#f5f7fa!important}.filters :deep(input)::placeholder{color:#aeb7c3!important}.filters :deep(.el-select){width:150px}.result-panel :deep(.el-table){--el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.075);--el-table-row-hover-bg-color:rgba(6,182,212,.1);--el-table-border-color:rgba(255,255,255,.1);--el-table-text-color:#eef2f6;--el-table-header-text-color:#b7bec8;background:transparent!important}.result-panel :deep(th.el-table__cell){background:rgba(255,255,255,.075)!important;color:#c4ccd6!important;border-bottom-color:rgba(255,255,255,.14)!important}.result-panel :deep(td.el-table__cell){height:46px;background:transparent!important;border-bottom-color:rgba(255,255,255,.1)!important}.result-panel :deep(.el-table__row){cursor:pointer;transition:background .24s cubic-bezier(.4,0,.2,1)}.result-panel :deep(.el-table__row:hover>td.el-table__cell){background:rgba(6,182,212,.1)!important}.result-panel :deep(.el-table__row:hover td:first-child){box-shadow:inset 2px 0 #38bdf8}.result-panel :deep(.el-button--primary.is-link){color:#67e8f9}.result-panel :deep(.el-empty__description){color:#b7bec8}.result-panel footer{padding:13px 16px;border-top:1px solid rgba(255,255,255,.1)}.result-panel footer :deep(.el-pager li),.result-panel footer :deep(.el-pagination button){background:rgba(255,255,255,.07);color:#b7bec8}.result-panel footer :deep(.el-pager li.is-active){background:rgba(6,182,212,.22);color:#a8f3fb}.page-error{border:1px solid rgba(244,63,94,.25);background:rgba(244,63,94,.1);color:#fda4af}.el-drawer{background:rgba(24,28,33,.96)!important}@keyframes result-rise{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:none}}@media(prefers-reduced-motion:reduce){*,*:before,*:after{animation-duration:.01ms!important;transition-duration:.01ms!important}}@media(max-width:700px){.page-heading{align-items:flex-start;flex-direction:column;gap:12px}.heading-actions{width:100%;justify-content:space-between}.filters{align-items:stretch;flex-wrap:wrap}.filters .el-input,.filters :deep(.el-select){width:100%!important}.filters>span{margin-left:0}}
</style>
