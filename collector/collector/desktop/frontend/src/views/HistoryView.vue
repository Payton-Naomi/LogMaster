<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Alert, Button, DatePicker, Input, Modal, Radio, RadioGroup, Select, Table, Tag, Tooltip } from 'ant-design-vue'
import { CloudUploadOutlined, CopyOutlined, DeleteOutlined, EyeOutlined, FolderOpenOutlined, ReloadOutlined, SaveOutlined, SearchOutlined } from '@ant-design/icons-vue'
const props = defineProps({ devices: { type: Array, default: () => [] }, invoke: Function })
const query = reactive({ deviceId: '', projectId: '', version: '', testTaskId: '', search: '', state: '', from: '', to: '', offset: 0, limit: 50 })
const page = ref({ items: [], total: 0 })
const preview = ref(null)
const message = ref('')
const dateRange = ref([])
const deleteDialog = reactive({ open: false, item: null, mode: 'record' })
const columns = [
  { title: '文件 / 串口', key: 'file', width: 240 },
  { title: '业务配置', key: 'business', width: 190 },
  { title: '时间', key: 'time', width: 180 },
  { title: '大小 / 行数', key: 'size', width: 120 },
  { title: '上传状态', key: 'state', width: 150 },
  { title: '操作', key: 'actions', width: 210, fixed: 'right' },
]
function updateDates(values) { query.from = values?.[0] || ''; query.to = values?.[1] || '' }
async function load() { try { page.value = await props.invoke('ListHistory', { ...query }) || { items: [], total: 0 } } catch (error) { message.value = error.message || String(error) } }
async function show(item) { try { preview.value = await props.invoke('ReadHistoryPreview', item.id) } catch (error) { message.value = error.message || String(error) } }
async function action(name, ...args) { try { await props.invoke(name, ...args); message.value = '操作已完成'; await load() } catch (error) { message.value = error.message || String(error) } }
function askDelete(item) { deleteDialog.item = item; deleteDialog.mode = 'record'; deleteDialog.open = true }
async function confirmDelete() {
  try {
    await props.invoke('DeleteHistoryFile', deleteDialog.item.id, deleteDialog.mode === 'record-and-file')
    message.value = deleteDialog.mode === 'record-and-file' ? '历史记录和本地文件已删除' : '历史记录已删除，本地文件已保留'
    deleteDialog.open = false
    await load()
  } catch (error) { message.value = error.message || String(error) }
}
function copy(value) { navigator.clipboard?.writeText(value); message.value = '已复制 SHA-256' }
function bytes(value) { if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(2)} GB`; if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB`; return `${Math.ceil(value / 1024)} KB` }
onMounted(load)
</script>
<template>
  <main class="page-view">
    <div class="page-heading"><h1>历史文件</h1><Tooltip title="刷新历史文件"><Button aria-label="刷新历史文件" shape="circle" @click="load"><template #icon><ReloadOutlined /></template></Button></Tooltip></div>
    <form class="filter-bar" @submit.prevent="load">
      <Select v-model:value="query.deviceId" :options="[{value:'',label:'全部串口'}, ...devices.map((device) => ({value:device.deviceId,label:device.portName}))]" />
      <Input v-model:value="query.search" allow-clear placeholder="文件名、SHA-256 或查询码" />
      <Select v-model:value="query.state" :options="[{value:'',label:'全部状态'},{value:'pending',label:'待上传'},{value:'uploading',label:'上传中'},{value:'uploaded',label:'已上传'},{value:'uncertain',label:'待核对'},{value:'dead',label:'失败'}]" />
      <DatePicker.RangePicker v-model:value="dateRange" value-format="YYYY-MM-DD" @change="updateDates" />
      <Button type="primary" html-type="submit"><template #icon><SearchOutlined /></template>查询</Button>
    </form>
    <Alert v-if="message" class="inline-message" type="info" show-icon closable :message="message" @close="message = ''" />
    <Table class="history-table" size="small" row-key="id" :columns="columns" :data-source="page.items || []" :pagination="{ total: page.total || 0, pageSize: query.limit, showSizeChanger: false }" :scroll="{ x: 1100 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'file'"><strong>{{ record.fileName }}</strong><small>{{ record.portName }}</small></template>
        <template v-else-if="column.key === 'business'">{{ record.projectName || '—' }}<small>{{ record.version || '—' }} · {{ record.testTaskName || '—' }}</small></template>
        <template v-else-if="column.key === 'time'">{{ new Date(record.completedAt).toLocaleString('zh-CN', { hour12: false }) }}</template>
        <template v-else-if="column.key === 'size'">{{ bytes(record.sizeBytes) }}<small>{{ record.lineCount }} 行</small></template>
        <template v-else-if="column.key === 'state'"><Tag :color="record.uploadState === 'uploaded' ? 'success' : record.uploadState === 'uncertain' ? 'warning' : record.uploadState === 'dead' ? 'error' : 'default'">{{ record.uploadState || '仅本地' }}</Tag><small>{{ record.queryCode || '平台未提供查询码' }}</small></template>
        <template v-else-if="column.key === 'actions'"><div class="row-actions">
          <Tooltip title="预览"><Button aria-label="预览" shape="circle" size="small" @click="show(record)"><template #icon><EyeOutlined /></template></Button></Tooltip>
          <Tooltip title="打开目录"><Button aria-label="打开目录" shape="circle" size="small" @click="action('OpenLogFolder', record.path)"><template #icon><FolderOpenOutlined /></template></Button></Tooltip>
          <Tooltip title="另存本次会话"><Button aria-label="另存本次会话" shape="circle" size="small" @click="action('SaveLogAs', record.deviceId, record.sessionId, 'session', '')"><template #icon><SaveOutlined /></template></Button></Tooltip>
          <Tooltip title="复制 SHA-256"><Button aria-label="复制 SHA-256" shape="circle" size="small" @click="copy(record.sha256)"><template #icon><CopyOutlined /></template></Button></Tooltip>
          <Tooltip title="手动上传"><Button aria-label="手动上传" shape="circle" size="small" :disabled="['pending','uploading','uncertain'].includes(record.uploadState)" @click="action('EnqueueHistoryFile', record.id)"><template #icon><CloudUploadOutlined /></template></Button></Tooltip>
          <Tooltip v-if="record.uploadState === 'local'" title="删除"><Button aria-label="删除历史文件" danger shape="circle" size="small" @click="askDelete(record)"><template #icon><DeleteOutlined /></template></Button></Tooltip>
        </div></template>
      </template>
      <template #emptyText>没有符合条件的历史文件</template>
    </Table>
    <Modal :open="Boolean(preview)" :title="preview?.file?.fileName" width="900px" :footer="null" @cancel="preview = null"><pre class="history-preview">{{ preview?.lines?.join('\n') }}</pre><Alert v-if="preview?.truncated" type="info" message="预览仅显示前 500 行。" /></Modal>
    <Modal :open="deleteDialog.open" title="删除未上传文件" ok-text="确认删除" cancel-text="取消" @ok="confirmDelete" @cancel="deleteDialog.open = false">
      <RadioGroup v-model:value="deleteDialog.mode" class="delete-options">
        <Radio value="record"><strong>仅删除记录</strong><span>本地日志文件继续保留。</span></Radio>
        <Radio value="record-and-file"><strong>同时删除本地文件</strong><span>历史记录和磁盘中的日志文件都会删除。</span></Radio>
      </RadioGroup>
    </Modal>
  </main>
</template>

<style scoped>
.delete-options { display: grid; gap: 10px; width: 100%; }
.delete-options :deep(.ant-radio-wrapper) { align-items: flex-start; margin: 0; padding: 12px; border: 1px solid #d9d9d9; border-radius: 6px; }
.delete-options :deep(.ant-radio-wrapper span:last-child) { display: grid; gap: 3px; }
.delete-options span { color: #667085; }
</style>
