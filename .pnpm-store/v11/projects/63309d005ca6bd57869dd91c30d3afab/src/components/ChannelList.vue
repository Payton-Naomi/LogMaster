<script setup>
import { computed } from 'vue'
import { Button, Empty, Switch, Tag, Tooltip } from 'ant-design-vue'
import { CloudUploadOutlined, DatabaseOutlined, ReloadOutlined, WarningOutlined } from '@ant-design/icons-vue'

const props = defineProps({ devices: { type: Array, default: () => [] }, selectedId: { type: String, default: '' }, busy: Boolean })
const emit = defineEmits(['select', 'toggle', 'refresh'])
const statusText = (device) => {
  if (device.noLogAlert) return '长时间无日志'
  if (!device.detected) return '当前未检测到'
  return ({ disconnected: '已关闭', connecting: '打开中', collecting: '采集中', error: '无法访问', disk_full: '磁盘不足' })[device.status] || device.status
}
const statusClass = (device) => device.noLogAlert || device.status === 'disk_full' ? 'warning' : device.status === 'error' ? 'danger' : device.enabled ? 'success' : 'muted'
const lastLog = (value) => {
  if (!value) return '尚未收到日志'
  const date = new Date(value)
  const pad = (number, size = 2) => String(number).padStart(size, '0')
  return `[${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}]`
}
const totalStorage = computed(() => props.devices.reduce((sum, item) => sum + Number(item.storageBytes || 0), 0))
const bytes = (value) => value >= 1024 ** 3 ? `${(value / 1024 ** 3).toFixed(1)} GB` : value >= 1024 ** 2 ? `${(value / 1024 ** 2).toFixed(1)} MB` : `${Math.ceil(value / 1024)} KB`
</script>

<template>
  <aside class="channel-panel">
    <div class="panel-title">
      <div><h1>串口通道</h1><p>检测到 {{ devices.filter((item) => item.detected).length }} 个设备</p></div>
      <Tooltip title="刷新串口"><Button aria-label="刷新串口" :disabled="busy" shape="circle" @click="emit('refresh')"><template #icon><ReloadOutlined /></template></Button></Tooltip>
    </div>

    <div class="channel-list">
      <article v-for="device in devices" :key="device.deviceId" class="channel-card" :class="{ selected: selectedId === device.deviceId }" @click="emit('select', device.deviceId)">
        <div class="channel-card__top">
          <span class="port-icon"><DatabaseOutlined /></span>
          <div class="channel-name"><strong>{{ device.name || device.portName }}</strong><small>{{ device.portName }}</small></div>
          <Switch class="capsule-switch" :checked="Boolean(device.enabled)" :loading="device.status === 'connecting'" :disabled="busy || !device.detected" @change="(value) => emit('toggle', device, value)" @click.stop />
        </div>
        <div class="channel-state"><span class="state-dot" :class="statusClass(device)"></span><span>{{ statusText(device) }}</span><Tooltip v-if="device.lastError" :title="device.lastError"><WarningOutlined class="error-icon" /></Tooltip></div>
        <div class="channel-meta"><span>{{ device.config?.projectName || '未配置项目' }}</span><span>{{ device.config?.version || '—' }}</span></div>
        <div class="channel-time"><strong>{{ lastLog(device.lastLogAt) }}</strong></div>
        <div class="channel-badges">
          <Tag><DatabaseOutlined />{{ bytes(device.storageBytes || 0) }}</Tag>
          <Tag v-if="device.config?.uploadEnabled" color="processing"><CloudUploadOutlined />待上传 {{ device.pendingUploads || 0 }}</Tag>
          <Tag v-if="device.configStatus !== 'saved'" color="warning">{{ device.configStatus === 'invalid' ? '配置失效' : '未配置串口参数' }}</Tag>
        </div>
      </article>
      <Empty v-if="!devices.length" class="empty-state" description="没有检测到串口" />
    </div>

    <div class="panel-storage"><DatabaseOutlined /><span>所有通道本地占用</span><strong>{{ bytes(totalStorage) }}</strong></div>
  </aside>
</template>
