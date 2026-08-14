<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { AutoComplete, Button, Checkbox, Empty, Form, FormItem, Input, Modal, Select, Switch, TabPane, Tabs, Tag, Tooltip } from 'ant-design-vue'
import { DownOutlined, FolderOpenOutlined, KeyOutlined, ReloadOutlined, SaveOutlined, SettingOutlined } from '@ant-design/icons-vue'

const props = defineProps({ device: { type: Object, default: null }, catalog: { type: Object, default: () => ({ projects: [] }) }, keywordHits: { type: Object, default: () => ({}) }, busy: Boolean })
const emit = defineEmits(['save', 'open-folder', 'load-hits', 'reset-hits', 'dirty-change', 'warning'])
const draft = reactive({})
const dirty = ref(false)
const keywordDialog = ref(false)
const pickerGroupId = ref('')
const pickerRuleIds = ref([])
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
const selectedRules = computed(() => rules.value.filter((rule) => draft.keywordRuleIds?.includes(rule.id)))
const versionOptions = computed(() => (project.value?.versions || []).map((value) => ({ value, label: value })))
const storageText = computed(() => formatBytes(props.device?.storageBytes || 0))
const uploadReady = computed(() => Boolean(draft.uploadEnabled && draft.saveEnabled && draft.projectId && String(draft.version || '').trim() && String(draft.uploaderName || '').trim()))

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
function openKeywordDialog() {
  const selectedGroup = groups.value.find((group) => group.rules?.some((rule) => draft.keywordRuleIds?.includes(rule.id)))
  pickerGroupId.value = selectedGroup?.id || groups.value[0]?.id || ''
  pickerRuleIds.value = [...(draft.keywordRuleIds || [])]
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
defineExpose({ confirmSaved })
async function expand(rule) { expandedRule.value = expandedRule.value === rule.id ? '' : rule.id; if (expandedRule.value && !props.keywordHits[rule.id]) emit('load-hits', rule.id) }
function formatBytes(value) { return value >= 1024 ** 3 ? `${(value / 1024 ** 3).toFixed(2)} GB` : value >= 1024 ** 2 ? `${(value / 1024 ** 2).toFixed(1)} MB` : `${Math.ceil(value / 1024)} KB` }
function time(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false, fractionalSecondDigits: 3 }) : '—' }
</script>

<template>
  <aside class="inspector-panel">
    <Empty v-if="!device" class="empty-state" description="选择一个串口后配置" />
    <template v-else>
      <div class="inspector-header"><h2>日志配置</h2><Tag v-if="dirty" color="warning">未保存</Tag></div>

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
      </Form>

      <Form class="form-section" layout="vertical" size="small">
        <div class="policy-grid">
          <div class="policy-item policy-item-wide"><div><strong>上传云端</strong><small>开启后填写项目、版本等上传参数</small></div><Switch class="capsule-switch" size="small" :checked="Boolean(draft.uploadEnabled)" :disabled="!draft.saveEnabled" @change="(value) => setSwitch('uploadEnabled', value)" /></div>
          <div class="policy-item"><div><strong>本地保存</strong><small>保存后写正式日志</small></div><Switch class="capsule-switch" size="small" :checked="Boolean(draft.saveEnabled)" @change="(value) => setSwitch('saveEnabled', value)" /></div>
          <div class="policy-item"><div><strong>关键字匹配</strong><small>暂停时不清零</small></div><Switch class="capsule-switch" size="small" :checked="Boolean(draft.keywordMatchingEnabled)" :disabled="!profile || !draft.keywordRuleIds?.length" @change="(value) => setSwitch('keywordMatchingEnabled', value)" /></div>
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
            <FormItem label="上传人"><Input v-model:value="draft.uploaderName" :maxlength="128" allow-clear placeholder="上传云端时必填" @input="change" /></FormItem>
          </div>
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
        <section class="keyword-rule-list"><Empty v-if="!pickerRules.length" :image="Empty.PRESENTED_IMAGE_SIMPLE" description="该方案没有关键字" /><Checkbox v-for="rule in pickerRules" v-else :key="rule.id" :checked="pickerRuleIds.includes(rule.id)" @change="togglePickerRule(rule.id, $event.target.checked)"><span class="check-row"><strong>{{ rule.name }}</strong><small>{{ rule.match }}</small></span></Checkbox></section>
      </div>
    </Modal>
  </aside>
</template>
