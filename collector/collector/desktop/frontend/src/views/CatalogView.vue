<script setup>
import { computed, onMounted, ref } from 'vue'
import { Button, Empty, Modal, Spin, TabPane, Tabs, Tag, Tooltip } from 'ant-design-vue'
import { CloudSyncOutlined, FolderOpenOutlined, ImportOutlined, ReloadOutlined } from '@ant-design/icons-vue'

const props = defineProps({ catalog: { type: Object, default: () => ({ projects: [] }) }, invoke: Function })
const preview = ref(null)
const files = ref(null)
const applying = ref(false)
const syncing = ref(false)
const loading = ref(false)
const syncMessage = ref('')
const kindText = { project: '项目', task: '任务', profile: '关键字方案', group: '关键字类别', rule: '关键字' }
const kindLabel = (kind) => kindText[kind] || kind
const changeLabel = (change) => ({ added: '新增', modified: '修改', deleted: '删除' })[change.kind] || change.kind
const projects = computed(() => props.catalog?.projects || [])
const tasks = computed(() => uniqueById(projects.value.flatMap((item) => item.tasks || [])))
const profiles = computed(() => uniqueById(tasks.value.flatMap((item) => item.keywordProfiles || [])))
const cloudProfile = computed(() => profiles.value.find((item) => item.id === 'cloud-standard-keywords'))
function uniqueById(items) { const seen = new Map(); for (const item of items) if (!seen.has(item.id)) seen.set(item.id, item); return [...seen.values()] }
function fileFor(name) { return files.value?.files?.find((item) => item.name === name) }
function modifiedAt(name) { const value = fileFor(name)?.modifiedAt; return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '未生成' }
async function refreshFiles() { files.value = await props.invoke('GetCatalogFiles') }
async function reloadCatalog() { try { loading.value = true; await props.invoke('ReloadCatalogFiles'); await refreshFiles(); syncMessage.value = '本地配置已重新加载' } catch (error) { syncMessage.value = `重新加载失败：${error.message || error}` } finally { loading.value = false } }
async function openDirectory() { try { await props.invoke('OpenCatalogDirectory') } catch (_) {} }
async function chooseFile() { try { preview.value = await props.invoke('SelectCatalogFile') } catch (_) {} }
async function applyImport() { if (!preview.value?.token) return; try { applying.value = true; await props.invoke('ApplyCatalogImport', preview.value.token); preview.value = null; await refreshFiles() } catch (_) {} finally { applying.value = false } }
async function syncCloud() { try { syncing.value = true; const result = await props.invoke('SyncCloudKeywords'); syncMessage.value = result?.message || '云端关键字同步完成' } catch (error) { syncMessage.value = `同步失败，已保留本地缓存：${error.message || error}` } finally { syncing.value = false } }
onMounted(async () => { try { await refreshFiles() } catch (_) {} })
</script>

<template>
  <main class="page-view">
    <div class="page-heading catalog-page-heading"><h1>配置管理</h1><div class="catalog-actions"><Tooltip title="打开配置目录"><Button aria-label="打开配置目录" shape="circle" @click="openDirectory"><template #icon><FolderOpenOutlined /></template></Button></Tooltip><Tooltip title="重新加载本地配置"><Button aria-label="重新加载本地配置" shape="circle" :loading="loading" @click="reloadCatalog"><template #icon><ReloadOutlined /></template></Button></Tooltip><Tooltip title="同步云端关键字"><Button aria-label="同步云端关键字" shape="circle" :loading="syncing" @click="syncCloud"><template #icon><CloudSyncOutlined /></template></Button></Tooltip><Tooltip title="导入旧配置"><Button aria-label="导入旧配置" shape="circle" type="primary" @click="chooseFile"><template #icon><ImportOutlined /></template></Button></Tooltip></div></div>
    <p v-if="syncMessage" class="warning-note">{{ syncMessage }}</p>
    <section class="catalog-file-bar"><div><strong>本地配置目录</strong><span>{{ files?.directory || '正在读取…' }}</span></div><Tag v-for="name in ['project-config.yaml', 'task-config.yaml', 'keyword-config.yaml']" :key="name" color="green">{{ name }} · {{ modifiedAt(name) }}</Tag><Tag :color="cloudProfile ? 'blue' : 'default'">云端缓存 · {{ cloudProfile ? `${cloudProfile.groups?.length || 0} 个分组` : '尚未同步' }}</Tag></section>
    <Tabs class="catalog-tabs" size="large">
      <TabPane key="projects" tab="项目配置"><section class="catalog-tree catalog-tree-readonly"><div class="section-heading"><h2>项目（{{ projects.length }}）</h2><Tag>project-config.yaml</Tag></div><div class="catalog-item-grid"><article v-for="project in projects" :key="project.id" class="catalog-project catalog-name-only"><strong>{{ project.name }}</strong></article></div><Empty v-if="!projects.length" description="没有项目配置" /></section></TabPane>
      <TabPane key="tasks" tab="任务配置"><section class="catalog-tree catalog-tree-readonly"><div class="section-heading"><h2>任务（{{ tasks.length }}）</h2><Tag>task-config.yaml</Tag></div><div class="catalog-item-grid"><article v-for="task in tasks" :key="task.id" class="catalog-project catalog-name-only"><strong>{{ task.name }}</strong></article></div><Empty v-if="!tasks.length" description="没有任务配置" /></section></TabPane>
      <TabPane key="keywords" tab="关键字配置"><section class="catalog-tree catalog-tree-readonly"><div class="section-heading"><h2>关键字配置（{{ profiles.length }}）</h2><Tag color="blue">云端配置只读</Tag></div><article v-for="profile in profiles" :key="profile.id" class="catalog-project" :class="{ 'cloud-catalog-profile': profile.id === 'cloud-standard-keywords' }"><strong>{{ profile.name }}</strong><small>{{ profile.groups?.length || 0 }} 个分类 · {{ profile.groups?.flatMap((group) => group.rules || []).length || profile.rules?.length || 0 }} 个关键字</small><div class="catalog-task-list"><Tag v-for="group in profile.groups || []" :key="group.id" :color="profile.id === 'cloud-standard-keywords' ? 'blue' : undefined">{{ group.name }}</Tag></div></article><Empty v-if="!profiles.length" description="没有关键字配置" /></section></TabPane>
    </Tabs>
    <Modal v-model:open="preview" :title="`旧配置预览 · ${preview?.fileName || ''}`" ok-text="应用" cancel-text="取消" :confirm-loading="applying" @ok="applyImport"><Spin v-if="applying" /><div class="diff-summary" v-if="preview"><span>新增 {{ preview.added || 0 }}</span><span>修改 {{ preview.modified || 0 }}</span><span>删除 {{ preview.deleted || 0 }}</span><span>未变 {{ preview.unchanged || 0 }}</span></div><p v-for="warning in preview?.warnings || []" :key="warning" class="warning-note">{{ warning }}</p><div class="change-list"><article v-for="change in preview?.changes || []" :key="change.entity + change.id"><Tag>{{ changeLabel(change) }}</Tag><span>{{ kindLabel(change.entity) }} · {{ change.name || change.id }}</span><small v-if="change.impact" class="upload-error">{{ change.impact }}</small></article><Empty v-if="!(preview?.changes?.length)" description="没有可导入的变化" /></div></Modal>
  </main>
</template>
