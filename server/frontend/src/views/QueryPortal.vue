<template>
  <main class="page query-page">
    <section class="portal-shell" aria-labelledby="portal-title">
      <header class="portal-header">
        <RouterLink to="/" class="brand" aria-label="返回 LogMaster 工作台">
          <span class="brand-mark"><el-icon><DataAnalysis /></el-icon></span>
          <span><strong>LogMaster</strong><small>日志分析服务</small></span>
        </RouterLink>
        <span class="service-pill"><i></i>服务状态可用</span>
      </header>

      <header class="page-heading">
        <div class="hero-copy">
          <h1 id="portal-title">采集日志查询</h1>
          <p>输入客户端上传后获得的查询码，查看服务端归档和分析进度。</p>
        </div>
        <form class="query-form" @submit.prevent="handleQuery">
          <label for="query-code">查询码</label>
          <div class="query-control">
            <el-input id="query-code" v-model="queryCode" size="large" maxlength="32" placeholder="例如 A1B2C3D4E5" :disabled="loading" @input="normalizeCode">
              <template #prefix><el-icon><Key /></el-icon></template>
            </el-input>
            <el-button type="primary" size="large" native-type="submit" :loading="loading">
              <el-icon v-if="!loading"><Search /></el-icon>查询
            </el-button>
          </div>
          <p v-if="inputError" class="form-error"><el-icon><WarningFilled /></el-icon>{{ inputError }}</p>
        </form>
      </header>

      <section v-if="loading" class="panel result-panel loading-panel" aria-live="polite">
        <el-skeleton animated>
          <template #template>
            <el-skeleton-item variant="rect" class="skeleton-title" />
            <el-skeleton-item variant="rect" class="skeleton-row" />
            <el-skeleton-item variant="rect" class="skeleton-row" />
          </template>
        </el-skeleton>
      </section>

      <section v-else-if="result" class="panel result-panel" aria-live="polite">
        <div class="result-heading">
          <div>
            <p class="panel-label">查询结果</p>
            <h2>查询码 <code>{{ result.query_code }}</code></h2>
          </div>
          <div class="result-tags"><el-tag :type="statusType" effect="dark" round>{{ statusText }}</el-tag><el-tag v-if="linked" type="success" effect="plain">已加入记录列表</el-tag></div>
        </div>

        <div class="progress-section">
          <div class="progress-label"><span>日志处理进度</span><strong>{{ progressText }}</strong></div>
          <el-progress :percentage="progressPercent" :stroke-width="8" :show-text="false" :status="progressStatus" />
          <p>{{ result.processed_files || 0 }} / {{ result.total_files || 0 }} 个文件已完成处理</p>
        </div>

        <div class="summary-grid stats-grid">
          <article class="summary-item"><span class="stat-icon files"><el-icon><Document /></el-icon></span><div><small>归档日志</small><strong>{{ result.total_files || 0 }}</strong><em>个文件</em></div></article>
          <article class="summary-item"><span class="stat-icon lines"><el-icon><Tickets /></el-icon></span><div><small>已分析行数</small><strong>{{ formatNumber(result.total_lines) }}</strong><em>行</em></div></article>
          <article class="summary-item"><span class="stat-icon errors"><el-icon><CircleCloseFilled /></el-icon></span><div><small>异常命中</small><strong>{{ formatNumber(result.error_count) }}</strong><em>条</em></div></article>
          <article class="summary-item"><span class="stat-icon warnings"><el-icon><WarningFilled /></el-icon></span><div><small>告警命中</small><strong>{{ formatNumber(result.warning_count) }}</strong><em>条</em></div></article>
        </div>

        <dl class="metadata-grid">
          <div><dt>所属项目</dt><dd>{{ result.project_name || '未命名项目' }}</dd></div>
          <div><dt>版本</dt><dd>{{ result.version || '-' }}</dd></div>
          <div><dt>上传批次</dt><dd>{{ result.batch_count || 0 }}</dd></div>
          <div><dt>上传人</dt><dd>{{ result.uploader_name || '-' }}</dd></div>
          <div><dt>测试任务</dt><dd>{{ result.test_task_name || '-' }}</dd></div>
          <div><dt>最近更新</dt><dd>{{ formatDate(result.updated_at) }}</dd></div>
        </dl>
        <div v-if="result.batches?.length" class="batch-list">
          <div v-for="batch in result.batches" :key="batch.upload_id" class="batch-row">
            <code>{{ batch.upload_id }}</code><span>{{ batch.status }}</span><span>{{ batch.processed_files || 0 }} / {{ batch.total_files || 0 }} 文件</span>
          </div>
        </div>
      </section>

      <section v-else class="panel empty-panel">
        <span class="empty-icon"><el-icon><Search /></el-icon></span>
        <h2>等待查询</h2>
        <p>客户端上传完成后会返回唯一查询码。</p>
      </section>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCloseFilled, Document, Key, Search, Tickets, WarningFilled } from '@element-plus/icons-vue'
import { collectQuerySession, getQueryStatus } from '@/api/log'

const queryCode = ref('')
const result = ref(null)
const loading = ref(false)
const inputError = ref('')
const linked = ref(false)
const queryStateKey = 'logmaster.collector-query-state.v1'

const progressPercent = computed(() => {
  const total = Number(result.value?.total_files || 0)
  const processed = Number(result.value?.processed_files || 0)
  if (!total) return result.value?.status === 'completed' ? 100 : 0
  return Math.min(100, Math.round((processed / total) * 100))
})
const progressText = computed(() => `${progressPercent.value}%`)
const statusText = computed(() => ({ queued: '等待处理', parsing: '正在分析', completed: '分析完成', failed: '处理失败', uploading: '正在接收' }[result.value?.status] || '处理中'))
const statusType = computed(() => ({ completed: 'success', failed: 'danger', parsing: 'primary', queued: 'warning', uploading: 'info' }[result.value?.status] || 'info'))
const progressStatus = computed(() => result.value?.status === 'failed' ? 'exception' : result.value?.status === 'completed' ? 'success' : '')

function normalizeCode() {
  queryCode.value = queryCode.value.toUpperCase().replace(/[^A-Z0-9-]/g, '')
  inputError.value = ''
}

async function handleQuery() {
  const code = queryCode.value.trim()
  if (!code) {
    inputError.value = '请输入客户端返回的查询码'
    return
  }
  loading.value = true
  result.value = null
  linked.value = false
  inputError.value = ''
  persistQueryState()
  try {
    result.value = await getQueryStatus(code)
    persistQueryState()
    await collectQuerySession(code)
    linked.value = true
    persistQueryState()
    ElMessage.success('采集日志已加入日志记录')
  } catch (error) {
    if (result.value) ElMessage.warning('查询成功，但加入日志记录失败，请重试')
    else inputError.value = error.response?.status === 404 ? '未找到对应的查询码，请确认后重试' : '查询暂时不可用，请稍后重试'
  } finally {
    loading.value = false
  }
}

function persistQueryState() {
  try {
    sessionStorage.setItem(queryStateKey, JSON.stringify({ queryCode: queryCode.value, result: result.value, linked: linked.value }))
  } catch { /* Storage can be unavailable in private contexts. */ }
}

onMounted(async () => {
  try {
    const stored = JSON.parse(sessionStorage.getItem(queryStateKey) || 'null')
    if (!stored?.queryCode) return
    queryCode.value = stored.queryCode
    if (!stored.result) return
    result.value = stored.result
    linked.value = Boolean(stored.linked)
    try {
      result.value = await getQueryStatus(queryCode.value)
      persistQueryState()
    } catch { /* Keep the last successful state visible when refresh is temporarily unavailable. */ }
  } catch { /* Ignore malformed or unavailable session storage. */ }
})

function formatNumber(value) {
  return new Intl.NumberFormat('zh-CN').format(Number(value || 0))
}

function formatDate(value) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}
</script>

<style scoped>
.query-portal{min-height:100dvh;padding:24px;background:#0b0f14;color:#e8edf3;font-family:Inter,"Microsoft YaHei",sans-serif}.portal-shell{width:min(100%,1040px);margin:0 auto}.portal-header{display:flex;align-items:center;justify-content:space-between;padding:4px 0 28px}.brand{display:flex;align-items:center;gap:11px;color:inherit;text-decoration:none}.brand-mark{display:grid;width:36px;height:36px;place-items:center;border:1px solid rgba(71,211,232,.36);border-radius:7px;background:rgba(6,182,212,.14);color:#67e8f9;font-size:19px}.brand strong,.brand small{display:block}.brand strong{font-size:16px;letter-spacing:.02em}.brand small{margin-top:3px;color:#8492a3;font-size:11px}.service-pill{display:flex;align-items:center;gap:7px;color:#9bacba;font-size:12px;white-space:nowrap}.service-pill i{width:7px;height:7px;border-radius:50%;background:#34d399;box-shadow:0 0 12px rgba(52,211,153,.8)}.query-hero{display:grid;grid-template-columns:minmax(0,1fr) minmax(340px,.78fr);gap:44px;align-items:end;padding:46px 48px;border:1px solid rgba(255,255,255,.09);border-radius:8px;background:linear-gradient(135deg,rgba(20,31,41,.96),rgba(13,19,26,.96));box-shadow:0 20px 55px rgba(0,0,0,.28)}.eyebrow,.panel-label{margin:0 0 10px;color:#65dcea;font:600 11px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.13em}.hero-copy h1{max-width:520px;margin:0;color:#f8fbff;font-size:31px;line-height:1.22;font-weight:680}.hero-copy>p:last-child{max-width:500px;margin:15px 0 0;color:#aab7c5;font-size:14px;line-height:1.75}.query-form label{display:block;margin-bottom:9px;color:#b7c3cf;font-size:12px;font-weight:600}.query-control{display:flex;gap:9px;min-width:0}.query-control .el-input{min-width:0;flex:1 1 auto}.query-control :deep(.el-input__wrapper){height:44px;background:rgba(1,6,10,.38);box-shadow:0 0 0 1px rgba(255,255,255,.14) inset}.query-control :deep(.el-input__wrapper.is-focus){box-shadow:0 0 0 1px #22d3ee inset,0 0 0 4px rgba(34,211,238,.1)}.query-control :deep(input){color:#eef6f8;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.08em}.query-control :deep(.el-input__prefix){color:#67e8f9}.query-control .el-button{height:44px;min-width:92px;flex:0 0 92px;border:0;border-radius:6px;background:#0891b2;font-weight:600}.query-control .el-button:hover{background:#0ea5c7}.form-error{display:flex;align-items:center;gap:6px;margin:9px 0 0;color:#fb7185;font-size:12px}.result-panel,.empty-panel{margin-top:18px;border:1px solid rgba(255,255,255,.09);border-radius:8px;background:rgba(19,27,36,.78);box-shadow:0 18px 44px rgba(0,0,0,.2)}.result-panel{padding:30px}.result-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20px}.result-heading h2{margin:0;color:#f4f8fb;font-size:18px}.result-heading code{color:#77e6ef;font:600 16px ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.07em}.result-heading :deep(.el-tag){border:0;border-radius:4px}.progress-section{margin:28px 0 25px;padding:18px 20px;border:1px solid rgba(255,255,255,.075);border-radius:6px;background:rgba(0,0,0,.17)}.progress-label{display:flex;justify-content:space-between;margin-bottom:11px;color:#c5d0da;font-size:13px}.progress-label strong{color:#6ee7f2;font:600 14px ui-monospace,SFMono-Regular,Menlo,monospace}.progress-section :deep(.el-progress-bar__outer){background:#26323d}.progress-section p{margin:9px 0 0;color:#8291a1;font-size:12px}.stats-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.stats-grid article{display:flex;align-items:center;gap:12px;min-height:82px;padding:14px;border:1px solid rgba(255,255,255,.075);border-radius:6px;background:rgba(255,255,255,.025)}.stat-icon{display:grid;width:34px;height:34px;flex:0 0 34px;place-items:center;border-radius:6px;font-size:17px}.files{background:rgba(56,189,248,.14);color:#7dd3fc}.lines{background:rgba(129,140,248,.14);color:#a5b4fc}.errors{background:rgba(251,113,133,.14);color:#fda4af}.warnings{background:rgba(251,191,36,.14);color:#fcd34d}.stats-grid small,.stats-grid em{display:block;color:#8392a2;font-size:11px;font-style:normal}.stats-grid strong{display:inline-block;margin:4px 4px 0 0;color:#edf5f8;font-size:20px;line-height:1}.metadata-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:18px 0 0}.metadata-grid div{min-width:0}.metadata-grid dt{margin-bottom:6px;color:#718192;font-size:10px;letter-spacing:.08em}.metadata-grid dd{overflow:hidden;margin:0;color:#cbd6df;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.metadata-grid code{color:#91a6b8;font-size:11px}.empty-panel{display:grid;min-height:245px;place-items:center;align-content:center;padding:30px;text-align:center}.empty-icon{display:grid;width:43px;height:43px;place-items:center;border-radius:50%;background:rgba(6,182,212,.11);color:#67e8f9;font-size:20px}.empty-panel h2{margin:14px 0 5px;font-size:17px}.empty-panel p{margin:0;color:#8492a3;font-size:13px}.loading-panel{padding:30px}.skeleton-title{width:26%;height:22px;margin-bottom:32px}.skeleton-row{width:100%;height:76px;margin-top:10px}@media(max-width:780px){.query-portal{padding:16px}.portal-header{padding-bottom:18px}.service-pill{font-size:11px}.query-hero{grid-template-columns:1fr;gap:27px;padding:30px 24px}.hero-copy h1{font-size:25px}.result-panel{padding:20px}.stats-grid,.metadata-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.stats-grid article{min-height:72px}.metadata-grid{row-gap:16px}}@media(max-width:440px){.portal-header{align-items:flex-start}.query-control{flex-direction:column}.query-control .el-button{width:100%;flex-basis:auto}.result-heading{align-items:flex-start;flex-direction:column;gap:10px}.stats-grid{gap:8px}.stats-grid article{gap:8px;padding:11px}.stat-icon{width:30px;height:30px;flex-basis:30px}.stats-grid strong{font-size:17px}}
</style>

<style scoped>
.query-portal{height:100%;min-height:0;overflow:auto;padding:22px 24px 34px;background:transparent}.portal-shell{width:100%;margin:0}.portal-header{display:none}.query-hero{padding:28px 30px;border-radius:14px}.hero-copy h1{font-size:25px}.result-panel,.empty-panel{margin-top:14px;margin-bottom:0;border-radius:14px}@media(max-width:780px){.query-portal{padding:12px}.query-hero{padding:24px 20px}}
</style>

<style>
html[data-log-theme="light"] .query-portal{color:#213741}
html[data-log-theme="light"] .query-portal .query-hero{border-color:rgba(51,91,107,.2);background:linear-gradient(135deg,rgba(248,253,254,.94),rgba(226,244,247,.88));box-shadow:0 16px 40px rgba(34,72,87,.12)}
html[data-log-theme="light"] .query-portal .eyebrow,html[data-log-theme="light"] .query-portal .panel-label{color:#087e96}
html[data-log-theme="light"] .query-portal .hero-copy h1,html[data-log-theme="light"] .query-portal .result-heading h2,html[data-log-theme="light"] .query-portal .empty-panel h2{color:#1d3641}
html[data-log-theme="light"] .query-portal .hero-copy>p:last-child,html[data-log-theme="light"] .query-portal .progress-section p,html[data-log-theme="light"] .query-portal .empty-panel p{color:#526f7b}
html[data-log-theme="light"] .query-portal .query-form label,html[data-log-theme="light"] .query-portal .progress-label,html[data-log-theme="light"] .query-portal .metadata-grid dd{color:#2c4a56}
html[data-log-theme="light"] .query-portal .query-control .el-input__wrapper{background:rgba(255,255,255,.82);box-shadow:0 0 0 1px rgba(51,91,107,.25) inset}
html[data-log-theme="light"] .query-portal .query-control .el-input__wrapper.is-focus{box-shadow:0 0 0 1px #0891b2 inset,0 0 0 4px rgba(8,145,178,.13)}
html[data-log-theme="light"] .query-portal .query-control input{color:#173541}
html[data-log-theme="light"] .query-portal .result-panel,html[data-log-theme="light"] .query-portal .empty-panel{border-color:rgba(51,91,107,.2);background:rgba(255,255,255,.72);box-shadow:0 14px 38px rgba(34,72,87,.1)}
html[data-log-theme="light"] .query-portal .progress-section,html[data-log-theme="light"] .query-portal .stats-grid article{border-color:rgba(51,91,107,.16);background:rgba(232,246,248,.54)}
html[data-log-theme="light"] .query-portal .progress-section .el-progress-bar__outer{background:#d6e6e9}
html[data-log-theme="light"] .query-portal .result-heading code,html[data-log-theme="light"] .query-portal .progress-label strong{color:#087e96}
html[data-log-theme="light"] .query-portal .stats-grid small,html[data-log-theme="light"] .query-portal .stats-grid em,html[data-log-theme="light"] .query-portal .metadata-grid dt{color:#5a7680}
html[data-log-theme="light"] .query-portal .stats-grid strong{color:#203d48}
html[data-log-theme="light"] .query-portal .metadata-grid code{color:#42616d}
html[data-log-theme="light"] .query-portal .empty-icon{background:rgba(8,145,178,.12);color:#087e96}
</style>

<style scoped>
.query-page{height:100%;overflow:auto}.query-page .page-heading{display:flex;grid-template-columns:none;align-items:flex-end;justify-content:space-between;gap:18px;margin-bottom:14px;padding:19px 20px;border-radius:14px}.query-page .hero-copy h1{font-size:22px;line-height:1.3}.query-page .hero-copy>p:last-child{margin-top:5px;font-size:13px;line-height:1.5}.query-page .query-form{width:min(100%,420px)}.query-page .result-panel,.query-page .empty-panel{margin:0 0 16px;padding:18px}.query-page .loading-panel{min-height:170px}.query-page .stats-grid{margin:0 0 18px}.query-page .stats-grid .summary-item{min-height:94px;padding:16px}.query-page .stats-grid article{background:transparent}.query-page .metadata-grid{margin-top:0}.query-page .empty-panel{min-height:240px}.batch-list{display:grid;gap:6px;margin-top:18px}.batch-row{display:grid;grid-template-columns:minmax(180px,1fr) 100px 130px;gap:12px;padding:9px 10px;border:1px solid rgba(127,145,160,.25);border-radius:6px;font-size:12px}.batch-row code{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}@media(max-width:780px){.query-page .page-heading{align-items:stretch;flex-direction:column}.query-page .query-form{width:100%}.query-page .result-panel,.query-page .empty-panel{padding:16px}.batch-row{grid-template-columns:1fr 80px}}@media(max-width:440px){.query-page .stats-grid .summary-item{min-height:78px;padding:12px}}
</style>

<style>
html[data-log-theme="dark"] .query-page .page-heading,
html[data-log-theme="dark"] .query-page .panel,
html[data-log-theme="dark"] .query-page .summary-item{
  border:1px solid rgba(255,255,255,.14)!important;
  border-radius:14px!important;
  background:linear-gradient(145deg,rgba(255,255,255,.085),rgba(255,255,255,.042))!important;
  box-shadow:inset 0 1px 0 rgba(255,255,255,.12),0 18px 52px rgba(0,0,0,.28)!important;
  
  
}
html[data-log-theme="dark"] .query-page .page-heading h1,
html[data-log-theme="dark"] .query-page .result-heading h2,
html[data-log-theme="dark"] .query-page .empty-panel h2,
html[data-log-theme="dark"] .query-page .stats-grid strong{color:#f5f7fa!important}
html[data-log-theme="dark"] .query-page .page-heading p,
html[data-log-theme="dark"] .query-page .stats-grid small,
html[data-log-theme="dark"] .query-page .stats-grid em,
html[data-log-theme="dark"] .query-page .metadata-grid dt{color:#b7bec8!important}
html[data-log-theme="dark"] .query-page .progress-section{border-color:rgba(255,255,255,.1);background:rgba(0,0,0,.18)}
</style>

<style scoped>
.result-tags{display:flex;align-items:center;justify-content:flex-end;gap:8px;flex-wrap:wrap}
</style>
