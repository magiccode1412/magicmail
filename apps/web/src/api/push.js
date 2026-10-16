// Web Push API 调用封装
import api from '@/api/request'

const BASE = '/push'

/** 获取 VAPID 公钥（公开接口） */
export async function getVapidPublicKey() {
  const res = await api.get(`${BASE}/vapid-public-key`)
  return res.public_key
}

/** 订阅 Web Push */
export async function subscribe(subscription) {
  return api.post(`${BASE}/subscribe`, subscription)
}

/** 取消订阅 */
export async function unsubscribe(endpoint) {
  return api.post(`${BASE}/unsubscribe`, { endpoint })
}

/** 发送测试推送 */
export async function sendTest() {
  return api.post(`${BASE}/test`)
}

/** 列出当前用户的订阅 */
export async function listSubscriptions() {
  const res = await api.get(`${BASE}/subscriptions`)
  return res.data || []
}
