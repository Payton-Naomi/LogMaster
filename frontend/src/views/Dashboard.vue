<template>
  <div class="page">
    <header class="page-heading"><div><h1>日志数据概览</h1><p>基于数据库中真实上传任务和解析结果汇总</p></div><el-segmented v-model="range" :options="rangeOptions" @change="load" /></header>
    <div class="summary-grid"><div v-for="item in summary" :key="item.label" class="summary-item"><el-icon :class="item.tone"><component :is="item.icon" /></el-icon><div><span>{{ item.label }}</span><strong>{{ formatNumber(stats[item.key]) }}</strong></div></div></div>
    <section class="panel trend"><div class="panel-heading"><div><h2>日志与异常趋势</h2><p>按解析任务创建日期统计</p></div></div><div ref="trendRef" class="chart" /><el-empty v-if="!stats.trend.length" description="暂无趋势数据" :image-size="70" /></section>
    <div class="two-columns">
      <section class="panel"><div class="panel-heading"><div><h2>异常关键字</h2><p>真实解析结果聚合</p></div></div><el-table :data="stats.top_matches"><el-table-column prop="name" label="关键字" /><el-table-column prop="count" label="次数" width="100" /><template #empty><el-empty description="暂无异常结果" :image-size="70" /></template></el-table></section>
      <section class="panel"><div class="panel-heading"><div><h2>最近任务</h2><p>最新上传记录</p></div><el-button link type="primary" @click="router.push('/tasks')">查看全部</el-button></div><el-table :data="stats.recent_tasks"><el-table-column prop="original_name" label="文件" min-width="150" /><el-table-column prop="project_name" label="项目" /><el-table-column prop="status" label="状态" /><template #empty><el-empty description="暂无任务" :image-size="70" /></template></el-table></section>
    </div>
  </div>
</template>

<script setup>
import { markRaw, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { CircleCheck, DataLine, Files, Warning } from '@element-plus/icons-vue'
import { getDashboardStats } from '@/api/task'

const router=useRouter();const range=ref('近 7 天');const rangeOptions=['近 7 天','近 30 天'];const trendRef=ref(null);let chart=null
const stats=ref({total_lines:0,error_count:0,warning_count:0,task_count:0,completed_count:0,failed_count:0,trend:[],top_matches:[],recent_tasks:[]})
const summary=[
  {label:'日志总行数',key:'total_lines',tone:'blue',icon:markRaw(Files)},
  {label:'错误日志',key:'error_count',tone:'red',icon:markRaw(Warning)},
  {label:'解析任务',key:'task_count',tone:'gold',icon:markRaw(DataLine)},
  {label:'已完成任务',key:'completed_count',tone:'green',icon:markRaw(CircleCheck)}
]
const formatNumber=(value)=>Number(value||0).toLocaleString()
const load=async()=>{stats.value=await getDashboardStats(range.value==='近 30 天'?30:7);await nextTick();draw()}
const draw=()=>{if(!trendRef.value||!stats.value.trend.length)return;chart?.dispose();chart=echarts.init(trendRef.value);chart.setOption({tooltip:{trigger:'axis'},grid:{left:20,right:20,top:20,bottom:20,containLabel:true},xAxis:{type:'category',data:stats.value.trend.map(i=>i.date),axisLabel:{color:'#7a8493'}},yAxis:{type:'value',axisLabel:{color:'#7a8493'}},series:[{name:'日志行数',type:'line',smooth:true,data:stats.value.trend.map(i=>i.lines),itemStyle:{color:'#3478dc'}},{name:'错误',type:'line',smooth:true,data:stats.value.trend.map(i=>i.errors),itemStyle:{color:'#d95858'}}]})}
onMounted(load);onBeforeUnmount(()=>chart?.dispose())
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page-heading{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page-heading h1{margin:0;font-size:22px}.page-heading p{margin:5px 0 0;color:#7a8493;font-size:13px}.summary-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:16px}.summary-item{display:flex;align-items:center;gap:13px;padding:16px 18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.summary-item>.el-icon{display:grid;width:38px;height:38px;place-items:center;border-radius:5px;font-size:20px}.summary-item .blue{color:#3478dc;background:#edf4ff}.summary-item .red{color:#d95858;background:#fff0f0}.summary-item .gold{color:#c9861b;background:#fff6e7}.summary-item .green{color:#2f9275;background:#eaf8f3}.summary-item div{display:flex;flex-direction:column;gap:4px}.summary-item span{color:#7a8493;font-size:12px}.summary-item strong{font-size:21px}.panel{padding:18px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.panel-heading{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:12px}.panel-heading h2{margin:0;font-size:15px}.panel-heading p{margin:4px 0 0;color:#8a94a3;font-size:11px}.trend{min-height:300px;margin-bottom:16px}.chart{height:240px}.two-columns{display:grid;grid-template-columns:1fr 1fr;gap:16px}@media(max-width:800px){.summary-grid{grid-template-columns:repeat(2,1fr)}.two-columns{grid-template-columns:1fr}}@media(max-width:520px){.summary-grid{grid-template-columns:1fr}.page-heading{align-items:flex-start;flex-direction:column;gap:10px}}
</style>
