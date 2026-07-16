<template>
  <div class="task-page">
    <div class="page-heading">
      <div>
        <h1>解析任务</h1>
        <p>查看日志解析进度、测试场景和问题统计</p>
      </div>
      <el-button type="primary" :icon="Upload" @click="router.push('/upload')">上传日志</el-button>
    </div>

    <div class="summary-grid">
      <div v-for="item in summary" :key="item.label" class="summary-item">
        <span class="summary-icon" :class="item.tone"><el-icon><component :is="item.icon" /></el-icon></span>
        <div><span>{{ item.label }}</span><strong>{{ item.value }}</strong></div>
      </div>
    </div>

    <section class="task-panel">
      <div class="filters">
        <el-input v-model="filters.keyword" :prefix-icon="Search" clearable placeholder="搜索任务名称、项目或文件" class="keyword-input" />
        <el-select v-model="filters.status" clearable placeholder="全部状态" class="filter-select">
          <el-option label="排队中" value="queued" />
          <el-option label="解析中" value="running" />
          <el-option label="已完成" value="success" />
          <el-option label="解析失败" value="failed" />
        </el-select>
        <el-select v-model="filters.scene" clearable placeholder="全部场景" class="scene-select">
          <el-option label="通用解析" value="通用解析" />
          <el-option label="开关机测试" value="开关机测试" />
          <el-option label="SD 卡挂测" value="SD 卡挂测" />
        </el-select>
        <el-date-picker v-model="filters.dateRange" type="daterange" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" class="date-filter" />
        <el-button :icon="Refresh" title="刷新" @click="refreshTasks" />
      </div>

      <el-table :data="pagedTasks" class="task-table" @row-click="openTask">
        <el-table-column label="任务信息" min-width="250">
          <template #default="scope">
            <div class="task-name-cell">
              <span class="file-icon"><el-icon><Files /></el-icon></span>
              <div>
                <strong>{{ scope.row.name }}</strong>
                <span>{{ scope.row.project }} · {{ scope.row.fileCount }} 个文件 / {{ scope.row.size }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="scene" label="测试场景" min-width="130">
          <template #default="scope"><el-tag effect="plain" type="info">{{ scope.row.scene }}</el-tag></template>
        </el-table-column>
        <el-table-column label="解析状态" min-width="170">
          <template #default="scope">
            <div v-if="scope.row.status === 'running'" class="progress-cell">
              <div><span>解析中</span><b>{{ scope.row.progress }}%</b></div>
              <el-progress :percentage="scope.row.progress" :show-text="false" :stroke-width="5" />
            </div>
            <el-tag v-else :type="statusMeta[scope.row.status].type" effect="plain">
              {{ statusMeta[scope.row.status].label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="日志行数" min-width="110">
          <template #default="scope">{{ scope.row.lines ? scope.row.lines.toLocaleString() : '-' }}</template>
        </el-table-column>
        <el-table-column label="发现问题" min-width="100">
          <template #default="scope">
            <span v-if="scope.row.status === 'success'" class="issue-value" :class="{ clean: scope.row.issues === 0 }">
              {{ scope.row.issues }}
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" min-width="165" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="scope">
            <div class="row-actions" @click.stop>
              <el-button type="primary" link @click="openTask(scope.row)">查看</el-button>
              <el-dropdown trigger="click" @command="(command) => handleCommand(command, scope.row)">
                <el-button text circle :icon="MoreFilled" title="更多操作" />
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="scope.row.status === 'failed'" command="retry" :icon="RefreshRight">重新解析</el-dropdown-item>
                    <el-dropdown-item command="rename" :icon="Edit">重命名</el-dropdown-item>
                    <el-dropdown-item command="delete" :icon="Delete" divided>删除任务</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer">
        <span>共 {{ filteredTasks.length }} 个任务</span>
        <el-pagination v-model:current-page="page" :page-size="pageSize" :total="filteredTasks.length" layout="prev, pager, next" />
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, markRaw, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CircleCheck,
  Clock,
  Delete,
  Edit,
  Files,
  Loading,
  MoreFilled,
  Refresh,
  RefreshRight,
  Search,
  Upload,
  Warning
} from '@element-plus/icons-vue'

const router = useRouter()
const page = ref(1)
const pageSize = 8
const filters = reactive({ keyword: '', status: '', scene: '', dateRange: [] })
const statusMeta = {
  queued: { label: '排队中', type: 'info' },
  running: { label: '解析中', type: 'primary' },
  success: { label: '已完成', type: 'success' },
  failed: { label: '解析失败', type: 'danger' }
}

const tasks = ref([
  { id: 'TASK-20260716-008', name: 'DR2860_稳定性回归_0716', project: 'DR2860', fileCount: 4, size: '286.4 MB', scene: '开关机测试', status: 'success', progress: 100, lines: 324580, issues: 6, createdAt: '2026-07-16 14:32' },
  { id: 'TASK-20260716-007', name: 'SD64G_24H挂测', project: '存储稳定性', fileCount: 12, size: '1.8 GB', scene: 'SD 卡挂测', status: 'running', progress: 68, lines: 412906, issues: 0, createdAt: '2026-07-16 13:08' },
  { id: 'TASK-20260716-006', name: 'A800SE_循环开关机', project: 'A800SE', fileCount: 2, size: '148.7 MB', scene: '开关机测试', status: 'running', progress: 31, lines: 127482, issues: 0, createdAt: '2026-07-16 11:46' },
  { id: 'TASK-20260716-005', name: 'DR5800_基础功能冒烟', project: 'DR5800', fileCount: 3, size: '92.1 MB', scene: '通用解析', status: 'success', progress: 100, lines: 86912, issues: 0, createdAt: '2026-07-16 10:22' },
  { id: 'TASK-20260716-004', name: 'SD128G_高温录像挂测', project: '存储稳定性', fileCount: 18, size: '2.4 GB', scene: 'SD 卡挂测', status: 'failed', progress: 43, lines: 219450, issues: 0, createdAt: '2026-07-16 09:15' },
  { id: 'TASK-20260715-003', name: 'DR2820_ACC通断电测试', project: 'DR2820', fileCount: 6, size: '356.8 MB', scene: '开关机测试', status: 'success', progress: 100, lines: 507231, issues: 9, createdAt: '2026-07-15 18:21' },
  { id: 'TASK-20260715-002', name: 'AHD后录稳定性测试', project: '后录功能', fileCount: 5, size: '611.2 MB', scene: '通用解析', status: 'queued', progress: 0, lines: 0, issues: 0, createdAt: '2026-07-15 17:58' },
  { id: 'TASK-20260715-001', name: 'DR4800_看门狗重启验证', project: 'DR4800', fileCount: 2, size: '76.5 MB', scene: '开关机测试', status: 'success', progress: 100, lines: 73420, issues: 2, createdAt: '2026-07-15 16:40' },
  { id: 'TASK-20260714-012', name: 'WiFi连接压力测试', project: '无线连接', fileCount: 8, size: '524.9 MB', scene: '通用解析', status: 'success', progress: 100, lines: 612844, issues: 4, createdAt: '2026-07-14 20:12' },
  { id: 'TASK-20260714-011', name: 'SD32G_低速卡兼容性', project: '存储稳定性', fileCount: 9, size: '987.3 MB', scene: 'SD 卡挂测', status: 'success', progress: 100, lines: 815292, issues: 17, createdAt: '2026-07-14 17:36' }
])

const summary = computed(() => [
  { label: '全部任务', value: tasks.value.length, tone: 'blue', icon: markRaw(Files) },
  { label: '解析中', value: tasks.value.filter((task) => task.status === 'running').length, tone: 'gold', icon: markRaw(Loading) },
  { label: '今日完成', value: tasks.value.filter((task) => task.status === 'success' && task.createdAt.startsWith('2026-07-16')).length, tone: 'green', icon: markRaw(CircleCheck) },
  { label: '失败任务', value: tasks.value.filter((task) => task.status === 'failed').length, tone: 'red', icon: markRaw(Warning) }
])

const filteredTasks = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase()
  return tasks.value.filter((task) => {
    const matchesKeyword = !keyword || [task.name, task.project, task.id].some((value) => value.toLowerCase().includes(keyword))
    const matchesStatus = !filters.status || task.status === filters.status
    const matchesScene = !filters.scene || task.scene === filters.scene
    return matchesKeyword && matchesStatus && matchesScene
  })
})

const pagedTasks = computed(() => filteredTasks.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch(filters, () => { page.value = 1 }, { deep: true })

const openTask = (task) => router.push(`/task/${task.id}`)
const refreshTasks = () => ElMessage.success('任务列表已刷新')

const handleCommand = async (command, task) => {
  if (command === 'retry') {
    task.status = 'queued'
    task.progress = 0
    ElMessage.success('任务已重新加入解析队列')
  } else if (command === 'rename') {
    const { value } = await ElMessageBox.prompt('请输入新的任务名称', '重命名任务', { inputValue: task.name })
    task.name = value.trim() || task.name
  } else if (command === 'delete') {
    await ElMessageBox.confirm(`确定删除任务“${task.name}”吗？`, '删除任务', { type: 'warning' })
    tasks.value = tasks.value.filter((item) => item.id !== task.id)
    ElMessage.success('任务已删除')
  }
}
</script>

<style scoped>
.task-page { height: 100%; overflow: auto; color: #1f2937; box-sizing: border-box; }
.page-heading { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 18px; }
.page-heading h1 { margin: 0; font-size: 22px; line-height: 1.4; letter-spacing: 0; }
.page-heading p { margin: 5px 0 0; color: #7a8493; font-size: 14px; }

.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-bottom: 16px; }
.summary-item { display: flex; min-width: 0; align-items: center; gap: 13px; padding: 16px 18px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.summary-icon { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 5px; font-size: 20px; }
.summary-icon.blue { color: #3478dc; background: #edf4ff; }
.summary-icon.gold { color: #c9861b; background: #fff6e7; }
.summary-icon.green { color: #2f9275; background: #eaf8f3; }
.summary-icon.red { color: #d95858; background: #fff0f0; }
.summary-item > div { display: flex; flex-direction: column; gap: 4px; }
.summary-item span { color: #7a8493; font-size: 12px; }
.summary-item strong { color: #253044; font-size: 22px; }

.task-panel { padding: 18px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.filters { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.keyword-input { width: min(320px, 28%); }
.filter-select { width: 130px; }
.scene-select { width: 150px; }
.date-filter { width: 250px; }

.task-table { width: 100%; }
.task-panel :deep(.el-table__header th) { background: #f8fafc; color: #667085; font-weight: 600; }
.task-panel :deep(.el-table__row) { cursor: pointer; }
.task-name-cell { display: flex; min-width: 0; align-items: center; gap: 10px; }
.file-icon { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border-radius: 4px; background: #edf4ff; color: #3478dc; font-size: 18px; }
.task-name-cell > div { display: flex; min-width: 0; flex-direction: column; gap: 5px; }
.task-name-cell strong { overflow: hidden; color: #253044; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.task-name-cell span { color: #8b95a3; font-size: 11px; }
.progress-cell { max-width: 145px; }
.progress-cell > div { display: flex; justify-content: space-between; margin-bottom: 5px; color: #667085; font-size: 12px; }
.progress-cell b { color: #3478dc; font-weight: 600; }
.issue-value { color: #d95858; font-weight: 700; }
.issue-value.clean { color: #2f9275; }
.row-actions { display: flex; align-items: center; gap: 4px; }
.table-footer { display: flex; align-items: center; justify-content: space-between; padding-top: 16px; color: #7a8493; font-size: 12px; }

@media (max-width: 1100px) {
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .filters { flex-wrap: wrap; }
  .keyword-input { width: 100%; }
}

@media (max-width: 680px) {
  .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }
  .summary-grid { grid-template-columns: 1fr; }
  .task-panel { padding: 12px; }
  .filter-select, .scene-select, .date-filter { width: 100%; }
}
</style>
