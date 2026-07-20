<template>
  <div class="page">
    <header class="page-heading">
      <div><h1>解析任务</h1><p>展示数据库中的日志上传与解析任务</p></div>
      <el-button type="primary" :icon="Upload" @click="router.push('/upload')">上传日志</el-button>
    </header>

    <div class="summary-grid">
      <div v-for="item in summary" :key="item.label" class="summary-item">
        <el-icon :class="item.tone"><component :is="item.icon" /></el-icon>
        <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
      </div>
    </div>

    <section class="panel">
      <div class="filters">
        <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索任务、项目或文件名" />
        <el-select v-model="status" clearable placeholder="全部状态">
          <el-option v-for="(meta, key) in statusMeta" :key="key" :label="meta.label" :value="key" />
        </el-select>
        <el-button :icon="Refresh" :loading="loading" title="刷新" @click="loadTasks" />
      </div>

      <el-table v-loading="loading" :data="pagedTasks" @row-click="openTask">
        <el-table-column label="任务" min-width="280">
          <template #default="scope">
            <div class="task-cell">
              <el-icon><Files /></el-icon>
              <div><strong>{{ scope.row.name }}</strong><span>{{ scope.row.id }}</span></div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="project" label="项目" min-width="130" />
        <el-table-column prop="version" label="版本" min-width="100"><template #default="scope">{{ scope.row.version || '-' }}</template></el-table-column>
        <el-table-column label="文件" min-width="140"><template #default="scope">{{ scope.row.fileCount }} 个 / {{ scope.row.size }}</template></el-table-column>
        <el-table-column label="状态" min-width="110"><template #default="scope"><el-tag :type="statusMeta[scope.row.status].type" effect="plain">{{ statusMeta[scope.row.status].label }}</el-tag></template></el-table-column>
        <el-table-column label="日志行数" min-width="110"><template #default="scope">{{ scope.row.lines.toLocaleString() }}</template></el-table-column>
        <el-table-column label="异常" min-width="90"><template #default="scope"><span :class="{ danger: scope.row.issues }">{{ scope.row.issues }}</span></template></el-table-column>
        <el-table-column prop="createdAt" label="创建时间" min-width="180" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="scope"><div @click.stop><el-button type="primary" link @click="openTask(scope.row)">查看</el-button><el-button type="danger" link @click="removeTask(scope.row)">删除</el-button></div></template>
        </el-table-column>
        <template #empty><el-empty description="数据库中暂无解析任务" /></template>
      </el-table>

      <footer><span>共 {{ filteredTasks.length }} 个任务</span><el-pagination v-model:current-page="page" :page-size="pageSize" :total="filteredTasks.length" layout="prev, pager, next" /></footer>
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheck, Files, Loading, Refresh, Search, Upload, Warning } from '@element-plus/icons-vue'
import { deleteTask, getTasks } from '@/api/task'

const router = useRouter()
const tasks = ref([])
const loading = ref(false)
const keyword = ref('')
const status = ref('')
const page = ref(1)
const pageSize = 10
const statusMeta = {
  queued: { label: '排队中', type: 'info' },
  running: { label: '解析中', type: 'primary' },
  success: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'danger' }
}

const formatSize = (bytes) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}
const normalizeStatus = (value) => ({ uploading: 'queued', queued: 'queued', parsing: 'running', completed: 'success', failed: 'failed' })[value] || 'queued'
const mapTask = (item) => ({ id: item.task_id, name: item.original_name || item.project_name, project: item.project_name, version: item.version, fileCount: item.file_count, size: formatSize(item.original_size), status: normalizeStatus(item.status), lines: item.total_lines, issues: item.error_count + item.warning_count, createdAt: new Date(item.created_at).toLocaleString('zh-CN', { hour12: false }) })

const summary = computed(() => [
  { label: '全部任务', value: tasks.value.length, tone: 'blue', icon: markRaw(Files) },
  { label: '解析中', value: tasks.value.filter((item) => item.status === 'running').length, tone: 'gold', icon: markRaw(Loading) },
  { label: '已完成', value: tasks.value.filter((item) => item.status === 'success').length, tone: 'green', icon: markRaw(CircleCheck) },
  { label: '失败', value: tasks.value.filter((item) => item.status === 'failed').length, tone: 'red', icon: markRaw(Warning) }
])
const filteredTasks = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  return tasks.value.filter((item) => (!status.value || item.status === status.value) && (!search || `${item.name}${item.project}${item.id}`.toLowerCase().includes(search)))
})
const pagedTasks = computed(() => filteredTasks.value.slice((page.value - 1) * pageSize, page.value * pageSize))

async function loadTasks() {
  loading.value = true
  try { const data = await getTasks({ page: 1, page_size: 200 }); tasks.value = data.list.map(mapTask) } finally { loading.value = false }
}
const openTask = (task) => router.push(`/task/${task.id}`)
async function removeTask(task) {
  await ElMessageBox.confirm(`确定删除“${task.name}”及其日志文件吗？`, '删除任务', { type: 'warning' })
  await deleteTask(task.id)
  ElMessage.success('任务已删除')
  await loadTasks()
}
onMounted(loadTasks)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page-heading{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page-heading h1{margin:0;font-size:22px}.page-heading p{margin:5px 0 0;color:#7a8493;font-size:13px}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px;margin-bottom:16px}.summary-item{display:flex;align-items:center;gap:13px;padding:16px 18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary-item>.el-icon{display:grid;width:38px;height:38px;place-items:center;border-radius:5px;font-size:20px}.summary-item .blue{color:#3478dc;background:#edf4ff}.summary-item .gold{color:#c9861b;background:#fff6e7}.summary-item .green{color:#2f9275;background:#eaf8f3}.summary-item .red{color:#d95858;background:#fff0f0}.summary-item div{display:flex;flex-direction:column;gap:3px}.summary-item span{color:#7a8493;font-size:12px}.summary-item strong{font-size:21px}.panel{padding:18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.filters{display:flex;gap:10px;margin-bottom:15px}.filters .el-input{width:min(360px,45%)}.filters .el-select{width:150px}.task-cell{display:flex;align-items:center;gap:10px}.task-cell>.el-icon{width:34px;height:34px;padding:8px;border-radius:4px;background:#edf4ff;color:#3478dc}.task-cell div{display:flex;min-width:0;flex-direction:column;gap:4px}.task-cell strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.task-cell span{color:#8a94a3;font:11px Consolas,monospace}.danger{color:#d95858;font-weight:700}footer{display:flex;align-items:center;justify-content:space-between;padding-top:15px;color:#7a8493;font-size:12px}@media(max-width:800px){.summary-grid{grid-template-columns:repeat(2,1fr)}.filters{flex-wrap:wrap}.filters .el-input,.filters .el-select{width:100%}}@media(max-width:520px){.summary-grid{grid-template-columns:1fr}.page-heading{align-items:flex-start;flex-direction:column;gap:12px}}
</style>
