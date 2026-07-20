<template>
  <div class="page" v-loading="loading">
    <header><div class="title"><el-button text circle :icon="ArrowLeft" @click="router.push('/tasks')" /><div><h1>{{ task.original_name || task.project_name || '任务详情' }}</h1><p>{{ task.task_id }}</p></div></div><el-button type="primary" :icon="DataAnalysis" :disabled="!task.task_id" @click="router.push(`/analysis/${task.task_id}`)">解析结果</el-button></header>
    <div v-if="task.task_id" class="overview">
      <section class="panel meta"><div class="status"><el-tag :type="statusMeta.type" effect="plain">{{ statusMeta.label }}</el-tag><span>{{ formatDate(task.updated_at) }}</span></div><dl><div><dt>项目</dt><dd>{{ task.project_name }}</dd></div><div><dt>版本</dt><dd>{{ task.version || '-' }}</dd></div><div><dt>文件数量</dt><dd>{{ task.file_count }}</dd></div><div><dt>原始大小</dt><dd>{{ formatSize(task.original_size) }}</dd></div><div><dt>日志行数</dt><dd>{{ task.total_lines.toLocaleString() }}</dd></div><div><dt>创建时间</dt><dd>{{ formatDate(task.created_at) }}</dd></div></dl></section>
      <section class="panel counts"><h2>解析统计</h2><div><span><strong class="error">{{ task.error_count }}</strong><small>错误</small></span><span><strong class="warning">{{ task.warning_count }}</strong><small>警告</small></span><span><strong>{{ results.length }}</strong><small>结果记录</small></span></div><el-alert v-if="task.error_message" :title="task.error_message" type="error" :closable="false" /></section>
    </div>
    <div v-if="task.task_id" class="content">
      <section class="panel"><div class="panel-heading"><div><h2>日志文件</h2><p>数据库记录的实际解压文件</p></div><span>{{ files.length }} 项</span></div><el-table :data="files"><el-table-column prop="relative_path" label="文件路径" min-width="260" /><el-table-column label="大小" width="110"><template #default="scope">{{ formatSize(scope.row.size_bytes) }}</template></el-table-column><el-table-column label="行数" width="110"><template #default="scope">{{ scope.row.line_count.toLocaleString() }}</template></el-table-column><template #empty><el-empty description="暂无文件记录" :image-size="70" /></template></el-table></section>
      <section class="panel"><div class="panel-heading"><div><h2>命中摘要</h2><p>当前接口返回的真实解析结果</p></div><span>{{ results.length }} 条</span></div><el-table :data="resultSummary"><el-table-column prop="matched_text" label="关键字" /><el-table-column prop="level" label="级别" width="90" /><el-table-column prop="count" label="次数" width="80" /><template #empty><el-empty description="暂无命中结果" :image-size="70" /></template></el-table></section>
    </div>
    <el-empty v-if="!loading && !task.task_id" description="任务不存在" />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, DataAnalysis } from '@element-plus/icons-vue'
import { getTaskDetail, getTaskResults } from '@/api/task'

const route=useRoute();const router=useRouter();const loading=ref(false);const task=ref({});const files=ref([]);const results=ref([])
const statusMeta=computed(()=>({uploading:{label:'上传中',type:'info'},queued:{label:'排队中',type:'info'},parsing:{label:'解析中',type:'primary'},completed:{label:'已完成',type:'success'},failed:{label:'失败',type:'danger'}})[task.value.status]||{label:'未知',type:'info'})
const resultSummary=computed(()=>{const map=new Map();for(const item of results.value){const key=`${item.level}:${item.matched_text}`;const current=map.get(key)||{level:item.level,matched_text:item.matched_text,count:0};current.count++;map.set(key,current)}return[...map.values()]})
const formatSize=(bytes)=>{if(!bytes)return'0 B';const units=['B','KB','MB','GB'];const i=Math.min(Math.floor(Math.log(bytes)/Math.log(1024)),3);return`${(bytes/1024**i).toFixed(i?1:0)} ${units[i]}`}
const formatDate=(value)=>value?new Date(value).toLocaleString('zh-CN',{hour12:false}):'-'
async function load(){loading.value=true;try{const [detail,parsed]=await Promise.all([getTaskDetail(route.params.taskId),getTaskResults(route.params.taskId,{page:1,page_size:200})]);task.value=detail.task;files.value=detail.files;results.value=parsed}finally{loading.value=false}}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header,.title,.status,.panel-heading{display:flex;align-items:center}.page>header{justify-content:space-between;margin-bottom:18px}.title{gap:8px}.title h1{margin:0;font-size:21px}.title p{margin:4px 0 0;color:#8a94a3;font:11px Consolas,monospace}.overview{display:grid;grid-template-columns:1.4fr .6fr;gap:16px;margin-bottom:16px}.panel{padding:18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.status{justify-content:space-between;padding-bottom:14px;border-bottom:1px solid #edf0f3;color:#8a94a3;font-size:11px}.meta dl{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin:16px 0 0}.meta dl div{display:flex;min-width:0;flex-direction:column;gap:5px}.meta dt{color:#8a94a3;font-size:11px}.meta dd{overflow:hidden;margin:0;font-size:13px;text-overflow:ellipsis;white-space:nowrap}.counts h2,.panel-heading h2{margin:0;font-size:15px}.counts>div{display:grid;grid-template-columns:repeat(3,1fr);margin:22px 0}.counts span{display:flex;align-items:center;flex-direction:column;gap:5px}.counts strong{font-size:23px}.counts small{color:#8a94a3}.counts .error{color:#d95858}.counts .warning{color:#c9861b}.content{display:grid;grid-template-columns:1fr 1fr;gap:16px}.panel-heading{justify-content:space-between;margin-bottom:10px}.panel-heading p{margin:4px 0 0;color:#8a94a3;font-size:11px}.panel-heading>span{color:#8a94a3;font-size:12px}@media(max-width:900px){.overview,.content{grid-template-columns:1fr}.meta dl{grid-template-columns:repeat(2,1fr)}}
</style>
