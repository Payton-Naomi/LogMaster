<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { AutoComplete, Button, Checkbox, Empty, Form, FormItem, Input, Modal, Select, Switch, TabPane, Tabs, Tag, Tooltip } from 'ant-design-vue'
import { DownOutlined, FolderOpenOutlined, KeyOutlined, ReloadOutlined, SaveOutlined, SettingOutlined } from '@ant-design/icons-vue'

const props = defineProps({ device: { type: Object, default: null }, catalog: { type: Object, default: () => ({ projects: [] }) }, keywordHits: { type: Object, default: () => ({}) }, busy: Boolean })
const emit = defineEmits(['save', 'reuse-previous', 'open-folder', 'load-hits', 'reset-hits', 'dirty-change', 'warning'])
const draft = reactive({})
const dirty = ref(false)
const keywordDialog = ref(false)
const pickerGroupId = ref('')
const pickerRuleIds = ref([])
const keywordSearch = ref('')
const expandedRule = ref('')
const baudRates = [9600, 19200, 115200, 921600, 1500000]
const baudOptions = baudRates.map((value) => ({ value: String(value), label: String(value) }))
const clonePlain = (value) => JSON.parse(JSON.stringify(value || {}))

watch(() => props.device?.deviceId, () => resetDraft(), { immediate: true })
watch(() => props.device?.config, (value) => { if (!dirty.value && value) Object.assign(draft, clonePlain(value)) }, { deep: true })
function resetDraft() { Object.keys(draft).forEach((key) => delete draft[key]); if (props.device?.config) Object.assign(draft, clonePlain(props.device.config)); dirty.value = false; emit('dirty-change', false) }
function change() { dirty.value = true; emit('dirty-change', true) }
const projects = computed(() => props.catalog?.projects || [])
const project = computed(() => projects.value.find((item) => item.id === draft.projectId))
const tasks = computed(() => uniqueCatalogItems(projects.value.flatMap((item) => item.tasks || [])))
const task = computed(() => tasks.value.find((item) => item.id === draft.testTaskId))
const profiles = computed(() => uniqueCatalogItems(projects.value.flatMap((item) => item.tasks || []).flatMap((item) => item.keywordProfiles || [])))
const profile = computed(() => profiles.value.find((item) => item.id === draft.keywordProfileId))
const groups = computed(() => {
  if (!profile.value) return []
  if (profile.value.groups?.length) return profile.value.groups
  return (profile.value.rules || []).map((rule) => ({ id: rule.id, name: rule.name, rules: [rule] }))
})
const rules = computed(() => groups.value.flatMap((group) => group.rules || []))
const pickerGroup = computed(() => groups.value.find((item) => item.id === pickerGroupId.value))
const pickerRules = computed(() => pickerGroup.value?.rules || [])
const filteredPickerRules = computed(() => {
  const keyword = keywordSearch.value.trim().toLowerCase()
  if (!keyword) return pickerRules.value
  return pickerRules.value.filter((rule) => [rule.name, rule.match, rule.description, rule.level].some((value) => String(value || '').toLowerCase().includes(keyword)))
})
const selectedRules = computed(() => rules.value.filter((rule) => draft.keywordRuleIds?.includes(rule.id)))
const versionOptions = computed(() => (project.value?.versions || []).map((value) => ({ value, label: value })))
const storageText = computed(() => formatBytes(props.device?.storageBytes || 0))
const uploadReady = computed(() => Boolean(draft.uploadEnabled && draft.saveEnabled && draft.projectId && String(draft.version || '').trim() && String(draft.uploaderEmail || '').trim()))

function uniqueCatalogItems(items) {
  const result = new Map()
  for (const item of items) {
    const current = result.get(item.id)
    if (!current || (!current.keywordProfiles?.length && item.keywordProfiles?.length)) result.set(item.id, item)
  }
  return [...result.values()]
}
function onProjectChange() { draft.projectName = project.value?.name || ''; change() }
function onTaskChange() { draft.testTaskName = task.value?.name || ''; change() }
function onProfileChange() { draft.keywordRuleIds = []; change() }
function onBaudChange(value) { draft.baudRate = Number(value); change() }
function onNumberChange(key, value) { draft[key] = Number(value); change() }
function openKeywordDialog() {
  const selectedGroup = groups.value.find((group) => group.rules?.some((rule) => draft.keywordRuleIds?.includes(rule.id)))
  pickerGroupId.value = selectedGroup?.id || groups.value[0]?.id || ''
  pickerRuleIds.value = [...(draft.keywordRuleIds || [])]
  keywordSearch.value = ''
  keywordDialog.value = true
}
function selectPickerGroup(id) { pickerGroupId.value = id }
function togglePickerRule(id, enabled) {
  const current = new Set(pickerRuleIds.value)
  enabled ? current.add(id) : current.delete(id)
  pickerRuleIds.value = [...current]
}
function confirmKeywordSelection() {
  draft.keywordRuleIds = [...pickerRuleIds.value]
  if (!draft.keywordRuleIds.length) draft.keywordMatchingEnabled = false
  change()
  keywordDialog.value = false
}
function setSwitch(key, value) {
  draft[key] = value
  if (key === 'saveEnabled' && !value) draft.uploadEnabled = false
  change()
}
function save() { emit('save', clonePlain(draft)) }
function confirmSaved() { dirty.value = false; emit('dirty-change', false) }
function applyDraft(value) { Object.keys(draft).forEach((key) => delete draft[key]); Object.assign(draft, clonePlain(value)); dirty.value = true; emit('dirty-change', true) }
defineExpose({ confirmSaved, applyDraft })
async function expand(rule) { expandedRule.value = expandedRule.value === rule.id ? '' : rule.id; if (expandedRule.value && !props.keywordHits[rule.id]) emit('load-hits', rule.id) }
function formatBytes(value) { return value >= 1024 ** 3 ? `${(value / 1024 ** 3).toFixed(2)} GB` : value >= 1024 ** 2 ? `${(value / 1024 ** 2).toFixed(1)} MB` : `${Math.ceil(value / 1024)} KB` }
function time(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false, fractionalSecondDigits: 3 }) : '—' }
</script>

<template>
  <aside class="inspector-panel">
    <Empty v-if="!device" class="empty-state" description="选择一个串口后配置" />
    <template v-else>
      <div class="inspector-header"><h2>日志配置</h2><div class="inspector-header-actions"><Button v-if="device.config?.previousConfigAvailable" size="small" @click="emit('reuse-previous')">复用上次业务配置</Button><Tag v-if="dirty" color="warning">未保存</Tag></div></div>

      <Tabs class="inspector-tabs" size="small">
      <TabPane key="config" tab="通道配置">
      <Form class="form-section" layout="vertical" size="small">
        <FormItem label="通道名称"><Input v-model:value="draft.name" @input="change" /></FormItem>
        <div class="field-grid">
          <FormItem label="串口"><Input :value="draft.portName" disabled /></FormItem>
          <FormItem label="波特率"><AutoComplete :value="String(draft.baudRate || '')" :options="baudOptions" placeholder="选择或手动输入" @change="onBaudChange" /></FormItem>
        </div>
        <div class="serial-format-grid">
          <FormItem label="数据位"><Select v-model:value="draft.dataBits" :options="[5,6,7,8].map((value) => ({ value, label: String(value) }))" @change="change" /></FormItem>
          <FormItem label="停止位"><Select v-model:value="draft.stopBits" :options="[1,2].map((value) => ({ value, label: String(value) }))" @change="change" /></FormItem>
          <FormItem label="校验位"><Select v-model:value="draft.parity" :options="[{value:'none',label:'无'},{value:'odd',label:'奇校验'},{value:'even',label:'偶校验'},{value:'mark',label:'标记'},{value:'space',label:'空格'}]" @change="change" /></FormItem>
          <FormItem label="编码"><Select v-model:value="draft.encoding" :options="[{value:'utf-8',label:'UTF-8'},{value:'gb18030',label:'GB18030'},{value:'ascii',label:'ASCII'}]" @change="change" /></FormItem>
        </div>
        <div class="field-grid">
          <FormItem class="signal-switches"><div class="inline-switches"><span>DTR <Switch class="capsule-switch" size="small" :checked="Boolean(draft.dtr)" @change="(value) => setSwitch('dtr', value)" /></span><span>RTS <Switch class="capsule-switch" size="small" :checked="Boolean(draft.rts)" @change="(value) => setSwitch('rts', value)" /></span></div></FormItem>
        </div>
        <div class="field-grid serial-tuning-grid">
          <FormItem label="读取超时（毫秒）"><Input :value="draft.readTimeoutMs" type="number" min="1" @change="(event) => onNumberChange('readTimeoutMs', event.target.value)" /></FormItem>
          <FormItem label="写入超时（毫秒）"><Input :value="draft.writeTimeoutMs" type="number" min="1" @change="(event) => onNumberChange('writeTimeoutMs', event.target.value)" /></FormItem>
          <FormItem label="空闲分帧（毫秒）"><Input :value="draft.idleGapMs" type="number" min="1" @change="(event) => onNumberChange('idleGapMs', event.target.value)" /></FormItem>
          <FormItem label="最大帧长度（字节）"><Input :value="draft.maxFrameBytes" type="number" min="256" @change="(event) => onNumberChange('maxFrameBytes', event.target.value)" /></FormItem>
        </div>
        <small class="field-help">空闲分帧用于没有换行符的串口日志；设备持续输出时达到最大帧长度会自动封口。</small>
      </Form>

      <Form class="form-section" layout="vertical" size="small">
        <div class="policy-grid">
          <div class="policy-item policy-item-wide"><Tooltip title="开启后填写项目、版本等上传参数"><strong>上传云端</strong></Tooltip><Switch class="capsule-switch" size="small" :checked="Boolean(draft.uploadEnabled)" :disabled="!draft.saveEnabled" @change="(value) => setSwitch('uploadEnabled', value)" /></div>
          <div class="policy-item"><Tooltip title="保存后写正式日志"><strong>本地保存</strong></Tooltip><Switch class="capsule-switch" size="small" :checked="Boolean(draft.saveEnabled)" @change="(value) => setSwitch('saveEnabled', value)" /></div>
          <div class="policy-item"><Tooltip title="暂停时不清零"><strong>关键字匹配</strong></Tooltip><Switch class="capsule-switch" size="small" :checked="Boolean(draft.keywordMatchingEnabled)" :disabled="!profile || !draft.keywordRuleIds?.length" @change="(value) => setSwitch('keywordMatchingEnabled', value)" /></div>
        </div>
        <FormItem label="关键字方案"><Select v-model:value="draft.keywordProfileId" allow-clear placeholder="请选择" :options="profiles.map((item) => ({ value: item.id, label: item.name }))" @change="onProfileChange" /></FormItem>
        <Button block :disabled="!profile" @click="openKeywordDialog"><template #icon><KeyOutlined /></template>选择关键字（{{ draft.keywordRuleIds?.length || 0 }}）</Button>
        <template v-if="draft.uploadEnabled">
          <div class="upload-fields-heading">云端上传参数</div>
          <div class="field-grid">
            <FormItem label="项目名称"><Select v-model:value="draft.projectId" allow-clear placeholder="请选择" :options="projects.map((item) => ({ value: item.id, label: item.name }))" @change="onProjectChange" /></FormItem>
            <FormItem label="版本号"><AutoComplete v-model:value="draft.version" allow-clear :options="versionOptions" placeholder="请选择或输入" @change="change" /></FormItem>
          </div>
          <div class="field-grid">
            <FormItem label="测试任务"><Select v-model:value="draft.testTaskId" allow-clear placeholder="请选择" :options="tasks.map((item) => ({ value: item.id, label: item.name }))" @change="onTaskChange" /></FormItem>
            <FormItem label="上传人企业邮箱"><Input v-model:value="draft.uploaderEmail" type="email" :maxlength="320" allow-clear placeholder="name@company.com" @input="change" /></FormItem>
          </div>
          <FormItem v-if="draft.uploaderName" label="已识别上传人"><Input :value="draft.uploaderName" disabled /></FormItem>
          <FormItem label="测试备注"><Input.TextArea v-model:value="draft.remark" :maxlength="4000" :auto-size="{ minRows: 1, maxRows: 3 }" allow-clear placeholder="选填" @input="change" /></FormItem>
        </template>
        <Button type="primary" block :loading="busy" :disabled="!dirty && device.config?.configured" @click="save"><template #icon><SaveOutlined /></template>{{ uploadReady ? '保存上传/通道配置' : '保存通道配置' }}</Button>
      </Form>

      <div class="storage-card"><SettingOutlined /><div><span>当前通道存储</span><strong>{{ storageText }}</strong></div><Tooltip title="打开日志目录"><Button aria-label="打开日志目录" shape="circle" @click="emit('open-folder')"><template #icon><FolderOpenOutlined /></template></Button></Tooltip></div>
      </TabPane>

      <TabPane key="keywords" tab="关键字统计">
      <div class="dashboard-section">
        <div class="section-title"><strong>关键字仪表盘</strong><Tooltip title="重置本次会话统计"><Button aria-label="重置关键字统计" size="small" shape="circle" :disabled="!device.sessionId" @click="emit('reset-hits')"><template #icon><ReloadOutlined /></template></Button></Tooltip></div>
        <Empty v-if="!selectedRules.length" :image="Empty.PRESENTED_IMAGE_SIMPLE" description="尚未选择关键字" />
        <div v-for="rule in selectedRules" :key="rule.id" class="rule-card">
          <Button block @click="expand(rule)"><span><strong>{{ rule.name }}</strong><small>{{ rule.match }}</small></span><b>{{ device.ruleCounts?.[rule.name] || 0 }}</b><DownOutlined :class="{ rotate: expandedRule === rule.id }" /></Button>
          <div v-if="expandedRule === rule.id" class="rule-hits"><div v-for="hit in keywordHits[rule.id] || []" :key="hit.id"><time>{{ time(hit.matchedAt) }}</time><span>#{{ hit.sequence }} {{ hit.lineText }}</span></div><Empty v-if="!(keywordHits[rule.id] || []).length" :image="Empty.PRESENTED_IMAGE_SIMPLE" description="暂无命中记录" /></div>
        </div>
      </div>
      </TabPane>
      </Tabs>
    </template>

    <Modal v-model:open="keywordDialog" title="选择关键字" ok-text="完成" @ok="confirmKeywordSelection">
      <div class="keyword-picker">
        <aside class="keyword-profile-list"><Button v-for="item in groups" :key="item.id" block :type="pickerGroupId === item.id ? 'primary' : 'text'" @click="selectPickerGroup(item.id)"><span>{{ item.name }}</span><small v-if="item.scope">{{ item.scope }}</small></Button></aside>
        <section class="keyword-rule-list"><Input.Search v-model:value="keywordSearch" allow-clear placeholder="搜索名称、内容或级别" /><Empty v-if="!filteredPickerRules.length" :image="Empty.PRESENTED_IMAGE_SIMPLE" :description="pickerRules.length ? '没有匹配的关键字' : '该方案没有关键字'" /><Checkbox v-for="rule in filteredPickerRules" v-else :key="rule.id" :checked="pickerRuleIds.includes(rule.id)" @change="togglePickerRule(rule.id, $event.target.checked)"><span class="check-row"><strong>{{ rule.name }}<Tag v-if="rule.readOnly" color="blue">云端</Tag></strong><small>{{ rule.match }}{{ rule.level ? ` · ${rule.level}` : '' }}{{ rule.description ? ` · ${rule.description}` : '' }}</small></span></Checkbox></section>
      </div>
    </Modal>
  </aside>
</template>
