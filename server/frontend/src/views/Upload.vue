<template>
  <div class="page upload-page">
    <header class="page-heading">
      <div><div class="eyebrow">日志处理</div><h1>日志上传</h1><p>提交日志或压缩包，后台将自动解压并创建解析任务</p></div>
      <div v-if="files.length" class="selection-summary"><strong>{{ files.length }}</strong><span>个文件</span><i /><strong>{{ formatSize(totalSize) }}</strong></div>
    </header>

    <div class="workspace">
      <section class="panel file-panel">
        <div class="panel-heading">
          <div><h2>来源文件</h2><p>支持单个或批量上传</p></div>
          <el-tooltip v-if="files.length" content="清空文件列表" placement="bottom">
            <el-button :icon="Delete" circle :disabled="filesLocked" aria-label="清空文件列表" @click="clearFiles" />
          </el-tooltip>
        </div>

        <input id="log-file-input" ref="input" type="file" hidden multiple accept=".log,.txt,.out,.csv,.zip,.gz,.tgz" :disabled="filesLocked" @change="selectFiles">
        <label
          for="log-file-input"
          class="drop-zone"
          :class="{ dragging, compact: files.length, locked: filesLocked }"
          :tabindex="filesLocked ? -1 : 0"
          role="button"
          @keydown.enter.prevent="input?.click()"
          @keydown.space.prevent="input?.click()"
          @dragenter.prevent="dragging = true"
          @dragover.prevent
          @dragleave.prevent="leaveDropZone"
          @drop.prevent="dropFiles"
        >
          <span class="upload-icon"><el-icon><UploadFilled /></el-icon></span>
          <div><strong>{{ files.length ? '继续添加文件' : '将日志拖放到这里' }}</strong><span>LOG、TXT、OUT、CSV、ZIP、GZ、TGZ，最多 {{ maxFilesPerUpload }} 个，合计不超过 {{ formatSize(maxUploadBytes) }}</span></div>
          <span class="browse-action"><el-icon><DocumentAdd /></el-icon>选择文件</span>
        </label>

        <div v-if="files.length" class="file-list">
          <div class="list-heading"><span>待上传文件</span><span>{{ files.length }} 项</span></div>
          <div v-for="(item, index) in files" :key="`${item.raw.name}:${item.raw.lastModified}`" class="file-row">
            <span class="extension">{{ extension(item.raw.name) }}</span>
            <div class="file-info"><strong :title="item.raw.name">{{ item.raw.name }}</strong><span>{{ formatSize(item.raw.size) }}</span></div>
            <span class="file-ready"><el-icon><CircleCheck /></el-icon>就绪</span>
            <el-tooltip content="移除此文件" placement="top">
              <el-button :icon="Close" text circle :disabled="filesLocked" aria-label="移除文件" @click="removeFile(index)" />
            </el-tooltip>
          </div>
        </div>

        <div v-else class="file-empty">
          <span>尚未添加来源文件</span><small>重复文件名会在上传前自动重命名</small>
        </div>
      </section>

      <aside class="panel upload-panel">
        <div class="panel-heading"><div><h2>上传设置</h2><p>设置日志归属信息</p></div><span class="step-indicator">{{ task ? '任务状态' : '准备提交' }}</span></div>
        <el-form label-position="top" @submit.prevent>
          <div class="draft-toolbar"><span>表单内容会自动保存，返回后可继续编辑。</span><el-button v-if="hasDraft" text :icon="Delete" :disabled="filesLocked" @click="clearDraft">清除草稿</el-button></div>
          <el-form-item label="项目名称" required>
            <el-select v-model="projectName" filterable placeholder="请选择项目" :disabled="filesLocked">
               <el-option v-for="item in projectOptions" :key="item" :label="item" :value="item" />
             </el-select>
             <p v-if="!projectOptions.length" class="project-warning">暂无可用项目，请联系管理员创建并授权后再上传。</p>
             <p v-else-if="projectName && !projectOptions.includes(projectName)" class="project-warning">当前项目已不可用，请重新选择有效项目。</p>
          </el-form-item>
          <el-form-item label="版本标识" required>
            <el-input v-model="version" maxlength="64" placeholder="例如 V1.2.0" :disabled="filesLocked" />
          </el-form-item>
          <div class="field-pair">
            <el-form-item label="测试任务 ID">
              <el-input v-model="testTaskId" maxlength="128" placeholder="选填" :disabled="filesLocked" />
            </el-form-item>
            <el-form-item label="测试任务名称">
              <el-input v-model="testTaskName" maxlength="256" placeholder="选填" :disabled="filesLocked" />
            </el-form-item>
          </div>
          <el-form-item label="上传人" required>
            <el-input v-model="uploaderName" maxlength="128" placeholder="必填" :disabled="filesLocked" />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="remark" type="textarea" :rows="2" maxlength="4000" show-word-limit placeholder="选填" :disabled="filesLocked" />
          </el-form-item>
          <el-form-item label="测试场景">
            <el-select v-model="scenarioIds" multiple filterable collapse-tags collapse-tags-tooltip clearable placeholder="不选择则按解析规则开关执行" :disabled="filesLocked">
              <el-option v-for="item in applicableScenarios" :key="item.id" :label="item.name" :value="item.id">
                <span>{{ item.name }}</span><span class="scenario-rule-count">{{ enabledCheckCount(item) }} 个关键词</span>
              </el-option>
            </el-select>
            <div v-if="selectedScenarios.length" class="scenario-note"><el-icon><CircleCheck /></el-icon><span>已选择 {{ selectedScenarios.length }} 个场景，共 {{ selectedCheckCount }} 个关键词</span></div>
            <div v-if="selectedScenarios.length" class="analysis-mode" :class="{ exclusive: disableParsingRules }">
              <div class="analysis-mode-heading">
                <div><strong>禁用解析规则</strong><span>控制本次分析是否忽略“解析规则”页面中的开关</span></div>
                <el-switch v-model="disableParsingRules" :disabled="filesLocked" aria-label="禁用解析规则" />
              </div>
              <div class="analysis-mode-status">
                <i />
                <span v-if="disableParsingRules"><strong>解析规则已禁用</strong>，本次仅执行所选测试场景的关键词</span>
                <span v-else><strong>解析规则未禁用</strong>，本次将同时执行测试场景和已启用的解析规则</span>
              </div>
            </div>
          </el-form-item>
        </el-form>

        <div class="upload-summary">
          <div><span>文件数量</span><strong>{{ files.length }} 个</strong></div>
          <div><span>上传大小</span><strong>{{ formatSize(totalSize) }}</strong></div>
          <div><span>分析模式</span><strong>{{ analysisModeLabel }}</strong></div>
        </div>

        <el-button class="submit-button" type="primary" size="large" :icon="Upload" :loading="submitting" :disabled="submitDisabled" @click="submit">
          {{ submitting ? `正在上传 ${uploadPercent}%` : task ? '当前批次已提交' : '上传并开始解析' }}
        </el-button>
        <p v-if="!files.length && !task" class="submit-hint">添加文件并填写项目名称后即可提交</p>

        <div v-if="task" class="task-state">
          <div class="task-heading">
            <div><span>解析任务</span><el-tooltip content="复制任务编号" placement="top"><button type="button" class="copy-task" @click="copyTaskId"><el-icon><CopyDocument /></el-icon>{{ task.task_id }}</button></el-tooltip></div>
            <el-tag :type="statusMeta.type" effect="plain" size="small">{{ statusMeta.label }}</el-tag>
          </div>
          <el-progress :percentage="progress" :status="task.status === 'failed' ? 'exception' : task.status === 'completed' ? 'success' : ''" />
          <div v-if="pollFailed" class="poll-warning"><el-icon><Warning /></el-icon>状态同步暂时中断，正在重试</div>
          <dl>
            <div><dt>日志行数</dt><dd>{{ formatNumber(task.total_lines) }}</dd></div>
            <div><dt>错误</dt><dd class="danger">{{ formatNumber(task.error_count) }}</dd></div>
            <div><dt>警告</dt><dd class="warning">{{ formatNumber(task.warning_count) }}</dd></div>
          </dl>
          <el-alert v-if="task.error_message" :title="task.error_message" type="error" :closable="false" show-icon />
          <div class="task-actions">
            <el-button v-if="task.status === 'completed'" type="primary" @click="router.push(`/analysis/${task.task_id}`)">查看解析结果</el-button>
            <el-button :plain="task.status !== 'completed'" @click="startNewUpload">上传另一批</el-button>
          </div>
        </div>

        <div v-else class="waiting-state"><span><el-icon><Clock /></el-icon></span><div><strong>等待提交</strong><p>任务进度和解析统计将在这里更新</p></div></div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CircleCheck, Clock, Close, CopyDocument, Delete, DocumentAdd, Upload, UploadFilled, Warning } from '@element-plus/icons-vue'
import { getProjects, getUploadConfig, uploadLogs } from '@/api/log'
import { getScenarios } from '@/api/scenarios'
import { getTaskDetail } from '@/api/task'

const router = useRouter()
const input = ref(null)
const files = ref([])
const projectOptions = ref([])
const scenarios = ref([])
const projectName = ref('')
const version = ref('')
const testTaskId = ref('')
const testTaskName = ref('')
const uploaderName = ref(window.localStorage.getItem('logmaster_uploader_name') || '')
const remark = ref('')
const scenarioIds = ref([])
const disableParsingRules = ref(true)
const dragging = ref(false)
const submitting = ref(false)
const task = ref(null)
const uploadPercent = ref(0)
const pollFailed = ref(false)
let pollTimer = null
let pollGeneration = 0

const maxUploadBytes = ref(2 * 1024 * 1024 * 1024)
const maxFilesPerUpload = ref(100)
const accepted = /\.(log|txt|out|csv|zip|gz|tgz)$/i
const terminalStatuses = new Set(['completed', 'failed'])
const uploadDraftKey = 'logmaster_upload_draft_v1'
const totalSize = computed(() => files.value.reduce((sum, item) => sum + item.raw.size, 0))
const filesLocked = computed(() => submitting.value || Boolean(task.value && !terminalStatuses.has(task.value.status)))
const submitDisabled = computed(() => !files.value.length || !projectName.value.trim() || !version.value.trim() || !uploaderName.value.trim() || totalSize.value > maxUploadBytes.value || files.value.length > maxFilesPerUpload.value || submitting.value || Boolean(task.value))
const applicableScenarios = computed(() => scenarios.value.filter(item => {
  const metadata = item.metadata || {}
  const published = typeof item.enabled === 'boolean' ? item.enabled : (metadata.status || 'published') === 'published'
  const allProjects = typeof item.all_projects === 'boolean' ? item.all_projects : (metadata.project_scope || 'all') === 'all'
  const selectedProjects = Array.isArray(item.projects) && item.projects.length ? item.projects : (metadata.projects || [])
  const applies = allProjects || selectedProjects.includes(projectName.value)
  return published && applies
}))
const selectedScenarios = computed(() => applicableScenarios.value.filter(item => scenarioIds.value.includes(item.id)))
const enabledCheckCount = scenario => Array.isArray(scenario?.keywords)
  ? scenario.keywords.filter(keyword => keyword.trim()).length
  : (scenario?.checks || []).filter(check => check.enabled && Array.isArray(check.keywords) && check.keywords.some(keyword => keyword.trim())).length
const selectedCheckCount = computed(() => selectedScenarios.value.reduce((sum, scenario) => sum + enabledCheckCount(scenario), 0))
const analysisModeLabel = computed(() => {
  if (!selectedScenarios.value.length) return '按解析规则开关'
  return disableParsingRules.value ? '仅测试场景' : '场景 + 解析规则'
})
const hasDraft = computed(() => Boolean(projectName.value || version.value || testTaskId.value || testTaskName.value || remark.value || scenarioIds.value.length || disableParsingRules.value !== true))

const progress = computed(() => {
  const current = task.value
  if (!current) return 0
  if (Number.isFinite(current.progress)) return Math.max(0, Math.min(100, current.progress))
  if (current.status === 'uploading') return Math.max(1, Math.min(99, uploadPercent.value))
  if (['parsing', 'running'].includes(current.status) && current.total_files > 0) {
    return Math.min(95, 30 + Math.floor((current.processed_files || 0) * 65 / current.total_files))
  }
  return ({ uploading: 10, queued: 25, parsing: 30, running: 30, completed: 100, failed: 100 })[current.status] || 0
})

const statusMeta = computed(() => ({
  uploading: { label: '上传中', type: 'info' },
  queued: { label: '排队中', type: 'info' },
  parsing: { label: '解析中', type: 'primary' },
  running: { label: '解析中', type: 'primary' },
  completed: { label: '已完成', type: 'success' },
  failed: { label: '失败', type: 'danger' }
}[task.value?.status] || { label: '等待', type: 'info' }))

async function loadSettings() {
  const [projectsResult, scenariosResult, capacityResult] = await Promise.allSettled([getProjects(), getScenarios(), getUploadConfig()])
  projectOptions.value = projectsResult.status === 'fulfilled' ? projectsResult.value || [] : []
  scenarios.value = scenariosResult.status === 'fulfilled' ? scenariosResult.value || [] : []
  if (capacityResult.status === 'fulfilled') applyUploadCapacity(capacityResult.value)
}

function applyUploadCapacity(capacity) {
  maxUploadBytes.value = Number(capacity?.max_upload_bytes) || maxUploadBytes.value
  maxFilesPerUpload.value = Number(capacity?.max_files_per_upload) || maxFilesPerUpload.value
}

async function refreshUploadCapacity() {
  const capacity = await getUploadConfig()
  applyUploadCapacity(capacity)
}

async function addFiles(items) {
  if (filesLocked.value) return
  const usedNames = new Set(files.value.map((item) => item.raw.name.toLowerCase()))
  let rejectedType = 0
  let rejectedSize = 0
  let rejectedCount = 0
  let pendingSize = totalSize.value

  for (const file of items) {
    if (files.value.length >= maxFilesPerUpload.value) { rejectedCount++; continue }
    const extensionless = !file.name.includes('.')
    if (!accepted.test(file.name) && !extensionless) { rejectedType++; continue }
    if (pendingSize + file.size > maxUploadBytes.value) { rejectedSize++; continue }
    const name = uniqueFileName(file.name, usedNames)
    const selected = name === file.name ? file : new File([file], name, { type: file.type, lastModified: file.lastModified })
    files.value.push({ raw: selected })
    pendingSize += file.size
  }

  if (rejectedType) ElMessage.warning(`${rejectedType} 个文件格式不支持`)
  if (rejectedCount) ElMessage.warning(`${rejectedCount} 个文件超出单次 ${maxFilesPerUpload.value} 个的数量限制`)
  if (rejectedSize) ElMessage.warning(`${rejectedSize} 个文件超出 ${formatSize(maxUploadBytes.value)} 总量限制`)
}

function uniqueFileName(name, usedNames) {
  const reserve = (candidate) => {
    const key = candidate.toLowerCase()
    if (usedNames.has(key)) return false
    usedNames.add(key)
    return true
  }
  if (reserve(name)) return name
  const lower = name.toLowerCase()
  const extensionStart = lower.endsWith('.tar.gz') ? name.length - 7 : name.lastIndexOf('.')
  const stem = extensionStart > 0 ? name.slice(0, extensionStart) : name
  const fileExtension = extensionStart > 0 ? name.slice(extensionStart) : ''
  const now = new Date()
  const part = (value, length = 2) => String(value).padStart(length, '0')
  const timestamp = `${now.getFullYear()}${part(now.getMonth() + 1)}${part(now.getDate())}_${part(now.getHours())}${part(now.getMinutes())}${part(now.getSeconds())}_${part(now.getMilliseconds(), 3)}`
  for (let sequence = 1; ; sequence++) {
    const suffix = sequence === 1 ? `_${timestamp}` : `_${timestamp}_${sequence}`
    const candidate = `${stem}${suffix}${fileExtension}`
    if (reserve(candidate)) return candidate
  }
}

async function selectFiles(event) {
  await addFiles(Array.from(event.target.files))
  event.target.value = ''
}

async function dropFiles(event) {
  dragging.value = false
  await addFiles(Array.from(event.dataTransfer.files))
}

function leaveDropZone(event) {
  if (!event.currentTarget.contains(event.relatedTarget)) dragging.value = false
}

function clearFiles() { if (!filesLocked.value) files.value = [] }
function removeFile(index) { if (!filesLocked.value) files.value.splice(index, 1) }
function extension(name) { return name.toLowerCase().endsWith('.tar.gz') ? 'TAR.GZ' : name.includes('.') ? name.split('.').pop()?.toUpperCase() : 'LOG' }
function formatNumber(value) { return Number(value || 0).toLocaleString() }
function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}

async function submit() {
  try {
    await refreshUploadCapacity()
  } catch {
    ElMessage.error('无法读取当前上传限制，请稍后重试')
    return
  }
  if (!projectName.value || !projectOptions.value.includes(projectName.value)) {
    ElMessage.warning('请选择有效项目后再上传，用户端不能自行创建项目')
    return
  }
  if (files.value.length > maxFilesPerUpload.value || totalSize.value > maxUploadBytes.value) {
    ElMessage.warning(`当前单次上传限制为 ${maxFilesPerUpload.value} 个文件、${formatSize(maxUploadBytes.value)}`)
    return
  }
  const generation = ++pollGeneration
  submitting.value = true
  uploadPercent.value = 0
  pollFailed.value = false
  task.value = { task_id: '等待服务器创建任务', status: 'uploading', progress: 1, total_files: files.value.length, processed_files: 0, total_lines: 0, error_count: 0, warning_count: 0, error_message: '' }
  try {
    const created = await uploadLogs(files.value.map((item) => item.raw), {
      project_name: projectName.value.trim(),
      version: version.value.trim(),
      test_task_id: testTaskId.value.trim(),
      test_task_name: testTaskName.value.trim(),
      uploader_name: uploaderName.value.trim(),
      remark: remark.value.trim(),
      client_request_id: crypto.randomUUID(),
      collector_version: 'web',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      disable_parsing_rules: selectedScenarios.value.length ? disableParsingRules.value : false,
      created_at: new Date().toISOString()
    }, scenarioIds.value, {
      onUploadProgress: (event) => {
        if (!event.total) return
        uploadPercent.value = Math.min(99, Math.round((event.loaded * 100) / event.total))
        if (task.value?.status === 'uploading') task.value.progress = uploadPercent.value
      }
    })
    task.value = { task_id: created.task_id, status: created.status, progress: 25, total_files: created.file_count || 0, processed_files: 0, total_lines: 0, error_count: 0, warning_count: 0, error_message: '' }
    ElMessage.success('日志已上传，后台开始解析')
    await poll(created.task_id, generation)
  } catch {
    task.value = null
  } finally {
    submitting.value = false
  }
}

async function poll(taskId, generation) {
  if (generation !== pollGeneration) return
  window.clearTimeout(pollTimer)
  try {
    const data = await getTaskDetail(taskId)
    if (generation !== pollGeneration) return
    task.value = data.task
    pollFailed.value = false
  } catch {
    pollFailed.value = true
  }
  if (task.value && terminalStatuses.has(task.value.status)) return
  pollTimer = window.setTimeout(() => poll(taskId, generation), 1500)
}

async function copyTaskId() {
  if (!task.value?.task_id || task.value.task_id.startsWith('等待')) return
  try {
    await navigator.clipboard.writeText(task.value.task_id)
    ElMessage.success('任务编号已复制')
  } catch {
    ElMessage.warning('无法访问剪贴板')
  }
}

function startNewUpload() {
  pollGeneration++
  window.clearTimeout(pollTimer)
  files.value = []
  version.value = ''
  testTaskId.value = ''
  testTaskName.value = ''
  remark.value = ''
  task.value = null
  uploadPercent.value = 0
  pollFailed.value = false
  clearDraft()
}

watch(projectName, () => {
  const applicableIds = new Set(applicableScenarios.value.map(item => item.id))
  scenarioIds.value = scenarioIds.value.filter(id => applicableIds.has(id))
})
watch(uploaderName, value => window.localStorage.setItem('logmaster_uploader_name', value.trim()))
watch([projectName, version, testTaskId, testTaskName, uploaderName, remark, scenarioIds, disableParsingRules], saveDraft, { deep: true })

function saveDraft() {
  if (task.value || submitting.value) return
  window.localStorage.setItem(uploadDraftKey, JSON.stringify({
    projectName: projectName.value, version: version.value, testTaskId: testTaskId.value,
    testTaskName: testTaskName.value, uploaderName: uploaderName.value, remark: remark.value,
    scenarioIds: scenarioIds.value, disableParsingRules: disableParsingRules.value
  }))
}

function restoreDraft() {
  try {
    const draft = JSON.parse(window.localStorage.getItem(uploadDraftKey) || 'null')
    if (!draft || typeof draft !== 'object') return
    projectName.value = typeof draft.projectName === 'string' ? draft.projectName : ''
    version.value = typeof draft.version === 'string' ? draft.version : ''
    testTaskId.value = typeof draft.testTaskId === 'string' ? draft.testTaskId : ''
    testTaskName.value = typeof draft.testTaskName === 'string' ? draft.testTaskName : ''
    uploaderName.value = typeof draft.uploaderName === 'string' ? draft.uploaderName : uploaderName.value
    remark.value = typeof draft.remark === 'string' ? draft.remark : ''
    scenarioIds.value = Array.isArray(draft.scenarioIds) ? draft.scenarioIds : []
    disableParsingRules.value = typeof draft.disableParsingRules === 'boolean' ? draft.disableParsingRules : true
  } catch { window.localStorage.removeItem(uploadDraftKey) }
}

function clearDraft() {
  window.localStorage.removeItem(uploadDraftKey)
  projectName.value = ''
  version.value = ''
  testTaskId.value = ''
  testTaskName.value = ''
  remark.value = ''
  scenarioIds.value = []
  disableParsingRules.value = true
}

onMounted(async () => {
  restoreDraft()
  await loadSettings()
})
onBeforeUnmount(() => {
  pollGeneration++
  window.clearTimeout(pollTimer)
})
</script>

<style scoped>
.page { height: 100%; overflow: auto; color: var(--lm-text-primary); }
.page-heading { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 16px; }
.eyebrow { margin-bottom: 5px; color: var(--lm-primary); font-size: 11px; font-weight: 700; }
.page-heading h1 { margin: 0; font-size: 22px; font-weight: 650; }.page-heading p,.panel-heading p { margin: 5px 0 0; color: var(--lm-text-secondary); font-size: 12px; }
.selection-summary { display: flex; align-items: baseline; gap: 5px; color: #778393; font-size: 11px; }.selection-summary strong { color: #344152; font-size: 14px; }.selection-summary i { width: 1px; height: 14px; margin: 0 5px; background: #d9dfe6; }
.workspace { display: grid; grid-template-columns: minmax(0,1.35fr) minmax(330px,.65fr); align-items: start; gap: 16px; }
.panel { padding: 18px; border: 1px solid var(--lm-border); border-radius: 6px; background: #fff; }.panel-heading { display: flex; align-items: flex-start; justify-content: space-between; min-height: 42px; }.panel-heading h2 { margin: 0; font-size: 15px; }.step-indicator { padding: 4px 7px; border-radius: 4px; background: #f0f3f6; color: #647183; font-size: 10px; }
.drop-zone { display: grid; min-height: 200px; grid-template-columns: 52px minmax(0,1fr) auto; align-items: center; gap: 14px; padding: 24px; border: 1px dashed #aebbc9; border-radius: 6px; background: #f8fafb; cursor: pointer; outline: none; transition: border-color 160ms ease,background 160ms ease; }.drop-zone:hover,.drop-zone:focus-visible,.drop-zone.dragging { border-color: #4d8cde; background: #f1f6fd; }.drop-zone.compact { min-height: 112px; padding-top: 18px; padding-bottom: 18px; }
.drop-zone.locked { cursor: default; opacity: .72; }.upload-icon { display: grid; width: 48px; height: 48px; place-items: center; border-radius: 6px; background: #e6f0fc; color: #2e75cf; font-size: 25px; }.drop-zone>div { display: flex; min-width: 0; flex-direction: column; gap: 6px; }.drop-zone strong { color: #344152; font-size: 14px; }.drop-zone>div span { color: #7b8796; font-size: 11px; }.browse-action { display: inline-flex; height: 34px; align-items: center; gap: 6px; padding: 0 12px; border: 1px solid #cbd5df; border-radius: 5px; background: #fff; color: #3c6fae; font-size: 12px; font-weight: 600; white-space: nowrap; }
.file-list { margin-top: 16px; }.list-heading { display: flex; justify-content: space-between; padding: 0 2px 8px; border-bottom: 1px solid #e8ecf0; color: #7b8796; font-size: 10px; }.file-row { display: grid; grid-template-columns: 46px minmax(0,1fr) 64px 32px; align-items: center; gap: 10px; min-height: 58px; padding: 8px 2px; border-bottom: 1px solid #edf0f3; }.extension { display: grid; width: 42px; height: 34px; place-items: center; border-radius: 4px; background: #edf3fb; color: #3976bf; font-size: 9px; font-weight: 700; }.file-info { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.file-info strong { overflow: hidden; color: #384555; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.file-info span { color: #8a94a3; font-size: 10px; }.file-ready { display: flex; align-items: center; gap: 4px; color: #438a70; font-size: 10px; }
.file-empty { display: flex; min-height: 76px; align-items: center; justify-content: center; flex-direction: column; gap: 5px; color: #788595; font-size: 11px; }.file-empty small { color: #9aa3ae; }
.upload-panel { position: sticky; top: 0; }.upload-panel .el-select { width: 100%; }.upload-panel :deep(.el-form-item__label) { color: #536174; font-size: 12px; font-weight: 600; }.upload-panel :deep(.el-form-item) { margin-bottom: 15px; }
.project-warning { margin: -8px 0 14px; color: #b45309; font-size: 11px; line-height: 1.5; }
.draft-toolbar{display:flex;align-items:center;justify-content:space-between;gap:10px;margin:-2px 0 14px;padding:8px 10px;border:1px solid #d8e3ee;border-radius:5px;background:#f5f8fc;color:#667789;font-size:11px}.draft-toolbar .el-button{margin:0}
.field-pair{display:grid;grid-template-columns:1fr 1fr;gap:10px}.field-pair :deep(.el-form-item){min-width:0}
.scenario-rule-count{float:right;margin-left:18px;color:#8a94a3;font-size:11px}.scenario-note{display:flex;width:100%;align-items:flex-start;gap:5px;margin-top:7px;color:#39785f;font-size:10px;line-height:1.5}.scenario-note .el-icon{margin-top:2px;flex:0 0 auto}
.analysis-mode{width:100%;margin-top:10px;overflow:hidden;border:1px solid #cfe0f3;border-radius:6px;background:#f7faff}.analysis-mode.exclusive{border-color:#f0d5ab;background:#fffaf2}.analysis-mode-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:11px 12px}.analysis-mode-heading>div{display:flex;min-width:0;flex-direction:column;gap:3px}.analysis-mode-heading strong{color:#344152;font-size:12px}.analysis-mode-heading span{color:#7b8796;font-size:10px;line-height:1.4}.analysis-mode-status{display:flex;align-items:flex-start;gap:7px;padding:8px 12px;border-top:1px solid #e3edf8;background:#eef5fd;color:#35689d;font-size:10px;line-height:1.5}.analysis-mode.exclusive .analysis-mode-status{border-top-color:#f3e1c4;background:#fff4e4;color:#97651d}.analysis-mode-status i{width:7px;height:7px;flex:0 0 auto;margin-top:4px;border-radius:50%;background:#3f83c8}.analysis-mode.exclusive .analysis-mode-status i{background:#d28b26}.analysis-mode-status strong{font-weight:700}
.upload-summary { margin: 2px 0 16px; border-top: 1px solid #e8ecf0; border-bottom: 1px solid #e8ecf0; }.upload-summary div { display: flex; align-items: center; justify-content: space-between; padding: 9px 0; color: #7b8796; font-size: 11px; }.upload-summary div+div { border-top: 1px solid #f0f2f5; }.upload-summary strong { color: #3b4857; font-size: 12px; font-weight: 600; }.submit-button { width: 100%; }.submit-hint { margin: 8px 0 0; color: #919ba7; font-size: 10px; text-align: center; }
.waiting-state { display: flex; align-items: center; gap: 11px; margin-top: 18px; padding-top: 16px; border-top: 1px solid #e8ecf0; }.waiting-state>span { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 5px; background: #f0f3f6; color: #7b8796; }.waiting-state div { display: flex; flex-direction: column; gap: 3px; }.waiting-state strong { color: #536174; font-size: 11px; }.waiting-state p { margin: 0; color: #929ca8; font-size: 10px; }
.task-state { margin-top: 18px; padding-top: 16px; border-top: 1px solid #e8ecf0; }.task-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; margin-bottom: 12px; }.task-heading>div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.task-heading>div>span { color: #536174; font-size: 11px; font-weight: 600; }.copy-task { display: flex; max-width: 230px; align-items: center; gap: 4px; overflow: hidden; padding: 0; border: 0; background: transparent; color: #8a94a3; font: 9px Consolas,monospace; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }.poll-warning { display: flex; align-items: center; gap: 5px; margin-top: 8px; color: #ae7622; font-size: 10px; }
.task-state dl { display: grid; grid-template-columns: repeat(3,1fr); margin: 15px 0; }.task-state dl div { display: flex; align-items: center; flex-direction: column; gap: 5px; border-right: 1px solid #edf0f3; }.task-state dl div:last-child { border: 0; }.task-state dt { color: #8a94a3; font-size: 10px; }.task-state dd { margin: 0; color: #344152; font-size: 17px; font-weight: 650; }.task-state dd.danger { color: var(--lm-danger); }.task-state dd.warning { color: var(--lm-warning); }.task-actions { display: flex; gap: 8px; margin-top: 14px; }.task-actions .el-button { flex: 1; margin: 0; }
@media(max-width:1000px){.workspace{grid-template-columns:minmax(0,1fr) 330px}.drop-zone{grid-template-columns:48px minmax(0,1fr)}.browse-action{grid-column:2;width:max-content}}
@media(max-width:820px){.workspace{grid-template-columns:1fr}.upload-panel{position:static}}
@media(max-width:560px){.page-heading{align-items:flex-start;flex-direction:column;gap:10px}.selection-summary{align-self:flex-end}.panel{padding:14px}.drop-zone{grid-template-columns:42px minmax(0,1fr);padding:18px 14px}.upload-icon{width:40px;height:40px}.browse-action{grid-column:1/-1;justify-self:stretch;justify-content:center}.file-row{grid-template-columns:42px minmax(0,1fr) 30px}.file-ready{display:none}.field-pair{grid-template-columns:1fr}}
</style>
<style scoped>
.page{position:relative;isolation:isolate;color:#e5edf8;padding:4px 4px 40px;background:#020617}
.page:before{position:fixed;z-index:-1;inset:0;pointer-events:none;background-image:linear-gradient(rgba(56,189,248,.035) 1px,transparent 1px),linear-gradient(90deg,rgba(56,189,248,.035) 1px,transparent 1px);background-size:28px 28px;content:'';mask-image:linear-gradient(to bottom,rgba(0,0,0,.8),transparent 78%)}
.page-heading{position:relative;align-items:flex-start;margin-bottom:22px;padding:6px 2px 0}.page-heading:after{position:absolute;right:0;bottom:0;left:0;height:1px;background:#1e293b;content:''}.eyebrow{color:#38bdf8;font-size:10px;letter-spacing:.16em;text-transform:uppercase}.page-heading h1{color:#f8fafc;font-size:28px;letter-spacing:-.02em}.page-heading p,.panel-heading p{color:#94a3b8;font-size:12px}.selection-summary{padding:7px 10px;border:1px solid #1e3a5f;border-radius:5px;background:#0b1e33;color:#7dd3fc}.selection-summary strong{color:#f8fafc}.selection-summary i{background:#27415f}
.workspace{gap:18px}.panel{position:relative;padding:20px;border:1px solid #1e293b;border-radius:9px;background:rgba(11,18,32,.9);box-shadow:0 18px 50px rgba(0,0,0,.24);}.panel:before{position:absolute;inset:0;pointer-events:none;border-radius:inherit;box-shadow:inset 0 1px 0 rgba(255,255,255,.035);content:''}.panel-heading{position:relative;margin-bottom:14px}.panel-heading h2{color:#f8fafc;font-size:15px;letter-spacing:.01em}.step-indicator{border:1px solid #263449;background:#111c2f;color:#7dd3fc}
.drop-zone{position:relative;min-height:224px;border:1px dashed #315478;border-radius:8px;background:#081525;transition:border-color .18s ease,background .18s ease,transform .18s ease,box-shadow .18s ease}.drop-zone:before{position:absolute;inset:10px;pointer-events:none;border:1px solid rgba(56,189,248,.08);border-radius:5px;content:''}.drop-zone:hover,.drop-zone:focus-visible,.drop-zone.dragging{border-color:#38bdf8;background:#0a1d32;box-shadow:0 0 0 3px rgba(56,189,248,.1),0 18px 36px rgba(2,132,199,.12);transform:translateY(-2px)}.drop-zone.locked{opacity:.55}.upload-icon{background:#082f49;color:#38bdf8;box-shadow:0 0 22px rgba(14,165,233,.18);animation:floatIcon 3s ease-in-out infinite}.drop-zone strong{color:#f8fafc;font-size:15px}.drop-zone>div span{color:#7f93aa}.browse-action{border:1px solid #2b5f87;background:#0b2036;color:#7dd3fc;transition:background .18s ease,border-color .18s ease}.browse-action:hover{border-color:#38bdf8;background:#0e304e}
.file-list{margin-top:18px}.list-heading{border-bottom-color:#1e293b;color:#64748b;text-transform:uppercase;letter-spacing:.12em}.file-row{border-bottom-color:#172236;transition:background .16s ease,transform .16s ease}.file-row:hover{background:#0f1d30;transform:translateX(4px)}.extension{background:#172554;color:#93c5fd}.file-info strong{color:#e2e8f0}.file-info span{color:#64748b}.file-ready{color:#4ade80}.file-empty{color:#94a3b8}.file-empty small{color:#64748b}
.upload-panel{top:10px}.upload-panel :deep(.el-form-item__label){color:#cbd5e1;font-size:11px;letter-spacing:.04em}.upload-panel :deep(.el-input__wrapper),.upload-panel :deep(.el-select__wrapper),.upload-panel :deep(.el-textarea__inner){box-shadow:0 0 0 1px #263449 inset;background:#0f1b2d;color:#e2e8f0;transition:box-shadow .18s ease,background .18s ease}.upload-panel :deep(.el-input__wrapper:hover),.upload-panel :deep(.el-select__wrapper:hover),.upload-panel :deep(.el-textarea__inner:hover),.upload-panel :deep(.is-focus){box-shadow:0 0 0 1px #38bdf8 inset,0 0 0 3px rgba(56,189,248,.1);background:#111f33}.upload-panel :deep(input),.upload-panel :deep(textarea),.upload-panel :deep(.el-select__selected-item){color:#e2e8f0}.upload-panel :deep(input::placeholder),.upload-panel :deep(textarea::placeholder){color:#5f748c}.field-pair{gap:12px}.scenario-note{color:#4ade80}.analysis-mode{border-color:#24527b;background:#081a2d}.analysis-mode.exclusive{border-color:#805a22;background:#261b0a}.analysis-mode-heading strong{color:#e2e8f0}.analysis-mode-heading span{color:#8298b0}.analysis-mode-status{border-top-color:#1d4668;background:#0b243b;color:#7dd3fc}.analysis-mode.exclusive .analysis-mode-status{border-top-color:#63471b;background:#35230d;color:#fbbf24}
.upload-summary{border-color:#1e293b}.upload-summary div+div{border-top-color:#172236}.upload-summary div{color:#64748b}.upload-summary strong{color:#e2e8f0}.project-warning{color:#fbbf24}.submit-button{position:relative;overflow:hidden;border:0;background:#2563eb;box-shadow:0 10px 24px rgba(37,99,235,.24);transition:transform .18s ease,box-shadow .18s ease}.submit-button:after{position:absolute;top:0;bottom:0;left:-45%;width:28%;background:rgba(255,255,255,.24);content:'';transform:skewX(-18deg);animation:buttonSweep 3.8s ease-in-out infinite}.submit-button:hover{background:#3b82f6;box-shadow:0 14px 30px rgba(37,99,235,.34);transform:translateY(-1px)}.submit-hint{color:#64748b}.waiting-state,.task-state{border-top-color:#1e293b}.waiting-state>span{background:#111c2f;color:#64748b}.waiting-state strong,.task-heading>div>span{color:#cbd5e1}.waiting-state p{color:#64748b}.task-state dl div{border-right-color:#1e293b}.task-state dt{color:#64748b}.task-state dd{color:#e2e8f0}.copy-task{color:#7dd3fc}.poll-warning{color:#fbbf24}
@keyframes floatIcon{0%,100%{transform:translateY(0)}50%{transform:translateY(-4px)}}@keyframes buttonSweep{0%,60%,100%{left:-45%;opacity:0}15%{opacity:1}45%{left:120%;opacity:0}}
@media(max-width:820px){.upload-panel{top:0}}@media(max-width:560px){.page-heading h1{font-size:23px}.panel{padding:15px}.selection-summary{align-self:flex-start}}
</style>

<style>
/* Keep the upload surface light when the app is in day mode. */
html[data-log-theme="light"] .upload-page .page-heading::after { display: none; }
html[data-log-theme="light"] .upload-page .step-indicator {
  border-color: rgba(47, 116, 137, .24);
  background: rgba(255, 255, 255, .48);
  color: #46616d;
}
html[data-log-theme="light"] .upload-page .selection-summary {
  border-color: rgba(47, 116, 137, .28);
  background: rgba(255, 255, 255, .7);
  color: #496575;
  box-shadow: 0 4px 14px rgba(47, 116, 137, .08);
}
html[data-log-theme="light"] .upload-page .selection-summary strong { color: #173f52; }
html[data-log-theme="light"] .upload-page .selection-summary i { background: rgba(47, 116, 137, .3); }
html[data-log-theme="light"] .upload-page .browse-action {
  border-color: rgba(6, 150, 180, .38);
  background: rgba(6, 150, 180, .13);
  color: #075a6d;
}
html[data-log-theme="light"] .upload-page .browse-action:hover {
  border-color: rgba(6, 150, 180, .58);
  background: rgba(6, 150, 180, .2);
}
html[data-log-theme="light"] .upload-page .submit-button {
  border-color: transparent;
  background: linear-gradient(135deg, #0891b2, #06b6d4);
  color: #fff;
  box-shadow: 0 9px 24px rgba(6, 150, 180, .22);
}
html[data-log-theme="light"] .upload-page .submit-button:hover {
  background: linear-gradient(135deg, #0e7490, #0891b2);
}
html[data-log-theme="light"] body #app .upload-page .analysis-mode.exclusive {
  border-color: #e6bd71;
  background: #fffaf0;
}
html[data-log-theme="light"] body #app .upload-page .analysis-mode.exclusive .analysis-mode-status {
  border-top-color: #efd6a6;
  background: #fff1d6;
  color: #754606 !important;
}
html[data-log-theme="light"] body #app .upload-page .analysis-mode.exclusive .analysis-mode-status span { color: #754606 !important; }
html[data-log-theme="light"] body #app .upload-page .analysis-mode.exclusive .analysis-mode-status strong { color: #5d3500 !important; }
html[data-log-theme="light"] body #app .upload-page .analysis-mode.exclusive .analysis-mode-status i { background: #bd7609; }
</style>
