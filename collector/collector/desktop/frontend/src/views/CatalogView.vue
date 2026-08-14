<script setup>
import { ref } from 'vue'
import { Button, Empty, Modal, Tag } from 'ant-design-vue'
import { ImportOutlined } from '@ant-design/icons-vue'

const props = defineProps({ catalog: { type: Object, default: () => ({ projects: [] }) }, invoke: Function })
const preview = ref(null)
const applying = ref(false)
const kindText = { project: '项目', task: '任务', profile: '关键字方案', group: '关键字类别', rule: '关键字' }
const kindLabel = (kind) => kindText[kind] || kind
const changeLabel = (change) => ({ added: '新增', modified: '修改', deleted: '删除' })[change.kind] || change.kind

async function chooseFile() { try { preview.value = await props.invoke('SelectCatalogFile') } catch (_) {} }
async function applyImport() {
  if (!preview.value?.token) return
  try { applying.value = true; await props.invoke('ApplyCatalogImport', preview.value.token); preview.value = null } catch (_) {} finally { applying.value = false }
}
</script>
<template>
  <main class="page-view">
    <div class="page-heading"><div><h1>项目配置</h1><p>内置配置随程序更新；可导入本地 YAML 追加或覆盖关键字方案（无需云端同步）。</p></div><Button type="primary" @click="chooseFile"><template #icon><ImportOutlined /></template>导入本地配置</Button></div>
    <section class="catalog-tree catalog-tree-readonly">
      <div class="section-heading"><h2>当前配置</h2><Tag>结构版本 {{ catalog.schemaVersion }}</Tag></div>
      <article v-for="project in catalog.projects" :key="project.id" class="catalog-project"><strong>{{ project.name }}</strong><small>{{ project.versions?.join(' / ') || '无预设版本' }}</small><div v-for="task in project.tasks" :key="task.id" class="catalog-task"><span>{{ task.name }}</span><Tag>{{ task.keywordProfiles?.length || 0 }} 个关键字方案</Tag></div></article>
      <Empty v-if="!catalog.projects?.length" description="程序未包含项目配置" />
    </section>

    <Modal v-model:open="preview" :title="`配置预览 · ${preview?.fileName || ''}`" ok-text="应用" cancel-text="取消" :confirm-loading="applying" @ok="applyImport">
      <div class="diff-summary" v-if="preview">
        <span>新增 {{ preview.added || 0 }}</span><span>修改 {{ preview.modified || 0 }}</span><span>删除 {{ preview.deleted || 0 }}</span><span>未变 {{ preview.unchanged || 0 }}</span>
      </div>
      <p v-for="warning in preview?.warnings || []" :key="warning" class="warning-note">{{ warning }}</p>
      <div class="change-list">
        <article v-for="change in preview?.changes || []" :key="change.entity + change.id"><Tag>{{ changeLabel(change) }}</Tag><span>{{ kindLabel(change.entity) }} · {{ change.name || change.id }}</span><small v-if="change.impact" class="upload-error">{{ change.impact }}</small></article>
        <Empty v-if="!(preview?.changes?.length)" description="没有可导入的变化" />
      </div>
    </Modal>
  </main>
</template>
