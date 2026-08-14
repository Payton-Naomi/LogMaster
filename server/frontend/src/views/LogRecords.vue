<template>
  <div class="records-page" :class="{ 'low-performance': lowPerformance }" @mousemove="trackGlass">
    <header class="page-heading">
      <div>
        <h1>日志记录</h1>
        <p>检索已上传的日志，并快速进入对应分析任务</p>
      </div>
      <div class="heading-actions">
        <el-button class="command-trigger" @click="commandOpen = true"><span class="command-key">Ctrl</span><span>K</span><span>命令面板</span></el-button>
        <el-tooltip content="刷新记录" placement="bottom">
          <el-button :icon="Refresh" :loading="loading" aria-label="刷新记录" @click="load" />
        </el-tooltip>
        <el-button type="primary" :icon="Upload" @click="router.push('/upload')">上传日志</el-button>
      </div>
    </header>

    <div class="summary-grid">
      <div v-for="item in summary" :key="item.label" class="summary-item">
        <el-icon :class="item.tone"><component :is="item.icon" /></el-icon>
        <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
      </div>
    </div>

    <section class="records-panel" v-loading="loading && !records.length">
      <div class="panel-toolbar">
        <div class="search-row">
          <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索文件名、版本或记录 ID" />
          <el-select v-model="project" clearable filterable placeholder="全部项目">
            <el-option v-for="name in projects" :key="name" :label="name" :value="name" />
          </el-select>
          <el-date-picker v-model="dateRange" type="daterange" unlink-panels range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" />
          <el-select v-model="sortBy" class="sort-select" aria-label="记录排序">
            <el-option label="最新上传" value="created_desc" /><el-option label="最早上传" value="created_asc" /><el-option label="文件最大" value="size_desc" /><el-option label="错误最多" value="errors_desc" />
          </el-select>
        </div>
        <div class="filter-row"><el-segmented v-model="status" :options="statusOptions" /><el-button v-if="hasFilters" link :icon="RefreshLeft" @click="clearFilters">清空筛选</el-button></div>
      </div>

      <div v-if="selectedRecords.length" class="batch-bar">
        <div><el-icon><Select /></el-icon><strong>已选择 {{ selectedRecords.length }} 条</strong><span>约 {{ formatSize(selectedSize) }}</span></div>
        <div><el-button v-if="selectedRecords.length < filtered.length" link @click="selectAllFiltered">选择全部 {{ filtered.length }} 条结果</el-button><el-button link @click="clearSelection">取消选择</el-button><el-button type="danger" :icon="Delete" :loading="batchDeleting" @click="removeSelected">批量删除</el-button></div>
      </div>

      <div class="list-meta">
        <strong>记录列表</strong>
        <span>显示 {{ filtered.length }} 条<span v-if="lastUpdated"> · 更新于 {{ lastUpdated }}</span></span>
      </div>

      <div v-if="loading && !records.length" class="skeleton-table" aria-hidden="true">
        <div v-for="row in 7" :key="row" class="skeleton-row"><i v-for="cell in 6" :key="cell" /></div>
      </div>
      <el-table class="desktop-table" :data="paged" row-key="id" @row-click="viewTask">
        <el-table-column width="46" align="center">
          <template #header><el-checkbox :model-value="pageSelected" :indeterminate="pagePartiallySelected" aria-label="选择当前页" @change="togglePageSelection" /></template>
          <template #default="scope"><el-checkbox :model-value="selectedIds.has(scope.row.id)" :disabled="!scope.row.task_id" :aria-label="`选择 ${scope.row.original_name}`" @click.stop @change="(checked) => toggleSelection(scope.row, checked)" /></template>
        </el-table-column>
        <el-table-column label="上传文件" min-width="300">
          <template #default="scope">
            <div class="file-cell">
              <el-icon><Document /></el-icon>
              <div><strong v-html="highlightText(scope.row.original_name || '-')" /><span>{{ scope.row.id }}</span></div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="项目 / 版本" min-width="170">
          <template #default="scope"><div class="project-cell"><strong>{{ scope.row.project_name || '-' }}</strong><span>{{ scope.row.version || '未填写版本' }}</span></div></template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="scope"><el-tag :type="statusMeta[scope.row.statusKey].type" effect="plain">{{ statusMeta[scope.row.statusKey].label }}</el-tag></template>
        </el-table-column>
        <el-table-column label="内容 / 结果" min-width="190">
          <template #default="scope"><div class="content-cell"><strong>{{ scope.row.file_count || 0 }} 个文件 · {{ formatSize(scope.row.original_size) }}</strong><span><b class="error-count">{{ scope.row.error_count || 0 }} 错误</b><b class="warning-count">{{ scope.row.warning_count || 0 }} 警告</b></span></div></template>
        </el-table-column>
        <el-table-column label="上传时间" min-width="180"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
        <el-table-column label="操作" width="104" align="center">
          <template #default="scope">
            <div class="row-actions" @click.stop>
              <el-button circle :icon="Document" aria-label="Preview record" @click="previewRecord(scope.row)" />
              <el-tooltip content="查看任务" placement="top"><el-button circle :icon="View" aria-label="查看任务" @click="viewTask(scope.row)" /></el-tooltip>
              <el-tooltip content="删除记录" placement="top"><el-button circle type="danger" plain :icon="Delete" aria-label="删除记录" @click="remove(scope.row)" /></el-tooltip>
            </div>
          </template>
        </el-table-column>
        <template #empty><div class="empty-state"><el-empty :description="hasFilters ? '没有符合条件的日志记录' : '还没有上传日志'" /><el-button v-if="!hasFilters" type="primary" :icon="Upload" @click="router.push('/upload')">上传第一份日志</el-button></div></template>
      </el-table>

      <div class="mobile-list">
        <article v-for="record in paged" :key="record.id" class="record-card" :class="{ selected: selectedIds.has(record.id) }" @click="viewTask(record)">
          <div class="record-card-head"><el-checkbox :model-value="selectedIds.has(record.id)" :disabled="!record.task_id" :aria-label="`选择 ${record.original_name}`" @click.stop @change="(checked) => toggleSelection(record, checked)" /><div class="file-icon"><el-icon><Document /></el-icon></div><div class="record-title"><strong>{{ record.original_name || '-' }}</strong><span>{{ record.project_name || '-' }} · {{ record.version || '未填写版本' }}</span></div><el-tag :type="statusMeta[record.statusKey].type" effect="plain">{{ statusMeta[record.statusKey].label }}</el-tag></div>
          <div class="record-details"><span><el-icon><Files /></el-icon>{{ record.file_count || 0 }} 个文件</span><span>{{ formatSize(record.original_size) }}</span><span class="error-count">{{ record.error_count || 0 }} 错误</span><span class="warning-count">{{ record.warning_count || 0 }} 警告</span><time>{{ formatDate(record.created_at) }}</time></div>
          <div class="record-card-foot"><code>{{ record.id }}</code><div @click.stop><el-button type="primary" link :icon="View" @click="viewTask(record)">查看</el-button><el-button type="danger" link :icon="Delete" @click="remove(record)">删除</el-button></div></div>
        </article>
        <div v-if="!paged.length" class="mobile-empty"><el-empty :description="hasFilters ? '没有符合条件的日志记录' : '还没有上传日志'" /></div>
      </div>

      <footer v-if="filtered.length">
        <span>共 {{ filtered.length }} 条记录</span>
        <el-pagination v-model:current-page="page" :page-size="pageSize" :total="filtered.length" :pager-count="5" layout="prev, pager, next" />
      </footer>
    </section>
    <el-dialog v-model="commandOpen" class="command-dialog" width="520px" :show-close="false" append-to-body>
      <div class="command-head"><span class="eyebrow">COMMAND CENTER</span><el-button text @click="commandOpen = false">Esc</el-button></div>
      <el-input v-model="commandQuery" autofocus placeholder="搜索动作或日志视图" />
      <div class="command-list"><button v-for="item in commandItems" :key="item.label" type="button" @click="runCommand(item.action)"><span>{{ item.icon }}</span><strong>{{ item.label }}</strong><small>{{ item.hint }}</small></button><p v-if="!commandItems.length">没有匹配的命令</p></div>
    </el-dialog>
    <el-drawer v-model="detailOpen" direction="rtl" size="420px" class="record-drawer" :with-header="false">
      <div v-if="preview" class="detail-content">
        <div class="detail-head"><div><span class="eyebrow">LOG RECORD</span><h2>{{ preview.original_name || '-' }}</h2></div><el-button text :icon="Close" aria-label="Close details" @click="detailOpen = false" /></div>
        <div class="detail-status"><span :class="['status-dot', preview.statusKey]" />{{ statusMeta[preview.statusKey].label }}</div>
        <dl class="detail-grid"><div><dt>Record ID</dt><dd>{{ preview.id }}</dd></div><div><dt>Project</dt><dd>{{ preview.project_name || '-' }}</dd></div><div><dt>Version</dt><dd>{{ preview.version || '-' }}</dd></div><div><dt>Uploaded</dt><dd>{{ formatDate(preview.created_at) }}</dd></div><div><dt>Files</dt><dd>{{ preview.file_count || 0 }}</dd></div><div><dt>Size</dt><dd>{{ formatSize(preview.original_size) }}</dd></div><div><dt>Errors</dt><dd class="error-text">{{ preview.error_count || 0 }}</dd></div><div><dt>Warnings</dt><dd class="warn-text">{{ preview.warning_count || 0 }}</dd></div></dl>
        <div class="detail-actions"><el-button type="primary" :icon="View" @click="viewTask(preview)">Open analysis</el-button><el-button plain :icon="Delete" @click="remove(preview)">Delete</el-button></div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, markRaw, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Close, DataLine, Delete, Document, Files, FolderOpened, Refresh, RefreshLeft, Search, Select, Upload, View } from '@element-plus/icons-vue'
import { getLogs, getProjects } from '@/api/log'
import { deleteTask } from '@/api/task'

const router = useRouter()
const records = ref([])
const loading = ref(false)
const keyword = ref('')
const project = ref('')
const status = ref('all')
const dateRange = ref([])
const sortBy = ref('created_desc')
const page = ref(1)
const lastUpdated = ref('')
const projects = ref([])
const selectedIds = ref(new Set())
const batchDeleting = ref(false)
const detailOpen = ref(false)
const preview = ref(null)
const lowPerformance = ref(false)
const commandOpen = ref(false)
const commandQuery = ref('')
const commandItems = computed(() => [{ label: '刷新日志记录', hint: '重新请求最新数据', icon: '↻', action: 'refresh' }, { label: '上传新的日志', hint: '进入日志上传页面', icon: '↑', action: 'upload' }, { label: '清空当前筛选', hint: '恢复默认筛选条件', icon: '⌘', action: 'clear' }].filter((item) => !commandQuery.value.trim() || `${item.label}${item.hint}`.includes(commandQuery.value.trim())))
const pageSize = 10

const statusMeta = {
  uploading: { label: '上传中', type: 'info', group: 'active' },
  queued: { label: '排队中', type: 'warning', group: 'active' },
  parsing: { label: '解析中', type: 'primary', group: 'active' },
  completed: { label: '已完成', type: 'success', group: 'completed' },
  failed: { label: '失败', type: 'danger', group: 'failed' },
  unknown: { label: '未知', type: 'info', group: 'active' }
}
const statusOptions = [
  { label: '全部', value: 'all' },
  { label: '处理中', value: 'active' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' }
]
const normalizeRecord = (item) => ({ ...item, statusKey: statusMeta[item.status] ? item.status : 'unknown' })
const totals = computed(() => {
  const projects = new Set(records.value.map((item) => item.project_name).filter(Boolean))
  return records.value.reduce((sum, item) => ({
    files: sum.files + (item.file_count || 0),
    size: sum.size + (item.original_size || 0),
    projects: projects.size
  }), { files: 0, size: 0, projects: projects.size })
})
const summary = computed(() => [
  { label: '上传记录', value: records.value.length.toLocaleString(), tone: 'blue', icon: markRaw(DataLine) },
  { label: '日志文件', value: totals.value.files.toLocaleString(), tone: 'violet', icon: markRaw(Files) },
  { label: '占用空间', value: formatSize(totals.value.size), tone: 'green', icon: markRaw(Document) },
  { label: '关联项目', value: totals.value.projects.toLocaleString(), tone: 'gold', icon: markRaw(FolderOpened) }
])
const filtered = computed(() => {
  const text = keyword.value.trim().toLowerCase()
  const startTime = dateRange.value?.[0] ? new Date(dateRange.value[0]).setHours(0, 0, 0, 0) : 0
  const endTime = dateRange.value?.[1] ? new Date(dateRange.value[1]).setHours(23, 59, 59, 999) : Number.POSITIVE_INFINITY
  const result = records.value.filter((item) => {
    const matchesText = !text || `${item.original_name}${item.project_name}${item.version}${item.id}`.toLowerCase().includes(text)
    const matchesProject = !project.value || item.project_name === project.value
    const matchesStatus = status.value === 'all' || statusMeta[item.statusKey].group === status.value
    const createdTime = item.created_at ? new Date(item.created_at).getTime() : 0
    return matchesText && matchesProject && matchesStatus && createdTime >= startTime && createdTime <= endTime
  })
  return result.sort((left, right) => {
    if (sortBy.value === 'created_asc') return new Date(left.created_at) - new Date(right.created_at)
    if (sortBy.value === 'size_desc') return (right.original_size || 0) - (left.original_size || 0)
    if (sortBy.value === 'errors_desc') return (right.error_count || 0) - (left.error_count || 0)
    return new Date(right.created_at) - new Date(left.created_at)
  })
})
const paged = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const hasFilters = computed(() => Boolean(keyword.value.trim() || project.value || status.value !== 'all' || dateRange.value?.length))
const selectedRecords = computed(() => records.value.filter((item) => selectedIds.value.has(item.id)))
const selectedSize = computed(() => selectedRecords.value.reduce((sum, item) => sum + (item.original_size || 0), 0))
const selectableOnPage = computed(() => paged.value.filter((item) => item.task_id))
const pageSelected = computed(() => Boolean(selectableOnPage.value.length) && selectableOnPage.value.every((item) => selectedIds.value.has(item.id)))
const pagePartiallySelected = computed(() => !pageSelected.value && selectableOnPage.value.some((item) => selectedIds.value.has(item.id)))
const formatSize = (bytes) => { if (!bytes) return '0 B'; const units = ['B', 'KB', 'MB', 'GB']; const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 3); return `${(bytes / 1024 ** i).toFixed(i ? 1 : 0)} ${units[i]}` }
const formatDate = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const highlightText = (value) => {
  const escaped = String(value).replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]))
  const query = keyword.value.trim()
  if (!query) return escaped
  const safeQuery = query.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')
  return escaped.replace(new RegExp(`(${safeQuery})`, 'ig'), '<mark>$1</mark>')
}
const viewTask = (record) => {
  if (!record.task_id) return ElMessage.warning('该记录没有关联任务')
  router.push({ name: 'TaskDetail', params: { taskId: record.task_id } })
}
function previewRecord(record) {
  preview.value = record
  detailOpen.value = true
}
function clearFilters() {
  keyword.value = ''
  project.value = ''
  status.value = 'all'
  dateRange.value = []
  sortBy.value = 'created_desc'
}
function toggleSelection(record, checked) {
  const next = new Set(selectedIds.value)
  if (checked && record.task_id) next.add(record.id)
  else next.delete(record.id)
  selectedIds.value = next
}
function togglePageSelection(checked) {
  const next = new Set(selectedIds.value)
  selectableOnPage.value.forEach((item) => checked ? next.add(item.id) : next.delete(item.id))
  selectedIds.value = next
}
function selectAllFiltered() {
  selectedIds.value = new Set(filtered.value.filter((item) => item.task_id).map((item) => item.id))
}
function clearSelection() { selectedIds.value = new Set() }
async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const first = await getLogs({ page: 1, page_size: 200 })
    const pages = Math.ceil((first.total || 0) / 200)
    const remaining = pages > 1 ? await Promise.all(Array.from({ length: pages - 1 }, (_, index) => getLogs({ page: index + 2, page_size: 200 }))) : []
    records.value = [first, ...remaining].flatMap((data) => data.list || []).map(normalizeRecord)
    selectedIds.value = new Set([...selectedIds.value].filter((id) => records.value.some((item) => item.id === id)))
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
  } catch {
    ElMessage.error('日志记录加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
async function remove(record) {
  try {
    await ElMessageBox.confirm(`将永久删除“${record.original_name}”、${record.file_count || 0} 个日志文件及全部分析结果。`, '删除日志记录', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
    await deleteTask(record.task_id)
    ElMessage.success('日志记录已删除')
    records.value = records.value.filter((item) => item.id !== record.id)
    const next = new Set(selectedIds.value); next.delete(record.id); selectedIds.value = next
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('删除失败，请稍后重试')
  }
}
async function removeSelected() {
  if (!selectedRecords.value.length || batchDeleting.value) return
  try {
    await ElMessageBox.confirm(`将永久删除选中的 ${selectedRecords.value.length} 条日志记录、原始文件及全部分析结果。`, '批量删除日志', { type: 'warning', confirmButtonText: '确认批量删除', cancelButtonText: '取消' })
    batchDeleting.value = true
    const targets = [...selectedRecords.value]
    const results = await Promise.allSettled(targets.map((record) => deleteTask(record.task_id)))
    const deletedIds = new Set(targets.filter((_, index) => results[index].status === 'fulfilled').map((item) => item.id))
    const failed = results.length - deletedIds.size
    records.value = records.value.filter((item) => !deletedIds.has(item.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => !deletedIds.has(id)))
    if (deletedIds.size) ElMessage.success(`已删除 ${deletedIds.size} 条日志记录`)
    if (failed) ElMessage.warning(`${failed} 条记录删除失败，可稍后重试`)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('批量删除失败，请稍后重试')
  } finally { batchDeleting.value = false }
}
async function loadProjects() {
  try { projects.value = await getProjects() || [] }
  catch { projects.value = [] }
}
watch([keyword, project, status, dateRange, sortBy], () => { page.value = 1 })
watch(() => filtered.value.length, (length) => { page.value = Math.min(page.value, Math.max(1, Math.ceil(length / pageSize))) })
onMounted(() => {
  load()
  loadProjects()
  lowPerformance.value = navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency < 4
  window.addEventListener('keydown', handleShortcut)
})
onBeforeUnmount(() => window.removeEventListener('keydown', handleShortcut))
function trackGlass(event) { if (lowPerformance.value) return; const target = event.target.closest?.('.glass-panel'); if (!target) return; const rect = target.getBoundingClientRect(); target.style.setProperty('--mx', `${event.clientX - rect.left}px`); target.style.setProperty('--my', `${event.clientY - rect.top}px`) }
function handleShortcut(event) { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); commandOpen.value = true } if (event.key === 'Escape') commandOpen.value = false }
function runCommand(action) { commandOpen.value = false; commandQuery.value = ''; if (action === 'refresh') load(); if (action === 'upload') router.push('/upload'); if (action === 'clear') clearFilters() }
</script>

<style scoped>
.records-page{height:100%;overflow:auto;color:var(--lm-text-primary)}
.page-heading{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page-heading h1{margin:0;font-size:22px}.page-heading p{margin:5px 0 0;color:var(--lm-text-secondary);font-size:13px}.heading-actions{display:flex;gap:9px}
.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px;margin-bottom:16px}.summary-item{display:flex;min-height:78px;align-items:center;gap:14px;padding:16px 18px;border:1px solid var(--lm-border);border-radius:6px;background:#fff}.summary-item>.el-icon{display:grid;width:40px;height:40px;flex:0 0 40px;place-items:center;border-radius:5px;font-size:20px}.summary-item .blue{color:#2877e8;background:#eaf2fe}.summary-item .violet{color:#6d62bf;background:#f0effb}.summary-item .green{color:#27936c;background:#e7f5ef}.summary-item .gold{color:#bd7d19;background:#fff4df}.summary-item div{display:flex;min-width:0;flex-direction:column;gap:4px}.summary-item span{color:#7a8493;font-size:12px}.summary-item strong{overflow:hidden;font-size:20px;text-overflow:ellipsis;white-space:nowrap}
.records-panel{min-height:390px;padding:18px;border:1px solid var(--lm-border);border-radius:6px;background:#fff}.panel-toolbar{display:flex;align-items:stretch;flex-direction:column;gap:12px;margin-bottom:15px}.search-row,.filter-row{display:flex;min-width:0;align-items:center;gap:10px}.search-row .el-input{min-width:220px;flex:1}.search-row .el-select{width:170px}.search-row :deep(.el-date-editor){width:250px;flex:0 0 250px}.search-row .sort-select{width:135px}.filter-row{justify-content:space-between}.filter-row>.el-button{margin-left:auto}.batch-bar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:0 0 13px;padding:10px 12px;border:1px solid #b9d2f5;border-radius:5px;background:#f1f7ff}.batch-bar>div{display:flex;align-items:center;gap:9px}.batch-bar>div:first-child{color:#2868ca}.batch-bar>div:first-child span{color:#7a8493;font-size:11px}.batch-bar .el-button{margin:0}.list-meta{display:flex;align-items:center;justify-content:space-between;padding:0 2px 11px}.list-meta strong{font-size:14px}.list-meta span{color:#8a94a3;font-size:11px}
.desktop-table :deep(.el-table__row){cursor:pointer}.desktop-table :deep(.el-table__row:hover>td.el-table__cell){background:#f6f9fc}.file-cell{display:flex;align-items:center;gap:11px}.file-cell>.el-icon,.file-icon{display:grid;width:36px;height:36px;flex:0 0 36px;place-items:center;border-radius:5px;background:#edf4ff;color:#3478dc;font-size:17px}.file-cell div,.project-cell,.content-cell{display:flex;min-width:0;flex-direction:column;gap:4px}.file-cell strong,.project-cell strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-cell span{overflow:hidden;color:#8a94a3;font:10px Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}.project-cell span,.content-cell span{color:#8a94a3;font-size:11px}.content-cell strong{font-size:12px}.content-cell span{display:flex;gap:8px}.content-cell b{font-size:10px;font-weight:600}.error-count{color:#c94e4e!important}.warning-count{color:#b6791f!important}.row-actions{display:flex;justify-content:center;gap:7px}.row-actions .el-button{width:30px;height:30px;margin:0}.empty-state{padding:18px 0 30px}.empty-state .el-empty{padding-bottom:8px}
.mobile-list{display:none}.record-card{padding:14px 6px;border-bottom:1px solid #edf0f3}.record-card.selected{margin:0 -6px;padding-right:12px;padding-left:12px;background:#f3f8ff}.record-card:first-child{padding-top:4px}.record-card-head{display:flex;align-items:center;gap:10px}.record-title{display:flex;min-width:0;flex:1;flex-direction:column;gap:4px}.record-title strong{overflow:hidden;font-size:13px;text-overflow:ellipsis;white-space:nowrap}.record-title span{color:#7a8493;font-size:11px}.record-details{display:flex;flex-wrap:wrap;gap:12px;margin:12px 0 11px;padding-left:70px;color:#657180;font-size:11px}.record-details span{display:flex;align-items:center;gap:4px}.record-card-foot{display:flex;align-items:center;justify-content:space-between;padding-left:70px}.record-card-foot code{max-width:45%;overflow:hidden;color:#929cab;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.record-card-foot .el-button{margin-left:8px}.mobile-empty{padding:20px 0}
footer{display:flex;align-items:center;justify-content:space-between;padding-top:16px;color:#7a8493;font-size:12px}
@media(max-width:1100px){.summary-grid{grid-template-columns:repeat(2,1fr)}.search-row{flex-wrap:wrap}.search-row .el-input{flex-basis:45%}.search-row .el-select{flex:1}.search-row :deep(.el-date-editor){width:260px;flex:1 1 260px}}
@media(max-width:767px){.page-heading{align-items:flex-start}.page-heading p{max-width:220px}.summary-grid{gap:10px}.summary-item{min-height:70px;padding:13px}.summary-item>.el-icon{width:34px;height:34px;flex-basis:34px;font-size:17px}.summary-item strong{font-size:18px}.records-panel{padding:14px}.search-row{align-items:stretch;flex-direction:column}.search-row .el-input,.search-row .el-select,.search-row :deep(.el-date-editor){width:100%;flex:none}.filter-row{align-items:stretch;flex-direction:column}.filter-row>.el-button{align-self:flex-end;margin:0}.panel-toolbar :deep(.el-segmented){width:100%}.panel-toolbar :deep(.el-segmented__item){flex:1}.batch-bar{align-items:stretch;flex-direction:column}.batch-bar>div:last-child{justify-content:flex-end;flex-wrap:wrap}.desktop-table{display:none}.mobile-list{display:block}.list-meta{padding-bottom:9px}.list-meta span span{display:none}footer{gap:10px}footer>span{display:none}}
@media(max-width:480px){.page-heading{gap:12px}.page-heading p{display:none}.heading-actions .el-button:first-child{width:32px;padding:0}.summary-grid{grid-template-columns:1fr 1fr}.summary-item{gap:10px}.records-panel{padding:12px}.record-details,.record-card-foot{padding-left:30px}.batch-bar>div:first-child span{display:none}.batch-bar>div:last-child .el-button:first-child{width:100%}}
</style>

<style scoped>
/* LogUI-Spectacle glass treatment and motion layer */
.records-page{position:relative;isolation:isolate;background:radial-gradient(circle at 50% -10%,rgba(6,182,212,.11),transparent 38%),linear-gradient(135deg,#0b0d10,#111418 55%,#0b0d10)!important;color:#e4e6ea!important}
.records-page:before{position:fixed;inset:0;z-index:-1;pointer-events:none;opacity:.14;background-image:linear-gradient(rgba(255,255,255,.03) 1px,transparent 1px),linear-gradient(90deg,rgba(255,255,255,.03) 1px,transparent 1px);background-size:32px 32px;content:''}
.page-heading,.summary-item,.records-panel,.record-card{background:rgba(255,255,255,.035)!important;border-color:rgba(255,255,255,.07)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.05),0 12px 36px rgba(0,0,0,.24)!important}
.page-heading:before,.summary-item:before,.records-panel:before,.record-card:before{position:absolute;inset:0;pointer-events:none;background:radial-gradient(320px circle at var(--mx,50%) var(--my,50%),rgba(255,255,255,.05),transparent 42%);content:''}
.page-heading{position:sticky;top:0;z-index:4;margin:0 -2px 14px;padding:16px 2px 14px;background:rgba(11,13,16,.82)!important;border-bottom-color:rgba(255,255,255,.08)!important;animation:log-fade-down .45s both}
.page-heading h1{color:#f8fafc!important;font-size:24px}.page-heading p{color:#8b9099!important}.page-heading .eyebrow,.eyebrow{color:#38bdf8!important;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.16em}
.heading-actions .el-button{border-radius:8px;transition:transform .18s ease,box-shadow .18s ease}.heading-actions .el-button:active,.row-actions .el-button:active{transform:scale(.97)}.heading-actions .el-button--primary{border:0;background:linear-gradient(135deg,#0891b2,#06b6d4);box-shadow:0 8px 20px rgba(6,182,212,.22)}
.summary-grid{gap:10px!important}.summary-item{position:relative;min-height:88px;padding:15px 16px!important;animation:log-rise .5s var(--delay,0ms) both}.summary-item>div:last-child strong{color:#f8fafc}.summary-item span{color:#8b9099}.summary-item>.el-icon{border-radius:10px}.summary-item .blue{color:#38bdf8;background:rgba(56,189,248,.14)}.summary-item .violet{color:#a78bfa;background:rgba(124,58,237,.16)}.summary-item .green{color:#34d399;background:rgba(52,211,153,.14)}.summary-item .gold{color:#f59e0b;background:rgba(245,158,11,.14)}
.records-panel{padding:0!important}.panel-toolbar{background:rgba(17,20,24,.72)!important;border-bottom-color:rgba(255,255,255,.07)!important}.search-row :deep(.el-input__wrapper),.search-row :deep(.el-select__wrapper),.search-row :deep(.el-range-editor){background:rgba(0,0,0,.18)!important;box-shadow:0 0 0 1px rgba(255,255,255,.1) inset!important;transition:box-shadow .25s ease}.search-row :deep(.el-input__wrapper:focus-within){box-shadow:0 0 0 1px rgba(6,182,212,.7) inset,0 0 0 3px rgba(6,182,212,.12)!important}.search-row :deep(input),.search-row :deep(.el-range-input){color:#e4e6ea!important}.filter-row :deep(.el-segmented){--el-segmented-bg-color:rgba(0,0,0,.18);--el-segmented-item-selected-bg-color:rgba(6,182,212,.2);--el-segmented-item-selected-color:#67e8f9;--el-segmented-color:#8b9099}
.batch-bar{background:rgba(6,182,212,.1)!important;border-color:rgba(6,182,212,.35)!important}.list-meta strong{color:#f8fafc}.list-meta span{color:#717780}.desktop-table{--el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.035);--el-table-row-hover-bg-color:rgba(6,182,212,.08);--el-table-border-color:rgba(255,255,255,.07);--el-table-text-color:#cbd5e1;--el-table-header-text-color:#64748b}.desktop-table :deep(th.el-table__cell){background:rgba(255,255,255,.035);border-bottom-color:rgba(255,255,255,.08);font-size:10px;letter-spacing:.14em}.desktop-table :deep(td.el-table__cell){height:44px;padding:5px 0}.desktop-table :deep(.el-table__row){transition:background .16s ease,transform .16s ease;animation:log-row .35s both}.desktop-table :deep(.el-table__row:hover){transform:translateX(2px)}.file-cell>.el-icon,.file-icon{background:rgba(56,189,248,.1);border:1px solid rgba(56,189,248,.2);color:#38bdf8}.file-cell strong,.project-cell strong,.content-cell strong,.record-title strong{color:#e4e6ea}.file-cell span,.project-cell span,.content-cell span,.record-title span,.record-details,.record-card-foot code{color:#717780}.error-count,.error-text{color:#f43f5e!important}.warning-count,.warn-text{color:#f59e0b!important}.row-actions{gap:4px}.row-actions .el-button{border-color:rgba(255,255,255,.1);background:rgba(255,255,255,.04);color:#94a3b8}.row-actions .el-button:hover{border-color:rgba(56,189,248,.5);color:#67e8f9;background:rgba(6,182,212,.13)}
.status-chip{border-color:rgba(255,255,255,.1);background:rgba(255,255,255,.04)}.status-chip.el-tag--danger,.status-chip.failed{animation:log-pulse 1.6s ease-in-out infinite}.skeleton-row i{background:linear-gradient(90deg,rgba(255,255,255,.03),rgba(255,255,255,.11),rgba(255,255,255,.03));background-size:200% 100%;animation:log-shimmer 1.4s ease-in-out infinite}.record-drawer :deep(.el-drawer),.record-drawer :deep(.el-drawer__body){background:#111418;color:#e4e6ea}.detail-content{background:rgba(255,255,255,.025);min-height:100%}.detail-head h2{color:#f8fafc}.detail-status{background:rgba(255,255,255,.04);border-color:rgba(255,255,255,.08)}footer{border-top-color:rgba(255,255,255,.07);color:#717780}footer :deep(.el-pagination button),footer :deep(.el-pager li){background:rgba(255,255,255,.04);color:#aab0b8}footer :deep(.el-pager li.is-active){background:rgba(6,182,212,.22);color:#67e8f9}
@keyframes log-fade-down{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:none}}@keyframes log-rise{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:none}}@keyframes log-row{from{opacity:0;transform:translateX(-8px)}to{opacity:1;transform:none}}@keyframes log-shimmer{to{background-position:-200% 0}}@keyframes log-pulse{50%{filter:drop-shadow(0 0 7px rgba(244,63,94,.8))}}
@media(max-width:767px){.records-page{padding:0}.summary-grid{grid-template-columns:repeat(2,1fr)}.desktop-table{display:none}.mobile-list{display:block}.record-card{margin:8px 0;padding:13px;border-radius:10px}.panel-toolbar{top:64px}.page-heading{padding:14px 0}.page-heading h1{font-size:20px}}@media(prefers-reduced-motion:reduce){*,*:before,*:after{animation-duration:.01ms!important;animation-iteration-count:1!important;transition-duration:.01ms!important;scroll-behavior:auto!important}}
</style>
<style scoped>
.file-cell mark,.record-title mark{padding:0 2px;border-radius:2px;background:#854d0e;color:#fef08a}
</style>

<style scoped>
.records-page{background:#020617;color:#e2e8f0;padding:2px 2px 28px;overflow:auto}
.page-heading{position:sticky;top:0;z-index:4;margin:0 -2px 14px;padding:14px 2px 12px;background:rgba(2,6,23,.94);border-bottom:1px solid #1e293b}
.page-heading h1{font-size:20px;letter-spacing:.01em;color:#f8fafc}.page-heading p{color:#94a3b8}
.summary-grid{gap:10px;margin-bottom:12px}.summary-item{min-height:68px;padding:12px 14px;border:1px solid #1e293b;border-radius:6px;background:#0f172a;box-shadow:none}.summary-item span{color:#94a3b8}.summary-item strong{color:#f8fafc}.summary-item>.el-icon{width:34px;height:34px;flex-basis:34px}.summary-item .blue{color:#60a5fa;background:#172554}.summary-item .violet{color:#c4b5fd;background:#2e1065}.summary-item .green{color:#4ade80;background:#052e16}.summary-item .gold{color:#facc15;background:#422006}
.records-panel{padding:0;border:1px solid #1e293b;border-radius:7px;background:#0b1220;box-shadow:0 16px 40px rgba(0,0,0,.22)}
.panel-toolbar{position:sticky;top:69px;z-index:3;margin:0;padding:12px;border-bottom:1px solid #1e293b;background:#0b1220}.search-row,.filter-row{gap:8px}.search-row .el-input,.search-row .el-select,.search-row :deep(.el-date-editor){--el-input-bg-color:#111c2f;--el-fill-color-blank:#111c2f;--el-border-color:#263449}.search-row :deep(.el-input__wrapper),.search-row :deep(.el-select__wrapper),.search-row :deep(.el-range-editor){box-shadow:0 0 0 1px #263449 inset;background:#111c2f}.search-row :deep(input),.search-row :deep(.el-range-input){color:#e2e8f0}.filter-row :deep(.el-segmented){--el-segmented-bg-color:#111c2f;--el-segmented-item-selected-bg-color:#1d4ed8;--el-segmented-color:#94a3b8}
.batch-bar{margin:0 12px 10px;border-color:#1d4ed8;background:#0f1d3a}.list-meta{padding:12px 14px 10px}.list-meta strong{color:#f8fafc;font-size:12px;text-transform:uppercase;letter-spacing:.12em}.list-meta span{color:#64748b}
.desktop-table{--el-table-bg-color:#0b1220;--el-table-tr-bg-color:#0b1220;--el-table-header-bg-color:#111c2f;--el-table-row-hover-bg-color:#111c2f;--el-table-border-color:#1e293b;--el-table-text-color:#cbd5e1;--el-table-header-text-color:#64748b}.desktop-table :deep(th.el-table__cell){height:34px;background:#111c2f;border-bottom:1px solid #263449;text-transform:uppercase;letter-spacing:.12em;font-size:10px}.desktop-table :deep(td.el-table__cell){height:48px;padding:7px 0;border-bottom-color:#172236;font-size:12px}.desktop-table :deep(.el-table__row){transition:background .16s ease,transform .16s ease}.desktop-table :deep(.el-table__row:hover){transform:translateX(2px)}.desktop-table :deep(.el-table__inner-wrapper:before){display:none}.file-cell>.el-icon,.file-icon{background:#172554;color:#60a5fa}.file-cell strong,.project-cell strong,.content-cell strong{color:#e2e8f0}.file-cell span,.project-cell span,.content-cell span{color:#64748b}.error-count,.error-text{color:#f87171!important}.warning-count,.warn-text{color:#fbbf24!important}.row-actions{gap:3px}.row-actions .el-button{border-color:#263449;background:#111c2f;color:#94a3b8}.row-actions .el-button:hover{color:#f8fafc;border-color:#3b82f6;background:#172554}
.skeleton-table{padding:0 12px 12px}.skeleton-row{display:grid;grid-template-columns:1.4fr 1fr .5fr 1fr 1fr .5fr;gap:16px;padding:14px 8px;border-bottom:1px solid #172236}.skeleton-row i{height:10px;border-radius:3px;background:linear-gradient(90deg,#111c2f,#1e293b,#111c2f);background-size:200% 100%;animation:skeleton 1.4s ease-in-out infinite}.skeleton-row i:first-child{height:14px}@keyframes skeleton{to{background-position:-200% 0}}
.record-drawer :deep(.el-drawer){background:#0b1220}.record-drawer :deep(.el-drawer__body){padding:0;background:#0b1220;color:#e2e8f0}.detail-content{padding:22px}.detail-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.detail-head h2{margin:6px 0 0;color:#f8fafc;font-size:18px;word-break:break-word}.eyebrow{color:#60a5fa;font-size:10px;letter-spacing:.16em}.detail-status{display:flex;align-items:center;gap:8px;margin:18px 0;padding:9px 10px;border:1px solid #1e293b;border-radius:5px;background:#111c2f;color:#cbd5e1;font-size:12px}.detail-status .status-dot{width:7px;height:7px;background:#38bdf8}.detail-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin:0}.detail-grid div{min-width:0}.detail-grid dt{margin-bottom:4px;color:#64748b;font-size:10px;text-transform:uppercase;letter-spacing:.1em}.detail-grid dd{margin:0;color:#e2e8f0;font:12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}.detail-actions{display:flex;gap:8px;margin-top:24px}.detail-actions .el-button{flex:1}.mobile-list{background:#0b1220}.record-card{border-bottom-color:#1e293b}.record-card.selected{background:#111c2f}.record-title strong{color:#e2e8f0}.record-title span,.record-details,.record-card-foot code{color:#64748b}.record-card:hover{background:#111c2f}
footer{padding:14px;color:#64748b;border-top:1px solid #1e293b}footer :deep(.el-pagination button),footer :deep(.el-pager li){background:#111c2f;color:#94a3b8}footer :deep(.el-pager li.is-active){background:#1d4ed8;color:#fff}
@media(max-width:767px){.panel-toolbar{top:64px}.records-page{padding:0}.page-heading{padding-left:0;padding-right:0}.records-panel{border-radius:6px}.summary-grid{grid-template-columns:repeat(2,1fr)}}
</style>

<style scoped>
.records-page{background:radial-gradient(circle at 50% -10%,rgba(6,182,212,.11),transparent 38%),linear-gradient(135deg,#0b0d10,#111418 55%,#0b0d10)!important;color:#e4e6ea!important}.page-heading,.summary-item,.records-panel,.record-card{background:rgba(255,255,255,.035)!important;border-color:rgba(255,255,255,.07)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.05),0 12px 36px rgba(0,0,0,.24)!important}.page-heading{background:rgba(11,13,16,.82)!important;border-bottom-color:rgba(255,255,255,.08)!important}.records-panel{padding:0!important}.panel-toolbar{background:rgba(17,20,24,.72)!important;border-bottom-color:rgba(255,255,255,.07)!important}.desktop-table{--el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.035);--el-table-row-hover-bg-color:rgba(6,182,212,.08);--el-table-border-color:rgba(255,255,255,.07);--el-table-text-color:#cbd5e1;--el-table-header-text-color:#64748b}.desktop-table :deep(th.el-table__cell){background:rgba(255,255,255,.035);border-bottom-color:rgba(255,255,255,.08)}.desktop-table :deep(td.el-table__cell){border-bottom-color:rgba(255,255,255,.06)}.file-cell>.el-icon,.file-icon{background:rgba(56,189,248,.1);border-color:rgba(56,189,248,.2);color:#38bdf8}.file-cell strong,.project-cell strong,.content-cell strong,.record-title strong{color:#e4e6ea}.file-cell span,.project-cell span,.content-cell span,.record-title span,.record-details,.record-card-foot code{color:#717780}.error-count,.error-text{color:#f43f5e!important}.warning-count,.warn-text{color:#f59e0b!important}.row-actions .el-button{border-color:rgba(255,255,255,.1);background:rgba(255,255,255,.04);color:#94a3b8}.row-actions .el-button:hover{border-color:rgba(56,189,248,.5);color:#67e8f9;background:rgba(6,182,212,.13)}.skeleton-row i{background:linear-gradient(90deg,rgba(255,255,255,.03),rgba(255,255,255,.11),rgba(255,255,255,.03));background-size:200% 100%;animation:log-shimmer 1.4s ease-in-out infinite}.record-drawer :deep(.el-drawer),.record-drawer :deep(.el-drawer__body){background:#111418;color:#e4e6ea}.detail-content{background:rgba(255,255,255,.025)}footer{border-top-color:rgba(255,255,255,.07);color:#717780}footer :deep(.el-pagination button),footer :deep(.el-pager li){background:rgba(255,255,255,.04);color:#aab0b8}footer :deep(.el-pager li.is-active){background:rgba(6,182,212,.22);color:#67e8f9}
@keyframes log-shimmer{to{background-position:-200% 0}}@media(prefers-reduced-motion:reduce){*,*:before,*:after{animation-duration:.01ms!important;animation-iteration-count:1!important;transition-duration:.01ms!important;scroll-behavior:auto!important}}.records-page.low-performance .glass-panel{}.records-page.low-performance:before{display:none}.records-page.low-performance .desktop-table :deep(.el-table__row),.records-page.low-performance .summary-item,.records-page.low-performance .page-heading{animation:none!important}.command-trigger{border:1px solid rgba(255,255,255,.1)!important;background:rgba(255,255,255,.04)!important;color:#aab0b8!important;border-radius:8px!important}.command-trigger:hover{border-color:rgba(6,182,212,.5)!important;color:#67e8f9!important}.command-trigger span{margin-left:5px}.command-key{padding:2px 5px;border:1px solid rgba(255,255,255,.12);border-radius:4px;font:10px ui-monospace,monospace}.command-dialog :deep(.el-dialog){overflow:hidden;border:1px solid rgba(255,255,255,.1);border-radius:14px;background:rgba(17,20,24,.92);box-shadow:0 22px 70px rgba(0,0,0,.52)}.command-dialog :deep(.el-dialog__header),.command-dialog :deep(.el-dialog__body){padding:0}.command-head{display:flex;align-items:center;justify-content:space-between;padding:14px 16px 10px}.command-dialog :deep(.el-input){margin:0 14px;width:calc(100% - 28px)}.command-dialog :deep(.el-input__wrapper){background:rgba(0,0,0,.2);box-shadow:0 0 0 1px rgba(255,255,255,.1) inset}.command-list{padding:10px 8px 8px}.command-list button{display:grid;grid-template-columns:28px 1fr auto;align-items:center;width:100%;padding:11px 10px;border:0;border-radius:8px;background:transparent;color:#e4e6ea;text-align:left;cursor:pointer}.command-list button:hover{background:rgba(6,182,212,.12);color:#67e8f9}.command-list button span{color:#38bdf8;font-size:17px}.command-list button small{color:#717780}.command-list p{padding:18px;color:#717780;text-align:center}
</style>
