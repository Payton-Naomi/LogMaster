<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { Button, Checkbox, Input, InputSearch, Tooltip } from 'ant-design-vue'
import { ArrowDownOutlined, ArrowUpOutlined, CopyOutlined, DeleteOutlined, DownloadOutlined, FileTextOutlined, FolderOpenOutlined, MenuFoldOutlined, MenuUnfoldOutlined, MinusOutlined, PlusOutlined, SendOutlined } from '@ant-design/icons-vue'
import { DynamicScroller, DynamicScrollerItem } from 'vue-virtual-scroller'
import { useLogSearch } from '../composables/useLogSearch'

const props = defineProps({ rows: { type: Array, default: () => [] }, device: { type: Object, default: null }, busy: Boolean, defaultWrap: { type: Boolean, default: true }, logFontSize: { type: Number, default: 12 }, inspectorOpen: Boolean })
const emit = defineEmits(['clear', 'save-session', 'export-window', 'open-folder', 'send-command', 'log-font-size-change', 'toggle-inspector'])
const scroller = ref(null)
const wrap = ref(props.defaultWrap)
const command = ref('')
const selectedKey = ref('')
const rowList = computed(() => props.rows)
const scrollToIndex = (index) => scroller.value?.scrollToItem?.(index)
const search = useLogSearch(rowList, scrollToIndex)
const activeMatchIndex = computed(() => search.matches.value[search.current.value])

function copyRow(row) { navigator.clipboard?.writeText(`${row.timestamp} ${row.message || row.text || ''}`); selectedKey.value = row.key }
function windowContent() { return props.rows.map((row) => `${row.timestamp} ${row.message || row.text || ''}`).join('\n') + (props.rows.length ? '\n' : '') }
function submitCommand() { const value = command.value.trim(); if (!value) return; emit('send-command', value); command.value = '' }
function changeFont(delta) { emit('log-font-size-change', Math.min(24, Math.max(10, props.logFontSize + delta))) }
function highlightSegments(text) { return search.segments(text) }

watch(() => props.rows.length, () => { if (search.follow.value) nextTick(() => scrollToIndex(Math.max(0, props.rows.length - 1))) })
watch(() => props.defaultWrap, (value) => { wrap.value = value })
</script>

<template>
  <section class="console-panel" :style="{ '--log-font-size': `${logFontSize}px` }">
    <div class="console-header">
      <div><h2>{{ device?.name || '选择串口' }}</h2><p>{{ device?.portName || '—' }} · {{ device?.enabled ? (device?.noLogAlert ? '长时间无日志' : '实时采集中') : '已关闭' }}</p></div>
      <div class="console-actions">
        <Tooltip title="清空当前窗口"><Button aria-label="清空当前窗口" shape="circle" :disabled="!device" @click="emit('clear')"><template #icon><DeleteOutlined /></template></Button></Tooltip>
        <Checkbox v-model:checked="wrap" class="wrap-checkbox">自动换行</Checkbox>
        <Tooltip title="减小日志字号"><Button aria-label="减小日志字号" shape="circle" :disabled="logFontSize <= 10" @click="changeFont(-1)"><template #icon><MinusOutlined /></template></Button></Tooltip>
        <span class="font-size-value" title="当前日志字号">{{ logFontSize }}</span>
        <Tooltip title="增大日志字号"><Button aria-label="增大日志字号" shape="circle" :disabled="logFontSize >= 24" @click="changeFont(1)"><template #icon><PlusOutlined /></template></Button></Tooltip>
        <Button :disabled="!device?.config?.configured" @click="emit('save-session', windowContent())"><template #icon><DownloadOutlined /></template><span class="save-label">另存日志</span></Button>
        <Tooltip title="打开日志目录"><Button aria-label="打开日志目录" shape="circle" @click="emit('open-folder')"><template #icon><FolderOpenOutlined /></template></Button></Tooltip>
        <Tooltip title="导出当前窗口"><Button aria-label="导出当前窗口" shape="circle" :disabled="!rows.length" @click="emit('export-window', windowContent())"><template #icon><FileTextOutlined /></template></Button></Tooltip>
        <Tooltip :title="inspectorOpen ? '收起日志配置' : '展开日志配置'"><Button aria-label="切换日志配置" shape="circle" @click="emit('toggle-inspector')"><template #icon><MenuFoldOutlined v-if="inspectorOpen" /><MenuUnfoldOutlined v-else /></template></Button></Tooltip>
      </div>
    </div>

    <div class="search-toolbar">
      <InputSearch v-model:value="search.draft.value" class="search-field" placeholder="输入关键字后点击搜索" enter-button="搜索" @search="search.search" />
      <Tooltip title="上一个匹配"><Button aria-label="上一个匹配" shape="circle" :disabled="!search.matches.value.length" @click="search.move(-1)"><template #icon><ArrowUpOutlined /></template></Button></Tooltip>
      <Tooltip title="下一个匹配"><Button aria-label="下一个匹配" shape="circle" :disabled="!search.matches.value.length" @click="search.move(1)"><template #icon><ArrowDownOutlined /></template></Button></Tooltip>
      <span class="match-count">{{ search.positionText.value }}</span>
      <Button v-if="!search.follow.value" @click="search.resume">继续滚动</Button>
      <span class="line-counter">{{ rows.length }} 行</span>
    </div>

    <DynamicScroller ref="scroller" class="log-console" :items="rows" :min-item-size="28" key-field="key">
      <template #default="{ item, index, active }">
        <DynamicScrollerItem :item="item" :active="active" :size-dependencies="[item.message, wrap, logFontSize]" :data-index="index">
          <div class="log-row" :class="{ nowrap: !wrap, selected: selectedKey === item.key, 'current-match': activeMatchIndex === index }" @click="copyRow(item)" @contextmenu.prevent="copyRow(item)">
            <span class="log-number">{{ item.lineNumber }}</span>
            <span class="log-time">{{ item.timestamp }}</span>
            <span class="log-message"><template v-for="(segment, segmentIndex) in highlightSegments(item.message || item.text || '')" :key="segmentIndex"><mark v-if="segment.match">{{ segment.text }}</mark><template v-else>{{ segment.text }}</template></template></span>
            <CopyOutlined class="copy-indicator" />
          </div>
        </DynamicScrollerItem>
      </template>
      <template #empty><div class="empty-log">等待串口日志</div></template>
    </DynamicScroller>

    <form class="command-bar" @submit.prevent="submitCommand"><span>&gt;</span><Input v-model:value="command" :disabled="!device?.enabled" placeholder="向当前串口发送指令" /><Tooltip title="发送指令"><Button class="command-send" aria-label="发送指令" html-type="submit" shape="circle" :disabled="!device?.enabled || !command.trim()"><template #icon><SendOutlined /></template></Button></Tooltip></form>
  </section>
</template>
