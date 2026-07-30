// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// 默认主题色
const DEFAULT_PRIMARY = '#4F6EF7'

// 轮询间隔选项（用于设置页面）
export const POLL_INTERVAL_OPTIONS = [
  { value: 30000,  label: '30 秒' },
  { value: 60000,  label: '1 分钟' },
  { value: 120000,  label: '2 分钟' },
  { value: 300000,  label: '5 分钟' },
]

// 每页显示数量选项（用于设置页面）
export const PAGE_SIZE_OPTIONS = [
  { value: 10, label: '10 封/页' },
  { value: 20, label: '20 封/页' },
  { value: 30, label: '30 封/页' },
  { value: 50, label: '50 封/页' },
  { value: 100, label: '100 封/页' },
]

// 预设主题色选项
export const PRESET_COLORS = [
  { value: '#4F6EF7', label: '默认蓝' },
  { value: '#6366F1', label: '靛蓝' },
  { value: '#8B5CF6', label: '紫罗兰' },
  { value: '#EC4899', label: '玫红' },
  { value: '#EF4444', label: '红色' },
  { value: '#F97316', label: '橙色' },
  { value: '#EAB308', label: '金色' },
  { value: '#22C55E', label: '绿色' },
  { value: '#14B8A6', label: '青色' },
  { value: '#06B6D4', label: '天蓝' },
  { value: '#0EA5E9', label: '湖蓝' },
  { value: '#64748B', label: '石板灰' },
]

/* ============================================
   颜色工具函数
   ============================================ */

/** hex → [h, s, l] */
function hexToHSL(hex) {
  hex = hex.replace('#', '')
  if (hex.length === 3) hex = hex.split('').map(c => c + c).join('')
  const num = parseInt(hex, 16)
  const r = ((num >> 16) & 255) / 255
  const g = ((num >> 8) & 255) / 255
  const b = (num & 255) / 255
  const max = Math.max(r, g, b), min = Math.min(r, g, b)
  let h = 0, s = 0, l = (max + min) / 2
  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break
      case g: h = ((b - r) / d + 2) / 6; break
      case b: h = ((r - g) / d + 4) / 6; break
    }
  }
  return [Math.round(h * 360), Math.round(s * 100), Math.round(l * 100)]
}

/** h,s,l → #RRGGBB */
function hslToHex(h, s, l) {
  s /= 100; l /= 100
  const a = s * Math.min(l, 1 - l)
  const f = n => {
    const k = (n + h / 30) % 12
    const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1)
    return Math.round(255 * color).toString(16).padStart(2, '0')
  }
  return `#${f(0)}${f(8)}${f(4)}`.toUpperCase()
}

/** h,s,l,a → hsla() string */
function hsla(h, s, l, a) {
  return `hsla(${h}, ${s}%, ${l}%, ${a})`
}

/* ============================================
   从主题色生成完整配色方案
   所有装饰性颜色均从主题色的色相衍生
   ============================================ */

/**
 * 将 HSL 数值格式化为 CSS hsl() 字符串
 */
function cssHSL(h, s, l) {
  return `hsl(${h}, ${Math.round(s)}%, ${Math.round(l)}%)`
}

/**
 * 将 HSL 数值格式化为 CSS hsla() 字符串
 */
function cssHSLA(h, s, l, a) {
  return `hsla(${h}, ${Math.round(s)}%, ${Math.round(l)}%, ${a})`
}

/**
 * @param {string} baseHex - 主题色 #RRGGBB
 * @param {boolean} isDark - 是否深色模式
 * @returns {Object} cssVarMap: { '--var-name': 'value', ... }
 */
function generateThemePalette(baseHex, isDark) {
  const [h, s, l] = hexToHSL(baseHex)

  if (!isDark) {
    // ========== 浅色模式 ==========
    const p = {
      50:  hslToHex(h, s * 0.28, 97),
      100: hslToHex(h, s * 0.42, 94),
      200: hslToHex(h, s * 0.58, 87),
      300: hslToHex(h, s * 0.74, 77),
      400: hslToHex(h, s * 0.88, 64),
      500: baseHex.toUpperCase(),
      600: hslToHex(h, Math.min(s + 6, 100), l < 42 ? l + 10 : l - 7),
      700: hslToHex(h, Math.min(s + 9, 100), l < 38 ? l + 14 : l - 11),
      800: hslToHex(h, Math.min(s + 11, 100), l < 32 ? l + 19 : l - 15),
      900: hslToHex(h, Math.min(s + 14, 100), l < 26 ? l + 26 : l - 20),
    }

    return {
      // ---- 画布背景（悬浮卡片背后的底色）----
      '--canvas-bg': cssHSL(h, Math.min(s * 0.45, 28), 95),

      // ---- 主色调 ----
      '--primary-50':  p[50],
      '--primary-100': p[100],
      '--primary-200': p[200],
      '--primary-300': p[300],
      '--primary-400': p[400],
      '--primary-500': p[500],
      '--primary-600': p[600],
      '--primary-700': p[700],
      '--primary-800': p[800],
      '--primary-900': p[900],

      // ---- 背景色（带微弱主题色调）----
      '--bg-primary': '#FFFFFF',
      '--bg-secondary': cssHSL(h, Math.min(s * 0.25, 12), 97),
      '--bg-tertiary': cssHSL(h, Math.min(s * 0.32, 16), 94),
      '--bg-elevated': '#FFFFFF',
      '--bg-hover': cssHSL(h, Math.min(s * 0.45, 22), 94),
      '--bg-active': cssHSL(h, Math.min(s * 0.55, 28), 90),
      '--bg-overlay': 'rgba(15, 23, 42, 0.5)',

      // ---- 边框（去饱和主题色）----
      '--border-color': cssHSLA(h, Math.min(s * 0.35, 18), 38, 0.13),
      '--border-light': cssHSLA(h, Math.min(s * 0.25, 12), 42, 0.07),
      '--divider-color': 'var(--border-color)',

      // ---- 特殊元素玻璃态（主题色润色）----
      '--sidebar-bg': 'rgba(248, 250, 252, 0.88)',
      '--header-bg': 'rgba(255, 255, 255, 0.75)',
      '--card-bg': '#FFFFFF',
      '--input-bg': cssHSL(h, Math.min(s * 0.22, 10), 97),

      '--glass-bg': 'rgba(255, 255, 255, 0.68)',
      '--glass-border': 'rgba(255, 255, 255, 0.28)',

      // ---- 邮件状态 ----
      '--mail-read-bg': 'transparent',
      '--mail-unread-bg': cssHSLA(h, s * 0.75, 68, 0.07),
      '--mail-selected-bg': cssHSLA(h, s * 0.80, 62, 0.13),

      // ---- 选中态背景（浅色：柔和浅色主色；深色：暗色主色，避免深色模式下近乎纯白刺眼）----
      '--selected-bg': p[50],

      // ---- 滚动条 ----
      '--scrollbar-track': cssHSL(h, Math.min(s * 0.25, 12), 95),
      '--scrollbar-thumb': cssHSL(h, Math.min(s * 0.35, 18), 82),
      '--scrollbar-thumb-hover': cssHSL(h, Math.min(s * 0.45, 24), 72),

      // ---- 图标按钮交互态 ----
      '--btn-icon-hover-bg': cssHSLA(h, s, l > 50 ? 56 : 46, 0.07),
      '--btn-icon-hover-border': cssHSLA(h, s, l > 50 ? 52 : 44, 0.14),
      '--btn-icon-active-bg': cssHSLA(h, s, l > 50 ? 54 : 48, 0.13),

      // ---- 主题色阴影光晕 ----
      '--shadow-glow': cssHSLA(h, s, 58, 0.22),

      // ---- 批量选择栏 ----
      '--batch-bar-bg': cssHSLA(h, s * 0.75, 68, 0.07),
      '--batch-bar-border': cssHSLA(h, s * 0.78, 62, 0.16),
    }

  } else {
    // ========== 深色模式 ==========
    const p = {
      50:  hslToHex(h, s * 0.22, 98),
      100: hslToHex(h, s * 0.36, 93),
      200: hslToHex(h, s * 0.54, 86),
      300: hslToHex(h, s * 0.72, 76),
      400: hslToHex(h, s * 0.88, 64),
      500: baseHex.toUpperCase(),
      600: hslToHex(h, Math.min(s + 8, 100), l < 50 ? l + 14 : l - 8),
      700: hslToHex(h, Math.min(s + 12, 100), l < 45 ? l + 18 : l - 12),
      800: hslToHex(h, Math.min(s + 14, 100), l < 40 ? l + 22 : l - 16),
      900: hslToHex(h, Math.min(s + 16, 100), l < 34 ? l + 28 : l - 20),
    }

    return {
      // ---- 画布背景 ----
      '--canvas-bg': cssHSL(h, Math.max(s * 0.18, 10), 9),

      // ---- 主色调（整体提亮） ----
      '--primary-50':  p[50],
      '--primary-100': p[100],
      '--primary-200': p[200],
      '--primary-300': p[300],
      '--primary-400': p[400],
      '--primary-500': p[500],
      '--primary-600': p[600],
      '--primary-700': p[700],
      '--primary-800': p[800],
      '--primary-900': p[900],

      // ---- 背景色（暗底 + 微弱主题色辉光）----
      '--bg-primary': cssHSL(h, Math.max(s * 0.08, 4), 10),
      '--bg-secondary': cssHSL(h, Math.max(s * 0.10, 5), 15),
      '--bg-tertiary': cssHSL(h, Math.max(s * 0.12, 6), 19),
      '--bg-elevated': cssHSL(h, Math.max(s * 0.09, 5), 13),
      '--bg-hover': cssHSLA(h, s * 0.5, l > 40 ? 55 : 45, 0.06),
      '--bg-active': cssHSLA(h, s * 0.55, l > 42 ? 56 : 48, 0.10),
      '--bg-overlay': 'rgba(0, 0, 0, 0.68)',

      // ---- 边框 ----
      '--border-color': cssHSLA(h, s * 0.35, 45, 0.14),
      '--border-light': cssHSLA(h, s * 0.25, 50, 0.07),
      '--divider-color': 'var(--border-color)',

      // ---- 特殊元素 ----
      '--sidebar-bg': cssHSLA(h, Math.max(s * 0.06, 3), 10, 0.92),
      '--header-bg': cssHSLA(h, Math.max(s * 0.05, 2), 12, 0.78),
      '--card-bg': cssHSL(h, Math.max(s * 0.08, 4), 13),
      '--input-bg': cssHSL(h, Math.max(s * 0.10, 5), 15),

      '--glass-bg': cssHSLA(h, Math.max(s * 0.08, 4), 14, 0.72),
      '--glass-border': cssHSLA(h, s * 0.25, 50, 0.10),

      // ---- 邮件状态 ----
      '--mail-read-bg': 'transparent',
      '--mail-unread-bg': cssHSLA(h, s * 0.7, 58, 0.10),
      '--mail-selected-bg': cssHSLA(h, s * 0.78, 54, 0.17),

      // ---- 选中态背景（深色：暗色主色半透明，叠在深色卡片上形成柔和选中色，不刺眼）----
      '--selected-bg': cssHSLA(h, Math.min(s + 8, 100), 32, 0.55),

      // ---- 滚动条 ----
      '--scrollbar-track': cssHSL(h, Math.max(s * 0.08, 4), 15),
      '--scrollbar-thumb': cssHSL(h, Math.max(s * 0.18, 8), 24),
      '--scrollbar-thumb-hover': cssHSL(h, Math.max(s * 0.26, 12), 30),

      // ---- 图标按钮交互态 ----
      '--btn-icon-hover-bg': cssHSLA(h, s * 0.85, l > 40 ? 58 : 50, 0.12),
      '--btn-icon-hover-border': cssHSLA(h, s * 0.8, l > 42 ? 55 : 48, 0.18),
      '--btn-icon-active-bg': cssHSLA(h, s * 0.82, l > 41 ? 56 : 49, 0.20),

      // ---- 主题色阴影光晕 ----
      '--shadow-glow': cssHSLA(h, s, 55, 0.28),

      // ---- 批量选择栏（暗底 + 主题色辉光，避免过亮）----
      '--batch-bar-bg': cssHSLA(h, s * 0.78, 54, 0.12),
      '--batch-bar-border': cssHSLA(h, s * 0.80, 56, 0.18),
    }
  }
}

export const useAppStore = defineStore('app', () => {
  // --- 状态 ---
  const themeMode = ref('system') // 'light' | 'dark' | 'system'
  const isDark = ref(false)
  const primaryColor = ref(DEFAULT_PRIMARY)
  const searchKeyword = ref('')
  const isLoading = ref(false)
  const unreadCount = ref(0)

  // --- 邮件渲染设置 ---
  const mailButtonCenter = ref(true)    // 按钮自动居中
  const mailLoadImages = ref(false)     // 加载远程图片
  const mailFontSize = ref('medium')   // 'small' | 'medium' | 'large'
  const mailRenderMode = ref('inline') // 'inline' | 'iframe'
  const mailPageSize = ref(20)         // 每页显示邮件数量

  // --- 数据同步设置 ---
  const pollInterval = ref(60000)      // 轮询间隔（毫秒），默认 60 秒
  const connectionMode = ref('unknown') // 'sse' | 'polling' | 'unknown'
  const lastRefreshAt = ref(0)          // 上次刷新时间戳（毫秒）

  const MAIL_FONT_SIZES = {
    small:  '13px',
    medium: 'var(--font-size-base)',
    large:  '18px',
  }


  // --- 初始化 ---
  function initTheme() {
    const saved = localStorage.getItem('theme-mode')
    themeMode.value = saved || 'system'

    const savedColor = localStorage.getItem('primary-color')
    if (savedColor && isValidColor(savedColor)) {
      primaryColor.value = savedColor
    }

    // 加载邮件渲染设置
    const savedBtnCenter = localStorage.getItem('mail-button-center')
    if (savedBtnCenter !== null) mailButtonCenter.value = savedBtnCenter === 'true'

    const savedLoadImg = localStorage.getItem('mail-load-images')
    if (savedLoadImg !== null) mailLoadImages.value = savedLoadImg === 'true'

    const savedFontSz = localStorage.getItem('mail-font-size')
    if (savedFontSz && ['small', 'medium', 'large'].includes(savedFontSz)) {
      mailFontSize.value = savedFontSz
    }

    const savedRenderMode = localStorage.getItem('mail-render-mode')
    if (savedRenderMode && ['inline', 'iframe'].includes(savedRenderMode)) {
      mailRenderMode.value = savedRenderMode
    }

    const savedPageSize = localStorage.getItem('mail-page-size')
    if (savedPageSize) {
      const parsed = parseInt(savedPageSize, 10)
      if (!isNaN(parsed) && [10, 20, 30, 50, 100].includes(parsed)) {
        mailPageSize.value = parsed
      }
    }

    // 加载数据同步设置
    const savedPollInterval = localStorage.getItem('poll-interval')
    if (savedPollInterval) {
      const parsed = parseInt(savedPollInterval, 10)
      if (!isNaN(parsed) && parsed >= 10000) pollInterval.value = parsed
    }

    applyTheme()
    applyPrimaryColor()

    if (window.matchMedia) {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        if (themeMode.value === 'system') applyTheme()
      })
    }
  }

  function isValidColor(color) {
    return /^#[0-9A-Fa-f]{6}$/.test(color)
  }

  // --- 应用明暗模式 ---
  function applyTheme() {
    let dark = false
    if (themeMode.value === 'dark') dark = true
    else if (themeMode.value === 'system') dark = window.matchMedia('(prefers-color-scheme: dark)').matches

    isDark.value = dark
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
    localStorage.setItem('theme-mode', themeMode.value)

    // 明暗切换时重新生成配色（因为同一主色在明/暗模式下衍生的表面色不同）
    applyPrimaryColor()
  }

  // --- 应用主题色（核心：根据当前明暗模式，将所有衍生颜色写入 CSS 变量）---
  function applyPrimaryColor() {
    const palette = generateThemePalette(primaryColor.value, isDark.value)
    const root = document.documentElement.style
    for (const [name, value] of Object.entries(palette)) {
      root.setProperty(name, value)
    }
    localStorage.setItem('primary-color', primaryColor.value)
    // 同步更新 meta theme-color（影响移动端浏览器地址栏颜色）
    let metaTheme = document.querySelector('meta[name="theme-color"]')
    if (!metaTheme) {
      metaTheme = document.createElement('meta')
      metaTheme.name = 'theme-color'
      document.head.appendChild(metaTheme)
    }
    metaTheme.content = primaryColor.value
  }

  function setPrimaryColor(hex) {
    if (!isValidColor(hex)) return
    primaryColor.value = hex.toUpperCase()
    applyPrimaryColor()
  }

  // --- 切换主题 ---
  function toggleTheme() {
    const modes = ['system', 'light', 'dark']
    const idx = modes.indexOf(themeMode.value)
    themeMode.value = modes[(idx + 1) % modes.length]
    applyTheme()
  }

  function setTheme(mode) {
    themeMode.value = mode
    applyTheme()
  }

  // --- 搜索 / 状态 ---
  function setSearchKeyword(keyword) { searchKeyword.value = keyword }
  function setLoading(v) { isLoading.value = v }
  function setUnreadCount(count) { unreadCount.value = count }

  // --- 邮件渲染设置操作 ---
  function setMailButtonCenter(v) {
    mailButtonCenter.value = v
    localStorage.setItem('mail-button-center', String(v))
  }

  function setMailLoadImages(v) {
    mailLoadImages.value = v
    localStorage.setItem('mail-load-images', String(v))
  }

  function setMailFontSize(size) {
    if (!['small', 'medium', 'large'].includes(size)) return
    mailFontSize.value = size
    localStorage.setItem('mail-font-size', size)
  }

  function setMailRenderMode(mode) {
    if (!['inline', 'iframe'].includes(mode)) return
    mailRenderMode.value = mode
    localStorage.setItem('mail-render-mode', mode)
  }

  function setMailPageSize(size) {
    if (![10, 20, 30, 50, 100].includes(size)) return
    mailPageSize.value = size
    localStorage.setItem('mail-page-size', String(size))
  }

  // --- 数据同步设置操作 ---
  function setPollInterval(ms) {
    if (ms < 10000) return // 最少 10 秒
    pollInterval.value = ms
    localStorage.setItem('poll-interval', String(ms))
  }

  function setConnectionMode(mode) {
    connectionMode.value = mode
  }

  function touchRefresh() {
    lastRefreshAt.value = Date.now()
  }

  return {
    themeMode, isDark, primaryColor, searchKeyword, isLoading, unreadCount,
    mailButtonCenter, mailLoadImages, mailFontSize, mailRenderMode, mailPageSize,
    pollInterval, connectionMode, lastRefreshAt,
    MAIL_FONT_SIZES,
    initTheme, toggleTheme, setTheme, setPrimaryColor, isValidColor,
    setMailButtonCenter, setMailLoadImages, setMailFontSize, setMailRenderMode, setMailPageSize,
    setPollInterval, setConnectionMode, touchRefresh,
    setSearchKeyword, setLoading, setUnreadCount,
  }
})
