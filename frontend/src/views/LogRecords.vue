<template>
  <div class="page">
    <header><div><h1>日志记录</h1><p>数据库中的真实上传记录与解析统计</p></div><el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button></header>
    <div class="summary"><div><span>上传记录</span><strong>{{ records.length }}</strong></div><div><span>文件数量</span><strong>{{ totals.files }}</strong></div><div><span>日志行数</span><strong>{{ totals.lines.toLocaleString() }}</strong></div><div><span>异常数量</span><strong class="danger">{{ totals.issues }}</strong></div></div>
    <section class="panel">
      <div class="filters"><el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索文件、项目或上传 ID" /><el-select v-model="status" clearable placeholder="全部状态"><el-option label="排队中" value="queued" /><el-option label="解析中" value="parsing" /><el-option label="已完成" value="completed" /><el-option label="失败" value="failed" /></el-select></div>
      <el-table v-loading="loading" :data="paged">
        <el-table-column label="日志文件" min-width="270"><template #default="scope"><div class="file-cell"><el-icon><UploadFilled /></el-icon><div><strong>{{ scope.row.original_name || '-' }}</strong><span>{{ scope.row.id }}</span></div></div></template></el-table-column>
        <el-table-column prop="project_name" label="项目" min-width="120" />
        <el-table-column prop="version" label="版本" width="100"><template #default="scope">{{ scope.row.version || '-' }}</template></el-table-column>
        <el-table-column label="大小" width="110"><template #default="scope">{{ formatSize(scope.row.original_size) }}</template></el-table-column>
        <el-table-column prop="file_count" label="文件数" width="90" />
        <el-table-column label="日志行数" width="120"><template #default="scope">{{ scope.row.total_lines.toLocaleString() }}</template></el-table-column>
        <el-table-column label="状态" width="110"><template #default="scope"><el-tag :type="statusType(scope.row.status)" effect="plain">{{ statusLabel(scope.row.status) }}</el-tag></template></el-table-column>
        <el-table-column label="创建时间" min-width="180"><template #default="scope">{{ formatDate(scope.row.created_at) }}</template></el-table-column>
        <el-table-column label="操作" width="120" fixed="right"><template #default="scope"><el-button type="primary" link @click="router.push(`/task/${scope.row.task_id}`)">详情</el-button><el-button type="danger" link @click="remove(scope.row)">删除</el-button></template></el-table-column>
        <template #empty><el-empty description="数据库中暂无日志记录" /></template>
      </el-table>
      <footer><span>共 {{ filtered.length }} 条记录</span><el-pagination v-model:current-page="page" :page-size="pageSize" :total="filtered.length" layout="prev, pager, next" /></footer>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, UploadFilled } from '@element-plus/icons-vue'
import { getLogs } from '@/api/log'
import { deleteTask } from '@/api/task'

const router=useRouter();const records=ref([]);const loading=ref(false);const keyword=ref('');const status=ref('');const page=ref(1);const pageSize=12
const totals=computed(()=>records.value.reduce((sum,item)=>({files:sum.files+item.file_count,lines:sum.lines+item.total_lines,issues:sum.issues+item.error_count+item.warning_count}),{files:0,lines:0,issues:0}))
const filtered=computed(()=>{const text=keyword.value.trim().toLowerCase();return records.value.filter(item=>(!status.value||item.status===status.value)&&(!text||`${item.original_name}${item.project_name}${item.id}`.toLowerCase().includes(text)))})
const paged=computed(()=>filtered.value.slice((page.value-1)*pageSize,page.value*pageSize))
const formatSize=(bytes)=>{if(!bytes)return'0 B';const units=['B','KB','MB','GB'];const i=Math.min(Math.floor(Math.log(bytes)/Math.log(1024)),3);return`${(bytes/1024**i).toFixed(i?1:0)} ${units[i]}`}
const formatDate=(value)=>new Date(value).toLocaleString('zh-CN',{hour12:false})
const statusLabel=(value)=>({uploading:'上传中',queued:'排队中',parsing:'解析中',completed:'已完成',failed:'失败'})[value]||value
const statusType=(value)=>({uploading:'info',queued:'info',parsing:'primary',completed:'success',failed:'danger'})[value]||'info'
async function load(){loading.value=true;try{const data=await getLogs({page:1,page_size:200});records.value=data.list}finally{loading.value=false}}
async function remove(record){await ElMessageBox.confirm(`确定删除“${record.original_name}”及其文件吗？`,'删除记录',{type:'warning'});await deleteTask(record.task_id);ElMessage.success('记录已删除');await load()}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p{margin:5px 0 0;color:#7a8493;font-size:13px}.summary{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:16px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary div{display:flex;align-items:center;flex-direction:column;gap:5px;padding:16px;border-right:1px solid #edf0f3}.summary div:last-child{border:0}.summary span{color:#8a94a3;font-size:11px}.summary strong{font-size:21px}.summary .danger{color:#d95858}.panel{padding:17px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.filters{display:flex;gap:10px;margin-bottom:14px}.filters .el-input{width:min(380px,50%)}.filters .el-select{width:150px}.file-cell{display:flex;align-items:center;gap:10px}.file-cell>.el-icon{width:36px;height:36px;padding:8px;border-radius:4px;background:#f0efff;color:#6a67c8}.file-cell div{display:flex;min-width:0;flex-direction:column;gap:4px}.file-cell strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-cell span{color:#8a94a3;font:10px Consolas,monospace}footer{display:flex;align-items:center;justify-content:space-between;padding-top:15px;color:#8a94a3;font-size:11px}@media(max-width:700px){.summary{grid-template-columns:repeat(2,1fr)}.filters{flex-wrap:wrap}.filters .el-input,.filters .el-select{width:100%}}
</style>
