// 生成 RFC4122 v4 UUID。
// crypto.randomUUID() 只在安全上下文（HTTPS 或 localhost）可用，
// 纯 HTTP + 公网 IP 环境下会 undefined，这里做兜底，保证上传/场景创建在非 HTTPS 环境也能用。
export function uuid() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}
