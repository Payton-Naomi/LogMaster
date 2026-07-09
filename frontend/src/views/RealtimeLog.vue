<template>
    <div class="realtime-log-page">
        <!-- ====== 顶部工具栏 ====== -->
        <div class="toolbar">
            <div class="toolbar-left">
                <el-button type="primary" plain size="small" @click="refreshPorts">
                    🔄 刷新串口
                </el-button>
                <el-button type="success" plain size="small" @click="exportLogs">
                    <el-icon>
                        <Download />
                    </el-icon> 导出日志
                </el-button>
                <span class="total-info">总日志: {{ totalLogs }} 条</span>
            </div>
            <div class="toolbar-right">
                <el-button size="small" :type="isGlobalPaused ? 'warning' : 'primary'" @click="toggleGlobalPause">
                    {{ isGlobalPaused ? '▶️ 恢复全部' : '⏸️ 暂停全部' }}
                </el-button>
                <el-button size="small" type="danger" plain @click="clearAllLogs">
                    🗑️ 清空全部
                </el-button>
            </div>
        </div>

        <!-- ====== 主体：左侧列表 + 右侧日志 ====== -->
        <div class="main-body">
            <!-- 左侧串口列表 -->
            <div class="port-list">
                <div class="list-header">
                    <span>串口列表</span>
                    <span class="port-count">{{ availablePorts.length }}</span>
                </div>
                <div v-for="port in availablePorts" :key="port.name" class="port-item"
                    :class="{ active: selectedPortName === port.name }" @click="selectPort(port.name)">
                    <div class="port-info">
                        <span class="status-dot" :class="getPortStatus(port.name)"></span>
                        <span class="port-name">{{ port.name }}</span>
                        <span class="port-label">{{ port.label.replace(/^[^-]*-\s*/, '') }}</span>
                    </div>
                    <div class="port-meta">
                        <span class="log-count">{{ getPortLogs(port.name).length }} 条</span>
                        <span v-if="selectedPortName === port.name" class="active-badge">▶</span>
                    </div>
                </div>
                <div v-if="availablePorts.length === 0" class="empty-list">
                    <span>暂无可用串口</span>
                </div>
            </div>

            <!-- 右侧日志显示区 -->
            <div class="log-display">
                <template v-if="selectedPort">
                    <!-- 当前串口的工具栏 -->
                    <div class="log-toolbar">
                        <el-input v-model="selectedPort.keyword" size="small" placeholder="🔍 关键词高亮..." clearable
                            style="width:180px" @input="saveKeyword(selectedPortName, selectedPort.keyword)" />
                        <el-input v-model="selectedPort.remark" size="small" placeholder="📝 设备备注..." clearable
                            style="width:150px" @input="saveRemark(selectedPortName, selectedPort.remark)" />
                        <el-button size="small" :type="selectedPort.paused ? 'warning' : 'primary'"
                            @click="togglePortPause(selectedPortName)">
                            {{ selectedPort.paused ? '▶ 恢复' : '⏸ 暂停' }}
                        </el-button>
                        <el-button size="small" type="danger" plain @click="clearPortLogs(selectedPortName)">
                            🗑 清空
                        </el-button>
                        <span class="port-title">{{ selectedPortName }} - {{ getPortLabel(selectedPortName) }}</span>
                    </div>

                    <!-- 日志列表 -->
                    <div ref="logContainer" class="log-container">
                        <div v-for="(log, idx) in selectedPort.logs" :key="idx" class="log-line"
                            :style="{ color: log.color }">
                            <span class="log-time">{{ log.time }}</span>
                            <span class="log-level" :style="{ color: log.color }">[{{ log.level }}]</span>
                            <span class="log-content" v-html="highlightText(log.content, selectedPort.keyword)"></span>
                        </div>
                        <div v-if="selectedPort.logs.length === 0" class="empty-tip">
                            <span v-if="!selectedPortName">⬅️ 请从左侧选择一个串口</span>
                            <span v-else>⏳ 等待日志...</span>
                        </div>
                    </div>
                </template>
                <div v-else class="no-selection">
                    <el-icon size="40">
                        <Connection />
                    </el-icon>
                    <p>请从左侧选择一个串口查看实时日志</p>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { Download, Connection } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// ============================================================
// 1. 状态
// ============================================================
const selectedPortName = ref('COM1') // 当前选中的串口名
const isGlobalPaused = ref(false)
const logContainer = ref(null)

// 可用串口列表
const availablePorts = ref([
    { name: 'COM1', label: 'COM1 - 行车记录仪A' },
    { name: 'COM2', label: 'COM2 - 行车记录仪B' },
    { name: 'COM3', label: 'COM3 - 行车记录仪C' },
    { name: 'COM4', label: 'COM4 - 行车记录仪D' },
    { name: 'COM5', label: 'COM5 - 传感器' },
    { name: 'COM6', label: 'COM6 - 备用设备' }
])

// 每个串口的数据（日志、关键词、备注、暂停状态）
const portDataMap = reactive({})

// 初始化每个串口的数据
function initPortData(portName) {
    if (!portDataMap[portName]) {
        portDataMap[portName] = {
            logs: [],
            keyword: '',
            remark: '',
            paused: false,
            timer: null
        }
    }
}

// 确保所有可用串口都有数据
availablePorts.value.forEach(p => initPortData(p.name))

// ============================================================
// 2. 计算属性
// ============================================================
const totalLogs = computed(() => {
    let sum = 0
    for (const key in portDataMap) {
        sum += portDataMap[key].logs.length
    }
    return sum
})

const selectedPort = computed(() => {
    if (!selectedPortName.value || !portDataMap[selectedPortName.value]) return null
    return portDataMap[selectedPortName.value]
})

function getPortLogs(portName) {
    return portDataMap[portName]?.logs || []
}

function getPortLabel(portName) {
    const found = availablePorts.value.find(p => p.name === portName)
    return found ? found.label.replace(/^[^-]*-\s*/, '') : portName
}

function getPortStatus(portName) {
    const data = portDataMap[portName]
    if (!data) return 'offline'
    if (data.paused) return 'paused'
    return 'online'
}

// ============================================================
// 3. 日志级别颜色
// ============================================================
const levelColors = {
    ERROR: '#F56C6C',
    WARN: '#E6A23C',
    INFO: '#409EFF',
    DEBUG: '#909399'
}

// ============================================================
// 4. 日志消息池
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
// 5. 日志生成
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

function startPortLogGenerator(portName) {
    const data = portDataMap[portName]
    if (!data) return
    if (data.timer) clearInterval(data.timer)

    data.timer = setInterval(() => {
        if (!data.paused && !isGlobalPaused.value) {
            const newLog = generateLog(portName)
            data.logs.push(newLog)
            if (data.logs.length > 500) {
                data.logs.splice(0, data.logs.length - 500)
            }
            // 如果当前选中的是这个端口，滚动到底部
            if (selectedPortName.value === portName) {
                nextTick(() => scrollToBottom())
            }
        }
    }, 800 + Math.random() * 600)
}

// ============================================================
// 6. 滚动到底部
// ============================================================
function scrollToBottom() {
    if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
}

// 监听选中端口变化，滚动到底部
watch(selectedPortName, () => {
    nextTick(() => scrollToBottom())
})

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
const STORAGE_KEYWORD_PREFIX = 'log_keyword_'
const STORAGE_REMARK_PREFIX = 'log_remark_'

function loadPersistedData() {
    availablePorts.value.forEach(p => {
        const name = p.name
        const data = portDataMap[name]
        if (!data) return
        const savedKeyword = localStorage.getItem(STORAGE_KEYWORD_PREFIX + name)
        if (savedKeyword !== null) data.keyword = savedKeyword
        const savedRemark = localStorage.getItem(STORAGE_REMARK_PREFIX + name)
        if (savedRemark !== null) data.remark = savedRemark
    })
}

function saveKeyword(portName, value) {
    localStorage.setItem(STORAGE_KEYWORD_PREFIX + portName, value || '')
}

function saveRemark(portName, value) {
    localStorage.setItem(STORAGE_REMARK_PREFIX + portName, value || '')
}

// ============================================================
// 9. 操作函数
// ============================================================
function selectPort(portName) {
    if (selectedPortName.value === portName) return
    // 确保该端口数据存在
    if (!portDataMap[portName]) {
        initPortData(portName)
        startPortLogGenerator(portName)
    }
    selectedPortName.value = portName
}

function toggleGlobalPause() {
    isGlobalPaused.value = !isGlobalPaused.value
}

function togglePortPause(portName) {
    const data = portDataMap[portName]
    if (data) {
        data.paused = !data.paused
    }
}

function clearPortLogs(portName) {
    const data = portDataMap[portName]
    if (data) {
        data.logs = []
        ElMessage.success(`${portName} 日志已清空`)
    }
}

function clearAllLogs() {
    for (const key in portDataMap) {
        portDataMap[key].logs = []
    }
    ElMessage.success('所有日志已清空')
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
    // 模拟随机离线
    const online = mockPorts.filter(() => Math.random() > 0.2)
    availablePorts.value = online.length > 0 ? online : mockPorts.slice(0, 2)

    // 清理不再存在的端口数据
    const existingNames = new Set(availablePorts.value.map(p => p.name))
    for (const key in portDataMap) {
        if (!existingNames.has(key)) {
            if (portDataMap[key].timer) clearInterval(portDataMap[key].timer)
            delete portDataMap[key]
        }
    }

    // 初始化新端口
    availablePorts.value.forEach(p => {
        if (!portDataMap[p.name]) {
            initPortData(p.name)
            startPortLogGenerator(p.name)
        }
    })

    // 如果当前选中的端口不在列表中，选中第一个
    if (!existingNames.has(selectedPortName.value)) {
        selectedPortName.value = availablePorts.value.length > 0 ? availablePorts.value[0].name : ''
    }

    ElMessage.success(`已刷新串口列表，当前可用 ${availablePorts.value.length} 个`)
}

// ============================================================
// 11. 导出日志
// ============================================================
function exportLogs() {
    // 如果选中了端口，只导出该端口；否则导出全部
    let targetNames = []
    if (selectedPortName.value && portDataMap[selectedPortName.value]?.logs.length > 0) {
        targetNames = [selectedPortName.value]
    } else {
        // 导出所有有日志的端口
        for (const key in portDataMap) {
            if (portDataMap[key].logs.length > 0) targetNames.push(key)
        }
    }

    if (targetNames.length === 0) {
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
    content.push(`导出端口: ${targetNames.join(', ')}`)
    content.push('='.repeat(60))
    content.push('')

    targetNames.forEach(name => {
        const data = portDataMap[name]
        if (!data || data.logs.length === 0) return
        const label = getPortLabel(name)
        content.push(`【${name} - ${label}】`)
        if (data.remark) content.push(`  备注: ${data.remark}`)
        content.push(`  日志条数: ${data.logs.length}`)
        content.push('-'.repeat(40))
        const sorted = [...data.logs]
        sorted.forEach(log => {
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
    const fileName = `日志导出_${targetNames.join('_')}_${now.toISOString().slice(0, 10)}.txt`
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
    // 初始化所有端口数据并启动生成
    availablePorts.value.forEach(p => {
        initPortData(p.name)
        startPortLogGenerator(p.name)
    })
    loadPersistedData()

    // 默认选中第一个
    if (availablePorts.value.length > 0) {
        selectedPortName.value = availablePorts.value[0].name
        setTimeout(scrollToBottom, 300)
    }
})

onUnmounted(() => {
    for (const key in portDataMap) {
        if (portDataMap[key].timer) clearInterval(portDataMap[key].timer)
    }
})
</script>

<style scoped>
/* ============================================================
   整体布局
   ============================================================ */
.realtime-log-page {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow: hidden;
}

/* ============================================================
   顶部工具栏
   ============================================================ */
.toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    background: #fff;
    padding: 8px 16px;
    border-radius: 8px;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
    flex-shrink: 0;
}

.toolbar-left {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
}

.toolbar-right {
    display: flex;
    gap: 6px;
    align-items: center;
}

.total-info {
    color: #606266;
    font-size: 13px;
    margin-left: 4px;
}

/* ============================================================
   主体：左右布局
   ============================================================ */
.main-body {
    flex: 1;
    display: flex;
    gap: 10px;
    min-height: 0;
    overflow: hidden;
}

/* ============================================================
   左侧串口列表
   ============================================================ */
.port-list {
    width: 210px;
    flex-shrink: 0;
    background: #1a1a2e;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid #2a2a4a;
}

.list-header {
    padding: 10px 14px;
    background: #16213e;
    border-bottom: 1px solid #2a2a4a;
    color: #c8d6e5;
    font-size: 14px;
    font-weight: 600;
    display: flex;
    justify-content: space-between;
    flex-shrink: 0;
}

.port-count {
    background: #2a2a4a;
    padding: 0 8px;
    border-radius: 12px;
    font-size: 12px;
    color: #8395a7;
}

.port-item {
    padding: 8px 14px;
    cursor: pointer;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    display: flex;
    justify-content: space-between;
    align-items: center;
    transition: background 0.15s;
    flex-shrink: 0;
}

.port-item:hover {
    background: rgba(255, 255, 255, 0.05);
}

.port-item.active {
    background: rgba(64, 158, 255, 0.15);
    border-left: 3px solid #409EFF;
}

.port-info {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
}

.status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
}

.status-dot.online {
    background: #00d2d3;
    box-shadow: 0 0 6px #00d2d3;
}

.status-dot.paused {
    background: #feca57;
    box-shadow: 0 0 6px #feca57;
}

.status-dot.offline {
    background: #ff6b6b;
    box-shadow: 0 0 6px #ff6b6b;
}

.port-name {
    color: #fff;
    font-weight: 500;
    font-size: 13px;
    white-space: nowrap;
}

.port-label {
    color: #8395a7;
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 70px;
}

.port-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
}

.log-count {
    color: #8395a7;
    font-size: 11px;
}

.active-badge {
    color: #409EFF;
    font-size: 12px;
}

.empty-list {
    color: #576574;
    text-align: center;
    padding: 40px 0;
    font-size: 13px;
}

/* ============================================================
   右侧日志显示区
   ============================================================ */
.log-display {
    flex: 1;
    background: #0f0f1a;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid #2a2a4a;
    min-width: 0;
}

/* ---- 未选择状态 ---- */
.no-selection {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    color: #576574;
    gap: 12px;
}

.no-selection p {
    margin: 0;
    font-size: 16px;
}

/* ---- 日志工具栏 ---- */
.log-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 14px;
    background: #1a1a2e;
    border-bottom: 1px solid #2a2a4a;
    flex-shrink: 0;
    flex-wrap: wrap;
    min-height: 44px;
}

.log-toolbar .el-input {
    flex-shrink: 0;
}

.log-toolbar .port-title {
    color: #c8d6e5;
    font-size: 13px;
    font-weight: 500;
    margin-left: auto;
    white-space: nowrap;
}

/* ---- 日志列表 ---- */
.log-container {
    flex: 1;
    overflow-y: auto;
    padding: 4px 14px;
    font-family: 'Consolas', 'Courier New', monospace;
    font-size: 13px;
    line-height: 1.6;
    min-height: 0;
    background: #0f0f1a;
}

.log-container::-webkit-scrollbar {
    width: 5px;
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
    gap: 12px;
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
    min-width: 82px;
    font-size: 12px;
}

.log-level {
    flex-shrink: 0;
    min-width: 58px;
    font-weight: 600;
    font-size: 12px;
}

.log-content {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    color: #c8d6e5;
    font-size: 13px;
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
    font-size: 14px;
    font-family: 'Segoe UI', sans-serif;
    padding: 40px 0;
}

/* ============================================================
   响应式
   ============================================================ */
@media (max-width: 700px) {
    .port-list {
        width: 140px;
    }

    .port-label {
        max-width: 40px;
    }

    .log-toolbar {
        flex-direction: column;
        align-items: stretch;
    }

    .log-toolbar .el-input {
        width: 100% !important;
    }

    .log-toolbar .port-title {
        margin-left: 0;
        text-align: center;
    }

    .log-line {
        font-size: 12px;
        gap: 6px;
    }

    .log-time {
        min-width: 60px;
        font-size: 11px;
    }

    .log-level {
        min-width: 40px;
        font-size: 11px;
    }

    .log-content {
        font-size: 12px;
    }
}
</style>