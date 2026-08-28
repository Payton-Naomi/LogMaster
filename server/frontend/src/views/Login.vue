<template>
  <main class="login-shell">
    <div class="ambient ambient-left" aria-hidden="true"></div>
    <div class="ambient ambient-right" aria-hidden="true"></div>

    <section class="login-layout">
      <aside class="brand-panel">
        <div class="brand-mark"><span>LM</span></div>
        <p class="eyebrow">LOGMASTER PLATFORM</p>
        <h1 class="brand-statement"><span class="statement-line statement-lead">让每一份日志</span><span class="statement-line statement-trace">都可追溯</span><span class="statement-line statement-action">可行动</span></h1>
        <p class="brand-copy">面向研发与交付团队的统一日志分析平台，安全接入、快速定位、协同闭环</p>
        <div class="brand-meta">
          <span class="meta-dot"></span>
          <span>企业级安全认证</span>
          <span class="meta-divider"></span>
          <span>统一工作空间</span>
        </div>
      </aside>

      <section class="auth-panel" aria-labelledby="login-title">
        <div class="panel-header">
          <div class="step-indicator" aria-label="登录步骤">
            <span :class="{ active: step === 1, complete: step > 1 }">01</span>
            <i></i>
            <span :class="{ active: step === 2 }">02</span>
          </div>
          <button v-if="step === 2" class="back-button" type="button" @click="backToMethods">返回方式选择</button>
        </div>

        <template v-if="step === 1">
          <div class="heading-block">
            <p class="section-kicker">欢迎回来</p>
            <h2 id="login-title">选择登录方式</h2>
            <p>请选择与你的账号类型对应的入口</p>
          </div>
          <div class="method-list">
            <button class="method-card feishu-card" type="button" @click="selectFeishu">
              <span class="method-icon feishu-icon">飞</span>
              <span class="method-copy"><strong>企业员工</strong><small>使用飞书账号安全登录</small></span>
              <span class="method-arrow">→</span>
            </button>
            <button class="method-card external-card" type="button" @click="selectExternal">
              <span class="method-icon external-icon">外</span>
              <span class="method-copy"><strong>外部用户</strong><small>使用邮箱和密码登录或注册</small></span>
              <span class="method-arrow">→</span>
            </button>
          </div>
          <p class="security-note"><span class="lock-icon">⌁</span> 你的登录信息将通过加密连接传输</p>
        </template>

        <template v-else>
          <div class="heading-block compact-heading">
            <p class="section-kicker">{{ mode === 'feishu' ? '企业统一认证' : (authTab === 'login' ? '外部用户登录' : authTab === 'reset' ? '找回外部用户密码' : '创建外部用户') }}</p>
            <h2 id="login-title">{{ mode === 'feishu' ? '使用飞书登录' : (authTab === 'login' ? '登录 LogMaster' : authTab === 'reset' ? '重置密码' : '注册外部用户') }}</h2>
            <p>{{ mode === 'feishu' ? '通过企业飞书完成身份验证' : '请输入账号信息以继续访问工作空间' }}</p>
          </div>

          <div v-if="mode === 'feishu'" class="feishu-login">
            <div class="feishu-visual"><span>飞</span><div><strong>飞书企业认证</strong><small>单点登录 · 安全可信</small></div></div>
            <button class="primary-button" type="button" @click="goFeishu">继续使用飞书登录 <span>→</span></button>
            <p class="fine-print">登录后将返回 LogMaster 工作空间</p>
          </div>

          <div v-else class="external-login">
            <div class="auth-tabs" role="tablist">
              <button type="button" :class="{ selected: authTab === 'login' }" @click="switchAuthTab('login')">登录</button>
              <button type="button" :class="{ selected: authTab === 'register' }" @click="switchAuthTab('register')">注册</button>
            </div>
            <form method="post" autocomplete="on" @submit.prevent="submitExternal">
              <div v-if="authTab === 'register' || authTab === 'reset'" class="field-group"><label for="name">姓名</label><input id="name" v-model.trim="form.name" name="name" autocomplete="name" placeholder="请输入姓名" /></div>
              <div class="field-group"><label for="email">邮箱地址</label><input id="email" v-model.trim="form.email" name="username" type="email" inputmode="email" autocomplete="username" autocapitalize="none" spellcheck="false" :readonly="authTab === 'login' && !credentialFieldsReady" placeholder="name@company.com" @pointerdown="activateCredentialFields" @keydown="activateCredentialFields" /></div>
              <div v-if="authTab === 'register'" class="field-group"><label for="company">所属公司</label><input id="company" v-model.trim="form.company" name="organization" autocomplete="organization" placeholder="请输入公司名称" /></div>
              <div class="field-group"><label for="password">{{ authTab === 'reset' ? '新密码' : '密码' }}</label><input id="password" v-model="form.password" name="password" type="password" :autocomplete="authTab === 'login' ? 'current-password' : 'new-password'" :readonly="authTab === 'login' && !credentialFieldsReady" placeholder="请输入密码" @pointerdown="activateCredentialFields" @keydown="activateCredentialFields" /></div>
              <div v-if="authTab === 'register' || authTab === 'reset'" class="field-group"><label for="confirm-password">确认密码</label><input id="confirm-password" v-model="form.confirm_password" name="confirm_password" type="password" autocomplete="new-password" placeholder="请再次输入密码" /></div>
              <div v-if="authTab === 'login'" class="form-options"><button type="button" class="text-button" @click="showForgotHint">忘记密码？</button></div>
              <div v-if="authTab === 'reset'" class="form-options"><button type="button" class="text-button" @click="switchAuthTab('login')">返回登录</button></div>
              <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
              <button class="primary-button" type="submit" :disabled="submitting">{{ submitting ? '提交中…' : (authTab === 'login' ? '登录工作空间' : authTab === 'reset' ? '重置密码' : '创建并登录') }} <span>→</span></button>
            </form>
            <p class="fine-print">注册即表示你同意平台的账号使用规范</p>
          </div>
        </template>
      </section>
    </section>
    <footer class="login-footer"><span>© 2026 LogMaster</span><span>需要帮助？请联系平台管理员</span></footer>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { externalLogin, externalPasswordReset, externalRegister } from '@/api/auth'

const router = useRouter()
const route = useRoute()
const step = ref(1)
const mode = ref('')
const authTab = ref('login')
const credentialFieldsReady = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const form = reactive({ name: '', email: '', company: '', password: '', confirm_password: '' })
const feishuLoginURL = import.meta.env.VITE_FEISHU_LOGIN_URL || '/api/auth/feishu-login'

function selectFeishu () { mode.value = 'feishu'; step.value = 2; errorMessage.value = '' }
function clearExternalForm () { Object.assign(form, { name: '', email: '', company: '', password: '', confirm_password: '' }) }
function selectExternal () { clearExternalForm(); credentialFieldsReady.value = false; mode.value = 'external'; step.value = 2; errorMessage.value = '' }
function backToMethods () { clearExternalForm(); credentialFieldsReady.value = false; step.value = 1; errorMessage.value = '' }
function switchAuthTab (tab) { authTab.value = tab; form.password = ''; form.confirm_password = ''; credentialFieldsReady.value = tab !== 'login'; errorMessage.value = '' }
function activateCredentialFields (event) { credentialFieldsReady.value = true; if (event?.currentTarget) event.currentTarget.readOnly = false }
function goFeishu () { window.location.href = feishuLoginURL }
function showForgotHint () { switchAuthTab('reset') }

async function submitExternal () {
  errorMessage.value = ''
  if (!form.email || !form.password) { errorMessage.value = authTab.value === 'reset' ? '请填写姓名、邮箱和新密码' : '请输入邮箱和密码'; return }
  if ((authTab.value === 'register' || authTab.value === 'reset') && !form.name) { errorMessage.value = '请输入姓名'; return }
  if (authTab.value === 'register' && !form.company) { errorMessage.value = '请输入所属公司'; return }
  if ((authTab.value === 'register' || authTab.value === 'reset') && form.password !== form.confirm_password) { errorMessage.value = '两次输入的密码不一致'; return }
  submitting.value = true
  try {
    const payload = authTab.value === 'login'
      ? { email: form.email, password: form.password }
      : { name: form.name, email: form.email, password: form.password, confirm_password: form.confirm_password, ...(authTab.value === 'register' ? { company: form.company } : {}) }
    if (authTab.value === 'login') await externalLogin(payload)
    else if (authTab.value === 'register') await externalRegister(payload)
    else { await externalPasswordReset(payload); switchAuthTab('login'); ElMessage.success('密码已重置，请使用新密码登录'); return }
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/upload'
    await router.push(redirect)
  } catch (error) {
    errorMessage.value = authTab.value === 'reset'
      ? '姓名、邮箱或账号信息不匹配'
      : error?.response?.data?.message || error?.message || '登录失败，请稍后重试'
  } finally { submitting.value = false }
}
</script>

<style scoped>
:global(body) { overflow: auto; }
.login-shell { min-height: 100%; position: relative; display: flex; flex-direction: column; justify-content: center; padding: 42px 6vw 24px; background: #07131c; color: #e9f2f4; overflow: hidden; }
.ambient { position: absolute; width: 540px; height: 540px; border-radius: 50%; filter: blur(1px); pointer-events: none; opacity: .5; }
.ambient-left { left: -300px; top: 10%; background: radial-gradient(circle, rgba(5, 183, 205, .2), transparent 68%); }
.ambient-right { right: -280px; bottom: -100px; background: radial-gradient(circle, rgba(70, 114, 215, .18), transparent 70%); }
.login-layout { width: min(1100px, 100%); margin: auto; display: grid; grid-template-columns: minmax(330px, 1fr) minmax(420px, 520px); gap: clamp(56px, 9vw, 150px); align-items: center; position: relative; z-index: 1; }
.brand-panel { max-width: 470px; }
.brand-mark { display: grid; place-items: center; width: 50px; height: 50px; border-radius: 13px; background: linear-gradient(135deg, #18c4cf, #3776dc); box-shadow: 0 12px 30px rgba(27, 193, 208, .2); font-weight: 800; letter-spacing: 0; }
.eyebrow, .section-kicker { margin: 22px 0 14px; color: #5cd2d4; font-size: 11px; font-weight: 700; letter-spacing: .14em; }
.brand-statement { position: relative; width: min(470px, 100%); margin: 0; font-size: 54px; line-height: 1; letter-spacing: 0; font-weight: 750; }
.brand-statement::before { position: absolute; top: 53%; left: 4px; width: 88%; height: 1px; background: linear-gradient(90deg, transparent, rgba(92, 210, 212, .52), transparent); content: ''; transform: rotate(-7deg); transform-origin: left center; }
.statement-line { position: relative; display: block; width: max-content; max-width: 100%; }
.statement-lead { color: #f0f7f8; transform: rotate(-1deg); }
.statement-trace { z-index: 1; margin-top: 13px; margin-left: 30px; color: #75d7dd; font-style: italic; transform: skewX(-7deg); }
.statement-action { z-index: 2; margin-top: 5px; margin-left: 174px; padding: 5px 13px 7px; color: #07131c; background: #75d7dd; font-size: 39px; font-style: italic; line-height: 1; transform: rotate(-3deg) skewX(-5deg); box-shadow: 8px 8px 0 rgba(47, 111, 191, .35); }
.brand-copy { margin: 24px 0 0; max-width: 390px; color: #90a7b2; line-height: 1.85; font-size: 15px; }
.brand-meta { display: flex; align-items: center; gap: 9px; margin-top: 34px; color: #6f8792; font-size: 12px; }
.meta-dot { width: 7px; height: 7px; border-radius: 50%; background: #45d8b0; box-shadow: 0 0 0 4px rgba(69, 216, 176, .12); }.meta-divider { width: 1px; height: 14px; background: #29414d; margin: 0 4px; }
.auth-panel { padding: clamp(28px, 4vw, 48px); border: 1px solid rgba(166, 220, 229, .16); border-radius: 18px; background: rgba(13, 28, 39, .9); box-shadow: 0 28px 90px rgba(0, 0, 0, .3), inset 0 1px rgba(255,255,255,.06); }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 42px; }.step-indicator { display: flex; align-items: center; gap: 12px; color: #4e6875; font: 700 11px/1 Inter, sans-serif; }.step-indicator span { display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid #2a4653; border-radius: 50%; }.step-indicator span.active, .step-indicator span.complete { border-color: #50ccd0; background: rgba(80, 204, 208, .13); color: #86e4e5; }.step-indicator i { width: 32px; height: 1px; background: #29434e; }.back-button, .text-button { border: 0; background: transparent; color: #76bac0; cursor: pointer; font-size: 12px; padding: 5px 0; }.back-button:hover, .text-button:hover { color: #a5eced; }
.heading-block h2 { margin: 0; color: #f2f8f9; font-size: 29px; letter-spacing: 0; }.heading-block p:last-child { margin: 10px 0 0; color: #8199a4; font-size: 14px; }.compact-heading { margin-bottom: 28px; }.compact-heading .section-kicker { margin-top: 0; }
.method-list { display: grid; gap: 13px; margin-top: 34px; }.method-card { width: 100%; display: flex; align-items: center; gap: 16px; padding: 18px; text-align: left; border: 1px solid rgba(158, 214, 221, .15); border-radius: 12px; color: #e9f5f5; background: rgba(24, 48, 61, .48); cursor: pointer; transition: border-color .2s, transform .2s, background .2s; }.method-card:hover { transform: translateY(-2px); border-color: rgba(105, 218, 221, .62); background: rgba(29, 65, 78, .65); }.method-icon { display: grid; place-items: center; width: 40px; height: 40px; flex: 0 0 40px; border-radius: 10px; font-size: 18px; font-weight: 700; }.feishu-icon { color: #b2f0f1; background: rgba(36, 177, 191, .18); }.external-icon { color: #c3d3ff; background: rgba(94, 119, 222, .2); }.method-copy { display: flex; flex-direction: column; gap: 5px; }.method-copy strong { font-size: 15px; }.method-copy small { color: #819ba5; font-size: 12px; }.method-arrow { margin-left: auto; color: #70c7cc; font-size: 22px; }.security-note, .fine-print { color: #607a86; font-size: 11px; text-align: center; }.security-note { margin: 26px 0 0; }.lock-icon { color: #54c8ca; margin-right: 5px; }
.feishu-login { margin-top: 36px; text-align: center; }.feishu-visual { display: flex; align-items: center; gap: 14px; padding: 18px; text-align: left; border: 1px solid rgba(102, 207, 212, .14); border-radius: 11px; background: rgba(27, 61, 71, .35); }.feishu-visual > span { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 10px; background: #1eafb8; color: #eaffff; font-weight: 800; font-size: 19px; }.feishu-visual div { display: flex; flex-direction: column; gap: 5px; }.feishu-visual strong { font-size: 14px; }.feishu-visual small { color: #7f9da5; font-size: 12px; }
.auth-tabs { display: flex; gap: 22px; margin-bottom: 24px; border-bottom: 1px solid rgba(155, 206, 215, .14); }.auth-tabs button { padding: 0 2px 12px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: #718994; cursor: pointer; font-size: 14px; }.auth-tabs button.selected { border-color: #64d4d7; color: #ddf8f8; font-weight: 700; }
.field-group { margin-bottom: 17px; }.field-group label { display: block; margin-bottom: 8px; color: #a4bbc3; font-size: 12px; }.field-group input { width: 100%; height: 44px; border: 1px solid rgba(156, 207, 214, .18); border-radius: 8px; outline: none; padding: 0 13px; background: rgba(5, 17, 26, .72); color: #e8f5f5; transition: border-color .2s, box-shadow .2s; }.field-group input::placeholder { color: #56717e; }.field-group input:focus { border-color: #4ecbd0; box-shadow: 0 0 0 3px rgba(78, 203, 208, .12); }.form-options { display: flex; justify-content: flex-end; margin: -3px 0 18px; }.form-error { margin: 0 0 14px; color: #ff9f9f; font-size: 12px; }
.field-group input:-webkit-autofill,.field-group input:-webkit-autofill:hover,.field-group input:-webkit-autofill:focus { border-color: rgba(78,203,208,.5); box-shadow:0 0 0 1000px #0b1b27 inset;-webkit-text-fill-color:#e8f5f5;caret-color:#e8f5f5;transition:background-color 9999s ease-out 0s; }
.primary-button { width: 100%; height: 46px; margin-top: 22px; border: 0; border-radius: 8px; background: linear-gradient(100deg, #27bfc5, #3a90dc); color: #06202a; font-weight: 750; cursor: pointer; box-shadow: 0 10px 25px rgba(37, 178, 199, .18); transition: transform .2s, filter .2s; }.primary-button:hover { filter: brightness(1.08); transform: translateY(-1px); }.primary-button:disabled { opacity: .6; cursor: wait; transform: none; }.primary-button span { float: right; margin-right: 13px; font-size: 18px; }.fine-print { margin: 16px 0 0; }.login-footer { width: min(1100px, 100%); margin: 34px auto 0; display: flex; justify-content: space-between; color: #506773; font-size: 11px; position: relative; z-index: 1; }
@media (max-width: 800px) { .login-shell { padding: 28px 20px 18px; }.login-layout { display: block; }.brand-panel { margin: 0 auto 34px; max-width: 520px; }.brand-statement { font-size: 42px; }.statement-action { margin-left: 136px; font-size: 31px; }.brand-copy { margin-top: 14px; }.brand-meta { margin-top: 20px; }.auth-panel { max-width: 520px; margin: auto; }.panel-header { margin-bottom: 30px; }.login-footer { margin-top: 22px; }.ambient { opacity: .28; } }
@media (max-width: 460px) { .brand-statement { font-size: 34px; }.statement-trace { margin-left: 18px; }.statement-action { margin-left: 100px; font-size: 27px; }.brand-copy { font-size: 13px; }.auth-panel { padding: 24px 20px; }.heading-block h2 { font-size: 25px; }.login-footer { flex-direction: column; gap: 7px; text-align: center; } }
</style>
