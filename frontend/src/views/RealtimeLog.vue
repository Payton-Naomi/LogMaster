<template>
    <div class="realtime-log-page">
        <!-- ====== 顶部工具栏 ====== -->
        <div class="filter-bar">
            <div class="filter-left">
                <el-select v-model="selectedDevice" placeholder="筛选/高亮设备" clearable style="width:180px">
                    <el-option label="全部" value="" />
                    <el-option v-for="port in availablePorts" :key="port.name" :label="port.label" :value="port.name" />
                </el-select>

                <el-button type="success" plain @click="exportLogs">
                    <el-icon>
                        <Download />
                    </el-icon>
                    导出日志
                </el-button>

                <el-button type="primary" plain @click="refreshPorts">
                    🔄 刷新串口
                </el-button>
            </div>

            <div class="filter-right">
                <span class="total-logs">总日志: {{ totalLogs }} 条</span>
                <el-button size="default" :type="isGlobalPaused ? 'warning' : 'primary'" @click="toggleGlobalPause">
                    {{ isGlobalPaused ? '▶️ 恢复全部' : '⏸️ 暂停全部' }}
                </el-button>
                <el-button size="default" type="danger" plain @click="clearAllLogs">
                    🗑️ 清空全部
                </el-button>
            </div>
        </div>

        <!-- ====== 4路日志窗口（2x2网格，固定大小） ====== -->
        <div class="log-grid">
            <div v-for="(window, idx) in windows" :key="idx" class="log-window" :class="{
                'window-highlight': selectedDevice && window.selectedPort === selectedDevice,
                'window-dimmed': selectedDevice && window.selectedPort && window.selectedPort !== selectedDevice
            }">
                <!-- ====== 窗口标题栏 ====== -->
                <div class="window-header">
                    <div class="header-left">
                        <span class="status-dot"
                            :class="window.paused ? 'offline' : (window.selectedPort ? 'online' : 'offline')"></span>

                        <el-select v-model="window.selectedPort" placeholder="选择串口" size="small" style="width:100px"
                            @change="onPortChange(idx)">
                            <el-option v-for="port in availablePorts" :key="port.name" :label="port.label"
                                :value="port.name" />
                        </el-select>

                        <span class="device-label">{{ getPortLabel(window.selectedPort) }}</span>
                        <span class="log-count">{{ getLogsForWindow(idx).length }} 条</span>
                        <span v-if="selectedDevice === window.selectedPort" class="selected-badge">📌 聚焦</span>
                    </div>
                    <div class="header-right">
                        <el-button size="small" :type="window.paused ? 'warning' : 'primary'"
                            @click="toggleWindowPause(idx)">
                            {{ window.paused ? '▶' : '⏸' }}
                        </el-button>
                        <el-button size="small" type="danger" plain @click="clearWindowLogs(idx)">
                            🗑
                        </el-button>
                    </div>
                </div>

                <!-- ====== 窗口工具栏（关键词 + 备注） ====== -->
                <div class="window-toolbar">
                    <el-input v-model="window.keyword" size="small" placeholder="🔍 关键词高亮..." clearable
                        class="keyword-input" @input="saveKeyword(idx, window.keyword)" />
                    <el-input v-model="window.remark" size="small" placeholder="📝 设备备注..." clearable
                        class="remark-input" @input="saveRemark(idx, window.remark)" />
                </div>

                <!-- ====== 日志列表 ====== -->
                <div class="log-container" :ref="el => setContainerRef(idx, el)">
                    <div v-for="(log, logIdx) in getLogsForWindow(idx)" :key="logIdx" class="log-line"
                        :style="{ color: log.color }">
                        <span class="log-time">{{ log.time }}</span>
                        <span class="log-level" :style="{ color: log.color }">[{{ log.level }}]</span>
                        <span class="log-content" v-html="highlightText(log.content, window.keyword)"></span>
                    </div>
                    <div v-if="getLogsForWindow(idx).length === 0" class="empty-tip">
                        <span v-if="!window.selectedPort">⬅️ 请选择串口</span>
                        <span v-else>⏳ 等待日志...</span>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { Download } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// ============================================================
// 1. 状态定义
// ============================================================
const selectedDevice = ref('')
const isGlobalPaused = ref(false)
const containerRefs = ref({})

// 可用串口列表
const availablePorts = ref([
    { name: 'COM1', label: 'COM1 - 行车记录仪A' },
    { name: 'COM2', label: 'COM2 - 行车记录仪B' },
    { name: 'COM3', label: 'COM3 - 行车记录仪C' },
    { name: 'COM4', label: 'COM4 - 行车记录仪D' },
    { name: 'COM5', label: 'COM5 - 传感器' },
    { name: 'COM6', label: 'COM6 - 备用设备' }
])

const windows = reactive([
    { selectedPort: 'COM1', keyword: '', remark: '', logs: [], paused: false, timer: null },
    { selectedPort: 'COM2', keyword: '', remark: '', logs: [], paused: false, timer: null },
    { selectedPort: 'COM3', keyword: '', remark: '', logs: [], paused: false, timer: null },
    { selectedPort: 'COM4', keyword: '', remark: '', logs: [], paused: false, timer: null }
])

// ============================================================
// 2. 日志级别颜色
// ============================================================
const levelColors = {
    ERROR: '#F56C6C',
    WARN: '#E6A23C',
    INFO: '#409EFF',
    DEBUG: '#909399'
}

// ============================================================
// 3. 日志消息池
// ============================================================
const logMessages = {
    INFO: [
        '系统启动完成', '录像开始', '帧同步成功', 'SD卡已挂载',
        'GPS信号正常', '传感器初始化完成', '网络已连接',
        '固件版本 v3.2.1', '电池电量 85%', '录像文件已保存',
        '循环录像启动', '曝光补偿已校准', '白平衡已设置'
    ],
    WARN: [
        'GPS信号弱', '温度偏高 (65°C)', '电压波动', 'WiFi信号不稳定',
        'SD卡剩余空间不足 (15%)', '帧率下降', '曝光补偿异常',
        '电池电量低 (20%)', '连接不稳定'
    ],
    ERROR: [
        '写入失败', '超时无响应', '传感器初始化失败',
        'GPS信号丢失', 'SD卡挂载失败', '内存不足', '录像文件损坏',
        '系统重启', '硬件异常'
    ],
    DEBUG: [
        '寄存器读取 0x3A', '中断触发', 'DMA传输完成',
        '缓冲区大小 2048', '采样率 1000Hz', '缓存命中'
    ]
}

// ============================================================
// 4. 计算属性
// ============================================================
const totalLogs = computed(() => {
    return windows.reduce((sum, w) => sum + w.logs.length, 0)
})

function getPortLabel(portName) {
    if (!portName) return '未选择'
    const found = availablePorts.value.find(p => p.name === portName)
    return found ? found.label.replace(/^[^-]*-\s*/, '') : portName
}

function getLogsForWindow(idx) {
    const win = windows[idx]
    if (!win.selectedPort) return []
    return win.logs
}

// ============================================================
// 5. DOM引用
// ============================================================
function setContainerRef(idx, el) {
    if (el) containerRefs.value[idx] = el
}

function scrollToBottom(idx) {
    const el = containerRefs.value[idx]
    if (el) {
        el.scrollTop = el.scrollHeight
    }
}

// ============================================================
// 6. 日志生成
// ============================================================
function generateLog(portName) {
    const rand = Math.random()
    let level
    if (rand < 0.80) level = 'INFO'
    else if (rand < 0.90) level = 'WARN'
    else if (rand < 0.95) level = 'ERROR'
    else level = 'DEBUG'

    const messages = logMessages[level]
    const content = messages[Math.floor(Math.random() * messages.length)]
    const now = new Date()
    const time = now.toTimeString().slice(0, 8) + '.' + String(now.getMilliseconds()).padStart(3, '0')

    return {
        port: portName,
        time,
        level,
        content,
        color: levelColors[level]
    }
}

function startWindowLogGenerator(idx) {
    const win = windows[idx]
    if (win.timer) clearInterval(win.timer)

    win.timer = setInterval(() => {
        if (!win.paused && !isGlobalPaused.value && win.selectedPort) {
            const newLog = generateLog(win.selectedPort)
            win.logs.push(newLog)
            if (win.logs.length > 500) {
                win.logs.splice(0, win.logs.length - 500)
            }
            nextTick(() => scrollToBottom(idx))
        }
    }, 800 + Math.random() * 600)
}

// ============================================================
// 7. 关键词高亮
// ============================================================
function highlightText(text, keyword) {
    if (!keyword || !keyword.trim()) return text
    const kw = keyword.trim()
    const escaped = kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const regex = new RegExp(escaped, 'gi')
    return text.replace(regex, match => `<span class="highlight">${match}</span>`)
}

// ============================================================
// 8. 持久化
// ============================================================
const STORAGE_KEYWORD_PREFIX = 'log_keyword_win_'
const STORAGE_REMARK_PREFIX = 'log_remark_win_'
const STORAGE_PORT_PREFIX = 'log_port_win_'

function loadPersistedData() {
    windows.forEach((win, idx) => {
        const savedKeyword = localStorage.getItem(STORAGE_KEYWORD_PREFIX + idx)
        if (savedKeyword !== null) win.keyword = savedKeyword

        const savedRemark = localStorage.getItem(STORAGE_REMARK_PREFIX + idx)
        if (savedRemark !== null) win.remark = savedRemark

        const savedPort = localStorage.getItem(STORAGE_PORT_PREFIX + idx)
        if (savedPort !== null && availablePorts.value.some(p => p.name === savedPort)) {
            win.selectedPort = savedPort
        }
    })
}

function saveKeyword(idx, value) {
    localStorage.setItem(STORAGE_KEYWORD_PREFIX + idx, value || '')
}

function saveRemark(idx, value) {
    localStorage.setItem(STORAGE_REMARK_PREFIX + idx, value || '')
}

function savePort(idx, value) {
    localStorage.setItem(STORAGE_PORT_PREFIX + idx, value || '')
}

// ============================================================
// 9. 控制方法
// ============================================================
function toggleGlobalPause() {
    isGlobalPaused.value = !isGlobalPaused.value
}

function toggleWindowPause(idx) {
    windows[idx].paused = !windows[idx].paused
}

function clearWindowLogs(idx) {
    windows[idx].logs = []
    ElMessage.success(`窗口 ${idx + 1} 已清空`)
}

function clearAllLogs() {
    windows.forEach(w => w.logs = [])
    ElMessage.success('已清空所有日志')
}

function onPortChange(idx) {
    const win = windows[idx]
    win.logs = []
    savePort(idx, win.selectedPort)
    nextTick(() => scrollToBottom(idx))
    ElMessage.success(`${win.selectedPort} 已连接`)
}

// ============================================================
// 10. 刷新串口
// ============================================================
function refreshPorts() {
    const mockPorts = [
        { name: 'COM1', label: 'COM1 - 行车记录仪A' },
        { name: 'COM2', label: 'COM2 - 行车记录仪B' },
        { name: 'COM3', label: 'COM3 - 行车记录仪C' },
        { name: 'COM4', label: 'COM4 - 行车记录仪D' },
        { name: 'COM5', label: 'COM5 - 传感器' },
        { name: 'COM6', label: 'COM6 - 备用设备' }
    ]
    const online = mockPorts.filter(() => Math.random() > 0.2)
    availablePorts.value = online.length > 0 ? online : mockPorts.slice(0, 2)

    windows.forEach((win, idx) => {
        if (win.selectedPort && !availablePorts.value.some(p => p.name === win.selectedPort)) {
            win.selectedPort = ''
            win.logs = []
            savePort(idx, '')
        }
    })

    ElMessage.success(`已刷新串口列表，当前可用 ${availablePorts.value.length} 个`)
}

// ============================================================
// 11. 导出日志
// ============================================================
function exportLogs() {
    let targetWindows = windows
    if (selectedDevice.value) {
        targetWindows = windows.filter(w => w.selectedPort === selectedDevice.value)
    }

    if (targetWindows.length === 0 || targetWindows.every(w => w.logs.length === 0)) {
        ElMessage.warning('没有可导出的日志')
        return
    }

    const now = new Date()
    const dateStr = now.toISOString().slice(0, 19).replace('T', ' ')

    let content = []
    content.push('='.repeat(60))
    content.push('  日志导出报告')
    content.push('='.repeat(60))
    content.push(`导出时间: ${dateStr}`)
    content.push(`筛选设备: ${selectedDevice.value || '全部'}`)
    content.push('='.repeat(60))
    content.push('')

    targetWindows.forEach((win, idx) => {
        if (win.logs.length === 0 || !win.selectedPort) return

        const portLabel = getPortLabel(win.selectedPort)
        content.push(`【窗口 ${idx + 1} - ${win.selectedPort}】`)
        content.push(`  设备: ${portLabel}`)
        if (win.remark) content.push(`  备注: ${win.remark}`)
        content.push(`  日志条数: ${win.logs.length}`)
        content.push('-'.repeat(40))

        const sortedLogs = [...win.logs]
        sortedLogs.forEach(log => {
            content.push(`  ${log.time} [${log.level}] ${log.content}`)
        })
        content.push('')
    })

    content.push('='.repeat(60))
    content.push('  导出完成')
    content.push('='.repeat(60))

    const blob = new Blob([content.join('\n')], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const fileName = `日志导出_${selectedDevice.value || '全部'}_${now.toISOString().slice(0, 10)}.txt`
    a.download = fileName
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)

    ElMessage.success(`导出成功: ${fileName}`)
}

// ============================================================
// 12. 生命周期
// ============================================================
onMounted(() => {
    loadPersistedData()
    windows.forEach((_, idx) => {
        startWindowLogGenerator(idx)
        if (windows[idx].selectedPort) {
            setTimeout(() => scrollToBottom(idx), 300)
        }
    })
})

onUnmounted(() => {
    windows.forEach(win => {
        if (win.timer) clearInterval(win.timer)
    })
})
</script>

<style scoped>
/* ============================================================
   页面整体布局
   ============================================================ */
.realtime-log-page {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 12px;
    overflow: hidden;
    /* 防止整体溢出 */
}

/* ============================================================
   顶部工具栏
   ============================================================ */
.filter-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    background: #fff;
    padding: 12px 16px;
    border-radius: 8px;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
    flex-shrink: 0;
}

.filter-left {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    align-items: center;
}

.filter-right {
    display: flex;
    gap: 8px;
    align-items: center;
}

.total-logs {
    color: #606266;
    font-size: 13px;
    margin-right: 4px;
}

/* ============================================================
   2x2 网格布局 - 固定大小！
   ============================================================ */
.log-grid {
    flex: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
    /* 两行各占一半，固定比例 */
    gap: 12px;
    min-height: 0;
    /* 防止 flex 溢出 */
    height: 0;
    /* 配合 flex:1，让 grid 撑满但不过度 */
}

/* ============================================================
   每个日志窗口 - 高度由 grid 控制，不随内容变化
   ============================================================ */
.log-window {
    background: #1a1a2e;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 2px solid #2a2a4a;
    min-height: 0;
    /* 防止内容撑开 */
    height: 100%;
    /* 完全填满 grid 单元格 */
    transition: border-color 0.3s, box-shadow 0.3s, opacity 0.3s;
}

.log-window.window-highlight {
    border-color: #F7C948;
    box-shadow: 0 0 20px rgba(247, 201, 72, 0.4), inset 0 0 20px rgba(247, 201, 72, 0.05);
}

.log-window.window-dimmed {
    opacity: 0.5;
    border-color: #1a1a2e;
}

/* ---- 窗口标题栏 ---- */
.window-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    background: #16213e;
    border-bottom: 1px solid #2a2a4a;
    flex-shrink: 0;
    gap: 6px;
    flex-wrap: wrap;
    min-height: 38px;
}

.header-left {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #c8d6e5;
    flex-wrap: wrap;
    flex: 1;
    min-width: 0;
}

.header-left .el-select {
    flex-shrink: 0;
}

.header-right {
    display: flex;
    gap: 3px;
    flex-shrink: 0;
}

.header-right .el-button {
    padding: 2px 8px;
    font-size: 12px;
}

/* ---- 状态点 ---- */
.status-dot {
    display: inline-block;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background-color 0.3s;
}

.status-dot.online {
    background: #00d2d3;
    box-shadow: 0 0 6px #00d2d3;
}

.status-dot.offline {
    background: #ff6b6b;
    box-shadow: 0 0 6px #ff6b6b;
}

.device-label {
    color: #8395a7;
    font-size: 11px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100px;
}

.log-count {
    color: #8395a7;
    font-size: 11px;
    flex-shrink: 0;
}

.selected-badge {
    font-size: 10px;
    color: #F7C948;
    background: rgba(247, 201, 72, 0.15);
    padding: 0 6px;
    border-radius: 4px;
    border: 1px solid rgba(247, 201, 72, 0.3);
    flex-shrink: 0;
}

/* ============================================================
   窗口工具栏（关键词 + 备注）
   ============================================================ */
.window-toolbar {
    display: flex;
    gap: 6px;
    padding: 4px 10px;
    background: #0f0f1a;
    border-bottom: 1px solid #1a1a2e;
    flex-shrink: 0;
    flex-wrap: wrap;
    min-height: 36px;
}

.window-toolbar .keyword-input {
    flex: 2;
    min-width: 70px;
}

.window-toolbar .remark-input {
    flex: 1.5;
    min-width: 50px;
}

.window-toolbar :deep(.el-input__wrapper) {
    background: #1a1a2e;
    border: 1px solid #2a2a4a;
    box-shadow: none;
    border-radius: 4px;
    padding: 0 8px;
    height: 28px;
}

.window-toolbar :deep(.el-input__wrapper:hover) {
    border-color: #3a3a5e;
}

.window-toolbar :deep(.el-input__wrapper.is-focus) {
    border-color: #409EFF;
}

.window-toolbar :deep(.el-input__inner) {
    color: #c8d6e5;
    font-size: 12px;
    height: 28px;
}

.window-toolbar :deep(.el-input__inner::placeholder) {
    color: #576574;
}

.window-toolbar :deep(.el-input__suffix) {
    display: flex;
    align-items: center;
}

/* ============================================================
   日志列表 - 占据所有剩余空间
   ============================================================ */
.log-container {
    flex: 1;
    overflow-y: auto;
    padding: 3px 8px;
    font-family: 'Consolas', 'Courier New', monospace;
    font-size: 12px;
    line-height: 1.5;
    min-height: 0;
    /* 重要：防止内容撑开 */
    background: #0f0f1a;
}

.log-container::-webkit-scrollbar {
    width: 4px;
}

.log-container::-webkit-scrollbar-track {
    background: #1a1a2e;
}

.log-container::-webkit-scrollbar-thumb {
    background: #3a3a5e;
    border-radius: 4px;
}

.log-line {
    display: flex;
    gap: 8px;
    padding: 0 2px;
    white-space: nowrap;
    border-bottom: 1px solid rgba(255, 255, 255, 0.02);
}

.log-line:hover {
    background: rgba(255, 255, 255, 0.03);
}

.log-time {
    color: #576574;
    flex-shrink: 0;
    min-width: 72px;
    font-size: 11px;
}

.log-level {
    flex-shrink: 0;
    min-width: 48px;
    font-weight: 600;
    font-size: 11px;
}

.log-content {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    color: #c8d6e5;
    font-size: 12px;
}

:deep(.highlight) {
    background: #feca57;
    color: #1a1a2e;
    padding: 0 2px;
    border-radius: 2px;
    font-weight: 600;
}

.empty-tip {
    display: flex;
    justify-content: center;
    align-items: center;
    height: 100%;
    color: #576574;
    font-size: 13px;
    font-family: 'Segoe UI', sans-serif;
    padding: 20px 0;
}

/* ============================================================
   响应式适配
   ============================================================ */
@media (max-width: 900px) {
    .log-grid {
        grid-template-columns: 1fr;
        grid-template-rows: 1fr 1fr 1fr 1fr;
    }

    .window-toolbar {
        flex-direction: column;
    }

    .window-toolbar .keyword-input,
    .window-toolbar .remark-input {
        flex: 1;
        width: 100%;
    }
}

@media (max-width: 600px) {
    .filter-bar {
        flex-direction: column;
        align-items: stretch;
    }

    .filter-left,
    .filter-right {
        flex-wrap: wrap;
    }

    .header-left {
        font-size: 11px;
    }

    .header-left .el-select {
        width: 80px !important;
    }

    .device-label {
        max-width: 60px;
    }

    .log-line {
        font-size: 11px;
        gap: 4px;
    }

    .log-time {
        min-width: 56px;
        font-size: 10px;
    }

    .log-level {
        min-width: 36px;
        font-size: 10px;
    }

    .log-content {
        font-size: 11px;
    }
}
</style>