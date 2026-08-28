<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, DatePicker, Input, Modal, Select, Table, Tag, Tooltip } from 'ant-design-vue'
import { CloudUploadOutlined, CopyOutlined, DeleteOutlined, EyeOutlined, FolderOpenOutlined, ReloadOutlined, SaveOutlined, SearchOutlined } from '@ant-design/icons-vue'
const props = defineProps({ devices: { type: Array, default: () => [] }, invoke: Function })
const query = reactive({ deviceId: '', projectId: '', version: '', testTaskId: '', search: '', state: '', from: '', to: '', offset: 0, limit: 50 })
const page = ref({ items: [], total: 0 })
const groupedItems = computed(() => {
  const groups = new Map()
  for (const item of page.value.items || []) {
    const key = item.sessionId || item.id
    const current = groups.get(key)
    if (!current) {
      const stamp = new Date(item.completedAt)
      const pad = (value) => String(value).padStart(2, '0')
      const timestamp = Number.isNaN(stamp.getTime()) ? '' : `${stamp.getFullYear()}${pad(stamp.getMonth() + 1)}${pad(stamp.getDate())}${pad(stamp.getHours())}${pad(stamp.getMinutes())}${pad(stamp.getSeconds())}`
      const safe = (value) => String(value || '未配置').replace(/[<>:"/\\|?*]+/g, '_')
      groups.set(key, { ...item, fileName: `${safe(item.deviceId || item.portName)}-${safe(item.portName)}-${timestamp}-${safe(item.projectName)}.log`, segmentCount: 1, sizeBytes: Number(item.sizeBytes || 0), lineCount: Number(item.lineCount || 0) })
      continue
    }
    current.segmentCount++
    current.sizeBytes += Number(item.sizeBytes || 0)
    current.lineCount += Number(item.lineCount || 0)
    if (new Date(item.completedAt) > new Date(current.completedAt)) current.completedAt = item.completedAt
    if (Number(item.lastSequence || 0) > Number(current.lastSequence || 0)) current.lastSequence = item.lastSequence
  }
  return [...groups.values()]
})
const preview = ref(null)
const message = ref('')
const dateRange = ref([])
const columns = [
  { title: '文件 / 串口', key: 'file', width: 240 },
  { title: '业务配置', key: 'business', width: 190 },
  { title: '时间', key: 'time', width: 180 },
  { title: '大小 / 行数', key: 'size', width: 120 },
  { title: '上传状态', key: 'state', width: 150 },
  { title: '操作', key: 'actions', width: 210, fixed: 'right' },
]
let messageTimer
function setMessage(value) { message.value = value; clearTimeout(messageTimer); if (value) messageTimer = window.setTimeout(() => { message.value = '' }, 5000) }
const projectOptions = () => [...new Map(props.devices.map((item) => [item.config?.projectId, item.config?.projectName]).filter(([id]) => id)).entries()].map(([value, label]) => ({ value, label }))
const taskOptions = () => [...new Map(props.devices.map((item) => [item.config?.testTaskId, item.config?.testTaskName]).filter(([id]) => id)).entries()].map(([value, label]) => ({ value, label }))
function updateDates(values) { query.from = values?.[0] || ''; query.to = values?.[1] || '' }
async function load() { try { page.value = await props.invoke('ListHistory', { ...query }) || { items: [], total: 0 } } catch (error) { setMessage(error.message || String(error)) } }
async function show(item) { try { preview.value = await props.invoke('ReadHistoryPreview', item.id) } catch (error) { setMessage(error.message || String(error)) } }
async function action(name, ...args) { try { await props.invoke(name, ...args); setMessage('操作已完成'); await load() } catch (error) { setMessage(error.message || String(error)) } }
function askDelete(item) { Modal.confirm({ title: '删除历史文件', content: `将删除该采集会话的 ${item.segmentCount || 1} 个本地日志文件及其历史记录，无法恢复。确认删除？`, okText: '确认删除', okType: 'danger', cancelText: '取消', async onOk() { try { const count = await props.invoke('DeleteHistorySession', item.sessionId, item.id); setMessage(`已删除 ${count} 个本地日志文件及记录`); await load() } catch (error) { setMessage(error.message || String(error)); throw error } } }) }
function copy(value) { navigator.clipboard?.writeText(value); setMessage('已复制 SHA-256') }
function bytes(value) { if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(2)} GB`; if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB`; return `${Math.ceil(value / 1024)} KB` }
onMounted(load)
</script>
<template>
  <main class="page-view">
    <div class="page-heading"><h1>历史文件</h1><Tooltip title="刷新历史文件"><Button aria-label="刷新历史文件" shape="circle" @click="load"><template #icon><ReloadOutlined /></template></Button></Tooltip></div>
    <form class="filter-bar" @submit.prevent="load">
      <Select v-model:value="query.deviceId" :options="[{value:'',label:'全部串口'}, ...devices.map((device) => ({value:device.deviceId,label:device.portName}))]" />
      <Select v-model:value="query.projectId" allow-clear :options="[{value:'',label:'全部项目'}, ...projectOptions()]" />
      <Select v-model:value="query.testTaskId" allow-clear :options="[{value:'',label:'全部任务'}, ...taskOptions()]" />
      <Input v-model:value="query.version" allow-clear placeholder="版本号" />
      <Input v-model:value="query.search" allow-clear placeholder="文件名、SHA-256 或查询码" />
      <Select v-model:value="query.state" :options="[{value:'',label:'全部状态'},{value:'pending',label:'待上传'},{value:'uploading',label:'上传中'},{value:'uploaded',label:'已上传'},{value:'uncertain',label:'待核对'},{value:'dead',label:'失败'}]" />
      <DatePicker.RangePicker v-model:value="dateRange" value-format="YYYY-MM-DD" @change="updateDates" />
      <Button type="primary" html-type="submit"><template #icon><SearchOutlined /></template>查询</Button>
    </form>
    <Alert v-if="message" class="inline-message" type="info" show-icon closable :message="message" @close="message = ''" />
    <div class="history-note">同一次采集已按会话合并展示；底层分片仍保留，用于断点续传和崩溃恢复。</div>
    <Table class="history-table" size="small" row-key="id" :columns="columns" :data-source="groupedItems" :pagination="{ total: groupedItems.length, pageSize: query.limit, showSizeChanger: false }" :scroll="{ x: 1100 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'file'"><strong>{{ record.fileName }}</strong><small>{{ record.portName }} · {{ record.segmentCount || 1 }} 个分片</small></template>
        <template v-else-if="column.key === 'business'">{{ record.projectName || '—' }}<small>{{ record.version || '—' }} · {{ record.testTaskName || '—' }}</small></template>
        <template v-else-if="column.key === 'time'">{{ new Date(record.completedAt).toLocaleString('zh-CN', { hour12: false }) }}</template>
        <template v-else-if="column.key === 'size'">{{ bytes(record.sizeBytes) }}<small>{{ record.lineCount }} 行</small></template>
        <template v-else-if="column.key === 'state'"><Tag :color="record.uploadState === 'uploaded' ? 'success' : record.uploadState === 'uncertain' ? 'warning' : record.uploadState === 'dead' ? 'error' : 'default'">{{ record.uploadState || '仅本地' }}</Tag><small>{{ record.queryCode || '平台未提供查询码' }}</small></template>
        <template v-else-if="column.key === 'actions'"><div class="row-actions">
          <Tooltip title="预览"><Button aria-label="预览" shape="circle" size="small" @click="show(record)"><template #icon><EyeOutlined /></template></Button></Tooltip>
          <Tooltip title="打开目录"><Button aria-label="打开目录" shape="circle" size="small" @click="action('OpenLogFolder', record.path)"><template #icon><FolderOpenOutlined /></template></Button></Tooltip>
          <Tooltip title="另存本次会话"><Button aria-label="另存本次会话" shape="circle" size="small" @click="action('SaveLogAs', record.deviceId, record.sessionId, 'session', '')"><template #icon><SaveOutlined /></template></Button></Tooltip>
          <Tooltip title="复制 SHA-256"><Button aria-label="复制 SHA-256" shape="circle" size="small" @click="copy(record.sha256)"><template #icon><CopyOutlined /></template></Button></Tooltip>
          <Tooltip title="手动上传"><Button aria-label="手动上传" shape="circle" size="small" :disabled="record.segmentCount > 1 || ['pending','uploading','uncertain'].includes(record.uploadState)" @click="action('EnqueueHistoryFile', record.id)"><template #icon><CloudUploadOutlined /></template></Button></Tooltip>
          <Tooltip title="删除本地文件和记录"><Button aria-label="删除历史文件" danger shape="circle" size="small" @click="askDelete(record)"><template #icon><DeleteOutlined /></template></Button></Tooltip>
        </div></template>
      </template>
      <template #emptyText>没有符合条件的历史文件</template>
    </Table>
    <Modal :open="Boolean(preview)" :title="preview?.file?.fileName" width="900px" :footer="null" @cancel="preview = null"><pre class="history-preview">{{ preview?.lines?.join('\n') }}</pre><Alert v-if="preview?.truncated" type="info" message="预览仅显示前 500 行。" /></Modal>
  </main>
</template>

<style scoped>
.history-note { margin-top: 12px; color: #667085; font-size: 12px; }
</style>
