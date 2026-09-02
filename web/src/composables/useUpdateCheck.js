/**
 * 版本更新检测 composable
 *
 * 通过请求 EdgeOne Pages 托管的 version.json 检测是否有新版本。
 * 支持本地缓存（默认 1 小时），避免频繁请求。
 *
 * 用法:
 *   const { hasUpdate, latestVersion, currentVersion, checkUpdate } = useUpdateCheck()
 *   await checkUpdate()          // 检查一次
 *   await checkUpdate(true)      // 强制检查（忽略缓存）
 *
 * 返回值区分两种「有新版本」：
 *   hasNewerVersion - 远端确实比本地新（客观事实，不受「忽略」影响）
 *   hasUpdate       - 是否需要向用户提示（= hasNewerVersion 且该版本未被忽略）
 */
import { ref } from 'vue'
import { APP_VERSION } from '@/appVersion'

const CACHE_KEY = 'magicmail-update-check'
const DISMISS_KEY = 'magicmail-update-dismissed'
const CACHE_DURATION = 60 * 60 * 1000 // 1 小时缓存

// 全局单例状态（多个组件共享同一份检查结果）
const latestVersion = ref('')
const hasUpdate = ref(false)
const hasNewerVersion = ref(false)
const changelog = ref({})
const downloadUrl = ref('')
const loading = ref(false)
let lastCheckedAt = 0

/** 统一版本号格式：去掉前缀 v，得到 1.2.0 形式（模板里自行加 v 展示） */
function normalizeVer(v) {
  return String(v || '').trim().replace(/^v/i, '')
}

/** 解析版本号字符串为可比较的数字数组 */
function parseVer(v) {
  if (!v) return [0, 0, 0]
  return normalizeVer(v).split('.').map(n => parseInt(n, 10) || 0)
}

/**
 * 比较 two versions.
 * Returns positive if b > a, negative if b < a, zero if equal.
 */
function compareVersions(a, b) {
  const pa = parseVer(a), pb = parseVer(b)
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pb[i] || 0) - (pa[i] || 0)
    if (diff !== 0) return diff
  }
  return 0
}

// ── 「忽略此版本」持久化 ──────────────────────────
// null 表示尚未从 localStorage 读取
let dismissedVersion = null

function getDismissed() {
  if (dismissedVersion === null) {
    try {
      dismissedVersion = localStorage.getItem(DISMISS_KEY) || ''
    } catch {
      dismissedVersion = ''
    }
  }
  return dismissedVersion
}

/**
 * 该远端版本是否已被用户忽略。
 * 忽略语义是「忽略这一个版本」：只有出现更高的版本时才重新提示。
 */
function isDismissed(ver) {
  const dismissed = getDismissed()
  if (!dismissed || !ver) return false
  return compareVersions(dismissed, ver) <= 0
}

export function useUpdateCheck() {
  /** 读取缓存 */
  function getCached() {
    try {
      const raw = localStorage.getItem(CACHE_KEY)
      if (!raw) return null
      const cached = JSON.parse(raw)
      if (Date.now() - cached.timestamp < CACHE_DURATION) {
        return cached
      }
    } catch {
      // ignore
    }
    return null
  }

  /** 写入缓存 */
  function setCache(data) {
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify({ ...data, timestamp: Date.now() }))
    } catch {
      // ignore
    }
  }

  /**
   * 执行版本检测
   * @param {boolean} force - 是否忽略缓存强制请求远程
   */
  async function checkUpdate(force = false) {
    // 非强制模式且缓存有效，直接使用缓存结果
    if (!force && Date.now() - lastCheckedAt < CACHE_DURATION) {
      // 已经有检查结果了，不需要重复计算
      return { hasUpdate: hasUpdate.value, latestVersion: latestVersion.value }
    }

    // 尝试读取缓存
    if (!force) {
      const cached = getCached()
      if (cached) {
        latestVersion.value = normalizeVer(cached.latestVersion)
        changelog.value = cached.changelog || {}
        downloadUrl.value = cached.downloadUrl || ''
        // compareVersions(a, b) > 0 表示 b 比 a 新，即远端版本比本地新才有更新
        hasNewerVersion.value = compareVersions(APP_VERSION, cached.latestVersion) > 0
        hasUpdate.value = hasNewerVersion.value && !isDismissed(latestVersion.value)
        lastCheckedAt = Date.now()
        return { hasUpdate: hasUpdate.value, latestVersion: latestVersion.value }
      }
    }

    // 远程请求
    const url = __UPDATE_CHECK_URL__ || ''
    if (!url) {
      console.warn('[UpdateCheck] 未配置 UPDATE_CHECK_URL，跳过版本检查')
      return { hasUpdate: false, latestVersion: '' }
    }

    loading.value = true
    try {
      const resp = await fetch(url, {
        cache: force ? 'no-cache' : 'default',
        signal: AbortSignal.timeout(8000),
      })
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)

      const data = await resp.json()
      const remote = normalizeVer(data.latest)

      if (!remote) throw new Error('无效的版本数据')

      latestVersion.value = remote
      changelog.value = data.changelog || {}
      downloadUrl.value = data.downloadUrl || data.githubUrl || ''

      // 缓存结果
      setCache({
        latestVersion: remote,
        changelog: data.changelog,
        downloadUrl: data.downloadUrl || '',
      })

      // 对比版本号：只有远端版本比本地新才算有更新
      // （本地手动安装了比远端更高的版本时，compareVersions 为负数，不应提示）
      hasNewerVersion.value = compareVersions(APP_VERSION, remote) > 0
      // 已忽略的版本不再提示，但更高版本仍会重新出现
      hasUpdate.value = hasNewerVersion.value && !isDismissed(remote)
      lastCheckedAt = Date.now()

      return { hasUpdate: hasUpdate.value, latestVersion: remote }
    } catch (e) {
      console.warn('[UpdateCheck] 检查失败:', e.message)
      return { hasUpdate: false, latestVersion: '', error: e.message }
    } finally {
      loading.value = false
    }
  }

  /**
   * 忽略当前这个远端版本（用户点击横幅上的"×"）。
   * 持久化到 localStorage：刷新、重启、下次自动检查都不会再弹同一个版本，
   * 直到远端出现更高的版本才重新提示。
   */
  function dismiss() {
    hasUpdate.value = false
    dismissedVersion = latestVersion.value || ''
    try {
      localStorage.setItem(DISMISS_KEY, dismissedVersion)
    } catch {
      // ignore
    }
  }

  return {
    // 统一不带 v 前缀，模板负责展示时补 v，避免渲染成 vv1.2.0
    latestVersion,
    currentVersion: normalizeVer(APP_VERSION),
    hasUpdate,
    hasNewerVersion,
    changelog,
    downloadUrl,
    loading,
    checkUpdate,
    dismiss,
  }
}
