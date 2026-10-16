// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

/**
 * 用法:
 *   import { useToast } from '@/composables/useToast'
 *   const toast = useToast()
 *
 *   // Toast
 *   toast.success('操作成功')
 *   toast.error('出错了')
 *   toast.warning('警告')
 *   toast.info('提示信息')
 *
 *   // Confirm (返回 Promise<boolean>)
 *   const ok = await toast.confirm('确定删除吗？')
 *
 *   // Prompt (返回 Promise<string|null>，取消/关闭时为 null)
 *   const url = await toast.prompt('请输入链接地址:', 'https://')
 *
 * 注意：所有弹窗均为代码实现的自定义弹窗，不会回退到浏览器自带的
 * alert / confirm / prompt 原生对话框。
 */

let toastState = null
let confirmState = null
let promptState = null

export function initToast(state) {
  toastState = state
}

export function initConfirm(state) {
  confirmState = state
}

export function initPrompt(state) {
  promptState = state
}

// 当 confirmState / promptState 尚未初始化时，使用纯 DOM 构建的自定义弹窗兜底，
// 保证任何情况下都不会回退到浏览器自带的原生对话框。
function fallbackModal({ title, message, input = false, defaultValue = '' }) {
  return new Promise((resolve) => {
    if (typeof document === 'undefined') {
      resolve(input ? null : false)
      return
    }
    const overlay = document.createElement('div')
    overlay.style.cssText =
      'position:fixed;inset:0;background:rgba(0,0,0,.45);backdrop-filter:blur(4px);' +
      'display:flex;align-items:center;justify-content:center;z-index:2147483647;' +
      'font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;'
    const box = document.createElement('div')
    box.style.cssText =
      'width:400px;max-width:90vw;background:#fff;border-radius:12px;padding:28px 24px 20px;' +
      'box-shadow:0 20px 60px rgba(0,0,0,.3);color:#1a1a2e;'
    const h = document.createElement('h4')
    h.style.cssText = 'margin:0 0 12px;font-size:17px;font-weight:600;'
    h.textContent = title
    const p = document.createElement('p')
    p.style.cssText = 'margin:0 0 24px;font-size:14px;line-height:1.6;color:#555;white-space:pre-line;'
    p.textContent = message
    const actions = document.createElement('div')
    actions.style.cssText = 'display:flex;justify-content:flex-end;gap:10px;'

    const cancelBtn = document.createElement('button')
    cancelBtn.textContent = '取消'
    cancelBtn.style.cssText =
      'padding:8px 18px;border:1px solid #d1d5db;border-radius:8px;background:#fff;' +
      'color:#374151;cursor:pointer;font-size:14px;'

    const confirmBtn = document.createElement('button')
    confirmBtn.textContent = '确定'
    confirmBtn.style.cssText =
      'padding:8px 18px;border:none;border-radius:8px;background:#4F6EF7;' +
      'color:#fff;cursor:pointer;font-size:14px;'

    let inputEl = null
    if (input) {
      inputEl = document.createElement('input')
      inputEl.type = 'text'
      inputEl.value = defaultValue
      inputEl.style.cssText =
        'width:100%;box-sizing:border-box;padding:9px 12px;margin:0 0 20px;' +
        'border:1px solid #d1d5db;border-radius:8px;font-size:14px;'
      box.appendChild(inputEl)
    }

    function cleanup() {
      overlay.remove()
    }
    function onConfirm() {
      cleanup()
      resolve(input && inputEl ? inputEl.value : true)
    }
    function onCancel() {
      cleanup()
      resolve(input ? null : false)
    }

    cancelBtn.onclick = onCancel
    confirmBtn.onclick = onConfirm
    overlay.onclick = (e) => {
      if (e.target === overlay) onCancel()
    }
    if (inputEl) {
      inputEl.onkeydown = (e) => {
        if (e.key === 'Enter') onConfirm()
        if (e.key === 'Escape') onCancel()
      }
    }

    actions.appendChild(cancelBtn)
    actions.appendChild(confirmBtn)
    box.appendChild(h)
    box.appendChild(p)
    box.appendChild(actions)
    overlay.appendChild(box)
    document.body.appendChild(overlay)
    if (inputEl) inputEl.focus()
  })
}

export function useToast() {
  // --- Toast ---
  const show = (message, type = 'info', duration = 3000) => {
    if (!toastState) {
      console.warn('[useToast] 未初始化，回退到 console')
      console.log(`[${type}] ${message}`)
      return
    }
    const id = Date.now() + Math.random()
    const item = { id, message, type, duration }
    toastState.items.value = [...toastState.items.value, item]
    if (duration > 0) {
      setTimeout(() => remove(id), duration)
    }
    return id
  }

  const remove = (id) => {
    if (!toastState) return
    toastState.items.value = toastState.items.value.filter(t => t.id !== id)
  }

  // --- Confirm (返回 Promise<boolean>) ---
  const confirm = (message, title = '确认操作') => {
    if (confirmState) {
      return new Promise((resolve) => {
        const id = Date.now() + Math.random()
        const dialog = { id, message, title, resolve }
        confirmState.dialogs.value = [...confirmState.dialogs.value, dialog]
      })
    }
    return fallbackModal({ title, message, input: false })
  }

  // --- Prompt (返回 Promise<string|null>) ---
  const prompt = (message, defaultValue = '', title = '请输入') => {
    if (promptState) {
      return new Promise((resolve) => {
        const id = Date.now() + Math.random()
        const dialog = { id, message, title, defaultValue, resolve }
        promptState.dialogs.value = [...promptState.dialogs.value, dialog]
      })
    }
    return fallbackModal({ title, message, input: true, defaultValue })
  }

  return {
    show,
    success: (msg, dur) => show(msg, 'success', dur),
    error: (msg, dur) => show(msg, 'error', dur),
    warning: (msg, dur) => show(msg, 'warning', dur),
    info: (msg, dur) => show(msg, 'info', dur),
    remove,
    confirm,
    prompt,
  }
}
