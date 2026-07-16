<template>
  <div class="upload-page">
    <div class="page-heading">
      <div>
        <h1>日志上传与解析</h1>
        <p>导入设备日志，配置规则后生成结构化分析结果</p>
      </div>
      <div class="heading-meta">
        <span>{{ files.length }} 个文件</span>
        <span>{{ formatSize(totalSize) }}</span>
      </div>
    </div>

    <div class="workspace">
      <section class="upload-panel">
        <div class="section-title">
          <div>
            <span class="step-number">1</span>
            <h2>选择日志</h2>
          </div>
          <el-button v-if="files.length" text :icon="Delete" @click="clearFiles">清空</el-button>
        </div>

        <input ref="fileInput" class="hidden-input" type="file" multiple accept=".log,.txt,.zip,.gz" @change="handleFileSelect">
        <input ref="folderInput" class="hidden-input" type="file" multiple webkitdirectory @change="handleFolderSelect">

        <div
          class="drop-zone"
          :class="{ dragging: isDragging }"
          @click="fileInput?.click()"
          @dragenter.prevent="isDragging = true"
          @dragover.prevent
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <el-icon class="drop-icon"><UploadFilled /></el-icon>
          <div class="drop-title">拖入日志文件或文件夹</div>
          <div class="drop-subtitle">支持 LOG、TXT、ZIP、GZ，单个文件不超过 2 GB</div>
          <div class="drop-actions" @click.stop>
            <el-button type="primary" :icon="DocumentAdd" @click="fileInput?.click()">选择文件</el-button>
            <el-button :icon="FolderOpened" @click="folderInput?.click()">选择文件夹</el-button>
          </div>
        </div>

        <div class="file-list-header">
          <span>待处理文件</span>
          <span>{{ files.length }} 项</span>
        </div>

        <div class="file-list">
          <div v-if="!files.length" class="empty-files">
            <el-icon><Document /></el-icon>
            <span>尚未添加日志文件</span>
          </div>
          <div v-for="file in files" :key="file.id" class="file-row">
            <div class="file-type">{{ fileExtension(file.name) }}</div>
            <div class="file-info">
              <div class="file-name" :title="file.path">{{ file.name }}</div>
              <div class="file-detail">
                <span>{{ formatSize(file.size) }}</span>
                <span v-if="file.path !== file.name">{{ file.path }}</span>
              </div>
              <el-progress
                v-if="file.status === 'parsing'"
                :percentage="file.progress"
                :show-text="false"
                :stroke-width="3"
              />
            </div>
            <el-tag v-if="file.status === 'completed'" size="small" type="success" effect="plain">已完成</el-tag>
            <el-tag v-else-if="file.status === 'parsing'" size="small" type="primary" effect="plain">解析中</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">待解析</el-tag>
            <el-button
              class="remove-button"
              text
              circle
              :icon="Close"
              :disabled="isParsing"
              title="移除文件"
              @click="removeFile(file.id)"
            />
          </div>
        </div>
      </section>

      <section class="analysis-panel">
        <div class="section-title">
          <div>
            <span class="step-number">2</span>
            <h2>日志解析</h2>
          </div>
          <el-tag v-if="analysisResult" type="success" effect="plain">解析完成</el-tag>
        </div>

        <el-form label-position="top" class="analysis-form">
          <div class="form-grid">
            <el-form-item label="日志格式">
              <el-select v-model="options.format" style="width: 100%">
                <el-option label="自动识别" value="auto" />
                <el-option label="系统日志" value="syslog" />
                <el-option label="JSON Lines" value="jsonl" />
                <el-option label="纯文本" value="text" />
              </el-select>
            </el-form-item>
            <el-form-item label="字符编码">
              <el-select v-model="options.encoding" style="width: 100%">
                <el-option label="UTF-8" value="utf-8" />
                <el-option label="GBK" value="gbk" />
                <el-option label="自动识别" value="auto" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="解析规则">
            <el-select v-model="options.rule" style="width: 100%">
              <el-option label="设备通用规则（推荐）" value="device-default" />
              <el-option label="开关机测试规则" value="power-cycle" />
              <el-option label="SD 卡挂测规则" value="sd-card-aging" />
              <el-option label="Android 系统日志" value="android" />
              <el-option label="服务端应用日志" value="server" />
            </el-select>
          </el-form-item>
          <div class="switch-row">
            <div>
              <strong>合并重复异常</strong>
              <span>相同错误堆栈合并为一项</span>
            </div>
            <el-switch v-model="options.mergeDuplicates" />
          </div>
        </el-form>

        <div class="parse-actions">
          <el-button
            type="primary"
            size="large"
            :icon="VideoPlay"
            :loading="isParsing"
            :disabled="!files.length"
            @click="startParsing"
          >
            {{ isParsing ? '正在解析' : '开始解析' }}
          </el-button>
          <el-button v-if="analysisResult" size="large" :icon="RefreshRight" @click="resetAnalysis">重新解析</el-button>
        </div>

        <div v-if="isParsing" class="progress-block">
          <div class="progress-copy">
            <div>
              <strong>{{ progressStage }}</strong>
              <span>{{ currentFileName }}</span>
            </div>
            <b>{{ overallProgress }}%</b>
          </div>
          <el-progress :percentage="overallProgress" :show-text="false" :stroke-width="10" />
          <div class="progress-meta">
            <span>已处理 {{ parsedLines.toLocaleString() }} 行</span>
            <span>预计剩余 {{ remainingSeconds }} 秒</span>
          </div>
        </div>

        <div v-else-if="analysisResult" class="result-block">
          <div class="result-summary">
            <div>
              <span>日志总行数</span>
              <strong>{{ analysisResult.totalLines.toLocaleString() }}</strong>
            </div>
            <div class="error-stat">
              <span>异常</span>
              <strong>{{ analysisResult.errors }}</strong>
            </div>
            <div class="warning-stat">
              <span>警告</span>
              <strong>{{ analysisResult.warnings }}</strong>
            </div>
            <div>
              <span>耗时</span>
              <strong>{{ analysisResult.duration }}</strong>
            </div>
          </div>

          <div class="result-heading">
            <h3>主要问题</h3>
            <el-button type="primary" link @click="showAllResults">查看完整结果</el-button>
          </div>
          <div class="issue-list">
            <div v-for="issue in analysisResult.issues" :key="issue.title" class="issue-row">
              <el-icon :class="issue.level"><WarningFilled /></el-icon>
              <div>
                <strong>{{ issue.title }}</strong>
                <span>{{ issue.module }} · 最近出现于 {{ issue.lastSeen }}</span>
              </div>
              <b>{{ issue.count }} 次</b>
            </div>
          </div>
        </div>

        <div v-else class="result-placeholder">
          <el-icon><DataAnalysis /></el-icon>
          <strong>解析结果将在这里显示</strong>
          <span>添加文件并开始解析后，可查看异常、警告和问题聚合</span>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Close,
  DataAnalysis,
  Delete,
  Document,
  DocumentAdd,
  FolderOpened,
  RefreshRight,
  UploadFilled,
  VideoPlay,
  WarningFilled
} from '@element-plus/icons-vue'

const ACCEPTED_EXTENSIONS = ['log', 'txt', 'zip', 'gz']
const MAX_FILE_SIZE = 2 * 1024 * 1024 * 1024

const fileInput = ref(null)
const folderInput = ref(null)
const isDragging = ref(false)
const isParsing = ref(false)
const overallProgress = ref(0)
const parsedLines = ref(0)
const analysisResult = ref(null)
let parseTimer = null

const files = ref([])
const options = reactive({
  format: 'auto',
  encoding: 'utf-8',
  rule: 'device-default',
  mergeDuplicates: true
})

const totalSize = computed(() => files.value.reduce((sum, file) => sum + file.size, 0))
const currentFileName = computed(() => files.value.find((file) => file.status === 'parsing')?.name || '')
const remainingSeconds = computed(() => Math.max(1, Math.ceil((100 - overallProgress.value) * 0.12)))
const progressStage = computed(() => {
  if (overallProgress.value < 18) return '读取日志文件'
  if (overallProgress.value < 72) return '匹配解析规则'
  if (overallProgress.value < 92) return '聚合异常事件'
  return '生成分析结果'
})

const fileExtension = (name) => name.split('.').pop()?.toUpperCase() || 'FILE'

const formatSize = (bytes) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

const addFiles = (entries) => {
  let rejected = 0
  entries.forEach(({ file, path }) => {
    const extension = fileExtension(file.name).toLowerCase()
    const duplicate = files.value.some((item) => item.path === path && item.size === file.size)
    if (!ACCEPTED_EXTENSIONS.includes(extension) || file.size > MAX_FILE_SIZE || duplicate) {
      rejected += 1
      return
    }
    files.value.push({
      id: `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`,
      raw: file,
      name: file.name,
      path: path || file.name,
      size: file.size,
      status: 'waiting',
      progress: 0
    })
  })
  if (rejected) ElMessage.warning(`${rejected} 个文件因格式、大小或重复未加入队列`)
  if (entries.length > rejected) analysisResult.value = null
}

const handleFileSelect = (event) => {
  addFiles(Array.from(event.target.files).map((file) => ({ file, path: file.name })))
  event.target.value = ''
}

const handleFolderSelect = (event) => {
  addFiles(Array.from(event.target.files).map((file) => ({
    file,
    path: file.webkitRelativePath || file.name
  })))
  event.target.value = ''
}

const readDirectoryEntries = (reader) => new Promise((resolve, reject) => {
  const entries = []
  const readBatch = () => reader.readEntries((batch) => {
    if (!batch.length) resolve(entries)
    else {
      entries.push(...batch)
      readBatch()
    }
  }, reject)
  readBatch()
})

const walkEntry = async (entry, parentPath = '') => {
  const path = parentPath ? `${parentPath}/${entry.name}` : entry.name
  if (entry.isFile) {
    const file = await new Promise((resolve, reject) => entry.file(resolve, reject))
    return [{ file, path }]
  }
  if (!entry.isDirectory) return []
  const children = await readDirectoryEntries(entry.createReader())
  const nested = await Promise.all(children.map((child) => walkEntry(child, path)))
  return nested.flat()
}

const handleDrop = async (event) => {
  isDragging.value = false
  const items = Array.from(event.dataTransfer.items || [])
  const entries = items.map((item) => item.webkitGetAsEntry?.()).filter(Boolean)
  if (entries.length) {
    const nested = await Promise.all(entries.map((entry) => walkEntry(entry)))
    addFiles(nested.flat())
    return
  }
  addFiles(Array.from(event.dataTransfer.files).map((file) => ({ file, path: file.name })))
}

const handleDragLeave = (event) => {
  if (!event.currentTarget.contains(event.relatedTarget)) isDragging.value = false
}

const removeFile = (id) => {
  files.value = files.value.filter((file) => file.id !== id)
  analysisResult.value = null
}

const clearFiles = () => {
  if (isParsing.value) return
  files.value = []
  resetAnalysis()
}

const startParsing = () => {
  if (!files.value.length || isParsing.value) return
  analysisResult.value = null
  overallProgress.value = 0
  parsedLines.value = 0
  isParsing.value = true
  files.value.forEach((file) => {
    file.status = 'parsing'
    file.progress = 0
  })

  parseTimer = window.setInterval(() => {
    const increment = Math.floor(Math.random() * 5) + 2
    overallProgress.value = Math.min(100, overallProgress.value + increment)
    parsedLines.value = Math.round(184326 * overallProgress.value / 100)
    files.value.forEach((file, index) => {
      file.progress = Math.min(100, overallProgress.value + index * 4)
    })
    if (overallProgress.value >= 100) finishParsing()
  }, 260)
}

const finishParsing = () => {
  window.clearInterval(parseTimer)
  parseTimer = null
  isParsing.value = false
  files.value.forEach((file) => {
    file.status = 'completed'
    file.progress = 100
  })
  analysisResult.value = {
    totalLines: 184326,
    errors: 128,
    warnings: 486,
    duration: '8.4 s',
    issues: [
      { title: 'Camera service initialization failed', module: 'camera_service', lastSeen: '14:32:08', count: 46, level: 'error' },
      { title: 'Network request timeout', module: 'network_manager', lastSeen: '14:31:42', count: 31, level: 'warning' },
      { title: 'Storage space below threshold', module: 'storage_monitor', lastSeen: '14:29:17', count: 18, level: 'warning' }
    ]
  }
  ElMessage.success('日志解析完成')
}

const resetAnalysis = () => {
  if (parseTimer) window.clearInterval(parseTimer)
  parseTimer = null
  isParsing.value = false
  overallProgress.value = 0
  parsedLines.value = 0
  analysisResult.value = null
  files.value.forEach((file) => {
    file.status = 'waiting'
    file.progress = 0
  })
}

const showAllResults = () => ElMessage.info('完整结果页将在后端接口对接后启用')

onBeforeUnmount(() => {
  if (parseTimer) window.clearInterval(parseTimer)
})
</script>

<style scoped>
.upload-page {
  height: 100%;
  overflow: auto;
  color: #1f2937;
  box-sizing: border-box;
}

.page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 18px;
}

.page-heading h1 {
  margin: 0;
  font-size: 22px;
  line-height: 1.4;
  letter-spacing: 0;
}

.page-heading p {
  margin: 5px 0 0;
  color: #7a8493;
  font-size: 14px;
}

.heading-meta {
  display: flex;
  gap: 18px;
  color: #667085;
  font-size: 13px;
}

.workspace {
  display: grid;
  grid-template-columns: minmax(480px, 1.08fr) minmax(420px, 0.92fr);
  gap: 18px;
  min-height: calc(100% - 76px);
}

.upload-panel,
.analysis-panel {
  min-width: 0;
  padding: 22px;
  background: #fff;
  border: 1px solid #dfe3e8;
  border-radius: 6px;
  box-sizing: border-box;
}

.section-title,
.section-title > div {
  display: flex;
  align-items: center;
}

.section-title {
  justify-content: space-between;
  min-height: 32px;
  margin-bottom: 18px;
}

.section-title > div {
  gap: 10px;
}

.section-title h2 {
  margin: 0;
  font-size: 17px;
  letter-spacing: 0;
}

.step-number {
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 50%;
  background: #eaf2ff;
  color: #2468d8;
  font-size: 13px;
  font-weight: 700;
}

.hidden-input {
  display: none;
}

.drop-zone {
  display: flex;
  min-height: 218px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 24px;
  border: 1px dashed #aeb8c5;
  border-radius: 6px;
  background: #f8fafc;
  cursor: pointer;
  transition: border-color 0.2s, background-color 0.2s;
  box-sizing: border-box;
}

.drop-zone:hover,
.drop-zone.dragging {
  border-color: #3378e3;
  background: #f2f7ff;
}

.drop-icon {
  margin-bottom: 12px;
  color: #3378e3;
  font-size: 42px;
}

.drop-title {
  color: #243044;
  font-size: 16px;
  font-weight: 600;
}

.drop-subtitle {
  margin-top: 7px;
  color: #8a94a3;
  font-size: 13px;
}

.drop-actions {
  display: flex;
  margin-top: 20px;
  gap: 8px;
}

.file-list-header {
  display: flex;
  justify-content: space-between;
  margin: 22px 0 10px;
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

.file-list {
  max-height: 310px;
  overflow-y: auto;
  border-top: 1px solid #edf0f3;
}

.empty-files {
  display: flex;
  height: 110px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 8px;
  color: #9aa3af;
  font-size: 13px;
}

.empty-files .el-icon {
  font-size: 24px;
}

.file-row {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) auto 32px;
  min-height: 70px;
  align-items: center;
  gap: 11px;
  border-bottom: 1px solid #edf0f3;
}

.file-type {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 4px;
  background: #eef4ff;
  color: #316bc4;
  font-size: 10px;
  font-weight: 700;
}

.file-info {
  min-width: 0;
}

.file-name {
  overflow: hidden;
  color: #253044;
  font-size: 14px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-detail {
  display: flex;
  min-width: 0;
  gap: 10px;
  margin-top: 5px;
  color: #9099a6;
  font-size: 12px;
}

.file-detail span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-info :deep(.el-progress) {
  margin-top: 7px;
}

.remove-button {
  color: #8a94a3;
}

.analysis-form {
  padding-bottom: 16px;
  border-bottom: 1px solid #edf0f3;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.analysis-form :deep(.el-form-item) {
  margin-bottom: 15px;
}

.analysis-form :deep(.el-form-item__label) {
  padding-bottom: 6px;
  color: #596273;
  line-height: 20px;
}

.analysis-form :deep(.el-select__wrapper) {
  min-height: 40px;
}

.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 2px;
}

.switch-row > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.switch-row strong {
  font-size: 14px;
}

.switch-row span {
  color: #8b95a3;
  font-size: 12px;
}

.parse-actions {
  display: flex;
  margin: 18px 0;
  gap: 8px;
}

.parse-actions .el-button--primary {
  min-width: 132px;
  font-weight: 600;
}

.progress-block {
  padding: 20px;
  border: 1px solid #dce7f7;
  border-radius: 6px;
  background: #f7faff;
}

.progress-copy,
.progress-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.progress-copy {
  margin-bottom: 14px;
}

.progress-copy > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.progress-copy strong {
  font-size: 14px;
}

.progress-copy span {
  overflow: hidden;
  color: #7d8795;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.progress-copy b {
  margin-left: 16px;
  color: #2468d8;
  font-size: 20px;
}

.progress-meta {
  margin-top: 9px;
  color: #8993a1;
  font-size: 12px;
}

.result-block {
  border-top: 1px solid #edf0f3;
}

.result-summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  padding: 18px 0;
  border-bottom: 1px solid #edf0f3;
}

.result-summary > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
  padding: 0 14px;
  border-right: 1px solid #edf0f3;
}

.result-summary > div:first-child {
  padding-left: 0;
}

.result-summary > div:last-child {
  border-right: 0;
}

.result-summary span {
  color: #818b98;
  font-size: 12px;
}

.result-summary strong {
  font-size: 20px;
}

.result-summary .error-stat strong {
  color: #d94848;
}

.result-summary .warning-stat strong {
  color: #d58a18;
}

.result-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 16px 0 6px;
}

.result-heading h3 {
  margin: 0;
  font-size: 14px;
  letter-spacing: 0;
}

.issue-row {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 58px;
  border-bottom: 1px solid #edf0f3;
}

.issue-row > .el-icon {
  font-size: 18px;
}

.issue-row > .el-icon.error {
  color: #d94848;
}

.issue-row > .el-icon.warning {
  color: #d58a18;
}

.issue-row > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.issue-row strong {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.issue-row span {
  color: #8b95a3;
  font-size: 11px;
}

.issue-row > b {
  color: #596273;
  font-size: 12px;
}

.result-placeholder {
  display: flex;
  min-height: 190px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  color: #929ba8;
  text-align: center;
}

.result-placeholder .el-icon {
  margin-bottom: 10px;
  color: #aeb7c3;
  font-size: 34px;
}

.result-placeholder strong {
  color: #6c7684;
  font-size: 14px;
}

.result-placeholder span {
  max-width: 320px;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 1180px) {
  .workspace {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 680px) {
  .page-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .workspace {
    display: block;
  }

  .analysis-panel {
    margin-top: 14px;
  }

  .upload-panel,
  .analysis-panel {
    padding: 16px;
  }

  .form-grid,
  .result-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .result-summary > div:nth-child(2) {
    border-right: 0;
  }

  .result-summary > div:nth-child(n + 3) {
    margin-top: 14px;
  }

  .drop-actions,
  .parse-actions {
    width: 100%;
    flex-direction: column;
  }

  .drop-actions .el-button,
  .parse-actions .el-button {
    width: 100%;
    margin-left: 0;
  }
}
</style>
