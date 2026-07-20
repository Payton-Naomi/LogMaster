<template>
    <div class="log-window">
        <!-- 窗口标题栏 -->
        <div class="window-header">
            <div class="header-left">
                <span class="status-dot online"></span>
                <span class="port-name">{{ portName }}</span>
                <span class="device-label">- {{ deviceLabel }}</span>
                <span class="log-count">{{ logs.length }} 条</span>
            </div>
            <div class="header-right">
                <el-button size="small" :type="isPaused ? 'warning' : 'primary'" @click="emitTogglePause">
                    {{ isPaused ? '▶' : '⏸' }}
                </el-button>
                <el-button size="small" type="danger" plain @click="emitClear">
                    🗑
                </el-button>
            </div>
        </div>

        <!-- 日志列表 -->
        <div ref="logContainer" class="log-container">
            <div v-for="(log, index) in logs" :key="index" class="log-line" :style="{ color: log.color }">
                <span class="log-time">{{ log.time }}</span>
                <span class="log-level">[{{ log.level }}]</span>
                <span class="log-content">{{ log.content }}</span>
            </div>
            <div v-if="logs.length === 0" class="empty-tip">
                <span>暂无日志</span>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
    portName: { type: String, required: true },
    deviceLabel: { type: String, required: true },
    logs: { type: Array, default: () => [] },
    isPaused: { type: Boolean, default: false }
})

const emit = defineEmits(['toggle-pause', 'clear'])

const logContainer = ref(null)

function scrollToBottom() {
    if (!logContainer.value) return
    logContainer.value.scrollTop = logContainer.value.scrollHeight - logContainer.value.clientHeight
}

watch(
    () => props.logs.length,
    () => {
        if (!props.isPaused) {
            nextTick(() => scrollToBottom())
        }
    },
    { immediate: true }
)

function emitTogglePause() {
    emit('toggle-pause')
}

function emitClear() {
    emit('clear')
}
</script>

<style scoped>
.log-window {
    background: #1a1a2e;
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid #2a2a4a;
    min-height: 200px;
}

.window-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px;
    background: #16213e;
    border-bottom: 1px solid #2a2a4a;
    flex-shrink: 0;
}

.header-left {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 14px;
    color: #c8d6e5;
}

.status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #00d2d3;
    box-shadow: 0 0 6px #00d2d3;
}

.status-dot.online {
    background: #00d2d3;
}

.status-dot.offline {
    background: #ff6b6b;
}

.port-name {
    font-weight: 600;
    color: #fff;
}

.device-label {
    color: #8395a7;
    font-size: 13px;
}

.log-count {
    color: #8395a7;
    font-size: 12px;
    margin-left: 4px;
}

.header-right {
    display: flex;
    gap: 4px;
}

.header-right .el-button {
    padding: 4px 10px;
    font-size: 13px;
}

.log-container {
    flex: 1;
    overflow-y: auto;
    padding: 6px 12px;
    font-family: 'Consolas', 'Courier New', monospace;
    font-size: 13px;
    line-height: 1.7;
    min-height: 120px;
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
    padding: 0 4px;
    white-space: nowrap;
    border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.log-time {
    color: #576574;
    flex-shrink: 0;
    min-width: 82px;
}

.log-level {
    flex-shrink: 0;
    min-width: 58px;
    font-weight: 600;
}

.log-content {
    color: #c8d6e5;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
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
</style>