<template>
  <div class="serial-debug">
    <h2>串口调试工具</h2>
    <el-row :gutter="16" style="flex: 1; min-height: 0;">
      <!-- 左侧配置面板 -->
      <el-col :span="6">
        <el-card class="config-card">
          <template #header><span>串口配置</span></template>
          <el-form label-width="60px" size="small">
            <el-form-item label="串口">
              <el-select v-model="selectedPort" placeholder="选择串口" style="width: 100%" :disabled="connected">
                <el-option v-for="p in portList" :key="p" :label="p" :value="p" />
              </el-select>
            </el-form-item>
            <el-form-item label="波特率">
              <el-select v-model="baudRate" style="width: 100%" :disabled="connected">
                <el-option v-for="b in baudRates" :key="b" :label="b" :value="b" />
              </el-select>
            </el-form-item>
          </el-form>

          <el-collapse>
            <el-collapse-item title="更多串口设置">
              <el-form label-width="60px" size="small">
                <el-form-item label="数据位">
                  <el-select v-model="dataBits" style="width: 100%" :disabled="connected">
                    <el-option v-for="d in [5,6,7,8]" :key="d" :label="d" :value="d" />
                  </el-select>
                </el-form-item>
                <el-form-item label="停止位">
                  <el-select v-model="stopBits" style="width: 100%" :disabled="connected">
                    <el-option :value="1" label="1" />
                    <el-option :value="2" label="2" />
                  </el-select>
                </el-form-item>
                <el-form-item label="校验位">
                  <el-select v-model="parity" style="width: 100%" :disabled="connected">
                    <el-option value="none" label="none" />
                    <el-option value="odd" label="odd" />
                    <el-option value="even" label="even" />
                    <el-option value="mark" label="mark" />
                    <el-option value="space" label="space" />
                  </el-select>
                </el-form-item>
              </el-form>
            </el-collapse-item>
          </el-collapse>

          <div style="margin-top: 12px; display: flex; gap: 8px;">
            <el-button type="success" @click="connect" :disabled="connected" size="small">连接</el-button>
            <el-button type="danger" @click="disconnect" :disabled="!connected" size="small">断开</el-button>
            <el-button @click="clearLog" size="small">清屏</el-button>
            <el-button @click="refreshPorts" size="small">刷新</el-button>
          </div>

          <el-divider />

          <div class="option-row">
            <span>HEX显示</span>
            <el-switch v-model="hexMode" size="small" />
          </div>
          <div class="option-row">
            <span>时间戳</span>
            <el-switch v-model="timestampMode" size="small" />
          </div>
          <div class="option-row">
            <span>自动滚动</span>
            <el-switch v-model="autoScroll" size="small" />
          </div>

          <el-divider />

          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <span>预设命令</span>
            <el-button size="small" @click="openPresetEditor">编辑</el-button>
          </div>
          <div style="display: flex; flex-wrap: wrap; gap: 4px;">
            <el-button v-for="(cmd, i) in presets" :key="i" size="small" @click="sendPreset(cmd)">
              {{ cmd.length > 10 ? cmd.substring(0, 10) + '...' : cmd }}
            </el-button>
          </div>
        </el-card>
      </el-col>

      <!-- 右侧日志区域 -->
      <el-col :span="18" style="display: flex; flex-direction: column; min-height: 0;">
        <el-card class="log-card" style="flex: 1; display: flex; flex-direction: column;">
          <div ref="logTerminal" class="log-terminal" @scroll="onScroll">
            <div v-for="(line, i) in logLines" :key="i" class="log-line">
              <span v-if="timestampMode" class="log-ts">[{{ line.ts }}]</span>
              {{ line.content }}
            </div>
            <div v-if="logLines.length === 0" class="log-line" style="color: #666;">
              --- 等待连接串口 ---
            </div>
          </div>

          <div class="send-area">
            <el-button size="small" @click="sendMode = sendMode === 'str' ? 'hex' : 'str'">
              {{ sendMode.toUpperCase() }}
            </el-button>
            <el-input
              v-model="sendInput"
              placeholder="输入要发送的命令..."
              @keydown.enter="sendData"
              size="small"
              style="flex: 1;"
            />
            <el-button type="primary" @click="sendData" size="small">发送</el-button>
          </div>

          <div class="status-bar">
            <span>{{ connected ? `已连接: ${statusDevice} @ ${statusBaud}` : '未连接' }}</span>
            <span class="rx-tx">
              <span>RX: <b>{{ rxCount }}</b></span>
              <span>TX: <b>{{ txCount }}</b></span>
            </span>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 预设命令编辑弹窗 -->
    <el-dialog v-model="presetDialogVisible" title="编辑预设命令" width="500px">
      <div v-for="(cmd, i) in editablePresets" :key="i" style="display: flex; gap: 4px; margin-bottom: 6px;">
        <el-input v-model="editablePresets[i]" size="small" />
        <el-button type="danger" size="small" @click="editablePresets.splice(i, 1)">X</el-button>
      </div>
      <el-button size="small" @click="editablePresets.push('')" style="margin-top: 8px;">+ 添加命令</el-button>
      <template #footer>
        <el-button @click="presetDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePresets">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'

const agentBase = 'http://localhost:9527'

// 状态
const portList = ref([])
const selectedPort = ref('')
const baudRate = ref(115200)
const dataBits = ref(8)
const stopBits = ref(1)
const parity = ref('none')
const connected = ref(false)
const hexMode = ref(false)
const timestampMode = ref(true)
const autoScroll = ref(true)
const sendMode = ref('str')
const sendInput = ref('')
const logLines = ref([])
const rxCount = ref(0)
const txCount = ref(0)
const statusDevice = ref('')
const statusBaud = ref(0)
const presetDialogVisible = ref(false)
const logTerminal = ref(null)

const baudRates = [9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600]

let ws = null
const presets = ref(JSON.parse(localStorage.getItem('serial-presets') || '["ls /","ping","help","reboot","status","log"]'))
const editablePresets = ref([])

// 刷新串口列表
async function refreshPorts() {
  try {
    const res = await fetch(`${agentBase}/api/ports`)
    const json = await res.json()
    if (json.ok) portList.value = json.data || []
  } catch (e) {
    // Agent 未启动时静默处理
  }
}

// 连接
async function connect() {
  if (!selectedPort.value) { ElMessage.warning('请选择串口'); return }
  try {
    const res = await fetch(`${agentBase}/api/connect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        device: selectedPort.value,
        baud_rate: baudRate.value,
        data_bits: dataBits.value,
        stop_bits: stopBits.value,
        parity: parity.value
      })
    })
    const json = await res.json()
    if (json.ok) {
      connected.value = true
      connectWS()
      ElMessage.success('已连接')
    } else {
      ElMessage.error('连接失败: ' + json.error)
    }
  } catch (e) {
    ElMessage.error('连接失败: ' + e.message)
  }
}

// 断开
async function disconnect() {
  try { await fetch(`${agentBase}/api/disconnect`, { method: 'POST' }) } catch (e) {}
  connected.value = false
  if (ws) { ws.close(); ws = null }
  appendLog({ ts: '', content: '--- 已断开连接 ---' })
  rxCount.value = 0
  txCount.value = 0
}

// WebSocket
function connectWS() {
  if (ws) ws.close()
  ws = new WebSocket(`ws://localhost:9527/ws`)
  ws.onmessage = (e) => {
    const msg = JSON.parse(e.data)
    if (msg.type === 'log') {
      appendLog(formatLog(msg))
    } else if (msg.type === 'status') {
      rxCount.value = msg.status.rx_count || 0
      txCount.value = msg.status.tx_count || 0
      statusDevice.value = msg.status.device || ''
      statusBaud.value = msg.status.baud_rate || 0
    }
  }
  ws.onclose = () => {
    if (connected.value) {
      setTimeout(() => { if (connected.value) connectWS() }, 2000)
    }
  }
}

function formatLog(msg) {
  let content = hexMode.value
    ? Array.from(new TextEncoder().encode(msg.content)).map(b => b.toString(16).padStart(2, '0').toUpperCase()).join(' ')
    : msg.content
  return { ts: msg.timestamp, content }
}

function appendLog(line) {
  logLines.value.push(line)
  if (logLines.value.length > 5000) logLines.value.shift()
  if (autoScroll.value) {
    nextTick(() => {
      const el = logTerminal.value
      if (el) el.scrollTop = el.scrollHeight
    })
  }
}

function clearLog() {
  logLines.value = []
}

async function sendData() {
  if (!sendInput.value) return
  try {
    const res = await fetch(`${agentBase}/api/send`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: sendInput.value, mode: sendMode.value })
    })
    const json = await res.json()
    if (json.ok) sendInput.value = ''
    else ElMessage.error('发送失败: ' + json.error)
  } catch (e) {
    ElMessage.error('发送失败: ' + e.message)
  }
}

function sendPreset(cmd) {
  sendInput.value = cmd
  sendData()
}

function onScroll() {
  const el = logTerminal.value
  if (!el) return
  autoScroll.value = el.scrollHeight - el.scrollTop - el.clientHeight < 20
}

function openPresetEditor() {
  editablePresets.value = [...presets.value]
  presetDialogVisible.value = true
}

function savePresets() {
  presets.value = editablePresets.value.filter(v => v.trim())
  localStorage.setItem('serial-presets', JSON.stringify(presets.value))
  presetDialogVisible.value = false
}

// 定时刷新串口列表
let timer = null
onMounted(() => {
  refreshPorts()
  timer = setInterval(refreshPorts, 3000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (ws) ws.close()
})
</script>

<style scoped>
.serial-debug {
  padding: 20px;
  height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
}
.serial-debug h2 { margin-bottom: 16px; font-size: 18px; }
.config-card { height: 100%; overflow-y: auto; }
.option-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  font-size: 13px;
}
.log-card {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.log-card :deep(.el-card__body) {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0;
}
.log-terminal {
  flex: 1;
  background: #0d0d0d;
  color: #0f0;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 13px;
  padding: 8px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  min-height: 200px;
}
.log-line { padding: 1px 0; }
.log-ts { color: #888; }
.send-area {
  display: flex;
  gap: 8px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-top: 1px solid #e4e7ed;
  align-items: center;
}
.status-bar {
  background: #409eff;
  color: #fff;
  padding: 4px 12px;
  font-size: 12px;
  display: flex;
  justify-content: space-between;
}
.status-bar .rx-tx { display: flex; gap: 16px; }
</style>