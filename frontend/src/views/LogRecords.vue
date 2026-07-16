<template>
  <div class="records-page">
    <div class="page-heading">
      <div>
        <h1>日志记录</h1>
        <p>查看并下载近 7 天上传或实时采集的原始日志</p>
      </div>
      <div class="heading-actions">
        <el-button :icon="Refresh" @click="refreshRecords">刷新</el-button>
        <el-button type="primary" :icon="Download" :disabled="!selectedRecords.length" @click="downloadSelected">
          批量下载{{ selectedRecords.length ? `（${selectedRecords.length}）` : '' }}
        </el-button>
      </div>
    </div>

    <div class="retention-notice">
      <span class="notice-icon"><el-icon><Clock /></el-icon></span>
      <div><strong>日志保留 7 天</strong><span>到期后原始文件会自动清理，解析结果和任务数据不受影响。</span></div>
      <el-button type="primary" link @click="filters.retention = 'expiring'">查看即将过期</el-button>
    </div>

    <div class="summary-grid">
      <div v-for="item in summary" :key="item.label" class="summary-item">
        <span class="summary-icon" :class="item.tone"><el-icon><component :is="item.icon" /></el-icon></span>
        <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
      </div>
    </div>

    <section class="records-panel">
      <div class="filters">
        <el-input v-model="filters.keyword" :prefix-icon="Search" clearable placeholder="搜索日志名称、项目或采集会话" class="search-input" />
        <el-segmented v-model="filters.source" :options="sourceOptions" />
        <el-select v-model="filters.retention" clearable placeholder="全部保存状态" class="retention-filter">
          <el-option label="保存中" value="active" />
          <el-option label="即将过期" value="expiring" />
          <el-option label="已过期" value="expired" />
        </el-select>
      </div>

      <el-table :data="pagedRecords" class="records-table" @selection-change="selectedRecords = $event">
        <el-table-column type="selection" width="48" :selectable="(row) => !retentionMeta(row).expired" />
        <el-table-column label="日志文件" min-width="270">
          <template #default="scope">
            <div class="record-name-cell">
              <span class="source-icon" :class="scope.row.source"><el-icon><UploadFilled v-if="scope.row.source === 'upload'" /><Monitor v-else /></el-icon></span>
              <div><strong>{{ scope.row.name }}</strong><span>{{ scope.row.project }} · {{ scope.row.id }}</span></div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="来源" width="115">
          <template #default="scope"><el-tag :type="scope.row.source === 'upload' ? 'primary' : 'success'" effect="plain">{{ scope.row.source === 'upload' ? '文件上传' : '实时采集' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="日志信息" min-width="150">
          <template #default="scope">
            <div class="log-meta"><strong>{{ scope.row.size }}</strong><span>{{ scope.row.lines.toLocaleString() }} 行</span></div>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="165">
          <template #default="scope">{{ formatDate(scope.row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="保存期限" min-width="200">
          <template #default="scope">
            <div class="retention-cell" :class="retentionMeta(scope.row).status">
              <div><span>{{ retentionMeta(scope.row).label }}</span><b>{{ retentionMeta(scope.row).detail }}</b></div>
              <el-progress :percentage="retentionMeta(scope.row).percentage" :show-text="false" :stroke-width="5" :status="retentionMeta(scope.row).expired ? 'exception' : undefined" />
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="scope">
            <el-button type="primary" link :disabled="retentionMeta(scope.row).expired" :icon="Download" @click="downloadRecord(scope.row)">下载</el-button>
            <el-dropdown trigger="click" @command="(command) => handleCommand(command, scope.row)">
              <el-button text circle :icon="MoreFilled" title="更多操作" />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="detail" :icon="View">查看信息</el-dropdown-item>
                  <el-dropdown-item command="delete" :icon="Delete" divided>删除记录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer">
        <span>共 {{ filteredRecords.length }} 条记录</span>
        <el-pagination v-model:current-page="page" :page-size="pageSize" :total="filteredRecords.length" layout="prev, pager, next" />
      </div>
    </section>

    <el-drawer v-model="drawerVisible" title="日志记录信息" size="420px">
      <div v-if="activeRecord" class="record-detail">
        <div class="detail-file"><span class="source-icon large" :class="activeRecord.source"><el-icon><UploadFilled v-if="activeRecord.source === 'upload'" /><Monitor v-else /></el-icon></span><div><strong>{{ activeRecord.name }}</strong><span>{{ activeRecord.id }}</span></div></div>
        <dl>
          <div><dt>所属项目</dt><dd>{{ activeRecord.project }}</dd></div>
          <div><dt>日志来源</dt><dd>{{ activeRecord.source === 'upload' ? '文件上传' : '实时采集' }}</dd></div>
          <div><dt>文件大小</dt><dd>{{ activeRecord.size }}</dd></div>
          <div><dt>日志行数</dt><dd>{{ activeRecord.lines.toLocaleString() }}</dd></div>
          <div><dt>创建时间</dt><dd>{{ formatDate(activeRecord.createdAt) }}</dd></div>
          <div><dt>自动清理时间</dt><dd>{{ formatDate(activeRecord.expiresAt) }}</dd></div>
          <div v-if="activeRecord.source === 'realtime'"><dt>采集端口</dt><dd>{{ activeRecord.port }}</dd></div>
        </dl>
        <el-button type="primary" :icon="Download" :disabled="retentionMeta(activeRecord).expired" @click="downloadRecord(activeRecord)">下载原始日志</el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, markRaw, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Clock, Delete, Download, Files, Monitor, MoreFilled, Refresh, Search, UploadFilled, View, Warning } from '@element-plus/icons-vue'

const DAY = 24 * 60 * 60 * 1000
const now = ref(Date.now())
const page = ref(1)
const pageSize = 8
const selectedRecords = ref([])
const drawerVisible = ref(false)
const activeRecord = ref(null)
const filters = reactive({ keyword: '', source: '全部', retention: '' })
const sourceOptions = ['全部', '文件上传', '实时采集']
let clockTimer = window.setInterval(() => { now.value = Date.now() }, 60 * 1000)

const createRecord = (id, name, project, source, ageHours, size, lines, port = '') => {
  const createdAt = Date.now() - ageHours * 60 * 60 * 1000
  return { id, name, project, source, createdAt, expiresAt: createdAt + 7 * DAY, size, lines, port }
}

const records = ref([
  createRecord('LOG-0716-018', 'DR2860_稳定性回归_0716.zip', 'DR2860', 'upload', 2, '286.4 MB', 324580),
  createRecord('LIVE-0716-017', 'DR2860_COM5_实时采集.log', 'DR2860', 'realtime', 5, '84.7 MB', 112406, 'COM5'),
  createRecord('LOG-0716-016', 'SD64G_24H挂测.tar.gz', '存储稳定性', 'upload', 18, '1.8 GB', 1518204),
  createRecord('LIVE-0715-015', 'A800SE_ACC循环采集.log', 'A800SE', 'realtime', 31, '148.7 MB', 207345, 'COM6'),
  createRecord('LOG-0714-014', 'DR5800_基础功能日志.zip', 'DR5800', 'upload', 53, '92.1 MB', 86912),
  createRecord('LIVE-0713-013', 'DR2820_通断电采集.log', 'DR2820', 'realtime', 82, '356.8 MB', 507231, 'COM5'),
  createRecord('LOG-0712-012', 'SD128G_高温录像挂测.zip', '存储稳定性', 'upload', 121, '2.4 GB', 2019450),
  createRecord('LIVE-0710-011', 'DR4800_看门狗验证.log', 'DR4800', 'realtime', 158, '76.5 MB', 73420, 'COM7'),
  createRecord('LOG-0709-010', 'WiFi压力测试日志.zip', '无线连接', 'upload', 176, '524.9 MB', 612844)
])

const retentionMeta = (record) => {
  const remaining = record.expiresAt - now.value
  if (remaining <= 0) return { status: 'expired', label: '已过期', detail: '文件已清理', percentage: 0, expired: true }
  const hours = Math.ceil(remaining / (60 * 60 * 1000))
  const days = Math.floor(hours / 24)
  const percentage = Math.max(1, Math.round(remaining / (7 * DAY) * 100))
  if (hours <= 24) return { status: 'expiring', label: '即将过期', detail: `${hours} 小时后`, percentage, expired: false }
  return { status: 'active', label: '保存中', detail: `${days} 天 ${hours % 24} 小时`, percentage, expired: false }
}

const summary = computed(() => [
  { label: '可下载日志', value: records.value.filter((item) => !retentionMeta(item).expired).length, tone: 'blue', icon: markRaw(Files) },
  { label: '文件上传', value: records.value.filter((item) => item.source === 'upload' && !retentionMeta(item).expired).length, tone: 'indigo', icon: markRaw(UploadFilled) },
  { label: '实时采集', value: records.value.filter((item) => item.source === 'realtime' && !retentionMeta(item).expired).length, tone: 'green', icon: markRaw(Monitor) },
  { label: '24 小时内过期', value: records.value.filter((item) => retentionMeta(item).status === 'expiring').length, tone: 'gold', icon: markRaw(Warning) }
])

const filteredRecords = computed(() => {
  const search = filters.keyword.trim().toLowerCase()
  const source = filters.source === '文件上传' ? 'upload' : filters.source === '实时采集' ? 'realtime' : ''
  return records.value.filter((record) => {
    const matchesSearch = !search || [record.name, record.project, record.id].some((value) => value.toLowerCase().includes(search))
    const matchesSource = !source || record.source === source
    const matchesRetention = !filters.retention || retentionMeta(record).status === filters.retention
    return matchesSearch && matchesSource && matchesRetention
  })
})
const pagedRecords = computed(() => filteredRecords.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch(filters, () => { page.value = 1 }, { deep: true })

const formatDate = (timestamp) => new Date(timestamp).toLocaleString('zh-CN', { hour12: false }).replaceAll('/', '-')
const createDownload = (content, filename) => {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

const recordContent = (record) => [
  `# 日志记录 ${record.id}`,
  `# 项目: ${record.project}`,
  `# 来源: ${record.source === 'upload' ? '文件上传' : `实时采集 ${record.port}`}`,
  `# 创建时间: ${formatDate(record.createdAt)}`,
  '[14:18:31.208 ERROR] power source: POWER_ID_SWRT',
  '[14:22:44.912 ERROR] queue is full!!! drop frame channel=0',
  '[14:22:45.103 WARN] SD write detected frame loss for 15842ms'
].join('\n')

const downloadRecord = (record) => {
  if (retentionMeta(record).expired) return
  createDownload(recordContent(record), record.name.replace(/\.(zip|gz)$/i, '.log'))
  ElMessage.success(`正在下载 ${record.name}`)
}

const downloadSelected = () => {
  const content = selectedRecords.value.map((record) => `${'='.repeat(60)}\n${recordContent(record)}`).join('\n\n')
  createDownload(content, `日志批量下载_${new Date().toISOString().slice(0, 10)}.log`)
  ElMessage.success(`已生成 ${selectedRecords.value.length} 条日志的下载文件`)
}

const refreshRecords = () => { now.value = Date.now(); ElMessage.success('日志记录已刷新') }
const handleCommand = async (command, record) => {
  if (command === 'detail') { activeRecord.value = record; drawerVisible.value = true }
  if (command === 'delete') {
    await ElMessageBox.confirm(`确定删除日志记录“${record.name}”吗？`, '删除记录', { type: 'warning' })
    records.value = records.value.filter((item) => item.id !== record.id)
    ElMessage.success('日志记录已删除')
  }
}

onBeforeUnmount(() => window.clearInterval(clockTimer))
</script>

<style scoped>
.records-page { height: 100%; overflow: auto; color: #1f2937; box-sizing: border-box; }
.page-heading, .heading-actions, .retention-notice, .filters, .record-name-cell, .detail-file { display: flex; align-items: center; }
.page-heading { justify-content: space-between; margin-bottom: 16px; }
.page-heading h1 { margin: 0; font-size: 22px; line-height: 1.4; letter-spacing: 0; }
.page-heading p { margin: 5px 0 0; color: #7a8493; font-size: 14px; }
.heading-actions { gap: 8px; }

.retention-notice { gap: 11px; margin-bottom: 14px; padding: 11px 14px; border: 1px solid #d9e5f5; border-radius: 5px; background: #f4f8fe; }
.notice-icon { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border-radius: 4px; background: #e3edfb; color: #3478dc; }
.retention-notice > div { display: flex; flex: 1; flex-direction: column; gap: 3px; }
.retention-notice strong { color: #405774; font-size: 12px; }.retention-notice span { color: #72839a; font-size: 11px; }

.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-bottom: 16px; }
.summary-item { display: flex; align-items: center; gap: 12px; padding: 14px 17px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.summary-icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 5px; font-size: 18px; }.summary-icon.blue { color: #3478dc; background: #edf4ff; }.summary-icon.indigo { color: #6a67c8; background: #f0efff; }.summary-icon.green { color: #2f9275; background: #eaf8f3; }.summary-icon.gold { color: #c9861b; background: #fff6e7; }
.summary-item > div { display: flex; flex-direction: column; gap: 3px; }.summary-item span { color: #7a8493; font-size: 11px; }.summary-item strong { font-size: 20px; }

.records-panel { padding: 17px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.filters { gap: 10px; margin-bottom: 15px; }.search-input { width: min(340px, 36%); }.retention-filter { width: 150px; }
.records-panel :deep(.el-table__header th) { background: #f8fafc; color: #667085; font-weight: 600; }
.record-name-cell { min-width: 0; gap: 10px; }.source-icon { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border-radius: 4px; font-size: 18px; }.source-icon.upload { color: #6a67c8; background: #f0efff; }.source-icon.realtime { color: #2f9275; background: #eaf8f3; }.record-name-cell > div, .detail-file > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.record-name-cell strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.record-name-cell span { color: #8a94a3; font-size: 11px; }
.log-meta { display: flex; flex-direction: column; gap: 4px; }.log-meta strong { font-size: 12px; }.log-meta span { color: #8a94a3; font-size: 11px; }
.retention-cell { max-width: 165px; }.retention-cell > div { display: flex; justify-content: space-between; margin-bottom: 6px; font-size: 11px; }.retention-cell span { color: #4f5d70; }.retention-cell b { color: #7d8795; font-weight: 500; }.retention-cell.expiring span, .retention-cell.expiring b { color: #c9861b; }.retention-cell.expired span, .retention-cell.expired b { color: #a0a7b2; }
.table-footer { display: flex; align-items: center; justify-content: space-between; padding-top: 15px; color: #7a8493; font-size: 11px; }
.detail-file { gap: 11px; padding-bottom: 18px; border-bottom: 1px solid #edf0f3; }.source-icon.large { width: 44px; height: 44px; font-size: 22px; }.detail-file strong { font-size: 14px; }.detail-file span { color: #8a94a3; font-size: 11px; }
.record-detail dl { margin: 18px 0; }.record-detail dl > div { display: flex; justify-content: space-between; padding: 11px 0; border-bottom: 1px solid #edf0f3; font-size: 12px; }.record-detail dt { color: #8a94a3; }.record-detail dd { margin: 0; color: #3e4a5d; text-align: right; }
@media (max-width: 1000px) { .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }.filters { flex-wrap: wrap; }.search-input { width: 100%; } }
@media (max-width: 680px) { .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.heading-actions { width: 100%; }.heading-actions .el-button { flex: 1; }.summary-grid { grid-template-columns: 1fr; }.retention-notice { align-items: flex-start; }.retention-notice > .el-button { display: none; }.records-panel { padding: 12px; }.retention-filter { width: 100%; }.table-footer { align-items: flex-start; flex-direction: column; gap: 10px; } }
</style>
