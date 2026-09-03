import { defineConfig } from 'vitepress'
import { readStableVersion, resolveChannel } from './channel'

/**
 * 通过环境变量 VITEPRESS_BASE 设置部署基础路径（二级目录）
 * 例如部署在 /magicmail 下：VITEPRESS_BASE=/magicmail npm run docs:build
 *
 * 规则：
 *   未设置或为空 → 默认 '/'（根路径部署）
 *   缺少前导 / → 自动补全（如 magicmail → /magicmail）
 *   尾部有多余 / → 自动去除（如 /magicmail/ → /magicmail）
 */
let rawBase = process.env.VITEPRESS_BASE || '/'
console.log(`[VITEPRESS_BASE] ${rawBase}`)
const base = rawBase.replace(/\/+$/, '').replace(/^([^/])/, '/$1')
console.log(`[base] ${base}`)

/**
 * 渠道判定：决定本次构建产出的是正式版（stable）还是开发版（dev）文档。
 * dev 渠道会额外带上顶部横幅、页面标题后缀与 noindex，
 * 避免尚未发布的内容被当成正式文档阅读或收录。
 *
 * 判定优先级见 ./channel.ts，可用 DOCS_CHANNEL=dev|stable 强制指定。
 */
const { channel, source } = resolveChannel()
const isDevChannel = channel === 'dev'
const stableVersion = readStableVersion()
const stableUrl = (process.env.DOCS_STABLE_URL || 'https://magicmail.160621.xyz/').replace(/\/+$/, '')
console.log(`[DOCS_CHANNEL] ${channel} (source: ${source})`)
console.log(`[DOCS_STABLE] version=${stableVersion || 'unknown'} url=${stableUrl}`)

/** dev 渠道页面标题后缀：浏览器标签页窄、站点标题被截断时也能区分 */
const DEV_TITLE_SUFFIX = '（开发版）'

/** 「更多」菜单项：dev 渠道在最前面插入一条前往正式版文档的入口 */
const moreNavItems = [
  { text: '开发指南', link: '/dev/overview' },
  { text: '配置参考', link: '/config/environment' },
  { text: 'GitHub', link: 'https://github.com/magiccode1412/magicmail' },
  { text: '官网', link: 'https://160621.xyz/magicmail' },
]
if (isDevChannel) {
  moreNavItems.unshift({ text: '查看正式版文档 →', link: stableUrl })
}

export default defineConfig({
  title: isDevChannel ? 'Magicmail 开发版' : 'Magicmail',
  description: isDevChannel
    ? '魔法邮箱开发版文档（dev 分支），可能包含尚未发布的内容'
    : '魔法邮箱 - 基于 IMAP 协议的统一邮件管理平台',
  lang: 'zh-CN',
  base,
  markdown: {
    config(md) {
      // 行内代码中的 `{{ }}` 会被 Vue 当作插值表达式编译（如 GitHub Actions 的
      // `${{ ... }}`、Go template 的 `{{.X}}`），渲染阶段直接报 “xxx is not a function”。
      // 这里统一给含 `{{` 的行内 <code> 加 v-pre，跳过编译，避免逐个文档手工转义。
      const defaultCodeInline = md.renderer.rules.code_inline
      md.renderer.rules.code_inline = (tokens, idx, options, env, self) => {
        if (tokens[idx].content.includes('{{')) tokens[idx].attrSet('v-pre', '')
        return defaultCodeInline
          ? defaultCodeInline(tokens, idx, options, env, self)
          : `<code${self.renderAttrs(tokens[idx])}>${md.utils.escapeHtml(tokens[idx].content)}</code>`
      }
    },
  },
  vite: {
    server: {
      host: '0.0.0.0',
      port: 3000,
      allowedHosts: true
    },
    define: {
      __DOCS_CHANNEL__: JSON.stringify(channel),
      __DOCS_STABLE_VERSION__: JSON.stringify(stableVersion),
      __DOCS_STABLE_URL__: JSON.stringify(stableUrl),
    },
  },

  // dev 渠道：页面标题加后缀（标签页可区分）+ 禁止搜索引擎收录未发布内容
  transformPageData(pageData) {
    if (!isDevChannel) return
    if (pageData.frontmatter?.layout === 'home') {
      pageData.title = 'Magicmail 开发版'
      pageData.description = '魔法邮箱开发版文档（dev 分支），可能包含尚未发布的内容'
      return
    }
    if (pageData.title) pageData.title = `${pageData.title}${DEV_TITLE_SUFFIX}`
  },

  transformHead({ head }) {
    if (!isDevChannel) return
    head.push(['meta', { name: 'robots', content: 'noindex,nofollow' }])
  },

  // 仅忽略开发环境的本地链接，保留对真实死链的检测能力
  ignoreDeadLinks: [/localhost:\d+/, /\d+\.\d+\.\d+\.\d+/],

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}/logo.svg` }],
    ['meta', { name: 'theme-color', content: '#646cff' }],
  ],

  themeConfig: {
    logo: `${base}/logo.svg`,

    nav: [
      { text: '指南', link: '/guide/getting-started' },
      { text: 'API', link: '/api/overview' },
      {
        text: '更多',
        items: moreNavItems,
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '开始使用',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '安装部署', link: '/guide/installation' },
            { text: '功能特性', link: '/guide/features' },
          ],
        },
        {
          text: '使用手册',
          items: [
            { text: '邮箱管理', link: '/guide/accounts' },
            { text: '邮件收发', link: '/guide/mails' },
            { text: 'Webhook 通知', link: '/guide/webhooks' },
            { text: 'PWA 客户端', link: '/guide/pwa' },
            { text: 'Outlook OAuth2 配置', link: '/guide/oauth2-microsoft' },
            { text: '特殊邮箱（Gmail / 189）', link: '/guide/特殊邮箱' },
          ],
        },
        {
          text: '版本信息',
          items: [
            { text: '更新日志', link: '/guide/changelog' },
            { text: '已知问题', link: '/guide/known-issues' },
          ],
        },
      ],
      '/api/': [
        { text: 'API 概览', link: '/api/overview' },
        { text: '认证接口', link: '/api/auth' },
        { text: '邮箱管理', link: '/api/accounts' },
        { text: '邮件管理', link: '/api/mails' },
        { text: '附件接口', link: '/api/attachments' },
        { text: 'Webhook 接口', link: '/api/webhooks' },
      ],
      '/dev/': [
        { text: '开发概览', link: '/dev/overview' },
        { text: '项目架构', link: '/dev/architecture' },
        { text: '后端开发', link: '/dev/backend' },
        { text: '前端开发', link: '/dev/frontend' },
        { text: '添加 IMAP 功能', link: '/dev/imap-extension' },
        { text: 'IMAP 诊断工具', link: '/dev/imap-diagnostics' },
        { text: '主题定制', link: '/dev/theming' },
        { text: '发布流程', link: '/dev/release' },
      ],
      '/config/': [
        { text: '环境变量', link: '/config/environment' },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/magiccode1412/magicmail' },
    ],

    footer: {
      message: isDevChannel
        ? '开发版文档（dev 分支）· 基于 AGPLv3 协议开源'
        : '基于 AGPLv3 协议开源',
      copyright: 'Copyright © 2024-present Magicmail Contributors',
    },

    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: { noResultsText: '无法找到相关结果', resetButtonTitle: '清除查询条件', footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' } },
        },
      },
    },

    outline: {
      label: '页面导航',
    },

    lastUpdated: {
      text: '最后更新于',
    },

    docFooter: {
      prev: '上一页',
      next: '下一页',
    },
  },
})
