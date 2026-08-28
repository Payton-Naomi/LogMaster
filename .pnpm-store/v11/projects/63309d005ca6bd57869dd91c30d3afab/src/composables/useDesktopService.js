import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Modal, notification } from 'ant-design-vue'

const DEFAULT_MAX_LINES = 100000
const DEFAULT_SETTINGS = {
  defaultLogDirectory: 'D:\\LogMaster\\LocalLog',
  defaultSaveEnabled: true,
  defaultUploadEnabled: false,
  segmentMaxAgeSeconds: 1800,
  segmentMaxBytes: 128 * 1024 * 1024,
  noLogTimeoutSeconds: 1800,
  maxLogLines: DEFAULT_MAX_LINES,
  logFontSize: 12,
  autoWrap: true,
  maxDiskBytes: 50 * 1024 * 1024 * 1024,
  storageWarningPercent: 80,
  autoDeleteUploaded: false,
  uploadedRetentionHours: 0,
  backendUrl: 'http://localhost:8080/api',
  uploadIntervalSeconds: 300,
  uploadConcurrency: 5,
  uploadGzip: true,
  programName: 'LogMaster采集端',
  programVersion: '0.0.10',
  buildVersion: '0.0.10',
  updateDate: '2026-08-14',
  companyName: '上海七十迈数字科技有限公司',
  communityTitle: '飞书交流群',
  communityText: '使用说明、问题反馈、获取最新版本请扫码加入飞书交流群。',
  communityUrl: '',
}

export function useDesktopService() {
  const serviceReady = ref(false)
  const banner = ref('正在连接本地采集服务…')
  const busy = ref(false)
  const devices = ref([])
  const selectedDeviceId = ref('')
  const queueStatus = ref({ pending: 0, uploading: 0, uploaded: 0, uncertain: 0, dead: 0 })
  const queuePage = ref({ items: [], total: 0 })

  const currentPendingBytes = ref(0)
  const catalog = ref({ schemaVersion: 1, projects: [] })
  const settings = ref({ ...DEFAULT_SETTINGS })
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
  let startupWarningsShown = false

  const api = () => window.go?.main?.Service
  const selectedDevice = computed(() => devices.value.find((item) => item.deviceId === selectedDeviceId.value) || devices.value[0] || null)
  const maxLines = computed(() => Math.max(100, settings.value.maxLogLines || DEFAULT_MAX_LINES))

  const currentPendingText = computed(() => formatBytes(currentPendingBytes.value))
  function formatBytes(value) { if (!value) return '0 B'; if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(2)} GB`; if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB`; if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`; return `${value} B` }
  watch(selectedDeviceId, async (id) => { if (!id) { currentPendingBytes.value = 0; return }; try { currentPendingBytes.value = await invoke('GetCurrentPendingUploadBytes', id) } catch (_) { currentPendingBytes.value = 0 } })

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
    notification.warning({ message: '警告', description: message, placement: 'bottomRight', duration: 5 })
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

    if (selectedDeviceId.value) currentPendingBytes.value = await invoke('GetCurrentPendingUploadBytes', selectedDeviceId.value)
  }

  async function refreshQueue(query = {}) {
    queueStatus.value = (await invoke('GetUploadQueueStatus')) || queueStatus.value
    queuePage.value = (await invoke('GetUploadQueue', { deviceId: '', states: [], search: '', includeUploaded: false, offset: 0, limit: 50, ...query })) || { items: [], total: 0 }
  }

  async function refreshAll() {
    const [devicesResult, statusResult, catalogResult, settingsResult] = await Promise.allSettled([
      invoke('GetDeviceStates'), invoke('GetUploadQueueStatus'), invoke('GetCatalog'), invoke('GetAppSettings'),
    ])
    if (devicesResult.status === 'fulfilled') applyDevices(devicesResult.value)
    if (statusResult.status === 'fulfilled') queueStatus.value = statusResult.value || queueStatus.value
    if (catalogResult.status === 'fulfilled') catalog.value = catalogResult.value || catalog.value
    if (settingsResult.status === 'fulfilled') settings.value = { ...DEFAULT_SETTINGS, ...(settingsResult.value || {}) }
    if (devicesResult.status === 'fulfilled' || statusResult.status === 'fulfilled' || catalogResult.status === 'fulfilled' || settingsResult.status === 'fulfilled') {
      if (!devices.value.some((item) => item.deviceId === selectedDeviceId.value)) selectedDeviceId.value = devices.value[0]?.deviceId || ''

      if (selectedDeviceId.value) { try { currentPendingBytes.value = await invoke('GetCurrentPendingUploadBytes', selectedDeviceId.value) } catch (_) { currentPendingBytes.value = 0 } }
      serviceReady.value = true
      banner.value = ''
    } else {
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
    if (!startupWarningsShown) {
      startupWarningsShown = true
      try { for (const warning of await invoke('GetStartupWarnings') || []) showWarning(warning) } catch (_) {}
    }
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

  return { serviceReady, banner, busy, devices, selectedDeviceId, selectedDevice, queueStatus, queuePage, currentPendingText, catalog, settings, logs, uploadProgress, invoke, showWarning, refreshAll, refreshDevices, refreshQueue, withBusy, clearLogs, deviceLogs, connect, disconnect, saveDeviceConfig, sendCommand }
}
