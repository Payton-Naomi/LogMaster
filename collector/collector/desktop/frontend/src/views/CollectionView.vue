<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { Alert, Modal, Radio, RadioGroup } from 'ant-design-vue'
import ChannelList from '../components/ChannelList.vue'
import LogConsole from '../components/LogConsole.vue'
import ChannelInspector from '../components/ChannelInspector.vue'

const props = defineProps({ desktop: { type: Object, required: true } })
const hitCache = reactive({})
const inspector = ref(null)
const leftWidth = ref(Number(localStorage.getItem('logmaster-left-width')) || 276)
const rightWidth = ref(Number(localStorage.getItem('logmaster-right-width')) || 330)
const inspectorVisible = ref(localStorage.getItem('logmaster-inspector-visible') !== 'false')
const closeDialog = reactive({ open: false, device: null, mode: 'continue', confirmStop: false })
normalizeLayout()
const selected = computed(() => props.desktop.selectedDevice.value)
const rows = computed(() => selected.value ? props.desktop.deviceLogs(selected.value.deviceId) : [])
const hasInspector = computed(() => Boolean(selected.value) && inspectorVisible.value)
const layoutStyle = computed(() => ({ gridTemplateColumns: hasInspector.value ? `${leftWidth.value}px 4px minmax(520px, 1fr) 4px ${rightWidth.value}px` : `${leftWidth.value}px 4px minmax(520px, 1fr) 0 0` }))
function normalizeLayout() {
  leftWidth.value = Math.max(220, Math.min(420, leftWidth.value, window.innerWidth - rightWidth.value - 432))
  rightWidth.value = Math.max(280, Math.min(500, rightWidth.value, window.innerWidth - leftWidth.value - 432))
}
async function toggle(device, enabled) {
  if (enabled) { try { await props.desktop.connect(device) } catch (_) {}; return }
  if (!device.config?.uploadEnabled) { try { await props.desktop.disconnect(device) } catch (_) {}; return }
  Object.assign(closeDialog, { open: true, device, mode: 'continue', confirmStop: false })
}
async function confirmDisconnect() {
  if (closeDialog.mode === 'stop' && !closeDialog.confirmStop) { closeDialog.confirmStop = true; return }
  const device = closeDialog.device
  if (!device) return
  try {
    await props.desktop.withBusy(() => props.desktop.invoke('DisconnectDeviceWithUploadPolicy', device.deviceId, closeDialog.mode === 'continue'))
    closeDialog.open = false
  } catch (_) {}
}
function cancelDisconnect() { Object.assign(closeDialog, { open: false, device: null, confirmStop: false }) }
async function save(config) {
  try {
    const result = await props.desktop.saveDeviceConfig(selected.value.deviceId, config)
    inspector.value?.confirmSaved()
    if (result?.uploadReady && result?.queryCode) {
      Modal.success({ title: '上传配置已保存', content: `查询码：${result.queryCode}`, okText: '知道了' })
    } else if (result?.message) {
      Modal.error({ title: '上传配置未生效', content: result.message, okText: '知道了' })
    }
  } catch (_) {}
}
async function reusePrevious() {
  if (!selected.value) return
  try {
    const config = await props.desktop.invoke('ReusePreviousDeviceConfig', selected.value.deviceId)
    inspector.value?.applyDraft(config)
  } catch (_) {}
}
async function saveSession(windowContent) { if (!selected.value) return; try { await props.desktop.withBusy(() => props.desktop.invoke('SaveLogAs', selected.value.deviceId, selected.value.sessionId || '', 'session', windowContent), false) } catch (_) {} }
async function exportWindow(content) { if (!selected.value) return; try { await props.desktop.withBusy(() => props.desktop.invoke('ExportWindowLogs', selected.value.deviceId, content), false) } catch (_) {} }
async function openFolder() { try { await props.desktop.withBusy(() => props.desktop.invoke('OpenLogFolder', ''), false) } catch (_) {} }
async function loadHits(ruleId) { if (!selected.value?.sessionId) return; try { hitCache[ruleId] = await props.desktop.invoke('GetKeywordHits', selected.value.sessionId, ruleId) || [] } catch (_) {} }
async function resetHits() { if (!selected.value?.sessionId) return; try { await props.desktop.invoke('ResetKeywordHits', selected.value.sessionId); Object.keys(hitCache).forEach((key) => delete hitCache[key]); await props.desktop.refreshDevices() } catch (_) {} }
function markDirty(value) { if (selected.value) props.desktop.invoke('SetDeviceConfigDirty', selected.value.deviceId, value).catch(() => {}) }
function startResize(side, event) {
  event.preventDefault()
  document.body.classList.add('resizing-layout')
  const startX = event.clientX
  const startWidth = side === 'left' ? leftWidth.value : rightWidth.value
  const move = (moveEvent) => {
    const delta = moveEvent.clientX - startX
    if (side === 'left') leftWidth.value = Math.min(420, Math.max(220, Math.min(startWidth + delta, window.innerWidth - rightWidth.value - 432)))
    else rightWidth.value = Math.min(500, Math.max(280, Math.min(startWidth - delta, window.innerWidth - leftWidth.value - 432)))
  }
  const stop = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stop)
    document.body.classList.remove('resizing-layout')
    localStorage.setItem('logmaster-left-width', String(leftWidth.value))
    localStorage.setItem('logmaster-right-width', String(rightWidth.value))
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop, { once: true })
}
async function setLogFontSize(value) {
  const settings = JSON.parse(JSON.stringify(props.desktop.settings.value || {}))
  settings.logFontSize = value
  try {
    await props.desktop.invoke('SaveAppSettings', settings)
    props.desktop.settings.value = settings
  } catch (_) {}
}
function toggleInspector() {
  inspectorVisible.value = !inspectorVisible.value
  localStorage.setItem('logmaster-inspector-visible', String(inspectorVisible.value))
}
onBeforeUnmount(() => document.body.classList.remove('resizing-layout'))
</script>
<template>
  <main class="collection-layout adjustable" :style="layoutStyle">
    <ChannelList :devices="desktop.devices.value" :selected-id="desktop.selectedDeviceId.value" :busy="desktop.busy.value" @select="desktop.selectedDeviceId.value = $event" @toggle="toggle" @refresh="desktop.refreshAll" />
    <div class="layout-resizer" title="拖动调整串口栏宽度" @pointerdown="startResize('left', $event)"></div>
    <LogConsole :rows="rows" :device="selected" :busy="desktop.busy.value" :default-wrap="desktop.settings.value.autoWrap" :log-font-size="desktop.settings.value.logFontSize || 12" :inspector-open="hasInspector" @clear="selected && desktop.clearLogs(selected.deviceId)" @save-session="saveSession" @export-window="exportWindow" @open-folder="openFolder" @send-command="selected && desktop.sendCommand(selected.deviceId, $event)" @log-font-size-change="setLogFontSize" @toggle-inspector="toggleInspector" />
    <div v-show="hasInspector" class="layout-resizer" title="拖动调整配置栏宽度" @pointerdown="startResize('right', $event)"></div>
    <ChannelInspector v-show="hasInspector" ref="inspector" :device="selected" :catalog="desktop.catalog.value" :busy="desktop.busy.value" :keyword-hits="hitCache" @save="save" @reuse-previous="reusePrevious" @open-folder="openFolder" @load-hits="loadHits" @reset-hits="resetHits" @dirty-change="markDirty" @warning="desktop.showWarning" />
    <Modal :open="closeDialog.open" :title="closeDialog.confirmStop ? '确认停止上传' : `关闭 ${closeDialog.device?.portName || '串口'}`" :ok-text="closeDialog.confirmStop ? '确认停止并关闭' : '关闭串口'" cancel-text="取消" :confirm-loading="desktop.busy.value" @ok="confirmDisconnect" @cancel="cancelDisconnect">
      <template v-if="!closeDialog.confirmStop">
        <RadioGroup v-model:value="closeDialog.mode" class="disconnect-options">
          <Radio value="continue"><strong>关闭串口，继续上传</strong><span>已保存的日志继续在后台上传。</span></Radio>
          <Radio value="stop"><strong>关闭串口，停止上传</strong><span>暂停该通道尚未开始的上传任务。</span></Radio>
        </RadioGroup>
      </template>
      <Alert v-else type="warning" show-icon message="未上传文件会保留在本机" description="已经开始发送的请求会完成；其他待上传批次将暂停。下次重新打开该通道且仍启用云端上传时会自动续传，无需重新采集。" />
    </Modal>
  </main>
</template>

<style scoped>
.disconnect-options { display: grid; gap: 12px; width: 100%; }
.disconnect-options :deep(.ant-radio-wrapper) { align-items: flex-start; margin: 0; padding: 12px; border: 1px solid #d9d9d9; border-radius: 6px; }
.disconnect-options strong, .disconnect-options span { display: block; }
.disconnect-options span { margin-top: 4px; color: #667085; font-size: 12px; }
</style>
