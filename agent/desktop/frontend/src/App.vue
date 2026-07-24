<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  Cable, ChevronDown, CircleStop, Database, HardDrive, Link, Link2Off,
  ListRestart, Pause, Play, RefreshCw, Search, Send, Settings, Terminal,
  TriangleAlert, WifiOff, X,
} from '@lucide/vue'

const MAX_LOG_LINES = 2000
const levels = ['全部', 'ERROR', 'WARN', 'INFO', 'DEBUG']
const activeView = ref('collector')
const selectedDevice = ref('')
const search = ref('')
const level = ref('全部')
const paused = ref(false)
const follow = ref(true)
const command = ref('')
const busy = ref(false)
const serviceReady = ref(false)
const banner = ref('正在连接本地采集服务...')
const ports = ref([])
const devices = ref([])
const logs = reactive(new Map())
const queue = ref({ pending: 0, uploading: 0, uploaded: 0, uncertain: 0, dead: 0 })
const queueBatches = ref([])
const confirmations = reactive({})
const pausedSnapshots = reactive(new Map())
const logConsole = ref(null)
let disposeLogs
let disposeState

const api = () => window.go?.main?.Service

const emptyDevice = { deviceId: '', name: '选择设备', portName: '', status: 'disconnected', config: { deviceId: '', name: '', portName: '', baudRate: 115200, dataBits: 8, stopBits: 1, parity: 'none', handshake: 'none', dtr: false, rts: false } }
const selected = computed(() => devices.value.find((item) => item.deviceId === selectedDevice.value) || devices.value[0] || emptyDevice)
const visibleLogs = computed(() => {
  const rows = paused.value ? (pausedSnapshots.get(selected.value?.deviceId) || []) : (logs.get(selected.value?.deviceId) || [])
  const term = search.value.trim().toLowerCase()
  return rows.filter((row) => {
    const levelMatch = level.value === '全部' || row.level === level.value
    const textMatch = !term || `${row.message} ${row.module || ''}`.toLowerCase().includes(term)
    return levelMatch && textMatch
  })
})

function statusLabel(status) {
  return ({ connected: '已连接', collecting: '采集中', disconnected: '未连接', reconnecting: '重连中', error: '错误', stopped: '已停止', disk_full: '磁盘已满' })[status] || status || '未知'
}

function statusClass(status) {
  if (status === 'collecting' || status === 'connected') return 'ok'
  if (status === 'reconnecting' || status === 'disk_full') return 'warn'
  if (status === 'error') return 'danger'
  return 'muted'
}

async function invoke(name, ...args) {
  const service = api()
  if (!service?.[name]) throw new Error(`本地接口 ${name} 不可用`)
  return service[name](...args)
}

async function refreshAll() {
  try {
    const [portResult, stateResult, queueResult, batchResult] = await Promise.all([
      invoke('ScanPorts'), invoke('GetDeviceStates'), invoke('GetUploadQueueStatus'), invoke('GetUploadQueueBatches'),
    ])
    ports.value = portResult || []
    devices.value = stateResult || []
    queue.value = queueResult || queue.value
    queueBatches.value = batchResult || []
    for (const batch of queueBatches.value) {
      if (!confirmations[batch.id]) confirmations[batch.id] = { uploadId: '', taskId: '' }
    }
    if (!selectedDevice.value && devices.value.length) selectedDevice.value = devices.value[0].deviceId
    await Promise.all(devices.value.map((device) => invoke('SubscribeLogEvents', device.deviceId).catch(() => undefined)))
    serviceReady.value = true
    banner.value = ''
  } catch (error) {
    serviceReady.value = false
    banner.value = error.message || String(error)
  }
}

async function withBusy(action) {
  busy.value = true
  try {
    await action()
    await refreshAll()
  } catch (error) {
    banner.value = error.message || String(error)
  } finally {
    busy.value = false
  }
}

function connect(device) {
  return withBusy(() => invoke('ConnectDevice', device.config))
}

function disconnect(device) {
  return withBusy(() => invoke('DisconnectDevice', device.deviceId))
}

function saveDeviceConfig() {
  if (!selected.value) return
  return withBusy(() => invoke('SaveDeviceConfig', selected.value.deviceId, selected.value.config))
}

function connectAll() {
  return withBusy(async () => {
    for (const device of devices.value.filter((item) => item.status === 'disconnected' || item.status === 'error')) {
      await invoke('ConnectDevice', device.config)
    }
  })
}

function disconnectAll() {
  return withBusy(async () => {
    for (const device of devices.value.filter((item) => item.status !== 'disconnected')) {
      await invoke('DisconnectDevice', device.deviceId)
    }
  })
}

function startTask() {
  return withBusy(async () => {
    for (const device of devices.value.filter((item) => item.status === 'disconnected' || item.status === 'error' || item.status === 'stopped')) await invoke('ConnectDevice', device.config)
    await invoke('StartTask', 'desktop-task')
  })
}

function stopTask() {
  return withBusy(() => invoke('StopTask', 'desktop-task'))
}

async function sendCommand() {
  if (!selected.value || !command.value.trim()) return
  const value = command.value
  command.value = ''
  await withBusy(() => invoke('SendCommand', selected.value.deviceId, value))
}

function retryBatch(id) {
  return withBusy(() => invoke('RetryUncertain', id))
}

function confirmBatch(id) {
  const values = confirmations[id] || {}
  return withBusy(() => invoke('ConfirmUncertain', id, values.uploadId || '', values.taskId || ''))
}

function addLogBatch(batch) {
  for (const entry of batch || []) {
    const current = logs.get(entry.deviceId) || []
    current.push(entry)
    if (current.length > MAX_LOG_LINES) current.splice(0, current.length - MAX_LOG_LINES)
    logs.set(entry.deviceId, current)
  }
}

function togglePaused() {
  if (!paused.value) {
    for (const [deviceId, rows] of logs.entries()) pausedSnapshots.set(deviceId, rows.slice())
  } else {
    pausedSnapshots.clear()
  }
  paused.value = !paused.value
}

watch(visibleLogs, () => {
  if (!follow.value || paused.value) return
  nextTick(() => { if (logConsole.value) logConsole.value.scrollTop = logConsole.value.scrollHeight })
}, { deep: true })

onMounted(async () => {
  await refreshAll()
  if (window.runtime?.EventsOn) {
    disposeLogs = window.runtime.EventsOn('collector:logs', addLogBatch)
    disposeState = window.runtime.EventsOn('collector:state', refreshAll)
  }
})

onBeforeUnmount(() => {
  disposeLogs?.()
  disposeState?.()
})
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand"><Terminal :size="20" /><strong>LogMaster</strong><span>采集端</span></div>
      <nav class="topnav" aria-label="主导航">
        <button :class="{ active: activeView === 'collector' }" @click="activeView = 'collector'"><Cable :size="16" />采集</button>
        <button :class="{ active: activeView === 'queue' }" @click="activeView = 'queue'"><Database :size="16" />上传队列</button>
        <button :class="{ active: activeView === 'settings' }" @click="activeView = 'settings'"><Settings :size="16" />设置</button>
      </nav>
      <div class="service-state" :class="serviceReady ? 'online' : 'offline'"><span></span>{{ serviceReady ? '本地服务正常' : '本地服务未连接' }}</div>
    </header>

    <div v-if="banner" class="banner"><TriangleAlert :size="17" /><span>{{ banner }}</span><button title="关闭" @click="banner = ''"><X :size="16" /></button></div>

    <main v-if="activeView === 'collector'" class="collector-layout">
      <aside class="device-pane">
        <div class="pane-heading">
          <div><h1>设备通道</h1><span>{{ devices.length }}/8 已配置</span></div>
          <button class="icon-button" title="扫描串口" :disabled="busy" @click="refreshAll"><RefreshCw :size="17" /></button>
        </div>
        <div class="batch-actions">
          <button class="primary" :disabled="busy" @click="connectAll"><Link :size="15" />全部连接</button>
          <button :disabled="busy" @click="disconnectAll"><Link2Off :size="15" />全部断开</button>
        </div>
        <div class="device-list">
          <button v-for="(device, deviceIndex) in devices" :key="device.deviceId" class="device-row" :class="{ selected: selected?.deviceId === device.deviceId }" @click="selectedDevice = device.deviceId">
            <span class="channel-index">{{ deviceIndex + 1 }}</span>
            <span class="device-main"><strong>{{ device.name || device.deviceId }}</strong><small>{{ device.portName || '未选择端口' }} · {{ device.config?.baudRate || 115200 }}</small></span>
            <span class="status-dot" :class="statusClass(device.status)"></span>
            <span class="status-text">{{ statusLabel(device.status) }}</span>
          </button>
          <div v-if="!devices.length" class="empty-small">尚未配置设备通道</div>
        </div>
        <div class="disk-meter">
          <div><HardDrive :size="16" /><span>本地存储</span><strong>{{ queue.diskUsageText || '--' }}</strong></div>
          <div class="meter"><span :style="{ width: `${Math.min(queue.diskUsagePercent || 0, 100)}%` }"></span></div>
        </div>
      </aside>

      <section class="workspace">
        <div class="workspace-toolbar">
          <div class="device-title"><span class="status-dot" :class="statusClass(selected?.status)"></span><div><h2>{{ selected?.name || '选择设备' }}</h2><p>{{ selected?.portName || '未配置端口' }} · {{ statusLabel(selected?.status) }}</p></div></div>
          <div class="toolbar-actions">
            <button v-if="selected?.status === 'disconnected' || selected?.status === 'error'" class="primary" :disabled="busy || !selected" @click="connect(selected)"><Link :size="16" />连接</button>
            <button v-else :disabled="busy || !selected" @click="disconnect(selected)"><Link2Off :size="16" />断开</button>
            <button class="success" :disabled="busy" @click="startTask"><Play :size="16" />开始采集</button>
            <button :disabled="busy" @click="stopTask"><CircleStop :size="16" />停止</button>
          </div>
        </div>

        <div class="log-controls">
          <label class="search-box"><Search :size="16" /><input v-model="search" placeholder="搜索当前通道日志" /></label>
          <label class="select-box"><select v-model="level"><option v-for="item in levels" :key="item">{{ item }}</option></select><ChevronDown :size="15" /></label>
          <button class="icon-button" :title="paused ? '继续显示' : '暂停显示'" @click="togglePaused"><Play v-if="paused" :size="17" /><Pause v-else :size="17" /></button>
          <label class="check-control"><input v-model="follow" type="checkbox" />自动滚动</label>
          <span class="line-count">{{ visibleLogs.length }} / {{ MAX_LOG_LINES }} 行</span>
        </div>

        <div ref="logConsole" class="log-console" :class="{ paused }">
          <div v-if="!visibleLogs.length" class="empty-log"><Terminal :size="28" /><span>等待串口日志</span></div>
          <div v-for="(row, index) in visibleLogs" :key="`${row.timestamp}-${index}`" class="log-line">
            <span class="timestamp">{{ row.timestamp }}</span><span class="level" :class="row.level?.toLowerCase()">{{ row.level || 'INFO' }}</span><span class="module">{{ row.module || '-' }}</span><span class="message">{{ row.message }}</span>
          </div>
        </div>

        <form class="command-bar" @submit.prevent="sendCommand"><span>&gt;</span><input v-model="command" :disabled="!selected" placeholder="向当前设备发送指令" /><button class="icon-button" type="submit" title="发送指令" :disabled="!command.trim()"><Send :size="17" /></button></form>
      </section>

      <aside class="inspector">
        <div class="inspector-section"><h3>通道配置</h3><label>设备名称<input v-model="selected.config.name" :disabled="!selected" /></label><label>串口<select v-model="selected.config.portName" :disabled="!selected"><option v-if="selected?.config.portName && !ports.some((port) => port.name === selected.config.portName)" :value="selected.config.portName">{{ selected.config.portName }}</option><option v-for="port in ports" :key="port.name" :value="port.name">{{ port.name }}{{ port.product ? ` · ${port.product}` : '' }}</option></select></label><div class="field-grid"><label>波特率<input v-model.number="selected.config.baudRate" type="number" min="300" max="4000000" /></label><label>数据位<select v-model.number="selected.config.dataBits"><option :value="5">5</option><option :value="6">6</option><option :value="7">7</option><option :value="8">8</option></select></label><label>停止位<select v-model.number="selected.config.stopBits"><option :value="1">1</option><option :value="2">2</option></select></label><label>校验位<select v-model="selected.config.parity"><option value="none">无</option><option value="odd">奇校验</option><option value="even">偶校验</option><option value="mark">标记</option><option value="space">空格</option></select></label></div><label>流控<select v-model="selected.config.handshake"><option value="none">无</option><option value="rtscts" disabled>RTS/CTS（当前驱动不支持）</option><option value="xonxoff" disabled>XON/XOFF（当前驱动不支持）</option></select></label><button class="primary config-save" :disabled="busy || !selected" @click="saveDeviceConfig"><Settings :size="15" />保存通道配置</button></div>
        <div class="inspector-section"><h3>实时统计</h3><dl><div><dt>已接收</dt><dd>{{ selected?.linesReceived || 0 }} 行</dd></div><div><dt>规则命中</dt><dd>{{ selected?.ruleHits || 0 }} 次</dd></div><div><dt>UI 丢弃</dt><dd>{{ selected?.droppedEvents || 0 }} 条</dd></div><div><dt>重连次数</dt><dd>{{ selected?.reconnects || 0 }} 次</dd></div></dl></div>
      </aside>
    </main>

    <main v-else-if="activeView === 'queue'" class="wide-view">
      <div class="view-heading"><div><h1>上传队列</h1><p>请求结果未知的批次不会自动重传。</p></div><button @click="refreshAll"><ListRestart :size="16" />刷新</button></div>
      <div class="queue-summary"><div><span>待上传</span><strong>{{ queue.pending || 0 }}</strong></div><div><span>上传中</span><strong>{{ queue.uploading || 0 }}</strong></div><div><span>已上传</span><strong>{{ queue.uploaded || 0 }}</strong></div><div class="warning-block"><span>待核对</span><strong>{{ queue.uncertain || 0 }}</strong></div><div><span>确定失败</span><strong>{{ queue.dead || 0 }}</strong></div></div>
      <div v-if="queueBatches.length" class="queue-table-wrap"><table class="queue-table"><thead><tr><th>状态</th><th>设备 / 文件</th><th>大小</th><th>错误</th><th>平台确认</th><th>操作</th></tr></thead><tbody><tr v-for="batch in queueBatches" :key="batch.id"><td><span class="state-label" :class="batch.state">{{ batch.state === 'uncertain' ? '待核对' : '确定失败' }}</span></td><td><strong>{{ batch.deviceId }}</strong><small>{{ batch.fileName }}</small></td><td>{{ Math.ceil(batch.sizeBytes / 1024) }} KB</td><td class="error-cell">{{ batch.lastError }}</td><td><div v-if="batch.state === 'uncertain'" class="confirm-inputs"><input v-model="confirmations[batch.id].uploadId" placeholder="Upload ID" /><input v-model="confirmations[batch.id].taskId" placeholder="Task ID" /></div><span v-else>-</span></td><td><div class="row-actions"><button v-if="batch.state === 'uncertain'" :disabled="busy" @click="retryBatch(batch.id)"><ListRestart :size="14" />重试</button><button v-if="batch.state === 'uncertain'" class="primary" :disabled="busy || !confirmations[batch.id]?.uploadId || !confirmations[batch.id]?.taskId" @click="confirmBatch(batch.id)">确认已上传</button></div></td></tr></tbody></table></div>
      <div v-else class="queue-empty"><WifiOff :size="28" /><h2>当前没有异常上传批次</h2><p>队列记录、文件摘要和错误信息保存在本地 SQLite 中。</p></div>
    </main>

    <main v-else class="wide-view">
      <div class="view-heading"><div><h1>采集设置</h1><p>配置保存在用户数据目录，覆盖 EXE 不会丢失。</p></div></div>
      <div class="settings-band"><Settings :size="26" /><div><h2>配置由桌面服务统一校验</h2><p>串口、分段、磁盘阈值和上传参数将通过原子写入保存。当前版本先以设备通道面板中的已加载配置运行。</p></div></div>
    </main>

    <footer class="statusbar"><span><span class="status-dot" :class="serviceReady ? 'ok' : 'danger'"></span>{{ serviceReady ? '采集核心运行中' : '采集核心不可用' }}</span><span>待上传 {{ queue.pending || 0 }}</span><span v-if="queue.uncertain" class="footer-warning">待人工核对 {{ queue.uncertain }}</span><span class="spacer"></span><span>UI 队列上限 {{ MAX_LOG_LINES }} 行/通道</span></footer>
  </div>
</template>
