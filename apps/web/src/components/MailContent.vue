<!-- 
  SPDX-License-Identifier: AGPL-3.0-or-later
  Copyright (C) 2026  magiccode (魔法代码)
-->
<template>
  <div class="mail-content">
    <!-- iframe 沙箱渲染模式 -->
    <iframe
      v-if="htmlBody && appStore.mailRenderMode === 'iframe'"
      ref="iframeRef"
      class="content-iframe"
      :srcdoc="iframeSrcdoc"
      sandbox="allow-same-origin"
    ></iframe>

    <!-- 内联渲染模式 -->
    <div
      v-else-if="htmlBody"
      class="content-html"
      :class="{ 'btn-center-fix': appStore.mailButtonCenter }"
      v-html="sanitizedHTML"
    ></div>

    <!-- 降级为纯文本 -->
    <pre
      v-else-if="textBody"
      class="content-text"
    >{{ textBody }}</pre>

    <!-- 无正文 -->
    <p v-else class="content-empty text-muted">（此邮件无正文内容）</p>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted, nextTick } from 'vue'
import { useAppStore } from '@/stores/appStore'
import DOMPurify from 'dompurify'

const props = defineProps({
  htmlBody: { type: String, default: null },
  textBody: { type: String, default: null },
})

const appStore = useAppStore()
const iframeRef = ref(null)

/**
 * 获取当前主题色值（从 CSS 变量读取）
 */
function getCSSVar(name) {
  if (typeof document === 'undefined') return ''
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/**
 * 构建 iframe 内部完整 HTML 文档
 * 包含基础样式适配、按钮居中修复等
 */
function buildIframeDocument(html) {
  const fontSize = appStore.MAIL_FONT_SIZES[appStore.mailFontSize] || '15px'
  const bgColor = getCSSVar('--bg-primary') || '#ffffff'
  const textColor = getCSSVar('--text-primary') || '#1e293b'
  const linkColor = getCSSVar('--text-link') || getCSSVar('--primary-500') || '#4F6EF7'
  const fontFamily = getCSSVar('--font-family') || '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'

  // 按钮居中修复 CSS
  const btnFixCSS = appStore.mailButtonCenter
    ? `a[style*="display: block"], a[style*="display:block"] { display: inline-block !important; }
       .btn { display: inline-block !important; }`
    : ''

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
  * { box-sizing: border-box; }
  body {
    margin: 0;
    padding: 16px;
    font-family: ${fontFamily};
    font-size: ${fontSize};
    line-height: 1.75;
    color: ${textColor};
    word-wrap: break-word;
    overflow-wrap: break-word;
    background: ${bgColor};
  }
  img { max-width: 100% !important; height: auto !important; border-radius: 8px; }
  table { max-width: 100%; border-collapse: collapse; }
  pre, code { background: #f1f5f9; border-radius: 4px; padding: 2px 6px; font-size: 0.9em; }
  pre { padding: 12px; overflow-x: auto; }
  blockquote { margin: 16px 0; padding: 12px 20px; border-left: 4px solid #93c5fd; border-radius: 0 6px 6px 0; color: #64748b; }
  a { color: ${linkColor}; text-decoration: underline; text-underline-offset: 3px; }
  .card { background: transparent !important; border: none !important; border-radius: 0 !important; padding: 0 !important; box-shadow: none !important; }
  ${btnFixCSS}
</style>
</head>
<body>${html}</body>
</html>`
}

// 使用 DOMPurify 进行完整的 HTML 消毒（防止 XSS），供内联模式使用
const sanitizedHTML = computed(() => {
  if (!props.htmlBody) return ''

  const clean = DOMPurify.sanitize(props.htmlBody, {
    ALLOWED_TAGS: undefined,
    ALLOWED_ATTR: undefined,
    ADD_ATTR: ['target'],
    ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel|ftp):|data:image\/)/i,
  })

  let html = clean

  // 图片处理
  if (appStore.mailLoadImages) {
    html = html.replace(/<img\s/gi, '<img style="max-width:100%;height:auto;border-radius:8px;" ')
  } else {
    html = html.replace(/<img\s([^>]*)>/gi, (match, attrs) => {
      if (/src\s*=\s*(?:"data:image|'data:image)/i.test(attrs)) {
        return `<img ${attrs} style="max-width:100%;height:auto;border-radius:8px;">`
      }
      const attrsNoSrc = attrs.replace(/\s*src\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
      return `<img ${attrsNoSrc} style="max-width:100%;height:auto;border-radius:8px;background:#f1f5f9;min-height:40px;" data-blocked-remote alt="[远程图片已屏蔽]">`
    })
  }

  // 为所有 a 标签补充 target="_blank" rel="noopener"
  html = html.replace(
    /<a\s+([^>]*)>/gi,
    (match, attrs) => {
      if (!attrs.includes('target=')) {
        return `<a ${attrs} target="_blank" rel="noopener noreferrer">`
      }
      return match
    }
  )

  // 容器样式
  const fontSize = appStore.MAIL_FONT_SIZES[appStore.mailFontSize] || appStore.MAIL_FONT_SIZES.medium
  const wrapperStyle = `font-family: var(--font-family); line-height: 1.75; color: var(--text-primary); word-wrap: break-word; overflow-wrap: break-word; font-size: ${fontSize};`

  return `<div style="${wrapperStyle}">${html}</div>`
})

// iframe 模式：生成 srcdoc 内容
const rawSanitized = computed(() => {
  if (!props.htmlBody) return ''
  const clean = DOMPurify.sanitize(props.htmlBody, {
    ALLOWED_TAGS: undefined,
    ALLOWED_ATTR: undefined,
    ADD_ATTR: ['target'],
    ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel|ftp):|data:image\/)/i,
  })
  let html = clean

  // 图片处理（与内联模式相同逻辑）
  if (appStore.mailLoadImages) {
    html = html.replace(/<img\s/gi, '<img style="max-width:100%;height:auto;border-radius:8px;" ')
  } else {
    html = html.replace(/<img\s([^>]*)>/gi, (match, attrs) => {
      if (/src\s*=\s*(?:"data:image|'data:image)/i.test(attrs)) {
        return `<img ${attrs} style="max-width:100%;height:auto;border-radius:8px;">`
      }
      const attrsNoSrc = attrs.replace(/\s*src\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
      return `<img ${attrsNoSrc} style="max-width:100%;height:auto;border-radius:8px;background:#f1f5f9;min-height:40px;" data-blocked-remote alt="[远程图片已屏蔽]">`
    })
  }

  // a 标签补充 target
  html = html.replace(
    /<a\s+([^>]*)>/gi,
    (match, attrs) => {
      if (!attrs.includes('target=')) {
        return `<a ${attrs} target="_blank" rel="noopener noreferrer">`
      }
      return match
    }
  )

  return html
})

const iframeSrcdoc = computed(() => buildIframeDocument(rawSanitized.value))

// 监听内容变化，重新写入 iframe（解决 srcdoc 不动态更新的问题）
watch(iframeSrcdoc, () => {
  nextTick(() => writeIframeContent())
})

onMounted(() => {
  nextTick(() => writeIframeContent())
})

function writeIframeContent() {
  const iframe = iframeRef.value
  if (!iframe || !iframe.contentDocument || !iframeSrcdoc.value) return
  iframe.contentDocument.open()
  iframe.contentDocument.write(iframeSrcdoc.value)
  iframe.contentDocument.close()
}

/**
 * 动态调整 iframe 高度以适应其内部文档内容
 */
function adjustIframeHeight() {
  const iframe = iframeRef.value
  if (!iframe || !iframe.contentDocument) return

  try {
    // 等待 iframe 内容完全加载后再计算高度
    const doc = iframe.contentDocument
    const body = doc.body
    const html = doc.documentElement

    // 获取实际内容高度（取 body 和 html 的较大值）
    const bodyHeight = body?.scrollHeight ?? 0
    const htmlHeight = html?.scrollHeight ?? 0
    const contentHeight = Math.max(bodyHeight, htmlHeight, 120)

    // 设置 iframe 高度为内容高度
    iframe.style.height = contentHeight + 'px'
  } catch (e) {
    // 跨域等异常情况时使用默认最小高度
    console.warn('adjustIframeHeight failed:', e)
  }
}

// 监听 iframe 加载事件来调整高度
const iframeEl = computed(() => iframeRef.value)
watch([iframeSrcdoc, iframeEl], () => {
  nextTick(() => {
    const iframe = iframeRef.value
    if (iframe) {
      iframe.addEventListener('load', adjustIframeHeight)
      adjustIframeHeight()
    }
  })
})
</script>

<style scoped>
.mail-content {
  min-height: 120px;
}

/* ---- iframe 渲染 ---- */
.content-iframe {
  width: 100%;
  height: auto;
  min-height: 120px;
  border: none;
  display: block;
}

/* ---- 内联渲染 ---- */
.content-text {
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  line-height: var(--line-height-relaxed);
  color: var(--text-secondary);
}

.content-empty {
  padding: 40px 0;
  text-align: center;
}

/* ---- HTML 内容内嵌样式覆盖 ---- */
/* 隔离应用全局样式，避免污染邮件原始布局 */
.content-html.btn-center-fix :deep(a[style*="display: block"]) {
  display: inline-block !important;
}
.content-html :deep(.btn) {
  display: inline-block !important;
}
.content-html :deep(.btn:hover) {
  transform: none !important;
}
.content-html :deep(.card) {
  background: transparent !important;
  border: none !important;
  border-radius: 0 !important;
  padding: 0 !important;
  box-shadow: none !important;
}
.content-html :deep(img) {
  max-width: 100% !important;
  height: auto !important;
  border-radius: var(--radius-md);
}
.content-html :deep(a) {
  color: var(--text-link);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.content-html :deep(blockquote) {
  margin: 16px 0;
  padding: 12px 20px;
  background: var(--bg-secondary);
  border-left: 4px solid var(--primary-300);
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  color: var(--text-secondary);
}
.content-html :deep(table) {
  max-width: 100%;
  border-collapse: collapse;
}
.content-html :deep(pre), .content-html :deep(code) {
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  padding: 2px 6px;
  font-size: 0.9em;
}
.content-html :deep(pre) {
  padding: var(--space-md);
  overflow-x: auto;
}
</style>
