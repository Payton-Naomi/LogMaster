<template>
  <div class="page">
    <header><div><h1>测试场景</h1><p>场景和检查项均保存到数据库</p></div><el-button type="primary" :icon="Plus" @click="create">新增场景</el-button></header>
    <div class="layout">
      <aside class="sidebar panel"><button v-for="item in scenarios" :key="item.id" :class="{active:item.id===selectedId}" @click="selectedId=item.id"><strong>{{ item.name }}</strong><span>{{ item.checks.length }} 项检查</span></button><el-empty v-if="!scenarios.length" description="暂无场景" :image-size="70" /></aside>
      <main class="panel" v-if="active">
        <div class="scene-heading"><div><el-input v-model="active.name" placeholder="场景名称" /><el-input v-model="active.description" placeholder="场景说明" /></div><div><el-button :icon="Delete" type="danger" plain @click="remove">删除</el-button><el-button type="primary" :icon="Check" @click="save">保存</el-button></div></div>
        <div class="settings"><label>判定方式</label><el-select v-model="active.judgement"><el-option label="任一错误即失败" value="any-error" /><el-option label="仅严重错误失败" value="critical-only" /></el-select><label>标识颜色</label><el-select v-model="active.color"><el-option label="蓝色" value="blue" /><el-option label="绿色" value="green" /><el-option label="橙色" value="orange" /></el-select></div>
        <div class="checks-heading"><div><h2>检查项</h2><p>这些检查项将作为后续规则解析配置</p></div><el-button :icon="Plus" @click="addCheck">添加检查项</el-button></div>
        <div v-for="(check,index) in active.checks" :key="check.id" class="check-row"><el-switch v-model="check.enabled" /><div class="check-fields"><el-input v-model="check.name" placeholder="检查项名称" /><el-input v-model="check.description" placeholder="说明" /><el-input v-model="check.keywordsText" type="textarea" :rows="2" placeholder="关键字，每行一个" /></div><el-select v-model="check.severity"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /><el-option label="信息" value="info" /></el-select><el-button text circle :icon="Close" @click="active.checks.splice(index,1)" /></div>
        <el-empty v-if="!active.checks.length" description="暂无检查项" :image-size="70" />
      </main>
      <main v-else class="panel empty"><el-empty description="从数据库创建一个测试场景" /></main>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, Close, Delete, Plus } from '@element-plus/icons-vue'
import { createScenario, deleteScenario, getScenarios, updateScenario } from '@/api/scenarios'

const scenarios=ref([]);const selectedId=ref('');const active=computed(()=>scenarios.value.find(item=>item.id===selectedId.value))
const fromAPI=(item)=>({...item,checks:(item.checks||[]).map(check=>({...check,keywordsText:(check.keywords||[]).join('\n')}))})
const toAPI=(item)=>({...item,checks:item.checks.map(({keywordsText,...check})=>({...check,keywords:keywordsText.split('\n').map(v=>v.trim()).filter(Boolean)}))})
async function load(){scenarios.value=(await getScenarios()).map(fromAPI);if(!scenarios.value.some(item=>item.id===selectedId.value))selectedId.value=scenarios.value[0]?.id||''}
async function create(){const id=crypto.randomUUID();const item={id,name:'新测试场景',description:'',color:'blue',judgement:'any-error',checks:[]};await createScenario(item);selectedId.value=id;await load()}
function addCheck(){active.value.checks.push({id:crypto.randomUUID(),name:'',description:'',severity:'warning',enabled:true,keywordsText:''})}
async function save(){if(!active.value.name.trim()){ElMessage.warning('请填写场景名称');return}await updateScenario(active.value.id,toAPI(active.value));ElMessage.success('场景已保存');await load()}
async function remove(){await ElMessageBox.confirm(`确定删除“${active.value.name}”吗？`,'删除场景',{type:'warning'});await deleteScenario(active.value.id);await load()}
onMounted(load)
</script>

<style scoped>
.page{height:100%;overflow:auto;color:#1f2937}.page>header{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18px}.page h1{margin:0;font-size:22px}.page header p,.checks-heading p{margin:5px 0 0;color:#7a8493;font-size:12px}.layout{display:grid;grid-template-columns:230px minmax(0,1fr);gap:16px}.panel{padding:16px;border:1px solid #dfe3e8;border-radius:6px;background:#fff}.sidebar{height:max-content}.sidebar button{display:flex;width:100%;align-items:flex-start;flex-direction:column;gap:5px;margin-bottom:5px;padding:11px;border:0;border-radius:4px;background:transparent;text-align:left;cursor:pointer}.sidebar button.active{background:#edf4ff;color:#2868ca}.sidebar span{color:#8a94a3;font-size:11px}.scene-heading,.settings,.checks-heading,.check-row{display:flex;align-items:center}.scene-heading{justify-content:space-between;gap:20px;padding-bottom:16px;border-bottom:1px solid #edf0f3}.scene-heading>div:first-child{display:grid;flex:1;grid-template-columns:220px 1fr;gap:10px}.settings{gap:10px;padding:16px 0}.settings label{color:#667085;font-size:12px}.settings .el-select{width:170px}.checks-heading{justify-content:space-between;margin:5px 0 12px}.checks-heading h2{margin:0;font-size:15px}.check-row{align-items:flex-start;gap:10px;padding:13px 0;border-bottom:1px solid #edf0f3}.check-fields{display:grid;flex:1;grid-template-columns:180px 1fr 1.3fr;gap:9px}.check-row>.el-select{width:100px}.empty{display:grid;min-height:360px;place-items:center}@media(max-width:900px){.layout{grid-template-columns:1fr}.sidebar{display:flex;overflow-x:auto}.sidebar button{min-width:180px}.check-fields{grid-template-columns:1fr}}@media(max-width:600px){.scene-heading{align-items:flex-start;flex-direction:column}.scene-heading>div:first-child{width:100%;grid-template-columns:1fr}.settings{align-items:flex-start;flex-direction:column}.check-row{flex-wrap:wrap}.check-fields{width:100%;flex-basis:100%}}
</style>
