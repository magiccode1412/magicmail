/**
 * 应用版本号（构建时由 vite.config.js 的 define 注入）
 *
 * ⚠️ 不要直接在 .vue 模板里写 __APP_VERSION__，也不要用 import.meta.env.__APP_VERSION__：
 * 1) Vite 的 define 只替换「标识符」，Vue 模板里的裸标识符会被编译成
 *    _ctx.__APP_VERSION__（成员访问），define 不替换 → 运行时为 undefined，页面显示空白；
 * 2) import.meta.env.* 只暴露 VITE_ 前缀的变量，__APP_VERSION__ 取不到 → 只会拿到 fallback。
 * 统一从这里导入，版本号只在 .js 里取值。
 */
export const APP_VERSION = __APP_VERSION__ || '0.0.0'
