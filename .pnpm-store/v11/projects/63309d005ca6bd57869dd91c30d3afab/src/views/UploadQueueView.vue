<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Alert, Button, Checkbox, Input, Modal, Progress, Select, Statistic, Tag, Tooltip } from 'ant-design-vue'
import { CopyOutlined, DeleteOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue'
const props = defineProps({ devices: { type: Array, default: () => [] }, queueStatus: { type: Object, default: () => ({}) }, active: Boolean, invoke: Function })
const query = reactive({ deviceId: '', states: [], search: '', includeUploaded: false, offset: 0, limit: 200 })
const page = ref({ items: [], total: 0 }), message = ref(''), running = ref(false)
let timer
function notice(value) { message.value = value; if (value) window.setTimeout(() => { if (message.value === value) message.value = '' }, 5000) }
async function load() { try { page.value = await props.invoke('GetUploadQueue', { ...query }) || { items: [], total: 0 } } catch (e) { notice(e.message || String(e)) } }
watch(() => props.active, (active) => { if (active) load() }, { immediate: true })
function bytes(value) { if (!value) return '0 B'; if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB`; if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`; return `${value} B` }
function state(value) { return ({ pending: '待上传', uploading: '上传中', uploaded: '已完成', uncertain: '待核对', dead: '失败' })[value] || value }
function reason(item) { const v = String(item.lastError || '').toLowerCase(); if (item.state === 'uncertain') return '等待平台核对'; if (/decompress|gzip|unzip/.test(v)) return '解压失败'; if (/parse|解析/.test(v)) return '解析失败'; if (/401|403|auth|认证/.test(v)) return '认证失败'; if (/timeout|超时/.test(v)) return '请求超时'; if (/network|socket|connect|网络/.test(v)) return '网络中断'; if (/storage|disk|磁盘/.test(v)) return '存储失败'; if (/400|invalid|参数/.test(v)) return '配置无效'; return '上传失败' }
function percent(item) { return item.bytesTotal ? Math.min(100, Math.round(item.bytesSent / item.bytesTotal * 100)) : 0 }
async function retry(item) { try { running.value = true; const result = await props.invoke('RetryUploadTask', item); notice(`已处理：成功 ${result.succeeded}，跳过 ${result.skipped}，失败 ${result.failed}`); await load() } catch (e) { notice(e.message || String(e)) } finally { running.value = false } }
function clearUploadHistory() {
  Modal.confirm({ title: '清空上传历史记录', content: '将清空所有上传任务、失败记录和查询码，本地原始日志不会删除。是否继续？', okText: '继续', okType: 'danger', cancelText: '取消', onOk() {
    Modal.confirm({ title: '再次确认清空', content: '待上传任务也会被移除，之后不会自动上传。确认清空全部上传历史记录？', okText: '确认清空', okType: 'danger', cancelText: '取消', async onOk() {
      try { running.value = true; const deleted = await props.invoke('ClearUploadHistory'); notice(`已清空 ${deleted} 条上传历史记录`); await load() } catch (e) { notice(e.message || String(e)); throw e } finally { running.value = false }
    } })
  } })
}
function copy(value) { if (value) { navigator.clipboard?.writeText(value); notice('查询码已复制') } }
const visible = computed(() => page.value.items.filter((item) => query.includeUploaded || item.state !== 'uploaded'))
onMounted(() => { load(); timer = window.setInterval(() => { if (props.active) load() }, 15000) })
onBeforeUnmount(() => window.clearInterval(timer))
</script>
<template>
  <main class="page-view"><div class="page-heading"><h1>上传队列</h1><div class="page-actions"><Tooltip title="清空上传历史记录"><Button danger shape="circle" aria-label="清空上传历史记录" :disabled="running || !page.total" @click="clearUploadHistory"><template #icon><DeleteOutlined /></template></Button></Tooltip><Tooltip title="刷新上传任务"><Button shape="circle" aria-label="刷新上传任务" @click="load"><template #icon><ReloadOutlined /></template></Button></Tooltip></div></div>
    <div class="summary-grid"><Statistic title="待上传" :value="queueStatus.pending || 0" /><Statistic title="上传中" :value="queueStatus.uploading || 0" /><Statistic title="已完成" :value="queueStatus.uploaded || 0" /><Statistic class="warning" title="待核对" :value="queueStatus.uncertain || 0" /><Statistic title="失败" :value="queueStatus.dead || 0" /></div>
    <form class="filter-bar" @submit.prevent="load"><Select v-model:value="query.deviceId" :options="[{value:'',label:'全部串口'}, ...devices.map((x) => ({value:x.deviceId,label:x.portName}))]" /><Input v-model:value="query.search" allow-clear placeholder="项目、任务或查询码" /><Checkbox v-model:checked="query.includeUploaded">显示已完成</Checkbox><Button type="primary" html-type="submit"><template #icon><SearchOutlined /></template>查询</Button></form><Alert v-if="message" class="inline-message" type="info" show-icon :message="message" />
    <section class="upload-list"><article v-for="item in visible" :key="item.id" class="upload-card"><div class="upload-main"><Tag :color="item.state === 'uploaded' ? 'success' : item.state === 'dead' ? 'error' : item.state === 'uncertain' ? 'warning' : 'processing'">{{ state(item.state) }}</Tag><div><strong>{{ item.projectName || '未配置项目' }} - {{ item.uploaderName || item.uploaderEmail || '未识别上传人' }} - {{ item.testTaskName || '未配置任务' }} - {{ item.portName || item.deviceId || '未知串口' }}</strong><small>{{ item.fileCount }} 个文件 · {{ bytes(item.bytesTotal) }}</small></div></div><div class="progress-area"><Progress :percent="percent(item)" size="small" :status="item.state === 'dead' ? 'exception' : item.state === 'uploaded' ? 'success' : 'active'" /><small>{{ bytes(item.bytesSent) }} / {{ bytes(item.bytesTotal) }}{{ item.speedBytes ? ` · ${bytes(item.speedBytes)}/s` : '' }}</small></div><div class="query-code"><span>{{ item.queryCode || '平台未提供查询码' }}</span><Button size="small" :disabled="!item.queryCode" @click="copy(item.queryCode)"><template #icon><CopyOutlined /></template></Button></div><div v-if="item.state === 'dead' || item.state === 'uncertain'" class="upload-actions"><small class="upload-error">{{ reason(item) }}</small><Button size="small" type="primary" :loading="running" @click="retry(item)"><template #icon><ReloadOutlined /></template>重新上传</Button></div></article><div v-if="!visible.length" class="table-empty">当前没有上传任务</div></section>
  </main>
</template>
