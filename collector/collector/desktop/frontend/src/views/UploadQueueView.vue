<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Checkbox, Input, Progress, Select, Statistic, Tag } from 'ant-design-vue'
import { CopyOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue'
const props = defineProps({ devices: { type: Array, default: () => [] }, queueStatus: { type: Object, default: () => ({}) }, uploadProgress: { type: Object, default: () => new Map() }, invoke: Function })
const query = reactive({ deviceId: '', states: [], search: '', includeUploaded: false, offset: 0, limit: 100 })
const page = ref({ items: [], total: 0 })
const message = ref('')
const retrying = ref(false)
const enabledIds = computed(() => new Set(props.devices.filter((item) => item.config?.uploadEnabled).map((item) => item.deviceId)))
const groups = computed(() => [{ title: '已开启云端上传的通道', note: '', items: page.value.items?.filter((item) => enabledIds.value.has(item.deviceId)) || [] }, { title: '历史遗留', note: '通道已关闭上传，但批次仍需处理', items: page.value.items?.filter((item) => !enabledIds.value.has(item.deviceId) && item.state !== 'uploaded') || [] }])
async function load() { try { page.value = await props.invoke('GetUploadQueue', { ...query }) || { items: [], total: 0 } } catch (error) { message.value = error.message || String(error) } }
function progress(item) { const live = props.uploadProgress.get?.(item.id); const sent = live?.sentBytes ?? item.bytesSent ?? 0; const total = live?.totalBytes ?? item.bytesTotal ?? item.sizeBytes ?? 0; return { sent, total, percent: total ? Math.min(100, Math.round(sent / total * 100)) : 0, speed: live?.speedBytes ?? item.speedBytes ?? 0 } }
function bytes(value) { if (!value) return '0 B'; if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB`; if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`; return `${value} B` }
function state(value) { return ({ pending: '待上传', uploading: '上传中', uploaded: '已上传', uncertain: '待核对', dead: '上传失败' })[value] || value }
function copy(value) { if (!value) return; navigator.clipboard?.writeText(value); message.value = '查询码已复制' }
async function retry(item) {
  const method = item.state === 'dead' ? 'RetryDeadBatch' : 'RetryUncertain'
  try { retrying.value = true; await props.invoke(method, item.id); message.value = '已重新加入上传队列'; await load() } catch (error) { message.value = error.message || String(error) } finally { retrying.value = false }
}
onMounted(load)
</script>
<template>
  <main class="page-view">
    <div class="page-heading"><div><h1>上传队列</h1><p>进度来自真实发送字节；响应不确定的批次不会盲目重试。</p></div><Button @click="load"><template #icon><ReloadOutlined /></template>刷新</Button></div>
    <div class="summary-grid"><Statistic title="待上传" :value="queueStatus.pending || 0" /><Statistic title="上传中" :value="queueStatus.uploading || 0" /><Statistic title="已上传" :value="queueStatus.uploaded || 0" /><Statistic class="warning" title="待核对" :value="queueStatus.uncertain || 0" /><Statistic title="失败" :value="queueStatus.dead || 0" /></div>
    <form class="filter-bar" @submit.prevent="load">
      <Select v-model:value="query.deviceId" :options="[{value:'',label:'全部上传通道'}, ...devices.filter((item) => item.config?.uploadEnabled).map((device) => ({value:device.deviceId,label:device.portName}))]" />
      <Input v-model:value="query.search" allow-clear placeholder="查询码、设备或任务" />
      <Checkbox v-model:checked="query.includeUploaded">显示已完成</Checkbox>
      <Button type="primary" html-type="submit"><template #icon><SearchOutlined /></template>查询</Button>
    </form>
    <Alert v-if="message" class="inline-message" type="info" show-icon closable :message="message" @close="message = ''" />
    <section v-for="(group, groupIndex) in groups" v-show="groupIndex === 0 || group.items.length" :key="group.title" class="queue-group" :class="{ legacy: groupIndex === 1 }">
      <div class="section-heading"><h2>{{ group.title }}</h2><span>{{ group.note || `${group.items.length} 条` }}</span></div>
      <div class="upload-list">
        <article v-for="item in group.items" :key="item.id" class="upload-card">
          <div class="upload-main"><Tag :color="item.state === 'uploaded' ? 'success' : item.state === 'uncertain' ? 'warning' : item.state === 'dead' ? 'error' : 'processing'">{{ state(item.state) }}</Tag><div><strong>第 {{ item.uploadPosition || 1 }} 次上传 · {{ item.fileName?.split(/[\\/]/).pop() || item.id }}</strong><small>{{ item.deviceId }} · {{ item.projectName || '未配置' }} · {{ item.version || '—' }}</small></div></div>
          <div class="progress-area"><Progress :percent="progress(item).percent" size="small" :status="item.state === 'dead' ? 'exception' : item.state === 'uploaded' ? 'success' : 'active'" /><small>{{ bytes(progress(item).sent) }} / {{ bytes(progress(item).total) }} · {{ bytes(progress(item).speed) }}/s</small></div>
          <div class="query-code"><span>{{ item.queryCode || '平台未提供查询码' }}</span><Button size="small" :disabled="!item.queryCode" @click="copy(item.queryCode)"><template #icon><CopyOutlined /></template>{{ item.queryCode ? '复制' : '不可查询' }}</Button></div>
          <div v-if="item.state === 'uncertain' || item.state === 'dead'" class="upload-actions"><Button size="small" type="primary" :loading="retrying" @click="retry(item)"><template #icon><ReloadOutlined /></template>重新上传</Button></div>
          <small class="upload-error">{{ item.lastError || `重试 ${item.attemptCount || 0} 次` }}</small>
        </article>
        <div v-if="!group.items.length" class="table-empty">当前没有上传记录</div>
      </div>
    </section>
  </main>
</template>
