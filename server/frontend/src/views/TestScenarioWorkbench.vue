<template>
  <div class="page">
    <header class="page-header">
      <div><h1>测试场景</h1><p>{{ scenarios.length }} 个场景 · {{ publishedCount }} 个已发布 · {{ totalChecks }} 项检查</p></div>
      <div class="page-actions"><el-button :icon="Setting" @click="router.push('/rules')">解析规则</el-button><el-button type="primary" :icon="Plus" @click="create">新建场景</el-button></div>
    </header>

    <div class="workspace-layout">
      <aside class="scene-library">
        <div class="library-header"><strong>场景库</strong><span>{{ filteredScenarios.length }}</span></div>
        <div class="library-filters">
          <el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索场景" />
          <el-select v-model="statusFilter" clearable placeholder="全部状态">
            <el-option v-for="item in statuses" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </div>
        <div v-loading="loading" class="scene-list">
          <button v-for="item in filteredScenarios" :key="item.id" type="button" class="scene-item" :class="{ active: item.id === selectedId }" @click="selectScene(item.id)">
            <span class="scene-color" :class="`color-${item.color}`" />
            <span class="scene-copy">
              <span class="scene-name">{{ item.name }}</span>
              <span class="scene-meta"><span>{{ item.checks.filter(check => check.enabled).length }} 项启用</span><span>{{ projectSummary(item) }}</span></span>
            </span>
            <el-tag size="small" effect="plain" :type="statusType(item.metadata.status)">{{ statusLabel(item.metadata.status) }}</el-tag>
          </button>
          <el-empty v-if="!loading && !filteredScenarios.length" description="暂无匹配场景" :image-size="64" />
        </div>
      </aside>

      <main v-if="editor" class="scene-editor">
        <div class="editor-header">
          <div class="editor-title">
            <span class="title-mark" :class="`color-${editor.color}`" />
            <div><div class="title-line"><h2>{{ editor.name || '未命名场景' }}</h2><span v-if="dirty" class="unsaved-dot">未保存</span></div><span>更新于 {{ formatTime(editor.updated_at) }}</span></div>
          </div>
          <div class="editor-actions">
            <el-tooltip content="复制场景" placement="bottom"><el-button :icon="CopyDocument" circle @click="duplicate" /></el-tooltip>
            <el-tooltip content="删除场景" placement="bottom"><el-button :icon="Delete" circle type="danger" plain @click="remove" /></el-tooltip>
            <el-button type="primary" :icon="Check" :loading="saving" @click="save">保存更改</el-button>
          </div>
        </div>

        <section class="editor-section">
          <div class="section-heading">
            <div><span class="section-index">01</span><h3>基本信息</h3></div>
            <el-radio-group v-model="editor.metadata.status" size="small">
              <el-radio-button v-for="item in statuses" :key="item.value" :value="item.value">{{ item.label }}</el-radio-button>
            </el-radio-group>
          </div>
          <div class="form-grid">
            <el-form-item label="场景名称" class="span-2"><el-input v-model="editor.name" maxlength="128" show-word-limit placeholder="例如：冷启动稳定性测试" /></el-form-item>
            <el-form-item label="场景说明" class="span-2"><el-input v-model="editor.description" type="textarea" :rows="2" maxlength="500" placeholder="填写场景目标和适用条件" /></el-form-item>
            <el-form-item label="判定方式">
              <el-select v-model="editor.judgement"><el-option label="任一严重或警告项失败" value="any-error" /><el-option label="仅严重项失败" value="critical-only" /><el-option label="全部检查项通过" value="all-pass" /></el-select>
            </el-form-item>
            <el-form-item label="标识颜色">
              <div class="color-picker" role="radiogroup" aria-label="标识颜色">
                <button v-for="item in colors" :key="item" type="button" class="color-swatch" :class="[`color-${item}`, { selected: editor.color === item }]" :aria-label="colorLabel(item)" @click="editor.color = item"><el-icon v-if="editor.color === item"><Check /></el-icon></button>
              </div>
            </el-form-item>
            <el-form-item label="适用项目" class="span-2 project-scope">
              <el-radio-group v-model="editor.metadata.project_scope"><el-radio value="all">全部项目</el-radio><el-radio value="selected">指定项目</el-radio></el-radio-group>
              <el-select v-if="editor.metadata.project_scope === 'selected'" v-model="editor.metadata.projects" multiple filterable collapse-tags collapse-tags-tooltip placeholder="选择适用项目">
                <el-option v-for="project in projects" :key="project" :label="project" :value="project" />
              </el-select>
            </el-form-item>
            <el-form-item label="场景标签" class="span-2"><el-select v-model="editor.metadata.tags" multiple filterable allow-create default-first-option placeholder="输入标签后回车" /></el-form-item>
          </div>
        </section>

        <section class="editor-section checks-section">
          <div class="section-heading">
            <div><span class="section-index">02</span><h3>检查项</h3><span class="count-label">{{ enabledCheckCount }}/{{ editor.checks.length }} 已启用</span></div>
            <div class="check-actions"><el-button :icon="Plus" @click="addCheck">自定义检查项</el-button><el-button type="primary" plain :icon="Link" @click="openRulePicker">从规则库添加</el-button></div>
          </div>
          <div class="check-list">
            <article v-for="(check, index) in editor.checks" :key="check.id" class="check-item" :class="{ disabled: !check.enabled }">
              <div class="check-header">
                <span class="check-order">{{ String(index + 1).padStart(2, '0') }}</span>
                <el-input v-model="check.name" class="check-name" placeholder="检查项名称" />
                <el-tag size="small" effect="plain" :type="severityType(check.severity)">{{ severityLabel(check.severity) }}</el-tag>
                <el-switch v-model="check.enabled" inline-prompt active-text="开" inactive-text="关" />
                <el-button text circle :icon="Delete" aria-label="删除检查项" @click="editor.checks.splice(index, 1)" />
              </div>
              <div class="check-fields">
                <el-form-item label="检测来源"><el-select v-model="check.source" @change="handleSourceChange(check)"><el-option label="自定义关键词" value="custom" /><el-option label="引用解析规则" value="rule" /></el-select></el-form-item>
                <el-form-item v-if="check.source === 'rule'" label="解析规则" class="span-2">
                  <el-select v-model="check.rule_id" filterable placeholder="选择规则" @change="syncRule(check)"><el-option v-for="rule in rules" :key="rule.id" :label="rule.name" :value="rule.id"><span>{{ rule.name }}</span><span class="rule-level">{{ rule.enabled ? severityLabel(rule.level) : '场景将强制启用' }}</span></el-option></el-select>
                </el-form-item>
                <el-form-item v-else label="关键词或表达式" class="span-2"><el-input v-model="check.keywordsText" type="textarea" :rows="2" placeholder="每行一个关键词或表达式" /></el-form-item>
                <el-form-item label="期望结果"><el-select v-model="check.match_type"><el-option label="必须出现" value="required" /><el-option label="禁止出现" value="forbidden" /><el-option label="达到次数" value="min-count" /></el-select></el-form-item>
                <el-form-item v-if="check.match_type === 'min-count'" label="最低次数"><el-input-number v-model="check.min_count" :min="1" :max="99999" controls-position="right" /></el-form-item>
                <el-form-item label="时间范围"><el-input-number v-model="check.time_window" :min="0" :max="86400" controls-position="right" /><span class="input-suffix">秒，0 为全部</span></el-form-item>
                <el-form-item label="失败等级"><el-select v-model="check.severity"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /><el-option label="提示" value="info" /></el-select></el-form-item>
                <el-form-item label="检查说明" class="span-3"><el-input v-model="check.description" placeholder="可选" /></el-form-item>
                <div v-if="check.source === 'rule'" class="rule-reference span-3" :class="{ invalid: ruleIssue(check) }">
                  <template v-if="ruleById(check.rule_id)"><div><el-icon><Link /></el-icon><strong>{{ ruleById(check.rule_id).name }}</strong><el-tag size="small" effect="plain" :type="ruleById(check.rule_id).enabled ? 'success' : 'warning'">{{ ruleById(check.rule_id).enabled ? '规则已启用' : '场景启用时强制开启' }}</el-tag></div><p>{{ ruleById(check.rule_id).keyword }}</p><span>{{ categoryLabel(ruleById(check.rule_id).category) }} · {{ ruleById(check.rule_id).scope || '全部日志' }}</span></template>
                  <template v-else><div><el-icon><Warning /></el-icon><strong>引用规则不可用</strong></div><p>请重新选择解析规则后再保存。</p></template>
                </div>
              </div>
            </article>
            <el-empty v-if="!editor.checks.length" description="暂无检查项" :image-size="72"><el-button type="primary" plain :icon="Plus" @click="addCheck">添加第一项</el-button></el-empty>
          </div>
        </section>
      </main>
      <main v-else class="scene-editor empty-editor"><el-empty description="选择或新建一个测试场景"><el-button type="primary" :icon="Plus" @click="create">新建场景</el-button></el-empty></main>
    </div>

    <el-dialog v-model="rulePickerOpen" title="从解析规则添加检查项" width="760px" class="rule-picker-dialog">
      <div class="rule-picker-filters"><el-input v-model="ruleSearch" :prefix-icon="Search" clearable placeholder="搜索规则名称、关键词或说明" /><el-select v-model="ruleCategory" clearable placeholder="全部分类"><el-option v-for="item in categories" :key="item.value" :label="item.label" :value="item.value" /></el-select></div>
      <el-checkbox-group v-model="selectedRuleIds" class="rule-picker-list">
        <label v-for="rule in pickerRules" :key="rule.id" class="rule-picker-item" :class="{ disabled: isRuleAdded(rule.id) }">
          <el-checkbox :value="rule.id" :disabled="isRuleAdded(rule.id)" />
          <span class="picker-main"><span><strong>{{ rule.name }}</strong><el-tag size="small" effect="plain" :type="severityType(rule.level)">{{ severityLabel(rule.level) }}</el-tag></span><small>{{ rule.keyword }}</small></span>
          <span class="picker-side"><span>{{ categoryLabel(rule.category) }}</span><small>{{ isRuleAdded(rule.id) ? '已添加' : rule.enabled ? `${rule.scenario_count || 0} 个场景引用` : '当前关闭，场景可强制开启' }}</small></span>
        </label>
        <el-empty v-if="!pickerRules.length" description="暂无匹配规则" :image-size="60" />
      </el-checkbox-group>
      <template #footer><span class="selection-count">已选择 {{ selectedRuleIds.length }} 项</span><el-button @click="rulePickerOpen = false">取消</el-button><el-button type="primary" :disabled="!selectedRuleIds.length" @click="addSelectedRules">添加检查项</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, CopyDocument, Delete, Link, Plus, Search, Setting, Warning } from '@element-plus/icons-vue'
import { getProjects } from '@/api/log'
import { getRules } from '@/api/rules'
import { createScenario, deleteScenario, getScenarios, updateScenario } from '@/api/scenarios'

const statuses = [{ value: 'draft', label: '草稿' }, { value: 'published', label: '已发布' }, { value: 'disabled', label: '已停用' }]
const colors = ['blue', 'green', 'orange', 'red']
const categories = [{ value: 'power', label: '开关机与电源' }, { value: 'storage', label: 'SD 卡与存储' }, { value: 'recording', label: '录像与视频' }, { value: 'system', label: '系统稳定性' }, { value: 'connectivity', label: '连接通信' }, { value: 'feature', label: '设备功能' }, { value: 'tool', label: '辅助工具' }]
const router = useRouter()
const scenarios = ref([])
const projects = ref([])
const rules = ref([])
const selectedId = ref('')
const editor = ref(null)
const search = ref('')
const statusFilter = ref('')
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const rulePickerOpen = ref(false)
const ruleSearch = ref('')
const ruleCategory = ref('')
const selectedRuleIds = ref([])
let hydrating = false

const publishedCount = computed(() => scenarios.value.filter(item => item.metadata.status === 'published').length)
const totalChecks = computed(() => scenarios.value.reduce((sum, item) => sum + item.checks.length, 0))
const enabledCheckCount = computed(() => editor.value?.checks.filter(item => item.enabled).length || 0)
const pickerRules = computed(() => {
  const text = ruleSearch.value.trim().toLowerCase()
  return rules.value.filter(rule => (!ruleCategory.value || rule.category === ruleCategory.value) && (!text || `${rule.name} ${rule.keyword} ${rule.description}`.toLowerCase().includes(text)))
})
const filteredScenarios = computed(() => {
  const text = search.value.trim().toLowerCase()
  return scenarios.value.filter(item => {
    const haystack = `${item.name} ${item.description} ${item.metadata.tags.join(' ')} ${item.metadata.projects.join(' ')}`.toLowerCase()
    return (!statusFilter.value || item.metadata.status === statusFilter.value) && (!text || haystack.includes(text))
  })
})

function normalizeCheck(check = {}) {
  return {
    id: check.id || crypto.randomUUID(), name: check.name || '', description: check.description || '',
    severity: check.severity || 'warning', enabled: check.enabled !== false,
    source: check.source || (check.rule_id ? 'rule' : 'custom'), rule_id: check.rule_id || null,
    rule_name: check.rule_name || '', rule_updated_at: check.rule_updated_at || null,
    match_type: check.match_type || 'required', min_count: Number(check.min_count) || 1,
    time_window: Number(check.time_window) || 0,
    keywordsText: Array.isArray(check.keywords) ? check.keywords.join('\n') : ''
  }
}

function fromAPI(item) {
  const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
  return {
    ...item,
    metadata: {
      status: metadata.status || 'published', project_scope: metadata.project_scope || 'all',
      projects: Array.isArray(metadata.projects) ? metadata.projects : [],
      tags: Array.isArray(metadata.tags) ? metadata.tags : []
    },
    checks: (Array.isArray(item.checks) ? item.checks : []).map(normalizeCheck)
  }
}

function toAPI(item) {
  return {
    ...item,
    metadata: { status: item.metadata.status, project_scope: item.metadata.project_scope, projects: item.metadata.project_scope === 'selected' ? item.metadata.projects : [], tags: item.metadata.tags },
    checks: item.checks.map(({ keywordsText, ...check }) => ({ ...check, keywords: keywordsText.split('\n').map(value => value.trim()).filter(Boolean) }))
  }
}

async function load(preferredId = selectedId.value) {
  loading.value = true
  try {
    scenarios.value = (await getScenarios()).map(fromAPI)
    selectedId.value = scenarios.value.some(item => item.id === preferredId) ? preferredId : (scenarios.value[0]?.id || '')
    await hydrateEditor()
  } finally { loading.value = false }
}

async function hydrateEditor() {
  hydrating = true
  const source = scenarios.value.find(item => item.id === selectedId.value)
  editor.value = source ? JSON.parse(JSON.stringify(source)) : null
  dirty.value = false
  await nextTick()
  hydrating = false
}

async function selectScene(id) {
  if (id === selectedId.value) return
  if (dirty.value) {
    try { await ElMessageBox.confirm('当前修改尚未保存，切换后将丢失。', '切换场景', { confirmButtonText: '继续切换', cancelButtonText: '留在当前页', type: 'warning' }) }
    catch { return }
  }
  selectedId.value = id
  await hydrateEditor()
}

async function create() {
  const id = crypto.randomUUID()
  const item = { id, name: '新测试场景', description: '', color: 'blue', judgement: 'any-error', metadata: { status: 'draft', project_scope: 'all', projects: [], tags: [] }, checks: [] }
  await createScenario(toAPI(item))
  await load(id)
}

function addCheck() { editor.value.checks.push(normalizeCheck()) }
function openRulePicker() { selectedRuleIds.value = []; ruleSearch.value = ''; ruleCategory.value = ''; rulePickerOpen.value = true }
function isRuleAdded(ruleId) { return editor.value?.checks.some(check => check.source === 'rule' && check.rule_id === ruleId) }
function addSelectedRules() {
  selectedRuleIds.value.forEach(ruleId => {
    const rule = rules.value.find(item => item.id === ruleId)
    if (!rule || isRuleAdded(ruleId)) return
    editor.value.checks.push(normalizeCheck({ id: crypto.randomUUID(), name: rule.name, description: rule.description, severity: rule.level, enabled: true, source: 'rule', rule_id: rule.id, rule_name: rule.name, rule_updated_at: rule.updated_at, match_type: 'forbidden', min_count: 1, time_window: 0, keywords: [rule.keyword] }))
  })
  rulePickerOpen.value = false
}
function handleSourceChange(check) { if (check.source === 'custom') check.rule_id = null; else check.keywordsText = '' }
function syncRule(check) {
  const rule = rules.value.find(item => item.id === check.rule_id)
  if (!rule) return
  check.name ||= rule.name
  check.description ||= rule.description
  check.severity = rule.level
  check.keywordsText = rule.keyword || ''
  check.rule_name = rule.name
  check.rule_updated_at = rule.updated_at
}

function validate() {
  if (!editor.value.name.trim()) return '请填写场景名称'
  if (editor.value.metadata.project_scope === 'selected' && !editor.value.metadata.projects.length) return '请选择至少一个适用项目'
  const unnamed = editor.value.checks.find(item => !item.name.trim())
  if (unnamed) return '请补充检查项名称'
  const incomplete = editor.value.checks.find(item => item.enabled && (!item.keywordsText.trim() || (item.source === 'rule' && !item.rule_id)))
  if (incomplete) return `请完善检查项“${incomplete.name}”的检测内容`
  const unavailable = editor.value.checks.find(item => item.enabled && item.source === 'rule' && ruleIssue(item))
  return unavailable ? `检查项“${unavailable.name}”引用的规则不可用` : ''
}

async function save() {
  const message = validate()
  if (message) { ElMessage.warning(message); return }
  saving.value = true
  try { await updateScenario(editor.value.id, toAPI(editor.value)); ElMessage.success('场景已保存'); await load(editor.value.id) }
  finally { saving.value = false }
}

async function duplicate() {
  const clone = structuredClone(editor.value)
  clone.id = crypto.randomUUID()
  clone.name = `${clone.name} - 副本`
  clone.metadata.status = 'draft'
  clone.checks = clone.checks.map(item => ({ ...item, id: crypto.randomUUID() }))
  await createScenario(toAPI(clone))
  ElMessage.success('已创建场景副本')
  await load(clone.id)
}

async function remove() {
  try { await ElMessageBox.confirm(`确定删除“${editor.value.name}”吗？`, '删除场景', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }) }
  catch { return }
  await deleteScenario(editor.value.id)
  ElMessage.success('场景已删除')
  selectedId.value = ''
  await load()
}

const statusLabel = value => statuses.find(item => item.value === value)?.label || value
const statusType = value => ({ draft: 'info', published: 'success', disabled: 'warning' })[value] || 'info'
const severityLabel = value => ({ critical: '严重', warning: '警告', info: '提示' })[value] || value
const severityType = value => ({ critical: 'danger', warning: 'warning', info: 'info' })[value] || 'info'
const colorLabel = value => ({ blue: '蓝色', green: '绿色', orange: '橙色', red: '红色' })[value]
const categoryLabel = value => categories.find(item => item.value === value)?.label || value
const ruleById = id => rules.value.find(item => item.id === id)
const ruleIssue = check => !ruleById(check.rule_id)
const projectSummary = item => item.metadata.project_scope === 'selected' ? `${item.metadata.projects.length} 个项目` : '全部项目'
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '刚刚'

watch(editor, () => { if (!hydrating && editor.value) dirty.value = true }, { deep: true })
onMounted(async () => {
  const [, projectResult, ruleResult] = await Promise.allSettled([load(), getProjects(), getRules()])
  if (projectResult.status === 'fulfilled') projects.value = projectResult.value || []
  if (ruleResult.status === 'fulfilled') rules.value = ruleResult.value || []
})
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#243142}.page-header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:16px}.page-header h1{margin:0;font-size:22px;letter-spacing:0}.page-header p{margin:5px 0 0;color:#7d8795;font-size:12px}.page-actions,.check-actions{display:flex;gap:8px}
.workspace-layout{display:grid;min-height:calc(100% - 62px);grid-template-columns:280px minmax(0,1fr);gap:16px}.scene-library,.scene-editor{border:1px solid #dfe4ea;border-radius:6px;background:#fff}.scene-library{display:flex;min-height:540px;flex-direction:column;overflow:hidden}.library-header{display:flex;align-items:center;justify-content:space-between;padding:16px 16px 12px}.library-header strong{font-size:14px}.library-header span{display:grid;min-width:24px;height:22px;place-items:center;border-radius:11px;background:#edf1f5;color:#667383;font-size:11px}.library-filters{display:grid;grid-template-columns:minmax(0,1fr) 98px;gap:8px;padding:0 12px 12px;border-bottom:1px solid #edf0f3}.scene-list{min-height:300px;padding:8px;overflow-y:auto}.scene-item{position:relative;display:flex;width:100%;min-height:72px;align-items:flex-start;gap:10px;margin:0 0 4px;padding:12px 10px;border:1px solid transparent;border-radius:5px;background:transparent;color:#334154;text-align:left;cursor:pointer}.scene-item:hover{background:#f6f8fa}.scene-item.active{border-color:#cfe0f7;background:#edf5ff}.scene-color{width:4px;height:34px;flex:0 0 4px;border-radius:2px}.scene-copy{display:flex;min-width:0;flex:1;flex-direction:column;gap:7px}.scene-name{overflow:hidden;font-size:13px;font-weight:600;text-overflow:ellipsis;white-space:nowrap}.scene-meta{display:flex;gap:8px;color:#85909e;font-size:11px}.scene-item .el-tag{flex:0 0 auto}
.scene-editor{min-width:0;overflow:hidden}.editor-header{display:flex;min-height:76px;align-items:center;justify-content:space-between;gap:18px;padding:14px 18px;border-bottom:1px solid #e6e9ed}.editor-title,.title-line,.editor-actions,.section-heading,.section-heading>div,.check-header{display:flex;align-items:center}.editor-title{min-width:0;gap:11px}.title-mark{width:5px;height:38px;flex:0 0 5px;border-radius:3px}.editor-title h2{overflow:hidden;max-width:440px;margin:0;font-size:17px;text-overflow:ellipsis;white-space:nowrap;letter-spacing:0}.editor-title>div>span{color:#8993a0;font-size:11px}.title-line{gap:9px;margin-bottom:5px}.unsaved-dot{padding:2px 6px;border-radius:3px;background:#fff3dc;color:#a56a00;font-size:10px}.editor-actions{flex:0 0 auto;gap:8px}
.editor-section{padding:18px}.editor-section+.editor-section{border-top:8px solid #f2f4f7}.section-heading{justify-content:space-between;margin-bottom:17px}.section-heading>div{gap:9px}.section-heading h3{margin:0;font-size:15px;letter-spacing:0}.section-index{color:#2f78d3;font-size:11px;font-weight:700}.count-label{color:#84909e;font-size:11px}.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 16px}.span-2{grid-column:span 2}.span-3{grid-column:span 3}.form-grid :deep(.el-form-item){margin-bottom:16px}.form-grid :deep(.el-select){width:100%}.color-picker{display:flex;height:32px;align-items:center;gap:11px}.color-swatch{display:grid;width:24px;height:24px;place-items:center;border:2px solid #fff;border-radius:50%;box-shadow:0 0 0 1px #d9dee5;color:#fff;cursor:pointer}.color-swatch.selected{box-shadow:0 0 0 2px #2c6fc4}.project-scope :deep(.el-form-item__content){display:flex;flex-wrap:nowrap;gap:16px}.project-scope .el-select{flex:1}
.check-list{display:flex;flex-direction:column;gap:10px}.check-item{border:1px solid #dfe4ea;border-radius:6px;background:#fff}.check-item.disabled{opacity:.62}.check-header{min-height:52px;gap:10px;padding:9px 12px;border-bottom:1px solid #edf0f3;background:#f8f9fb}.check-order{width:26px;flex:0 0 26px;color:#8190a1;font-size:11px;font-weight:700}.check-name{min-width:160px;flex:1}.check-name :deep(.el-input__wrapper){box-shadow:none;background:transparent}.check-name :deep(.el-input__inner){font-weight:600}.check-fields{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:0 14px;padding:15px 48px}.check-fields :deep(.el-form-item){margin-bottom:13px}.check-fields :deep(.el-select),.check-fields :deep(.el-input-number){width:100%}.input-suffix{display:block;margin-top:4px;color:#8a95a3;font-size:10px}.rule-level{float:right;margin-left:18px;color:#9099a6;font-size:11px}.rule-reference{padding:10px 12px;border:1px solid #dbe7f5;border-radius:5px;background:#f6f9fd}.rule-reference.invalid{border-color:#efd3a4;background:#fff9ed}.rule-reference>div{display:flex;align-items:center;gap:7px}.rule-reference p{overflow:hidden;margin:7px 0 5px;color:#4c5b6e;font-family:Consolas,monospace;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.rule-reference>span{color:#8792a0;font-size:10px}.empty-editor{display:grid;min-height:500px;place-items:center}
.rule-picker-filters{display:grid;grid-template-columns:minmax(0,1fr) 170px;gap:10px;margin-bottom:12px}.rule-picker-list{display:flex;max-height:430px;flex-direction:column;gap:6px;overflow-y:auto}.rule-picker-item{display:flex;min-height:66px;align-items:center;gap:10px;padding:10px 12px;border:1px solid #e1e5ea;border-radius:5px;cursor:pointer}.rule-picker-item:hover{border-color:#bcd2ee;background:#f8fbff}.rule-picker-item.disabled{cursor:not-allowed;opacity:.58}.picker-main{display:flex;min-width:0;flex:1;flex-direction:column;gap:6px}.picker-main>span{display:flex;align-items:center;gap:7px}.picker-main small{overflow:hidden;color:#778393;font-family:Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}.picker-side{display:flex;min-width:108px;flex-direction:column;gap:5px;color:#657283;font-size:11px;text-align:right}.picker-side small{color:#929ba7}.selection-count{margin-right:auto;color:#6e7987;font-size:12px}:deep(.rule-picker-dialog .el-dialog__footer){display:flex;align-items:center}
.color-blue{background:#3985e6}.color-green{background:#2c9a72}.color-orange{background:#e19134}.color-red{background:#d85c5c}
@media(max-width:1100px){.workspace-layout{grid-template-columns:240px minmax(0,1fr)}.check-fields{grid-template-columns:repeat(2,minmax(0,1fr));padding:15px}.check-fields .span-3{grid-column:span 2}.editor-title h2{max-width:240px}}
@media(max-width:820px){.workspace-layout{grid-template-columns:1fr}.scene-library{min-height:auto}.scene-list{display:flex;gap:8px;overflow-x:auto}.scene-item{min-width:220px}.editor-header{align-items:flex-start;flex-direction:column}.editor-actions{width:100%;justify-content:flex-end}}
@media(max-width:600px){.page-header{align-items:flex-start;gap:12px}.page-actions .el-button:first-child{display:none}.workspace-layout{min-height:0}.library-filters,.form-grid,.check-fields,.rule-picker-filters{grid-template-columns:1fr}.span-2,.span-3,.check-fields .span-3{grid-column:span 1}.section-heading{align-items:flex-start;gap:12px}.section-heading .el-radio-group{flex-wrap:nowrap}.check-actions{flex-direction:column}.project-scope :deep(.el-form-item__content){align-items:flex-start;flex-direction:column}.project-scope .el-select{width:100%}.check-header{flex-wrap:wrap}.check-name{order:-1;width:calc(100% - 38px);flex-basis:calc(100% - 38px)}.check-order{order:-2}.editor-actions .el-button:not(.el-button--primary){display:none}.picker-side{display:none}}
</style>
