<template>
    <main class="realtime-page">
        <section class="collector-card">
            <h1>串口配置与实时采集</h1>

            <div class="form-grid project-grid">
                <label class="form-field">
                    <span>新项目名称</span>
                    <el-input v-model="newProjectName" placeholder="请输入项目名称" />
                </label>

                <label class="form-field">
                    <span>分析关键字（逗号分隔）</span>
                    <el-input v-model="keywords" placeholder="ERROR, timeout, dropped" />
                </label>
            </div>

            <div class="project-actions">
                <el-button @click="createProject">创建项目</el-button>
                <el-button :loading="isRefreshing" @click="refreshData">刷新串口与项目</el-button>
            </div>

            <div class="form-grid source-grid">
                <label class="form-field">
                    <span>串口</span>
                    <el-select v-model="selectedPort" placeholder="请选择串口" :disabled="isConnected">
                        <el-option v-for="port in ports" :key="port" :label="port" :value="port" />
                    </el-select>
                </label>

                <label class="form-field">
                    <span>所属项目</span>
                    <el-select v-model="selectedProject" placeholder="请选择项目" :disabled="isConnected">
                        <el-option v-for="project in projects" :key="project" :label="project" :value="project" />
                    </el-select>
                </label>
            </div>

            <div class="serial-grid">
                <label class="form-field">
                    <span>波特率</span>
                    <el-select v-model="baudRate" :disabled="isConnected">
                        <el-option v-for="item in baudRates" :key="item" :label="item" :value="item" />
                    </el-select>
                </label>

                <label class="form-field">
                    <span>数据位</span>
                    <el-select v-model="dataBits" :disabled="isConnected">
                        <el-option v-for="item in dataBitOptions" :key="item" :label="item" :value="item" />
                    </el-select>
                </label>

                <label class="form-field">
                    <span>停止位</span>
                    <el-select v-model="stopBits" :disabled="isConnected">
                        <el-option v-for="item in stopBitOptions" :key="item" :label="item" :value="item" />
                    </el-select>
                </label>

                <label class="form-field">
                    <span>校验位</span>
                    <el-select v-model="parity" :disabled="isConnected">
                        <el-option label="无" value="none" />
                        <el-option label="奇校验" value="odd" />
                        <el-option label="偶校验" value="even" />
                    </el-select>
                </label>
            </div>

            <div class="connection-actions">
                <el-button type="primary" :disabled="isConnected" @click="connectPort">
                    连接并开始采集
                </el-button>
                <el-button :disabled="!isConnected" @click="disconnectPort">断开连接</el-button>
            </div>

            <p v-if="isConnected" class="connection-status">
                已连接 {{ selectedPort }} · {{ selectedProject }} · 会话 {{ sessionId }}
            </p>
            <p v-else class="connection-status muted">尚未连接串口</p>

            <div class="monitor-bar">
                <div>
                    <strong>实时规则监控</strong>
                    <span v-for="rule in ruleDefinitions" :key="rule.name">{{ rule.name }}</span>
                </div>
                <div><b>{{ matchedCount }}</b> 次命中</div>
            </div>

            <div ref="logContainer" class="log-console" aria-live="polite">
                <div v-if="logs.length === 0" class="empty-log">连接串口后将在此显示实时日志</div>
                <div v-for="(log, index) in logs" :key="`${log.time}-${index}`" class="log-line" :class="{ matched: log.matchedRule }">
                    <span class="log-meta">[{{ log.time }} {{ log.level }}</span>
                    <span>{{ log.message }}</span>
                    <em v-if="log.matchedRule">{{ log.matchedRule }}</em>
                </div>
            </div>
        </section>
    </main>
</template>

<script setup>
import { computed, nextTick, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'

const newProjectName = ref('DR2860')
const keywords = ref('backtrace, FAT-fs, queue is full!!! drop frame')
const projects = ref(['DR2860'])
const ports = ref(['COM5', 'COM6', 'COM7'])
const selectedPort = ref('COM5')
const selectedProject = ref('DR2860')
const baudRate = ref('115200')
const dataBits = ref('8')
const stopBits = ref('1')
const parity = ref('none')
const isConnected = ref(false)
const isRefreshing = ref(false)
const sessionId = ref('')
const logs = ref([])
const logContainer = ref(null)

const baudRates = ['9600', '19200', '38400', '57600', '115200', '230400']
const dataBitOptions = ['5', '6', '7', '8']
const stopBitOptions = ['1', '1.5', '2']

const sampleLogs = [
    ['DEBUG', 'ui_close_top_page-1078]:end'],
    ['DEBUG', 'ui_page_open-961]:enter home page'],
    ['INFO', 'XA_DevMng_Process_Event-1104]:event:UIMNG_EV_HOME_PAGE_OPEN_NOTIFY'],
    ['INFO', 'xa_dev_monitor_set_home_page_state-29]:xa_dev_monitor_set_home_page_state[1]'],
    ['INFO', 'DevMng_Key_Handle-459]:press_key == 1'],
    ['DEBUG', 'home_page_callback-1202]:[home_page_callback] LV_PLUGIN_EVENT_SCR_OPEN'],
    ['DEBUG', 'XA_UIMng_Homepage_Refresh-396]:XA_UIMng_Homepage_Refresh timer_state = 0'],
    ['DEBUG', 'SystemMng_normal_record_handle-339]:event:SYSTEMMNG_EV_RECORD_SWITCH_CMD'],
    ['INFO', 'Audio_Out_AACPlay-921]:read aac file, ret:3045, file_len:3045'],
    ['INFO', 'Audio_Out_AACPlay-934]:aac decoder init, sample:16000, ch:1, pcm_size:0!'],
    ['INFO', 'voice_play_loop_task-258]:[VOICE][play]voice play end = /mnt/app/voice/res/audio/public/key_tone.aac'],
    ['INFO', 'filemng_monitor_thread-4511]:movie_mb:4219, total_mb:236131, avail_mb:231587, warning_mb:23613, total_full_threshold:23613'],
    ['INFO', 'PowerMng]:wakeup source POWER_ID_SWRT, 2f0050080 : 00000001 00000000'],
    ['ERROR', 'recorder]:queue is full!!! drop frame channel=0 seq=98231'],
    ['ERROR', 'storage]:FAT-fs (mmcblk0p1): invalid access to FAT'],
    ['WARN', 'storage]:speed monitor state cb, state = low_speed'],
    ['ERROR', 'storage]:SD write detected frame loss for 15842ms'],
    ['FATAL', 'signal]:Log_Signal_Data backtrace: #00 recorder_service']
]

const ruleDefinitions = [
    { name: '异常重启', patterns: ['POWER_ID_SWRT', '2f0050080 :'] },
    { name: '系统崩溃', patterns: ['backtrace', 'Log_Signal_Data'] },
    { name: '视频丢帧', patterns: ['queue is full!!! drop frame', 'SD write detected frame loss for'] },
    { name: '存储异常', patterns: ['FAT-fs', 'STGMNG_SD_ERROR_STATE'] }
]
const matchedCount = computed(() => logs.value.filter((log) => log.matchedRule).length)

let logTimer = null
let sampleIndex = 0

function createProject() {
    const projectName = newProjectName.value.trim()
    if (!projectName) {
        ElMessage.warning('请输入项目名称')
        return
    }

    if (!projects.value.includes(projectName)) projects.value.push(projectName)
    selectedProject.value = projectName
    ElMessage.success(`项目“${projectName}”创建成功`)
}

function refreshData() {
    isRefreshing.value = true
    window.setTimeout(() => {
        ports.value = ['COM5', 'COM6', 'COM7']
        if (!ports.value.includes(selectedPort.value)) selectedPort.value = ports.value[0]
        isRefreshing.value = false
        ElMessage.success('串口与项目已刷新')
    }, 450)
}

function connectPort() {
    if (!selectedPort.value || !selectedProject.value) {
        ElMessage.warning('请选择串口和所属项目')
        return
    }

    isConnected.value = true
    sessionId.value = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
    logs.value = []
    sampleIndex = 0
    appendLog()
    logTimer = window.setInterval(appendLog, 650)
    ElMessage.success(`${selectedPort.value} 已连接`)
}

function disconnectPort() {
    isConnected.value = false
    stopLogTimer()
    ElMessage.info('串口连接已断开')
}

function appendLog() {
    const [level, message] = sampleLogs[sampleIndex % sampleLogs.length]
    const matchedRule = ruleDefinitions.find((rule) => rule.patterns.some((pattern) => message.includes(pattern)))?.name || ''
    logs.value.push({ time: formatTime(new Date()), level, message, matchedRule })
    sampleIndex += 1

    if (logs.value.length > 500) logs.value.shift()
    nextTick(() => {
        if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight
    })
}

function formatTime(date) {
    const pad = (value, length = 2) => String(value).padStart(length, '0')
    return `${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}:${pad(date.getMilliseconds(), 3)}`
}

function stopLogTimer() {
    if (!logTimer) return
    window.clearInterval(logTimer)
    logTimer = null
}

onUnmounted(stopLogTimer)
</script>

<style scoped>
.realtime-page {
    min-height: 100%;
    padding: 28px 32px;
    background: #f4f6f9;
    box-sizing: border-box;
}

.collector-card {
    width: min(100%, 1220px);
    margin: 0 auto;
    padding: 24px 26px;
    background: #fff;
    border: 1px solid #dcdfe6;
    border-radius: 6px;
    box-shadow: 0 1px 4px rgba(31, 45, 61, 0.04);
    box-sizing: border-box;
}

h1 {
    margin: 0 0 22px;
    color: #20252b;
    font-size: 20px;
    font-weight: 700;
    line-height: 1.4;
    letter-spacing: 0;
}

.form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
}

.project-grid {
    margin-bottom: 16px;
}

.source-grid {
    margin-top: 20px;
}

.serial-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 14px;
    margin-top: 15px;
}

.form-field {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 8px;
}

.form-field>span {
    color: #606266;
    font-size: 14px;
    line-height: 1.4;
}

.form-field :deep(.el-select) {
    width: 100%;
}

.form-field :deep(.el-input__wrapper),
.form-field :deep(.el-select__wrapper) {
    min-height: 44px;
    border-radius: 4px;
    box-shadow: 0 0 0 1px #cfd3d8 inset;
}

.form-field :deep(.el-input__inner),
.form-field :deep(.el-select__placeholder) {
    font-size: 15px;
}

.project-actions,
.connection-actions {
    display: flex;
    align-items: center;
    gap: 10px;
}

.project-actions :deep(.el-button),
.connection-actions :deep(.el-button) {
    height: 44px;
    margin: 0;
    padding: 0 15px;
    border-radius: 4px;
    font-size: 15px;
}

.connection-actions {
    margin-top: 18px;
}

.connection-actions :deep(.el-button--primary) {
    font-weight: 600;
}

.connection-status {
    min-height: 20px;
    margin: 15px 0 14px;
    color: #606266;
    font-family: Consolas, 'Courier New', monospace;
    font-size: 13px;
    line-height: 20px;
    overflow-wrap: anywhere;
}

.connection-status.muted {
    color: #a0a5ad;
}

.monitor-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 38px;
    padding: 0 12px;
    border: 1px solid #dce3ec;
    border-bottom: 0;
    border-radius: 4px 4px 0 0;
    background: #f7f9fc;
}

.monitor-bar > div,
.monitor-bar > div:first-child {
    display: flex;
    align-items: center;
    gap: 8px;
}

.monitor-bar strong { color: #455266; font-size: 12px; }
.monitor-bar span { padding: 3px 7px; border-radius: 3px; background: #e9eef5; color: #667085; font-size: 10px; }
.monitor-bar > div:last-child { color: #7a8493; font-size: 11px; }
.monitor-bar b { color: #d95858; font-size: 14px; }

.log-console {
    height: clamp(300px, 42vh, 430px);
    padding: 0 16px 14px;
    overflow: auto;
    color: #edf2fa;
    background: #121827;
    border: 1px solid #20293a;
    border-radius: 0 0 4px 4px;
    box-sizing: border-box;
    font-family: Consolas, 'Courier New', monospace;
    font-size: 14px;
    line-height: 1.72;
}

.log-line {
    min-width: max-content;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
}

.log-line.matched { margin: 0 -16px; padding: 0 16px; background: rgba(217, 88, 88, 0.12); }
.log-line em { margin-left: 10px; padding: 1px 5px; border-radius: 3px; background: #7f3038; color: #ffdfe2; font: 10px sans-serif; font-style: normal; }

.log-meta {
    margin-right: 6px;
    color: #f3f6fb;
    font-weight: 600;
}

.empty-log {
    display: grid;
    min-height: 100%;
    place-items: center;
    color: #7e8798;
}

.log-console::-webkit-scrollbar {
    width: 10px;
    height: 10px;
}

.log-console::-webkit-scrollbar-track {
    background: #f3f3f3;
}

.log-console::-webkit-scrollbar-thumb {
    background: #858585;
    border: 2px solid #f3f3f3;
    border-radius: 5px;
}

@media (max-width: 820px) {
    .realtime-page {
        padding: 16px;
    }

    .collector-card {
        padding: 20px;
    }

    .serial-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }
}

@media (max-width: 560px) {
    .realtime-page {
        padding: 0;
    }

    .collector-card {
        padding: 18px 14px;
        border-width: 0;
        border-radius: 0;
    }

    .form-grid,
    .serial-grid {
        grid-template-columns: 1fr;
    }

    .project-actions,
    .connection-actions {
        align-items: stretch;
        flex-direction: column;
    }

    .project-actions :deep(.el-button),
    .connection-actions :deep(.el-button) {
        width: 100%;
    }

    .log-console {
        height: 360px;
        font-size: 12px;
    }

    .monitor-bar { align-items: flex-start; flex-direction: column; gap: 8px; padding: 9px 10px; }
    .monitor-bar > div:first-child { flex-wrap: wrap; }
}
</style>
