import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Modal, notification } from 'ant-design-vue'

const DEFAULT_MAX_LINES = 2000

export function useDesktopService() {
  const serviceReady = ref(false)
  const banner = ref('正在连接本地采集服务…')
  const busy = ref(false)
  const devices = ref([])
  const selectedDeviceId = ref('')
  const queueStatus = ref({ pending: 0, uploading: 0, uploaded: 0, uncertain: 0, dead: 0 })
  const queuePage = ref({ items: [], total: 0 })
  const catalog = ref({ schemaVersion: 1, projects: [] })
  const settings = ref({ maxLogLines: DEFAULT_MAX_LINES, autoWrap: true, noLogTimeoutSeconds: 300 })
  const logs = reactive(new Map())
  const lineCounters = new Map()
  const uploadProgress = reactive(new Map())
  const notifiedStates = new Set()
  let lastWarning = { text: '', at: 0 }
  let disposeLogs
  let disposeState
  let disposePorts
  let disposeProgress
  let disposeCatalog
  let disposeServerFailure

  const api = () => window.go?.main?.Service
  const selectedDevice = computed(() => devices.value.find((item) => item.deviceId === selectedDeviceId.value) || devices.value[0] || null)
  const maxLines = computed(() => Math.max(100, settings.value.maxLogLines || DEFAULT_MAX_LINES))

  async function invoke(name, ...args) {
    const service = api()
    try {
      if (!service?.[name]) throw new Error(`本地接口 ${name} 不可用，请重新构建程序`)
      return await service[name](...args)
    } catch (error) {
      showWarning(error.message || String(error))
      throw error
    }
  }

  function showWarning(text) {
    const message = String(text || '操作失败')
    const now = Date.now()
    if (lastWarning.text === message && now - lastWarning.at < 800) return
    lastWarning = { text: message, at: now }
    notification.warning({ message: '警告', description: message, placement: 'bottomRight', duration: 3 })
  }

  function applyDevices(next) {
    devices.value = next || []
    const active = new Set()
    for (const device of devices.value) {
      const reason = device.noLogAlert ? '长时间未收到日志' : device.status === 'disk_full' ? '存储空间不足' : ''
      if (!reason) continue
      const key = `${device.deviceId}:${reason}`
      active.add(key)
      if (!notifiedStates.has(key)) showWarning(`${device.portName || device.name}：${reason}`)
    }
    for (const key of notifiedStates) if (!active.has(key)) notifiedStates.delete(key)
    for (const key of active) notifiedStates.add(key)
  }

  async function refreshDevices() {
    applyDevices(await invoke('GetDeviceStates'))
    if (!devices.value.some((item) => item.deviceId === selectedDeviceId.value)) selectedDeviceId.value = devices.value[0]?.deviceId || ''
  }

  async function refreshQueue(query = {}) {
    queueStatus.value = (await invoke('GetUploadQueueStatus')) || queueStatus.value
    queuePage.value = (await invoke('GetUploadQueue', { deviceId: '', states: [], search: '', includeUploaded: false, offset: 0, limit: 50, ...query })) || { items: [], total: 0 }
  }

  async function refreshAll() {
    try {
      const [deviceResult, statusResult, catalogResult, settingsResult] = await Promise.all([invoke('GetDeviceStates'), invoke('GetUploadQueueStatus'), invoke('GetCatalog'), invoke('GetAppSettings')])
      applyDevices(deviceResult)
      queueStatus.value = statusResult || queueStatus.value
      catalog.value = catalogResult || catalog.value
      settings.value = settingsResult || settings.value
      if (!devices.value.some((item) => item.deviceId === selectedDeviceId.value)) selectedDeviceId.value = devices.value[0]?.deviceId || ''
      serviceReady.value = true
      banner.value = ''
    } catch (error) {
      serviceReady.value = false
      banner.value = ''
    }
  }

  async function withBusy(action, refresh = true) {
    busy.value = true
    try { const result = await action(); if (refresh) await refreshAll(); return result }
    catch (error) { throw error }
    finally { busy.value = false }
  }

  function addLogBatch(batch) {
    for (const entry of batch || []) {
      const rows = logs.get(entry.deviceId) || []
      const nextLine = (lineCounters.get(entry.deviceId) || 0) + 1
      lineCounters.set(entry.deviceId, nextLine)
      rows.push({ ...entry, lineNumber: nextLine, key: `${entry.deviceId}-${entry.sequence || 0}-${entry.capturedAt || entry.timestamp}-${nextLine}` })
      if (rows.length > maxLines.value) rows.splice(0, rows.length - maxLines.value)
      logs.set(entry.deviceId, rows)
    }
  }

  function clearLogs(deviceId) { logs.set(deviceId, []); lineCounters.set(deviceId, 0) }
  function deviceLogs(deviceId) { return logs.get(deviceId) || [] }
  function connect(device) { return withBusy(() => invoke('ConnectDevice', device.config)) }
  function disconnect(device) { return withBusy(() => invoke('DisconnectDevice', device.deviceId)) }
  function saveDeviceConfig(deviceId, config) { return withBusy(() => invoke('SaveDeviceConfigWithResult', deviceId, config)) }
  function sendCommand(deviceId, command) { return withBusy(() => invoke('SendCommand', deviceId, command), false) }

  onMounted(async () => {
    await refreshAll()
    if (window.runtime?.EventsOn) {
      disposeLogs = window.runtime.EventsOn('collector:logs', addLogBatch)
      disposeState = window.runtime.EventsOn('collector:state', refreshDevices)
      disposePorts = window.runtime.EventsOn('serial:ports', refreshDevices)
      disposeProgress = window.runtime.EventsOn('upload:progress', (progress) => {
        uploadProgress.set(progress.batchId, progress)
        const item = queuePage.value.items?.find((entry) => entry.id === progress.batchId)
        if (item) Object.assign(item, { bytesSent: progress.sentBytes, bytesTotal: progress.totalBytes, speedBytes: progress.speedBytes })
      })
      disposeCatalog = window.runtime.EventsOn('catalog:updated', (value) => { catalog.value = value })
      disposeServerFailure = window.runtime.EventsOn('upload:server-failure', (failure) => {
        const labels = { decompress_failed: '解压失败', parse_failed: '解析失败', storage_failed: '存储失败', unknown_failed: '处理失败' }
        Modal.error({ title: labels[failure.errorType] || '云端处理失败', content: `文件：${failure.fileName || '未知'}\n查询码：${failure.queryCode || '—'}\n原因：${failure.errorMessage || '服务端未返回具体原因'}`, okText: '知道了' })
      })
    }
  })

  onBeforeUnmount(() => { disposeLogs?.(); disposeState?.(); disposePorts?.(); disposeProgress?.(); disposeCatalog?.(); disposeServerFailure?.() })

  return { serviceReady, banner, busy, devices, selectedDeviceId, selectedDevice, queueStatus, queuePage, catalog, settings, logs, uploadProgress, invoke, showWarning, refreshAll, refreshDevices, refreshQueue, withBusy, clearLogs, deviceLogs, connect, disconnect, saveDeviceConfig, sendCommand }
}
