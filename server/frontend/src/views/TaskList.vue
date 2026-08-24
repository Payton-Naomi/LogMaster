<template>
  <div class="tasks-page">
    <header class="page-heading">
      <div><h1>解析任务</h1><p>跟踪日志解析进度，并进入任务详情查看文件与结果</p></div>
      <div class="heading-actions">
        <el-tooltip content="刷新任务" placement="bottom"><el-button :icon="Refresh" :loading="loading" aria-label="刷新任务" @click="loadTasks" /></el-tooltip>
        <el-button :type="autoRefresh ? 'success' : 'default'" @click="toggleAutoRefresh">{{ autoRefresh ? '自动刷新中' : '已暂停刷新' }}</el-button>
        <el-button type="primary" :icon="Upload" @click="router.push('/upload')">上传日志</el-button>
      </div>
    </header>

    <div class="summary-grid">
      <div v-for="item in summary" :key="item.label" class="summary-item">
        <el-icon :class="item.tone"><component :is="item.icon" /></el-icon>
        <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
      </div>
    </div>

    <section class="tasks-panel" v-loading="loading && !tasks.length">
      <div class="panel-toolbar">
        <div class="search-row">
          <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="筛选当前页文件名或任务 ID" />
          <el-select v-model="project" clearable filterable placeholder="全部项目">
            <el-option v-for="name in projects" :key="name" :label="name" :value="name" />
          </el-select>
          <el-input v-model="version" clearable placeholder="版本" />
          <el-select v-model="sortBy" placeholder="排序方式"><el-option label="最近创建" value="updated_at" /><el-option label="最早创建" value="oldest" /><el-option label="错误最多" value="errors" /></el-select>
        </div>
        <div class="filter-actions"><el-segmented v-model="status" :options="statusOptions" /><el-select v-model="aiStatus" clearable placeholder="AI 状态"><el-option v-for="item in aiStatusOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></div>
      </div>

      <div class="list-meta">
        <div><strong>任务列表</strong><span>{{ totalTasks }} 个任务</span></div>
        <div class="refresh-state" :class="{ active: activeTaskCount }"><span class="state-dot" />{{ refreshStateText }}</div>
      </div>
      <div v-if="selectedTaskIds.length" class="batch-toolbar"><span>已选择 {{ selectedTaskIds.length }} 个任务</span><el-button size="small" @click="batchAction('retry')">批量重试</el-button><el-button size="small" @click="batchAction('cancel')">批量取消</el-button><el-button type="danger" size="small" @click="batchAction('delete')">批量删除</el-button><el-button size="small" @click="selectedTaskIds = []">取消选择</el-button></div>

      <el-table ref="taskTable" class="desktop-table" :data="pagedTasks" row-key="id" @row-click="openTask">
        <el-table-column width="48" align="center"><template #header><el-checkbox :model-value="pageSelected" :indeterminate="pagePartial" aria-label="选择当前页" @change="togglePageSelection" /></template><template #default="scope"><el-checkbox :model-value="selectedTaskIds.includes(scope.row.id)" aria-label="选择任务" @click.stop @change="(checked) => toggleTask(scope.row.id, checked)" /></template></el-table-column>
        <el-table-column label="任务" min-width="300">
          <template #default="scope"><div class="task-cell"><el-icon><Document /></el-icon><div><strong>{{ scope.row.name }}</strong><span>{{ scope.row.id }}</span></div></div></template>
        </el-table-column>
        <el-table-column label="项目 / 版本" min-width="170"><template #default="scope"><div class="project-cell"><strong>{{ scope.row.project || '-' }}</strong><span>{{ scope.row.version || '未填写版本' }}</span></div></template></el-table-column>
        <el-table-column label="状态" width="145"><template #default="scope"><el-tag :type="statusMeta[scope.row.status].type" effect="plain">{{ statusMeta[scope.row.status].label }}</el-tag><el-tag v-if="scope.row.aiStatus !== 'disabled'" class="ai-status-tag" :type="aiStatusMeta[scope.row.aiStatus].type" effect="plain">AI {{ aiStatusMeta[scope.row.aiStatus].label }}</el-tag><small v-if="scope.row.errorCount" class="error-count">{{ scope.row.errorCount }} 个异常</small></template></el-table-column>
        <el-table-column label="解析进度" min-width="190">
          <template #default="scope"><div class="progress-cell"><el-progress :percentage="scope.row.progress" :status="progressStatus(scope.row)" :stroke-width="7" /><span>{{ scope.row.processedFiles }} / {{ scope.row.totalFiles }} 个文件</span></div></template>
        </el-table-column>
        <el-table-column label="日志行数" min-width="110"><template #default="scope">{{ scope.row.lines.toLocaleString() }}</template></el-table-column>
        <el-table-column prop="updatedAt" label="更新时间" min-width="180" />
        <el-table-column label="操作" width="104" align="center">
          <template #default="scope"><div class="row-actions" @click.stop><el-tooltip content="查看详情" placement="top"><el-button circle :icon="View" aria-label="查看详情" @click="openTask(scope.row)" /></el-tooltip><el-tooltip v-if="scope.row.status === 'failed'" content="重新解析" placement="top"><el-button circle type="warning" plain :icon="Refresh" aria-label="重新解析" @click="runTaskAction('retry', scope.row)" /></el-tooltip><el-tooltip v-if="['queued', 'running', 'paused'].includes(scope.row.status)" :content="scope.row.status === 'paused' ? '恢复任务' : '暂停任务'" placement="top"><el-button circle plain :icon="scope.row.status === 'paused' ? VideoPlay : VideoPause" :aria-label="scope.row.status === 'paused' ? '恢复任务' : '暂停任务'" @click="runTaskAction(scope.row.status === 'paused' ? 'resume' : 'pause', scope.row)" /></el-tooltip><el-tooltip content="删除任务" placement="top"><el-button circle type="danger" plain :icon="Delete" aria-label="删除任务" @click="removeTask(scope.row)" /></el-tooltip></div></template>
        </el-table-column>
        <template #empty><div class="empty-state"><el-empty :description="hasFilters ? '没有符合条件的解析任务' : '还没有解析任务'" /><el-button v-if="!hasFilters" type="primary" :icon="Upload" @click="router.push('/upload')">上传第一份日志</el-button></div></template>
      </el-table>

      <div class="mobile-list">
        <article v-for="task in pagedTasks" :key="task.id" class="task-card" @click="openTask(task)">
          <div class="task-card-head"><el-checkbox :model-value="selectedTaskIds.includes(task.id)" aria-label="选择任务" @click.stop @change="(checked) => toggleTask(task.id, checked)" /><div class="task-icon"><el-icon><Document /></el-icon></div><div class="task-title"><strong>{{ task.name }}</strong><span>{{ task.project || '-' }} · {{ task.version || '未填写版本' }}</span></div><el-tag :type="statusMeta[task.status].type" effect="plain">{{ statusMeta[task.status].label }}</el-tag></div>
          <div class="mobile-progress"><el-progress :percentage="task.progress" :status="progressStatus(task)" :stroke-width="7" /><span>{{ task.processedFiles }} / {{ task.totalFiles }} 个文件 · {{ task.lines.toLocaleString() }} 行</span></div>
          <div class="task-card-foot"><time>{{ task.updatedAt }}</time><div @click.stop><el-button type="primary" link :icon="View" @click="openTask(task)">查看</el-button><el-button v-if="task.status === 'failed'" type="warning" link :icon="Refresh" @click="runTaskAction('retry', task)">重试</el-button><el-button type="danger" link :icon="Delete" @click="removeTask(task)">删除</el-button></div></div>
        </article>
        <div v-if="!pagedTasks.length" class="mobile-empty"><el-empty :description="hasFilters ? '没有符合条件的解析任务' : '还没有解析任务'" /></div>
      </div>

      <footer v-if="totalTasks"><span>共 {{ totalTasks }} 个任务</span><el-pagination v-model:current-page="page" :page-size="pageSize" :total="totalTasks" :pager-count="5" layout="prev, pager, next" @current-change="loadTasks" /></footer>
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { CircleCheck, Delete, Document, Files, Loading, Refresh, Search, Upload, View, VideoPause, VideoPlay, Warning } from '@element-plus/icons-vue'
import { batchTaskAction, cancelTask, deleteTask, getTasks, pauseTask, retryTask, resumeTask } from '@/api/task'
import { getProjects } from '@/api/log'

const router = useRouter()
const route = useRoute()
const taskTable = ref(null)
const tasks = ref([])
const loading = ref(false)
const keyword = ref('')
const project = ref('')
const version = ref('')
const status = ref(['active', 'completed', 'failed'].includes(route.query.status) ? route.query.status : 'all')
const aiStatus = ref('')
const dateFilter = ref(route.query.date === 'today' ? 'today' : '')
const errorsFilter = ref(route.query.errors === '1')
const sortBy = ref('updated_at')
const autoRefresh = ref(true)
const selectedTaskIds = ref([])
const page = ref(1)
const lastUpdated = ref('')
const projects = ref([])
const pageSize = 20
const totalTasks = ref(0)
let refreshTimer = null
let stopped = false
let focusedTaskId = ''
const previousStatuses = new Map()

const statusMeta = {
  queued: { label: '排队中', type: 'info' },
  running: { label: '解析中', type: 'primary' },
  paused: { label: '已暂停', type: 'warning' },
  completed: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'danger' },
  cancelled: { label: '已取消', type: 'info' }
}
const statusOptions = [
  { label: '全部', value: 'all' },
  { label: '处理中', value: 'active' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' }
]
const aiStatusOptions = [
  { label: '排队中', value: 'queued' },
  { label: '分析中', value: 'running' },
  { label: '已完成', value: 'completed' },
  { label: '部分失败', value: 'partial_failed' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'cancelled' },
  { label: '未启用', value: 'disabled' }
]
const aiStatusMeta = {
  disabled: { label: '未启用', type: 'info' }, queued: { label: '排队中', type: 'warning' }, running: { label: '分析中', type: 'primary' }, completed: { label: '完成', type: 'success' }, partial_failed: { label: '部分失败', type: 'warning' }, failed: { label: '失败', type: 'danger' }, cancelled: { label: '已取消', type: 'info' }
}

const normalizeStatus = (value) => ({ uploading: 'queued', queued: 'queued', parsing: 'running', running: 'running', paused: 'paused', cancelled: 'cancelled', completed: 'completed', failed: 'failed' })[value] || 'queued'
const formatDate = (value) => new Date(value).toLocaleString('zh-CN', { hour12: false })
const clampProgress = (value) => Math.max(0, Math.min(100, Number(value) || 0))
const mapTask = (item) => ({
  id: item.task_id,
  name: item.original_name || item.project_name,
  project: item.project_name,
  version: item.version,
  fileCount: item.file_count || 0,
  totalFiles: item.total_files || item.file_count || 0,
  processedFiles: item.processed_files || 0,
  progress: clampProgress(item.progress),
  status: normalizeStatus(item.status),
  rawStatus: item.status,
  aiStatus: item.ai_status || 'disabled',
  lines: item.total_lines || 0,
  updatedAt: formatDate(item.updated_at),
  rawUpdatedAt: item.updated_at,
  errorCount: Number(item.error_count || 0),
  searchable: `${item.original_name}${item.project_name}${item.task_id}`.toLowerCase()
})

const activeTaskCount = computed(() => tasks.value.filter((item) => item.status === 'queued' || item.status === 'running').length)
const summary = computed(() => [
  { label: '全部任务', value: totalTasks.value, tone: 'blue', icon: markRaw(Files) },
  { label: '处理中', value: activeTaskCount.value, tone: 'gold', icon: markRaw(Loading) },
  { label: '已完成', value: tasks.value.filter((item) => item.status === 'completed').length, tone: 'green', icon: markRaw(CircleCheck) },
  { label: '失败', value: tasks.value.filter((item) => item.status === 'failed').length, tone: 'red', icon: markRaw(Warning) }
])
const refreshStateText = computed(() => autoRefresh.value && activeTaskCount.value ? `自动刷新 · ${activeTaskCount.value} 个处理中` : lastUpdated.value ? `更新于 ${lastUpdated.value}` : '等待刷新')
const filteredTasks = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  return tasks.value.filter((item) => {
    const matchesStatus = status.value === 'all' || (status.value === 'active' ? ['queued', 'running'].includes(item.status) : item.status === status.value)
    const matchesProject = !project.value || item.project === project.value
    const matchesVersion = !version.value || item.version === version.value
    const matchesAi = !aiStatus.value || item.aiStatus === aiStatus.value
    const matchesDate = dateFilter.value !== 'today' || new Date(item.rawUpdatedAt || 0).toDateString() === new Date().toDateString()
    const matchesErrors = !errorsFilter.value || item.errorCount > 0
    return matchesStatus && matchesProject && matchesVersion && matchesAi && matchesDate && matchesErrors && (!search || item.searchable.includes(search))
  })
})
const pagedTasks = computed(() => filteredTasks.value)
const hasFilters = computed(() => Boolean(keyword.value.trim() || project.value || version.value || aiStatus.value || status.value !== 'all'))
const progressStatus = (task) => task.status === 'failed' ? 'exception' : task.status === 'completed' ? 'success' : ''
const pageIds = computed(() => pagedTasks.value.map(item => item.id))
const pageSelected = computed(() => pageIds.value.length > 0 && pageIds.value.every(id => selectedTaskIds.value.includes(id)))
const pagePartial = computed(() => !pageSelected.value && pageIds.value.some(id => selectedTaskIds.value.includes(id)))
async function focusRouteTask() {
	const taskId = typeof route.query.task_id === 'string' ? route.query.task_id : ''
	if (!taskId || focusedTaskId === taskId || !tasks.value.length) return
	status.value = 'all'
	project.value = ''
	keyword.value = ''
	const index = filteredTasks.value.findIndex((item) => item.id === taskId)
	if (index < 0) return
	focusedTaskId = taskId
	page.value = Math.floor(index / pageSize) + 1
	await nextTick()
	const target = tasks.value.find((item) => item.id === taskId)
	taskTable.value?.setCurrentRow(target)
}

async function loadTasks() {
	if (loading.value) return
  loading.value = true
  try {
	const response = await getTasks({ page: page.value, page_size: pageSize, status: status.value === 'all' ? undefined : status.value === 'active' ? undefined : status.value, project: project.value || undefined, version: version.value || undefined, sort: sortBy.value, ai_status: aiStatus.value || undefined })
	const mapped = (response.list || []).map(mapTask)
	totalTasks.value = Number(response.total || 0)
	if (previousStatuses.size) {
	  mapped.forEach((item) => {
	    const previous = previousStatuses.get(item.id)
	    if (previous && previous !== item.status && (item.status === 'completed' || item.status === 'failed')) {
	      ElNotification({ title: item.status === 'completed' ? '解析完成' : '解析失败', message: `${item.name}${item.status === 'completed' ? ' 已完成解析' : ' 解析失败，请查看任务详情'}`, type: item.status === 'completed' ? 'success' : 'error', duration: 5000 })
	    }
	  })
	}
	previousStatuses.clear(); mapped.forEach((item) => previousStatuses.set(item.id, item.status))
	tasks.value = mapped
	lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
	await focusRouteTask()
  } catch {
	ElMessage.error('任务列表加载失败，请稍后重试')
  } finally {
	loading.value = false
  }
}
async function refreshLoop() {
	try {
	  await loadTasks()
	} finally {
	  if (!stopped && autoRefresh.value) refreshTimer = window.setTimeout(refreshLoop, activeTaskCount.value ? 2000 : 15000)
	}
}
function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  window.clearTimeout(refreshTimer)
  if (autoRefresh.value) refreshLoop()
}
function openTask(task) {
  const taskId = String(task?.id || task?.task_id || '').trim()
  if (!taskId) {
    ElMessage.warning('任务 ID 无效，无法查看任务')
    return
  }
  router.push({ name: 'TaskDetail', params: { taskId } })
}
async function removeTask(task) {
  try {
    await ElMessageBox.confirm(`将永久删除“${task.name}”、关联日志文件及全部分析结果。`, '删除解析任务', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
    await deleteTask(task.id)
    ElMessage.success('解析任务已删除')
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('删除失败，请稍后重试')
  }
}
async function runTaskAction(action, task) {
  try {
    const labels = { retry: '重新解析', cancel: '取消任务', pause: '暂停任务', resume: '恢复任务' }
    if (action === 'cancel' || action === 'pause') await ElMessageBox.confirm(`确认${labels[action]}“${task.name}”？`, labels[action], { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    const actions = { retry: retryTask, cancel: cancelTask, pause: pauseTask, resume: resumeTask }
    await actions[action](task.id)
    ElMessage.success(`${labels[action]}请求已提交`)
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error?.response?.data?.message || `${action}操作失败，请稍后重试`)
  }
}
function toggleTask(id, checked) {
  selectedTaskIds.value = checked ? [...new Set([...selectedTaskIds.value, id])] : selectedTaskIds.value.filter(item => item !== id)
}
function togglePageSelection(checked) {
  selectedTaskIds.value = checked ? [...new Set([...selectedTaskIds.value, ...pageIds.value])] : selectedTaskIds.value.filter(id => !pageIds.value.includes(id))
}
async function removeSelected() {
  if (!selectedTaskIds.value.length) return
  try {
    await ElMessageBox.confirm(`将永久删除选中的 ${selectedTaskIds.value.length} 个任务及其分析结果。`, '批量删除任务', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
    await batchTaskAction('delete', selectedTaskIds.value)
    selectedTaskIds.value = []
    ElMessage.success('选中任务已删除')
    await loadTasks()
  } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error('批量删除失败，请稍后重试') }
}
async function batchAction(action) {
  if (!selectedTaskIds.value.length) return
  const labels = { retry: '批量重试', cancel: '批量取消', delete: '批量删除' }
  try {
    await ElMessageBox.confirm(`确认对选中的 ${selectedTaskIds.value.length} 个任务执行${labels[action]}？`, labels[action], { type: action === 'delete' ? 'warning' : 'info', confirmButtonText: '确认', cancelButtonText: '取消' })
    const result = await batchTaskAction(action, selectedTaskIds.value)
    selectedTaskIds.value = []
    ElMessage.success(result?.message || `${labels[action]}请求已提交`)
    await loadTasks()
  } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error?.response?.data?.message || `${labels[action]}失败，请稍后重试`) }
}
async function loadProjects() {
  try { projects.value = await getProjects() || [] }
  catch { projects.value = [] }
}
onMounted(() => {
	refreshLoop()
	loadProjects()
})
watch(() => route.query.task_id, () => {
	focusedTaskId = ''
	focusRouteTask()
})
watch(() => route.query.status, (value) => {
  const next = typeof value === 'string' ? value : ''
  status.value = ['active', 'completed', 'failed'].includes(next) ? next : 'all'
})
watch(() => route.query.date, (value) => { dateFilter.value = value === 'today' ? 'today' : '' })
watch(() => route.query.errors, (value) => { errorsFilter.value = value === '1' })
watch([project, version, status, sortBy, aiStatus], () => { page.value = 1; loadTasks() })
watch(keyword, () => { page.value = 1 })
onBeforeUnmount(() => {
	stopped = true
	window.clearTimeout(refreshTimer)
})
</script>

<style scoped>
.tasks-page{height:100%;overflow:auto;color:var(--lm-text-primary)}.page-heading{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page-heading h1{margin:0;font-size:22px}.page-heading p{margin:5px 0 0;color:var(--lm-text-secondary);font-size:13px}.heading-actions{display:flex;gap:9px}
.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px;margin-bottom:16px}.summary-item{display:flex;min-height:78px;align-items:center;gap:14px;padding:16px 18px;border:1px solid var(--lm-border);border-radius:6px;background:#fff}.summary-item>.el-icon{display:grid;width:40px;height:40px;flex:0 0 40px;place-items:center;border-radius:5px;font-size:20px}.summary-item .blue{color:#3478dc;background:#edf4ff}.summary-item .gold{color:#bd7d19;background:#fff4df}.summary-item .green{color:#27936c;background:#e7f5ef}.summary-item .red{color:#cf4f4f;background:#fdecec}.summary-item div{display:flex;flex-direction:column;gap:4px}.summary-item span{color:#7a8493;font-size:12px}.summary-item strong{font-size:20px}
.tasks-panel{min-height:390px;padding:18px;border:1px solid var(--lm-border);border-radius:6px;background:#fff}.panel-toolbar{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:18px}.search-row{display:flex;min-width:0;flex:1;gap:10px}.search-row .el-input{width:min(380px,55%)}.search-row .el-select{width:180px}.list-meta{display:flex;align-items:center;justify-content:space-between;padding:0 2px 11px}.list-meta>div{display:flex;align-items:center;gap:9px}.list-meta strong{font-size:14px}.list-meta span,.refresh-state{color:#8a94a3;font-size:11px}.refresh-state{gap:7px}.state-dot{width:7px;height:7px;border-radius:50%;background:#a9b1bc}.refresh-state.active .state-dot{background:#2877e8;box-shadow:0 0 0 3px #e4effe}
.desktop-table :deep(.el-table__row){cursor:pointer}.desktop-table :deep(.el-table__row:hover>td.el-table__cell){background:#f6f9fc}.desktop-table :deep(.current-row>td.el-table__cell){background:#edf4ff!important}.task-cell{display:flex;align-items:center;gap:11px}.task-cell>.el-icon,.task-icon{display:grid;width:36px;height:36px;flex:0 0 36px;place-items:center;border-radius:5px;background:#edf4ff;color:#3478dc;font-size:17px}.task-cell div,.project-cell,.progress-cell{display:flex;min-width:0;flex-direction:column;gap:4px}.task-cell strong,.project-cell strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.task-cell span{overflow:hidden;color:#8a94a3;font:10px Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}.project-cell span,.progress-cell>span{color:#8a94a3;font-size:11px}.progress-cell .el-progress{width:100%}.row-actions{display:flex;justify-content:center;gap:7px}.row-actions .el-button{width:30px;height:30px;margin:0}.empty-state{padding:18px 0 30px}.empty-state .el-empty{padding-bottom:8px}
.error-count{display:block;margin-top:3px;color:#c94b58;font-size:10px}
.mobile-list{display:none}.task-card{padding:14px 0;border-bottom:1px solid #edf0f3}.task-card:first-child{padding-top:4px}.task-card-head{display:flex;align-items:center;gap:10px}.task-title{display:flex;min-width:0;flex:1;flex-direction:column;gap:4px}.task-title strong{overflow:hidden;font-size:13px;text-overflow:ellipsis;white-space:nowrap}.task-title span{color:#7a8493;font-size:11px}.mobile-progress{display:flex;flex-direction:column;gap:5px;margin:13px 0 11px;padding-left:46px}.mobile-progress>span{color:#7a8493;font-size:11px}.task-card-foot{display:flex;align-items:center;justify-content:space-between;padding-left:46px}.task-card-foot time{color:#8a94a3;font-size:10px}.task-card-foot .el-button{margin-left:8px}.mobile-empty{padding:20px 0}footer{display:flex;align-items:center;justify-content:space-between;padding-top:16px;color:#7a8493;font-size:12px}
@media(max-width:1050px){.summary-grid{grid-template-columns:repeat(2,1fr)}.panel-toolbar{align-items:stretch;flex-direction:column}.search-row .el-input{width:min(460px,65%)}.search-row .el-select{flex:1}}
@media(max-width:1180px){.page-heading{align-items:flex-start}.summary-grid{gap:10px}.summary-item{min-height:70px;padding:13px}.summary-item>.el-icon{width:34px;height:34px;flex-basis:34px;font-size:17px}.tasks-panel{padding:14px}.panel-toolbar{align-items:stretch;flex-direction:column}.search-row .el-input{width:min(460px,65%)}.search-row .el-select{flex:1}.panel-toolbar :deep(.el-segmented){width:100%}.panel-toolbar :deep(.el-segmented__item){flex:1}.desktop-table{display:none}.mobile-list{display:block}footer>span{display:none}}
@media(max-width:767px){.page-heading p{max-width:230px}.search-row{flex-direction:column}.search-row .el-input,.search-row .el-select{width:100%}}
@media(max-width:480px){.page-heading{gap:12px}.page-heading p{display:none}.summary-grid{grid-template-columns:1fr 1fr}.summary-item{gap:10px}.tasks-panel{padding:12px}.mobile-progress,.task-card-foot{padding-left:0}.list-meta>div>span{display:none}}
 </style>

<style>
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented {
  --el-segmented-bg-color: rgba(54, 84, 96, .12);
  --el-segmented-color: #304b57;
  --el-segmented-item-selected-bg-color: rgba(6, 150, 180, .24);
  --el-segmented-item-selected-color: #064f60;
  border: 1px solid rgba(47, 92, 110, .28);
  background: rgba(255, 255, 255, .5);
}
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented__item-selected span {
  color: #064f60;
}
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented__item {
  color: #304b57 !important;
  opacity: 1 !important;
}
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented__item-label {
  color: inherit !important;
  font-weight: 600;
}
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented__item.is-selected,
html[data-log-theme="light"] .tasks-page .panel-toolbar .el-segmented__item.is-selected .el-segmented__item-label {
  color: #064f60 !important;
}
</style>

<style scoped>
.desktop-table{--el-table-current-row-bg-color:transparent!important}.desktop-table :deep(.current-row>td.el-table__cell),.desktop-table :deep(.current-row:hover>td.el-table__cell){background:transparent!important;color:#eef2f6!important;border-top-color:rgba(103,232,249,.2)!important;border-bottom-color:rgba(103,232,249,.2)!important}.desktop-table :deep(.current-row .task-cell strong),.desktop-table :deep(.current-row .project-cell strong),.desktop-table :deep(.current-row .task-cell span),.desktop-table :deep(.current-row .project-cell span),.desktop-table :deep(.current-row .progress-cell>span){color:inherit!important}.desktop-table :deep(.current-row td:first-child){box-shadow:inset 3px 0 #38bdf8!important}
</style>

<style scoped>
.desktop-table :deep(.el-table__row:hover>td.el-table__cell){background:rgba(6,182,212,.1)!important;color:#eef2f6!important}.desktop-table :deep(.el-table__row:hover td:first-child){box-shadow:inset 2px 0 #38bdf8!important}
:global(.tasks-page){font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif!important;-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;background:radial-gradient(circle at 50% 8%,rgba(255,255,255,.11),transparent 34%),radial-gradient(circle at 50% 80%,rgba(56,189,248,.07),transparent 50%),linear-gradient(145deg,#14171b,#1b1f25 48%,#111418)!important;color:#f5f7fa!important}.tasks-page .mono,.tasks-page time,.tasks-page .task-cell span{font-family:"JetBrains Mono",Consolas,ui-monospace,monospace!important;font-variant-numeric:tabular-nums}.page-heading,.summary-item,.tasks-panel,.task-card{background:rgba(255,255,255,.085)!important;border:1px solid rgba(255,255,255,.16)!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 18px 52px rgba(0,0,0,.28)!important;border-radius:14px!important}.page-heading{padding:19px 20px!important;animation:task-rise .55s cubic-bezier(.4,0,.2,1) both}.page-heading h1,.summary-item strong,.list-meta strong,.task-cell strong,.project-cell strong{color:#f5f7fa!important}.page-heading p,.summary-item span,.task-cell span,.project-cell span,.progress-cell>span,.list-meta span,.refresh-state,footer,.task-title span,.task-card-foot time{color:#b7bec8!important}.heading-actions .el-button{border-radius:9px;transition:all .24s cubic-bezier(.4,0,.2,1)}.heading-actions .el-button--primary{border:0;background:linear-gradient(135deg,#0891b2,#06b6d4);box-shadow:0 9px 24px rgba(6,182,212,.22)}.heading-actions .el-button:active,.row-actions .el-button:active{transform:scale(.94)}.summary-grid{gap:12px!important}.summary-item{position:relative;min-height:94px;padding:16px!important;animation:task-rise .5s var(--stagger,0ms) cubic-bezier(.4,0,.2,1) both}.summary-item>.el-icon{border-radius:11px}.summary-item .blue{color:#67e8f9;background:rgba(56,189,248,.14)}.summary-item .gold{color:#fbbf24;background:rgba(245,158,11,.14)}.summary-item .green{color:#6ee7b7;background:rgba(52,211,153,.14)}.summary-item .red{color:#fb7185;background:rgba(244,63,94,.14)}.tasks-panel{padding:0!important;overflow:hidden}.panel-toolbar{padding:15px 16px!important;margin:0!important;border-bottom:1px solid rgba(255,255,255,.13);background:rgba(255,255,255,.035)}.search-row :deep(.el-input__wrapper),.search-row :deep(.el-select__wrapper){background:rgba(0,0,0,.22)!important;box-shadow:0 0 0 1px rgba(255,255,255,.17) inset!important}.search-row :deep(input){color:#f5f7fa!important}.search-row :deep(input)::placeholder{color:#aeb7c3!important}.panel-toolbar :deep(.el-segmented){--el-segmented-bg-color:rgba(0,0,0,.22);--el-segmented-item-selected-bg-color:rgba(6,182,212,.27);--el-segmented-item-selected-color:#a8f3fb;--el-segmented-color:#b7bec8}.list-meta{padding:16px!important}.state-dot{background:#9aa4b0}.refresh-state.active .state-dot{background:#34d399;box-shadow:0 0 0 3px rgba(52,211,153,.14),0 0 10px rgba(52,211,153,.5)}.desktop-table{--el-table-bg-color:transparent;--el-table-tr-bg-color:transparent;--el-table-header-bg-color:rgba(255,255,255,.075);--el-table-row-hover-bg-color:rgba(103,232,249,.13);--el-table-border-color:rgba(255,255,255,.1);--el-table-text-color:#eef2f6;--el-table-header-text-color:#b7bec8}.desktop-table :deep(th.el-table__cell){background:rgba(255,255,255,.075)!important;color:#c4ccd6!important;border-bottom-color:rgba(255,255,255,.14)!important}.desktop-table :deep(td.el-table__cell){height:48px;border-bottom-color:rgba(255,255,255,.1)!important}.desktop-table :deep(.el-table__row){animation:task-row .42s both;transition:background .24s cubic-bezier(.4,0,.2,1),transform .24s cubic-bezier(.4,0,.2,1)}.desktop-table :deep(.el-table__row:hover){transform:none}.desktop-table :deep(.el-table__row:hover td:first-child){box-shadow:inset 2px 0 #38bdf8}.desktop-table :deep(.current-row>td.el-table__cell){background:rgba(103,232,249,.16)!important}.task-cell>.el-icon,.task-icon{background:rgba(56,189,248,.12);border:1px solid rgba(56,189,248,.25);color:#67e8f9}.project-cell strong,.task-cell strong{color:#f5f7fa!important}.progress-cell :deep(.el-progress-bar__outer){background:rgba(255,255,255,.1)}.progress-cell :deep(.el-progress-bar__inner){background:linear-gradient(90deg,#0891b2,#67e8f9)}.row-actions .el-button{background:rgba(255,255,255,.075)!important;border-color:rgba(255,255,255,.16)!important;color:#c5ced8}.row-actions .el-button:hover{background:rgba(103,232,249,.18)!important;color:#b8f7ff!important}.row-actions .el-button--danger:hover{background:rgba(244,63,94,.16)!important;color:#fb7185!important}.mobile-list{background:transparent}.task-card{margin:8px 0;padding:14px!important}.task-card:hover{background:rgba(255,255,255,.13)!important;transform:none}.task-title strong{color:#f5f7fa}.empty-state{padding:34px;color:#b7bec8}.empty-state .el-empty__description{color:#b7bec8}footer{padding:13px 16px;border-top-color:rgba(255,255,255,.1)}footer :deep(.el-pager li),footer :deep(.el-pagination button){background:rgba(255,255,255,.07);color:#b7bec8}footer :deep(.el-pager li.is-active){background:rgba(6,182,212,.24);color:#a8f3fb}@keyframes task-rise{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:none}}@keyframes task-row{from{opacity:0;transform:translateY(9px)}to{opacity:1;transform:none}}@media(prefers-reduced-motion:reduce){*,*:before,*:after{animation-duration:.01ms!important;transition-duration:.01ms!important;scroll-behavior:auto!important}}
</style>
