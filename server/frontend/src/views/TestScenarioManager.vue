<template>
  <div class="scenario-page">
    <header class="page-header">
      <div>
        <span class="eyebrow">分析配置</span>
        <h1>测试场景</h1>
        <p>集中维护专项测试的项目范围与筛查关键词</p>
      </div>
      <el-button v-if="mode === 'list'" type="primary" :icon="Plus" @click="startCreate">新建测试场景</el-button>
    </header>

    <el-alert class="analysis-tip" type="info" :closable="false" show-icon>
      <template #title>启用的测试场景会出现在日志上传页</template>
      选择测试场景后，本次日志将只按照所选场景的关键词分析，不再遵循解析规则页面的开关。
    </el-alert>

    <section v-if="mode === 'list'" class="list-panel">
      <div class="list-toolbar">
        <div class="summary">
          <strong>{{ scenarios.length }}</strong><span>个测试场景</span>
          <i></i>
          <strong>{{ enabledCount }}</strong><span>个已启用</span>
        </div>
        <div class="filters">
          <el-radio-group v-model="statusFilter" size="small">
            <el-radio-button value="all">全部</el-radio-button>
            <el-radio-button value="enabled">已启用</el-radio-button>
            <el-radio-button value="disabled">已停用</el-radio-button>
          </el-radio-group>
          <el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索名称、项目或关键词" />
        </div>
      </div>

      <div v-loading="loading" class="scenario-list">
        <article
          v-for="scene in filteredScenarios"
          :key="scene.id"
          class="scenario-card"
          tabindex="0"
          @click="openDetail(scene.id)"
          @keydown.enter="openDetail(scene.id)"
        >
          <div class="card-accent" :class="scene.enabled ? 'enabled' : 'disabled'"></div>
          <div class="card-main">
            <div class="card-title-row">
              <div>
                <h2>{{ scene.name }}</h2>
                <p>{{ projectLabel(scene) }}</p>
              </div>
              <div class="status-control" @click.stop>
                <span>{{ scene.enabled ? '已启用' : '已停用' }}</span>
                <el-switch
                  v-model="scene.enabled"
                  :loading="togglingId === scene.id"
                  @change="toggleEnabled(scene)"
                />
              </div>
            </div>

            <div class="keyword-preview">
              <el-tag v-for="keyword in scene.keywords.slice(0, 4)" :key="keyword" type="danger" effect="plain">{{ keyword }}</el-tag>
              <span v-if="scene.keywords.length > 4">+{{ scene.keywords.length - 4 }}</span>
              <span v-if="!scene.keywords.length" class="empty-text">暂无关键词</span>
            </div>

            <div class="card-footer">
              <p>{{ scene.remark || '暂无备注' }}</p>
              <span>{{ scene.keywords.length }} 个关键词 <ArrowRight /></span>
            </div>
          </div>
        </article>

        <el-empty v-if="!loading && !filteredScenarios.length" description="没有符合条件的测试场景">
          <el-button type="primary" :icon="Plus" @click="startCreate">新建测试场景</el-button>
        </el-empty>
      </div>
    </section>

    <section v-else-if="current && mode === 'detail'" v-loading="loadingDetail" class="detail-panel">
      <button type="button" class="back-button" @click="backToList"><ArrowLeft />返回场景列表</button>
      <div class="detail-heading">
        <div>
          <div class="detail-status"><span :class="current.enabled ? 'on' : 'off'"></span>{{ current.enabled ? '已启用' : '已停用' }}</div>
          <h2>{{ current.name }}</h2>
          <p>最近更新 {{ formatDate(current.updated_at) }}</p>
        </div>
        <div class="detail-actions">
          <div class="detail-switch">
            <span>参与日志分析</span>
            <el-switch v-model="current.enabled" :loading="togglingId === current.id" @change="toggleEnabled(current)" />
          </div>
          <el-button :icon="Edit" @click="startEdit">编辑</el-button>
          <el-button type="danger" plain :icon="Delete" @click="removeCurrent">删除</el-button>
        </div>
      </div>

      <div class="detail-grid">
        <section class="info-block">
          <span class="block-label">专项测试名称</span>
          <strong>{{ current.name }}</strong>
        </section>
        <section class="info-block">
          <span class="block-label">所属项目</span>
          <div class="project-tags">
            <el-tag v-if="current.all_projects" type="info" effect="plain">全部项目</el-tag>
            <el-tag v-for="project in current.projects" v-else :key="project" type="info" effect="plain">{{ project }}</el-tag>
          </div>
        </section>
        <section class="info-block full-span">
          <div class="block-heading"><span class="block-label">筛查关键词</span><em>{{ current.keywords.length }} 个</em></div>
          <div class="keyword-list">
            <span v-for="keyword in current.keywords" :key="keyword">{{ keyword }}</span>
          </div>
        </section>
        <section class="info-block full-span">
          <span class="block-label">备注</span>
          <p class="remark-text">{{ current.remark || '暂无备注' }}</p>
        </section>
      </div>
    </section>

    <section v-else-if="editor" class="editor-panel">
      <button type="button" class="back-button" @click="cancelEdit"><ArrowLeft />{{ mode === 'create' ? '取消新建' : '返回场景详情' }}</button>
      <div class="editor-heading">
        <div><span>{{ mode === 'create' ? '新建场景' : '编辑场景' }}</span><h2>{{ editor.name || '未命名测试场景' }}</h2></div>
        <div class="editor-actions"><el-button @click="cancelEdit">取消</el-button><el-button type="primary" :icon="Check" :loading="saving" @click="save">{{ mode === 'create' ? '创建并保存' : '保存修改' }}</el-button></div>
      </div>

      <el-form label-position="top" class="scenario-form" @submit.prevent>
        <div class="form-row">
          <el-form-item label="专项测试名称" required>
            <el-input v-model="editor.name" maxlength="128" show-word-limit placeholder="例如：SD 卡稳定性专项测试" />
          </el-form-item>
          <el-form-item label="所属项目" required>
            <el-select v-model="editor.projectValues" multiple filterable collapse-tags collapse-tags-tooltip placeholder="选择所属项目" @change="normalizeProjects">
              <el-option label="全部项目" value="__all__" />
              <el-option v-for="project in projects" :key="project" :label="project" :value="project" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="关键字规则" required>
          <div class="keyword-rule-editor">
            <div class="keyword-rule-header"><span>关键字</span><span>严重等级</span><span>备注</span><span></span></div>
            <div v-for="(rule, index) in editor.keywordRules" :key="rule.id" class="keyword-rule-row">
              <el-input v-model.trim="rule.keyword" maxlength="512" placeholder="输入需要筛查的关键字" />
              <el-select v-model="rule.severity" placeholder="选择等级"><el-option v-for="item in severityOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
              <el-input v-model.trim="rule.remark" maxlength="256" placeholder="例如：可能存在存储异常" />
              <el-tooltip content="删除关键字"><el-button :icon="Delete" text circle type="danger" :disabled="editor.keywordRules.length === 1" @click="removeKeywordRule(index)" /></el-tooltip>
            </div>
            <el-button class="add-keyword-button" :icon="Plus" plain :disabled="editor.keywordRules.length >= 200" @click="addKeywordRule">添加关键字</el-button>
          </div>
          <div class="field-meta"><span>日志中匹配到关键字后，按所选严重等级记录，并展示对应备注</span><strong>{{ editorKeywords.length }} 个关键字</strong></div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editor.remark" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="填写测试目的、适用条件或注意事项" />
        </el-form-item>
        <div class="enable-setting">
          <div><strong>参与日志分析</strong><p>启用后，该场景会在匹配项目的日志上传页中提供选择。</p></div>
          <el-switch v-model="editor.enabled" inline-prompt active-text="开" inactive-text="关" />
        </div>
      </el-form>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, ArrowRight, Check, Delete, Edit, Plus, Search } from '@element-plus/icons-vue'
import { getProjects } from '@/api/log'
import { createScenario, deleteScenario, getScenario, getScenarios, setScenarioEnabled, updateScenario } from '@/api/scenarios'
import { uuid } from '@/utils/uuid'

const ALL_PROJECTS = '__all__'
const scenarios = ref([])
const projects = ref([])
const current = ref(null)
const editor = ref(null)
const mode = ref('list')
const search = ref('')
const statusFilter = ref('all')
const loading = ref(false)
const loadingDetail = ref(false)
const saving = ref(false)
const togglingId = ref('')

const severityOptions = [{ value: 'critical', label: '严重' }, { value: 'warning', label: '警告' }, { value: 'info', label: '信息' }]
const newKeywordRule = (keyword = '', severity = 'critical', remark = '') => ({ id: uuid(), keyword, severity, remark })
const enabledCount = computed(() => scenarios.value.filter(item => item.enabled).length)
const editorKeywords = computed(() => [...new Set((editor.value?.keywordRules || []).map(item => item.keyword.trim()).filter(Boolean))])
const filteredScenarios = computed(() => {
  const text = search.value.trim().toLowerCase()
  return scenarios.value.filter(item => {
    const statusMatches = statusFilter.value === 'all' || (statusFilter.value === 'enabled' ? item.enabled : !item.enabled)
    const textMatches = !text || `${item.name} ${item.remark} ${item.projects.join(' ')} ${item.keywords.join(' ')}`.toLowerCase().includes(text)
    return statusMatches && textMatches
  })
})

function normalizeScenario(item) {
  const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
  const checks = Array.isArray(item.checks) ? item.checks : []
  const explicitKeywords = Array.isArray(item.keywords) ? item.keywords : []
  const legacyKeywords = checks.filter(check => check.enabled !== false).flatMap(check => Array.isArray(check.keywords) ? check.keywords : [])
  const allProjects = typeof item.all_projects === 'boolean' ? item.all_projects : (metadata.project_scope || 'all') === 'all'
  return {
    ...item,
    enabled: typeof item.enabled === 'boolean' ? item.enabled : (metadata.status || 'published') === 'published',
    all_projects: allProjects,
    projects: allProjects ? [] : (Array.isArray(item.projects) && item.projects.length ? item.projects : (metadata.projects || [])),
    keywords: [...new Set((explicitKeywords.length ? explicitKeywords : legacyKeywords).map(value => String(value).trim()).filter(Boolean))],
    checks,
    remark: item.remark ?? item.description ?? ''
  }
}

async function loadScenarios() {
  loading.value = true
  try { scenarios.value = (await getScenarios()).map(normalizeScenario) }
  finally { loading.value = false }
}

async function openDetail(id) {
  loadingDetail.value = true
  mode.value = 'detail'
  current.value = scenarios.value.find(item => item.id === id) || null
  try { current.value = normalizeScenario(await getScenario(id)) }
  catch { mode.value = 'list'; current.value = null }
  finally { loadingDetail.value = false }
}

function backToList() {
  mode.value = 'list'
  current.value = null
  editor.value = null
}

function startCreate() {
  editor.value = { id: uuid(), name: '', remark: '', enabled: true, projectValues: [ALL_PROJECTS], keywordRules: [newKeywordRule()] }
  mode.value = 'create'
}

function startEdit() {
  const existingRules = current.value.checks.flatMap(check => (check.keywords || []).map(keyword => newKeywordRule(keyword, check.severity || 'warning', check.description || '')))
  editor.value = {
    id: current.value.id,
    name: current.value.name,
    remark: current.value.remark,
    enabled: current.value.enabled,
    projectValues: current.value.all_projects ? [ALL_PROJECTS] : [...current.value.projects],
    keywordRules: existingRules.length ? existingRules : current.value.keywords.map(keyword => newKeywordRule(keyword))
  }
  mode.value = 'edit'
}

function addKeywordRule() { editor.value.keywordRules.push(newKeywordRule()) }
function removeKeywordRule(index) { if (editor.value.keywordRules.length > 1) editor.value.keywordRules.splice(index, 1) }

function cancelEdit() {
  if (mode.value === 'create') backToList()
  else mode.value = 'detail'
  editor.value = null
}

function normalizeProjects(values) {
  if (!values.includes(ALL_PROJECTS) || values.length === 1) return
  editor.value.projectValues = values.at(-1) === ALL_PROJECTS ? [ALL_PROJECTS] : values.filter(value => value !== ALL_PROJECTS)
}

function validate() {
  if (!editor.value.name.trim()) return '请填写专项测试名称'
  if (!editor.value.projectValues.length) return '请至少选择一个所属项目'
  if (!editorKeywords.value.length) return '请至少填写一个关键字'
  if (editorKeywords.value.length !== editor.value.keywordRules.filter(item => item.keyword.trim()).length) return '关键字不能重复'
  return ''
}

function payloadFromEditor() {
  const allProjects = editor.value.projectValues.includes(ALL_PROJECTS)
  return {
    id: editor.value.id,
    name: editor.value.name.trim(),
    remark: editor.value.remark.trim(),
    enabled: editor.value.enabled,
    all_projects: allProjects,
    projects: allProjects ? [] : editor.value.projectValues,
    checks: editor.value.keywordRules.filter(rule => rule.keyword.trim()).map((rule, index) => ({
      id: `keyword-${editor.value.id}-${index + 1}`,
      name: rule.keyword.trim(),
      description: rule.remark.trim(),
      severity: rule.severity,
      enabled: true,
      source: 'custom',
      match_type: 'forbidden',
      min_count: 1,
      time_window: 0,
      keywords: [rule.keyword.trim()]
    }))
  }
}

async function save() {
  const message = validate()
  if (message) { ElMessage.warning(message); return }
  saving.value = true
  try {
    const payload = payloadFromEditor()
    const saved = mode.value === 'create' ? await createScenario(payload) : await updateScenario(editor.value.id, payload)
    ElMessage.success(mode.value === 'create' ? '测试场景已创建' : '测试场景已保存')
    await loadScenarios()
    current.value = normalizeScenario(saved)
    mode.value = 'detail'
    editor.value = null
  } finally { saving.value = false }
}

async function toggleEnabled(scene) {
  const enabled = scene.enabled
  togglingId.value = scene.id
  try {
    const saved = normalizeScenario(await setScenarioEnabled(scene.id, enabled))
    const index = scenarios.value.findIndex(item => item.id === scene.id)
    if (index >= 0) scenarios.value[index] = saved
    if (current.value?.id === scene.id) current.value = saved
    ElMessage.success(enabled ? '场景已启用，可在日志上传时选择' : '场景已停用')
  } catch {
    scene.enabled = !enabled
  } finally { togglingId.value = '' }
}

async function removeCurrent() {
  try {
    await ElMessageBox.confirm(`删除后无法恢复，确认删除“${current.value.name}”吗？`, '删除测试场景', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
  } catch { return }
  await deleteScenario(current.value.id)
  ElMessage.success('测试场景已删除')
  backToList()
  await loadScenarios()
}

function projectLabel(scene) {
  if (scene.all_projects) return '适用于全部项目'
  if (scene.projects.length <= 2) return scene.projects.join('、')
  return `${scene.projects.slice(0, 2).join('、')} 等 ${scene.projects.length} 个项目`
}

function formatDate(value) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
}

onMounted(async () => {
  const [scenarioResult, projectResult] = await Promise.allSettled([loadScenarios(), getProjects()])
  if (scenarioResult.status === 'rejected') ElMessage.error('测试场景加载失败')
  if (projectResult.status === 'fulfilled') projects.value = projectResult.value || []
  else ElMessage.error('项目列表加载失败')
})
</script>

<style scoped>
.scenario-page { min-height: 100%; padding: 24px; background: #f4f6f9; color: #182235; }
.page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 18px; }
.eyebrow { color: #2775c9; font-size: 12px; font-weight: 700; }
.page-header h1 { margin: 4px 0; font-size: 26px; letter-spacing: 0; }
.page-header p { margin: 0; color: #778398; font-size: 14px; }
.analysis-tip { margin-bottom: 16px; border: 1px solid #cfe1f5; background: #f2f7fd; }
.list-panel, .detail-panel, .editor-panel { border: 1px solid #dfe5ed; border-radius: 8px; background: #fff; }
.list-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 18px 20px; border-bottom: 1px solid #e8ecf2; }
.summary { display: flex; align-items: baseline; gap: 5px; color: #788397; font-size: 13px; }
.summary strong { color: #1f2a3d; font-size: 20px; }
.summary i { width: 1px; height: 18px; margin: 0 8px; background: #d9dfe8; }
.filters { display: flex; gap: 10px; }
.filters .el-input { width: 260px; }
.scenario-list { display: grid; min-height: 300px; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; padding: 18px; }
.scenario-list > .el-empty { grid-column: 1 / -1; }
.scenario-card { position: relative; display: flex; min-width: 0; overflow: hidden; border: 1px solid #e1e6ed; border-radius: 8px; background: #fff; cursor: pointer; transition: border-color .18s, box-shadow .18s, transform .18s; }
.scenario-card:hover, .scenario-card:focus-visible { border-color: #a9c9ee; box-shadow: 0 8px 22px rgba(31, 63, 99, .08); outline: 0; transform: translateY(-1px); }
.card-accent { width: 4px; flex: 0 0 4px; }
.card-accent.enabled { background: #2f80d0; }
.card-accent.disabled { background: #b8c1ce; }
.card-main { min-width: 0; flex: 1; padding: 17px 18px 15px; }
.card-title-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.card-title-row h2 { margin: 0 0 7px; overflow: hidden; font-size: 17px; letter-spacing: 0; text-overflow: ellipsis; white-space: nowrap; }
.card-title-row p { margin: 0; color: #768196; font-size: 12px; }
.status-control { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; color: #68758a; font-size: 12px; }
.keyword-preview { display: flex; min-height: 26px; flex-wrap: wrap; align-items: center; gap: 6px; margin: 18px 0; }
.keyword-preview .el-tag { max-width: 150px; overflow: hidden; text-overflow: ellipsis; }
.keyword-preview > span { color: #7c8799; font-size: 12px; }
.empty-text { color: #a0a9b8; }
.card-footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding-top: 13px; border-top: 1px solid #eef1f5; }
.card-footer p { margin: 0; overflow: hidden; color: #7c8799; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.card-footer span { display: flex; flex: 0 0 auto; align-items: center; gap: 4px; color: #2f76c1; font-size: 12px; }
.card-footer svg { width: 14px; }
.detail-panel, .editor-panel { padding: 22px 26px 34px; }
.back-button { display: inline-flex; align-items: center; gap: 6px; padding: 0; border: 0; background: transparent; color: #5f6e83; cursor: pointer; }
.back-button:hover { color: #2775c9; }
.back-button svg { width: 16px; }
.detail-heading, .editor-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-top: 24px; padding-bottom: 23px; border-bottom: 1px solid #e8ecf2; }
.detail-heading h2, .editor-heading h2 { margin: 7px 0 5px; font-size: 24px; letter-spacing: 0; }
.detail-heading p { margin: 0; color: #8792a3; font-size: 12px; }
.detail-status { display: flex; align-items: center; gap: 7px; color: #68758a; font-size: 12px; }
.detail-status span { width: 8px; height: 8px; border-radius: 50%; }
.detail-status .on { background: #27a56a; }
.detail-status .off { background: #a7b0bd; }
.detail-actions, .editor-actions { display: flex; align-items: center; gap: 9px; }
.detail-switch { display: flex; align-items: center; gap: 9px; margin-right: 5px; color: #546176; font-size: 13px; }
.detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; padding-top: 22px; }
.info-block { min-width: 0; padding: 20px; border: 1px solid #e5e9ef; border-radius: 8px; background: #fbfcfd; }
.info-block strong { display: block; margin-top: 11px; font-size: 17px; }
.full-span { grid-column: 1 / -1; }
.block-label { color: #6e7b90; font-size: 12px; font-weight: 600; }
.block-heading { display: flex; justify-content: space-between; }
.block-heading em { color: #2775c9; font-size: 12px; font-style: normal; }
.project-tags, .keyword-list { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.keyword-list span { max-width: 100%; padding: 7px 10px; overflow-wrap: anywhere; border: 1px solid #f1c7c7; border-radius: 5px; background: #fff5f5; color: #c33636; font: 13px/1.35 Consolas, monospace; }
.remark-text { margin: 12px 0 0; color: #3e4a5d; line-height: 1.7; white-space: pre-wrap; }
.editor-heading span { color: #2775c9; font-size: 12px; font-weight: 700; }
.scenario-form { width: min(860px, 100%); padding-top: 25px; }
.scenario-form :deep(.el-form-item) { margin-bottom: 25px; }
.scenario-form :deep(.el-form-item__label) { padding-bottom: 9px; color: #344054; font-weight: 600; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }
.form-row .el-select { width: 100%; }
.field-meta { display: flex; width: 100%; justify-content: space-between; margin-top: 7px; color: #8a95a7; font-size: 12px; }
.field-meta strong { color: #2775c9; }
.keyword-rule-editor { display: grid; width: 100%; gap: 9px; }
.keyword-rule-header, .keyword-rule-row { display: grid; grid-template-columns: minmax(220px, 1fr) 120px minmax(220px, .8fr) 38px; align-items: center; gap: 9px; }
.keyword-rule-header { padding: 0 4px; color: #8496a3; font-size: 10px; }
.keyword-rule-row { padding: 10px; border: 1px solid rgba(208,230,237,.15); border-radius: 7px; background: rgba(10,18,26,.46); }
.keyword-rule-row :deep(.el-select), .keyword-rule-row :deep(.el-input-number) { width: 100%; }
.add-keyword-button { justify-self: start; margin-top: 2px; }
.enable-setting { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 16px 18px; border: 1px solid #dbe5f1; border-radius: 8px; background: #f7fafd; }
.enable-setting strong { font-size: 14px; }
.enable-setting p { margin: 5px 0 0; color: #7a8799; font-size: 12px; }
@media (max-width: 900px) {
  .scenario-page { padding: 16px; }
  .scenario-list { grid-template-columns: 1fr; }
  .list-toolbar, .detail-heading, .editor-heading { align-items: stretch; flex-direction: column; }
  .filters { flex-wrap: wrap; }
  .filters .el-input { width: 100%; }
  .detail-actions, .editor-actions { flex-wrap: wrap; }
  .detail-grid, .form-row { grid-template-columns: 1fr; }
  .keyword-rule-header { display: none; }
  .keyword-rule-row { grid-template-columns: 1fr 1fr; }
  .keyword-rule-row > :first-child { grid-column: 1 / -1; }
  .full-span { grid-column: auto; }
}
@media (max-width: 560px) {
  .page-header { align-items: stretch; flex-direction: column; }
  .page-header .el-button { width: 100%; }
  .scenario-list { padding: 12px; }
  .card-title-row { flex-direction: column; }
  .detail-panel, .editor-panel { padding: 18px 16px 26px; }
  .detail-switch { width: 100%; justify-content: space-between; }
  .keyword-rule-row { grid-template-columns: 1fr; }
  .keyword-rule-row > :first-child { grid-column: auto; }
}

/* Dark glass presentation shared with the log workspace. */
.scenario-page {
  position: relative;
  min-height: 100%;
  padding: 28px clamp(18px, 2.4vw, 34px) 42px;
  overflow: hidden;
  background:
    radial-gradient(circle at 58% -12%, rgba(64, 199, 223, .11), transparent 42%),
    radial-gradient(circle at 10% 35%, rgba(36, 86, 142, .08), transparent 36%),
    #080d16;
  color: #e8f0f7;
}
.scenario-page::after {
  position: absolute;
  z-index: 0;
  inset: 0;
  pointer-events: none;
  content: '';
  background: linear-gradient(115deg, transparent 20%, rgba(89, 215, 238, .035) 50%, transparent 80%);
}
.scenario-page > * { position: relative; z-index: 1; }
.scenario-page {
  height: 100%;
  max-height: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-color: rgba(121, 213, 230, .42) transparent;
  scrollbar-width: thin;
}
.scenario-page::-webkit-scrollbar { width: 8px; }
.scenario-page::-webkit-scrollbar-track { background: transparent; }
.scenario-page::-webkit-scrollbar-thumb { border: 2px solid transparent; border-radius: 999px; background: rgba(121, 213, 230, .34); background-clip: padding-box; }
.scenario-page::-webkit-scrollbar-thumb:hover { background: rgba(121, 213, 230, .56); background-clip: padding-box; }
.page-header { align-items: center; margin-bottom: 20px; }
.eyebrow, .editor-heading > div > span { color: #76e5f5; letter-spacing: .16em; text-transform: uppercase; }
.page-header h1 { color: #f4f8fb; font-size: clamp(26px, 2.2vw, 34px); font-weight: 700; }
.page-header p { color: #91a5b5; }
.analysis-tip {
  border: 1px solid rgba(121, 213, 230, .24) !important;
  border-radius: 12px !important;
  background: rgba(29, 72, 87, .28) !important;
  color: #b7d1d8 !important;
  box-shadow: inset 0 1px rgba(255,255,255,.06), 0 12px 36px rgba(0,0,0,.18);
  
}
.analysis-tip :deep(.el-alert__title), .analysis-tip :deep(.el-alert__description) { color: #b7d1d8; }
.list-panel, .detail-panel, .editor-panel {
  border: 1px solid rgba(220, 238, 245, .2);
  border-radius: 16px;
  background: rgba(29, 37, 46, .66);
  box-shadow: inset 0 1px rgba(255,255,255,.1), 0 20px 48px rgba(0,0,0,.24);
  
}
.list-toolbar { border-bottom-color: rgba(208, 230, 237, .13); }
.summary, .status-control, .detail-switch, .detail-status, .detail-heading p, .card-title-row p { color: #9eb1bf; }
.summary strong, .card-title-row h2, .detail-heading h2, .editor-heading h2, .info-block strong, .enable-setting strong { color: #edf5f8; }
.summary i, .card-footer { border-color: rgba(208, 230, 237, .13); background-color: transparent; }
.scenario-card {
  border-color: rgba(210, 232, 240, .16);
  border-radius: 13px;
  background: rgba(36, 48, 58, .58);
  box-shadow: inset 0 1px rgba(255,255,255,.07);
  transition: transform .24s cubic-bezier(.4,0,.2,1), border-color .24s cubic-bezier(.4,0,.2,1), box-shadow .24s cubic-bezier(.4,0,.2,1);
}
.scenario-card:hover, .scenario-card:focus-visible { border-color: rgba(82, 218, 240, .62); box-shadow: 0 14px 30px rgba(0,0,0,.3), 0 0 24px rgba(54, 190, 220, .12); transform: translateY(-3px); }
.card-accent.enabled { background: #37d2e5; box-shadow: 0 0 12px rgba(55,210,229,.65); }
.card-accent.disabled { background: #556473; }
.keyword-preview .el-tag { border-color: rgba(255, 117, 127, .42); background: rgba(125, 44, 57, .22); color: #ff9ba1; }
.keyword-preview > span, .empty-text, .card-footer p { color: #8da3b2; }
.card-footer span, .back-button:hover, .block-heading em, .field-meta strong { color: #67ddec; }
.back-button { color: #9eb1bf; }
.detail-heading, .editor-heading { border-bottom-color: rgba(208,230,237,.13); }
.detail-status .on { background: #36d49a; box-shadow: 0 0 10px rgba(54,212,154,.55); }
.detail-status .off { background: #657484; }
.info-block, .enable-setting {
  border-color: rgba(210, 232, 240, .14);
  background: rgba(20, 28, 37, .48);
  box-shadow: inset 0 1px rgba(255,255,255,.045);
}
.block-label { color: #91a9b7; }
.keyword-list span { border-color: rgba(255, 117, 127, .35); background: rgba(121, 42, 54, .18); color: #ff9da5; }
.remark-text, .enable-setting p { color: #a9bbc5; }
.scenario-form :deep(.el-form-item__label) { color: #b8cbd3; }
.scenario-form :deep(.el-input__wrapper), .scenario-form :deep(.el-textarea__inner), .filters :deep(.el-input__wrapper), .filters :deep(.el-select__wrapper) {
  border: 1px solid rgba(208, 230, 237, .18);
  background: rgba(8, 15, 23, .56);
  box-shadow: inset 0 1px rgba(255,255,255,.045), 0 0 0 1px transparent;
}
.scenario-form :deep(.el-input__inner), .scenario-form :deep(.el-textarea__inner), .filters :deep(.el-input__inner), .filters :deep(.el-select__selected-item) { color: #e5f0f4; }
.scenario-form :deep(.el-input__inner::placeholder), .scenario-form :deep(.el-textarea__inner::placeholder), .filters :deep(input::placeholder) { color: #718895; }
.scenario-form :deep(.el-input__wrapper:hover), .scenario-form :deep(.el-input__wrapper.is-focus), .scenario-form :deep(.el-textarea__inner:focus), .filters :deep(.el-input__wrapper:hover), .filters :deep(.el-select__wrapper:hover) { border-color: rgba(79, 213, 235, .65); box-shadow: 0 0 0 1px rgba(79,213,235,.18); }
.enable-setting { border-color: rgba(90, 210, 226, .22); background: rgba(24, 67, 78, .26); }
.scenario-page :deep(.el-button--default) { border-color: rgba(213,235,241,.22); background: rgba(255,255,255,.06); color: #c3d5dc; }
.scenario-page :deep(.el-button--default:hover) { border-color: rgba(91,220,240,.65); background: rgba(52,170,193,.14); color: #e6fbff; }
.scenario-page :deep(.el-button--primary) { border-color: #16b7d1; background: #0daac4; color: #06151b; box-shadow: 0 8px 24px rgba(13,170,196,.24); }
.scenario-page :deep(.el-button--danger.is-plain) { border-color: rgba(255,106,119,.55); background: rgba(130,38,52,.18); color: #ff9ba2; }
.scenario-page :deep(.el-radio-button__inner) { border-color: rgba(211,233,240,.12); background: rgba(8,15,23,.42); color: #95a9b5; box-shadow: none; }
.scenario-page :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) { border-color: #22c8df; background: rgba(18,126,148,.34); color: #a2f3fb; box-shadow: 0 0 16px rgba(34,200,223,.16); }
.scenario-form :deep(.el-select__wrapper),
.scenario-form :deep(.el-input__wrapper),
.scenario-form :deep(.el-textarea__inner) {
  background: rgba(8, 15, 23, .72) !important;
  color: #e5f0f4 !important;
}
.scenario-form :deep(.el-select__wrapper .el-select__selected-item),
.scenario-form :deep(.el-select__wrapper .el-tag) {
  color: #d8e9ee !important;
}
.scenario-form :deep(.el-input__count),
.scenario-form :deep(.el-input__count-inner) {
  background: rgba(18, 29, 39, .94) !important;
  color: #8198a7 !important;
}
.scenario-form :deep(.el-input__count-inner) { padding: 0 3px; }
@media (prefers-reduced-motion: reduce) { .scenario-card { transition: none; } }
</style>
