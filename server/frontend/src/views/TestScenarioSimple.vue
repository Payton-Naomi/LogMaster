<template>
  <div class="scenario-page">
    <header class="page-header">
      <div>
        <span class="eyebrow">分析配置</span>
        <h1>测试场景</h1>
        <p>{{ scenarios.length }} 个场景 · {{ totalKeywords }} 个筛查关键词</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="startCreate">新建场景</el-button>
    </header>

    <section class="workspace">
      <aside class="scene-library">
        <div class="library-title">
          <strong>场景列表</strong>
          <span>{{ filteredScenarios.length }}</span>
        </div>
        <el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索名称、关键词或项目" />

        <div v-loading="loading" class="scene-list">
          <button
            v-for="scene in filteredScenarios"
            :key="scene.id"
            type="button"
            class="scene-item"
            :class="{ active: !creating && scene.id === selectedId }"
            @click="selectScene(scene.id)"
          >
            <span class="scene-item-main">
              <strong>{{ scene.name }}</strong>
              <small>{{ scene.keywords.length }} 个关键词 · {{ projectSummary(scene.projects) }}</small>
            </span>
            <ArrowRight class="scene-arrow" />
          </button>
          <el-empty v-if="!loading && !filteredScenarios.length" description="暂无匹配场景" :image-size="72" />
        </div>
      </aside>

      <main v-if="editor" class="scene-editor">
        <div class="editor-header">
          <div>
            <span class="editor-kicker">{{ creating ? '新场景' : '场景配置' }}</span>
            <h2>{{ editor.name || '未命名场景' }}</h2>
            <p>{{ creating ? '填写必要信息后创建' : `最近更新 ${formatDate(editor.updated_at)}` }}</p>
          </div>
          <div class="editor-actions">
            <el-button v-if="creating" @click="cancelCreate">取消</el-button>
            <el-button v-else type="danger" plain :icon="Delete" @click="removeCurrent">删除</el-button>
            <el-button type="primary" :icon="Check" :loading="saving" @click="save">
              {{ creating ? '创建场景' : '保存修改' }}
            </el-button>
          </div>
        </div>

        <el-form label-position="top" class="scene-form" @submit.prevent>
          <el-form-item label="测试场景名称" required>
            <el-input v-model="editor.name" maxlength="128" show-word-limit placeholder="例如：开关机稳定性测试" />
          </el-form-item>

          <el-form-item label="所需要筛查的关键词" required>
            <el-input
              v-model="editor.keywordsText"
              type="textarea"
              :rows="11"
              resize="vertical"
              placeholder="每行输入一个关键词"
            />
            <div class="field-meta">
              <span>空行和重复项会自动忽略</span>
              <strong>{{ currentKeywords.length }} 个关键词</strong>
            </div>
          </el-form-item>

          <el-form-item label="项目" required>
            <el-select
              v-model="editor.projects"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              placeholder="选择适用项目"
              class="full-width"
              @change="normalizeProjects"
            >
              <el-option label="全部项目" value="__all__" />
              <el-option v-for="project in projects" :key="project" :label="project" :value="project" />
            </el-select>
          </el-form-item>

          <el-form-item label="备注">
            <el-input
              v-model="editor.description"
              type="textarea"
              :rows="4"
              maxlength="1000"
              show-word-limit
              placeholder="补充适用条件、测试目的或注意事项"
            />
          </el-form-item>
        </el-form>
      </main>

      <main v-else class="scene-editor empty-editor">
        <el-empty description="选择一个场景，或新建场景">
          <el-button type="primary" :icon="Plus" @click="startCreate">新建场景</el-button>
        </el-empty>
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, Check, Delete, Plus, Search } from '@element-plus/icons-vue'
import { getProjects } from '@/api/log'
import { createScenario, deleteScenario, getScenarios, updateScenario } from '@/api/scenarios'

const ALL_PROJECTS = '__all__'
const scenarios = ref([])
const projects = ref([])
const selectedId = ref('')
const editor = ref(null)
const search = ref('')
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const creating = ref(false)
let hydrating = false

const keywordLines = value => [...new Set(String(value || '').split('\n').map(item => item.trim()).filter(Boolean))]
const currentKeywords = computed(() => keywordLines(editor.value?.keywordsText))
const totalKeywords = computed(() => scenarios.value.reduce((sum, item) => sum + item.keywords.length, 0))
const filteredScenarios = computed(() => {
  const text = search.value.trim().toLowerCase()
  if (!text) return scenarios.value
  return scenarios.value.filter(item => `${item.name} ${item.description} ${item.keywords.join(' ')} ${item.projects.join(' ')}`.toLowerCase().includes(text))
})

function fromAPI(item) {
  const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
  const checks = Array.isArray(item.checks) ? item.checks : []
  const keywords = [...new Set(checks
    .filter(check => check.enabled !== false)
    .flatMap(check => Array.isArray(check.keywords) ? check.keywords : [])
    .map(keyword => String(keyword).trim())
    .filter(Boolean))]
  const selectedProjects = metadata.project_scope === 'selected' && Array.isArray(metadata.projects) && metadata.projects.length
    ? metadata.projects
    : [ALL_PROJECTS]

  return {
    id: item.id,
    name: item.name || '',
    description: item.description || '',
    updated_at: item.updated_at,
    keywords,
    keywordsText: keywords.join('\n'),
    projects: selectedProjects
  }
}

function toAPI(item) {
  const keywords = keywordLines(item.keywordsText)
  const allProjects = item.projects.includes(ALL_PROJECTS)
  const name = item.name.trim()
  return {
    id: item.id,
    name,
    description: item.description.trim(),
    color: 'blue',
    judgement: 'any-error',
    metadata: {
      status: 'published',
      project_scope: allProjects ? 'all' : 'selected',
      projects: allProjects ? [] : item.projects,
      tags: []
    },
    checks: [{
      id: `keywords-${item.id}`,
      name: `${name}关键词筛查`,
      description: item.description.trim(),
      severity: 'warning',
      enabled: true,
      source: 'custom',
      match_type: 'forbidden',
      min_count: 1,
      time_window: 0,
      keywords
    }]
  }
}

async function load(preferredId = selectedId.value) {
  loading.value = true
  try {
    scenarios.value = (await getScenarios()).map(fromAPI)
    selectedId.value = scenarios.value.some(item => item.id === preferredId) ? preferredId : (scenarios.value[0]?.id || '')
    await hydrateEditor()
  } finally {
    loading.value = false
  }
}

async function hydrateEditor() {
  hydrating = true
  const source = scenarios.value.find(item => item.id === selectedId.value)
  editor.value = source ? structuredClone(source) : null
  creating.value = false
  dirty.value = false
  await nextTick()
  hydrating = false
}

async function confirmDiscard() {
  if (!dirty.value) return true
  try {
    await ElMessageBox.confirm('当前修改尚未保存，离开后将丢失。', '放弃修改', {
      confirmButtonText: '放弃修改',
      cancelButtonText: '继续编辑',
      type: 'warning'
    })
    return true
  } catch {
    return false
  }
}

async function selectScene(id) {
  if (!creating.value && id === selectedId.value) return
  if (!await confirmDiscard()) return
  selectedId.value = id
  await hydrateEditor()
}

async function startCreate() {
  if (!await confirmDiscard()) return
  hydrating = true
  creating.value = true
  selectedId.value = ''
  editor.value = {
    id: crypto.randomUUID(),
    name: '',
    description: '',
    keywordsText: '',
    projects: [ALL_PROJECTS],
    updated_at: null
  }
  dirty.value = false
  await nextTick()
  hydrating = false
}

async function cancelCreate() {
  if (!await confirmDiscard()) return
  selectedId.value = scenarios.value[0]?.id || ''
  await hydrateEditor()
}

function normalizeProjects(values) {
  if (!values.includes(ALL_PROJECTS) || values.length === 1) return
  editor.value.projects = values.at(-1) === ALL_PROJECTS
    ? [ALL_PROJECTS]
    : values.filter(value => value !== ALL_PROJECTS)
}

function validate() {
  if (!editor.value.name.trim()) return '请填写测试场景名称'
  if (!currentKeywords.value.length) return '请至少填写一个筛查关键词'
  if (!editor.value.projects.length) return '请至少选择一个项目'
  return ''
}

async function save() {
  const message = validate()
  if (message) {
    ElMessage.warning(message)
    return
  }
  saving.value = true
  try {
    const payload = toAPI(editor.value)
    if (creating.value) await createScenario(payload)
    else await updateScenario(editor.value.id, payload)
    ElMessage.success(creating.value ? '场景已创建' : '场景已保存')
    await load(editor.value.id)
  } finally {
    saving.value = false
  }
}

async function removeCurrent() {
  try {
    await ElMessageBox.confirm(`确认删除“${editor.value.name}”吗？`, '删除场景', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await deleteScenario(editor.value.id)
  ElMessage.success('场景已删除')
  await load('')
}

function projectSummary(values) {
  return values.includes(ALL_PROJECTS) ? '全部项目' : `${values.length} 个项目`
}

function formatDate(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

watch(editor, () => {
  if (!hydrating && editor.value) dirty.value = true
}, { deep: true })

onMounted(async () => {
  const [scenarioResult, projectResult] = await Promise.allSettled([load(), getProjects()])
  if (scenarioResult.status === 'rejected') ElMessage.error('测试场景加载失败')
  if (projectResult.status === 'fulfilled') projects.value = projectResult.value || []
  else ElMessage.error('项目列表加载失败')
})
</script>

<style scoped>
.scenario-page { min-height: 100%; padding: 24px; background: #f4f6f9; color: #172033; }
.page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.page-header h1 { margin: 4px 0 4px; font-size: 26px; line-height: 1.3; letter-spacing: 0; }
.page-header p { margin: 0; color: #748096; font-size: 14px; }
.eyebrow { color: #2878d0; font-size: 12px; font-weight: 700; }
.workspace { display: grid; grid-template-columns: 290px minmax(0, 1fr); min-height: calc(100vh - 176px); overflow: hidden; border: 1px solid #dfe4ec; border-radius: 8px; background: #fff; }
.scene-library { display: flex; min-width: 0; flex-direction: column; padding: 18px 14px; border-right: 1px solid #e6eaf0; background: #fafbfd; }
.library-title { display: flex; align-items: center; justify-content: space-between; margin: 0 4px 14px; }
.library-title strong { font-size: 16px; }
.library-title span { min-width: 26px; height: 26px; padding: 0 8px; border-radius: 13px; background: #e9eef5; color: #5d687b; font-size: 12px; line-height: 26px; text-align: center; }
.scene-list { display: flex; min-height: 180px; margin-top: 14px; flex: 1; flex-direction: column; gap: 6px; overflow-y: auto; }
.scene-item { display: flex; width: 100%; min-height: 68px; align-items: center; justify-content: space-between; gap: 10px; padding: 12px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.scene-item:hover { background: #f1f5fa; }
.scene-item.active { border-color: #b8d6fa; background: #eaf3ff; }
.scene-item-main { display: flex; min-width: 0; flex-direction: column; gap: 7px; }
.scene-item-main strong { overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.scene-item-main small { overflow: hidden; color: #7b8799; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.scene-arrow { width: 16px; flex: 0 0 16px; color: #a2adbd; }
.scene-editor { min-width: 0; padding: 28px 32px 40px; }
.editor-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; padding-bottom: 22px; border-bottom: 1px solid #edf0f4; }
.editor-header h2 { margin: 5px 0; font-size: 20px; letter-spacing: 0; }
.editor-header p { margin: 0; color: #8792a4; font-size: 13px; }
.editor-kicker { color: #2878d0; font-size: 12px; font-weight: 700; }
.editor-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.scene-form { width: min(760px, 100%); padding-top: 26px; }
.scene-form :deep(.el-form-item) { margin-bottom: 26px; }
.scene-form :deep(.el-form-item__label) { padding-bottom: 9px; color: #344054; font-weight: 600; }
.full-width { width: 100%; }
.field-meta { display: flex; width: 100%; justify-content: space-between; margin-top: 7px; color: #8a95a7; font-size: 12px; }
.field-meta strong { color: #2878d0; }
.empty-editor { display: grid; place-items: center; }
@media (max-width: 900px) {
  .scenario-page { padding: 16px; }
  .workspace { grid-template-columns: 1fr; }
  .scene-library { max-height: 330px; border-right: 0; border-bottom: 1px solid #e6eaf0; }
  .scene-editor { padding: 22px 18px 30px; }
  .editor-header { align-items: stretch; flex-direction: column; }
  .editor-actions { justify-content: flex-start; }
}
@media (max-width: 560px) {
  .page-header { align-items: stretch; flex-direction: column; }
  .page-header .el-button { width: 100%; }
  .editor-actions { display: grid; grid-template-columns: 1fr 1fr; }
  .editor-actions .el-button { width: 100%; margin: 0; }
}
</style>
