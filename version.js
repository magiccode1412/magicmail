#!/usr/bin/env node
/**
 * Magicmail 版本管理脚本
 *
 * 用法:
 *   node version.js <major|minor|patch> [changelog消息] [--dry-run]
 *
 * 示例:
 *   node version.js patch "修复邮件列表加载问题"
 *   node version.js minor "新增主题切换功能"
 *   node version.js major "重构整个架构"
 *   node version.js patch                    # 不写 changelog
 *   node version.js --current               # 查看当前版本
 *   node version.js --dry-run patch "测试"  # 预览，不实际修改
 */

import { readFileSync, writeFileSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT = __dirname

// ============ 配置 ============
const PACKAGE_JSON_PATH = join(ROOT, 'web', 'package.json')
const VERSION_JSON_PATH = join(ROOT, 'version.json')        // 发布到 EdgeOne Pages 的版本文件
const CHANGELOG_PATH = join(ROOT, 'docs', 'guide', 'changelog.md')

// 项目地址配置
const PROJECT_CONFIG = {
  githubUrl: 'https://github.com/magiccode1412/magicmail',
  homepageUrl: 'https://160621.xyz/magicmail',
  versionApiUrl: 'https://api.160621.xyz/v1/magicmail/version',
}

// ============ 工具函数 ============

function parseVersion(versionStr) {
  const match = (versionStr || '').match(/^v?(\d+)\.(\d+)\.(\d+)/)
  if (!match) return null
  return {
    raw: versionStr,
    major: parseInt(match[1], 10),
    minor: parseInt(match[2], 10),
    patch: parseInt(match[3], 10),
  }
}

function formatVersion({ major, minor, patch }, prefixV = false) {
  return `${prefixV ? 'v' : ''}${major}.${minor}.${patch}`
}

function bumpVersion(parsed, type) {
  const next = { ...parsed }
  switch (type) {
    case 'major':
      next.major += 1
      next.minor = 0
      next.patch = 0
      break
    case 'minor':
      next.minor += 1
      next.patch = 0
      break
    case 'patch':
      next.patch += 1
      break
    default:
      throw new Error(`未知版本类型: ${type}，请使用 major/minor/patch`)
  }
  return next
}

function formatDate() {
  return new Date().toISOString().split('T')[0]
}

/**
 * 把 downloadUrl 归一化成带 {version} 占位符的模板。
 *
 * 背景：历史上 version.json 里存的是写死的实际 URL（.../releases/tag/v1.2.0），
 * 不含占位符，升版本时不会被替换，于是长期指向上一个版本的 Release 页
 * ——表现为用户收到「发现新版本」提示，点进去却是旧版本页面甚至 404。
 * 所以这里先把 URL 里已有的版本号还原成占位符，再统一替换成新版本号。
 */
function toDownloadUrlTemplate(downloadUrl, githubUrl) {
  const fallback = `${githubUrl || PROJECT_CONFIG.githubUrl}/releases/tag/{version}`

  if (typeof downloadUrl !== 'string' || !downloadUrl.trim()) return fallback

  // .../releases/tag/v1.2.0(-rc.1 等后缀) → .../releases/tag/{version}
  const normalized = downloadUrl
    .trim()
    .replace(/(releases\/tag\/)v?\d+\.\d+\.\d+[^\s]*/, '$1{version}')

  // 替换后仍拿不到占位符（如自定义域名）时退回默认模板，避免又留下一条写死的旧链接
  return normalized.includes('{version}') ? normalized : fallback
}

// ============ 文件操作 ============

/** 读取 package.json */
function readPackageJson() {
  if (!existsSync(PACKAGE_JSON_PATH)) {
    console.error(`❌ 找不到 ${PACKAGE_JSON_PATH}`)
    process.exit(1)
  }
  return JSON.parse(readFileSync(PACKAGE_JSON_PATH, 'utf-8'))
}

/** 写入 package.json */
function writePackageJson(data) {
  writeFileSync(PACKAGE_JSON_PATH, JSON.stringify(data, null, 2) + '\n', 'utf-8')
  console.log(`✅ 已更新 package.json → ${data.version}`)
}

/** 读取或初始化 version.json（EdgeOne Pages 版本检查用） */
function readVersionJson() {
  if (existsSync(VERSION_JSON_PATH)) {
    return JSON.parse(readFileSync(VERSION_JSON_PATH, 'utf-8'))
  }
  // 初始化空结构（使用项目默认地址）
  return {
    latest: '',
    releaseDate: '',
    changelog: {},
    downloadUrl: `${PROJECT_CONFIG.githubUrl}/releases/tag/{version}`,
    githubUrl: PROJECT_CONFIG.githubUrl,
    homepageUrl: PROJECT_CONFIG.homepageUrl,
    versionApiUrl: PROJECT_CONFIG.versionApiUrl,
  }
}

/** 写入 version.json */
function writeVersionJson(data) {
  writeFileSync(VERSION_JSON_PATH, JSON.stringify(data, null, 2) + '\n', 'utf-8')
  console.log(`✅ 已更新 version.json → ${data.latest}`)
}

/** 更新 CHANGELOG.md */
function updateChangelog(newVersion, changelogMessage) {
  const date = formatDate()
  const entry = `## [${newVersion}] - ${date}\n\n${changelogMessage ? '- ' + changelogMessage + '\n' : ''}\n`
  let content = ''

  if (existsSync(CHANGELOG_PATH)) {
    content = readFileSync(CHANGELOG_PATH, 'utf-8')
  } else {
    content = '# Changelog\n\n所有重要变更都会记录在此文件中。\n\n'
  }

  // 在第一个 ## 版本条目前面插入新条目
  const firstEntryIndex = content.indexOf('\n## ')
  if (firstEntryIndex !== -1) {
    content = content.slice(0, firstEntryIndex) + entry + content.slice(firstEntryIndex)
  } else {
    content += '\n' + entry
  }

  writeFileSync(CHANGELOG_PATH, content, 'utf-8')
  console.log(`✅ 已更新 CHANGELOG.md`)
}

/**
 * 自动从 git remote 获取 GitHub URL
 * 用于填充 version.json 的 githubUrl 和 downloadUrl
 */
function detectGitHubUrls() {
  try {
    // 尝试读取 .git/config 获取 remote url
    const gitConfigPath = join(ROOT, '.git', 'config')
    if (!existsSync(gitConfigPath)) return {}

    const gitConfig = readFileSync(gitConfigPath, 'utf-8')
    const match = gitConfig.match(/\s+url\s*=\s*(.+?)(?:\r?\n|$)/)
    if (!match || !match[1]) return {}

    let remoteUrl = match[1].trim()
    // https -> github.com/owner/repo
    // git@github.com:owner/repo.git
    const httpsMatch = remoteUrl.match(/github\.com\/([^/]+\/[^/.]+?)(?:\.git)?$/)
    const sshMatch = remoteUrl.match(/github\.com:([^/]+\/[^/.]+?)(?:\.git)?$/)
    const repoMatch = httpsMatch || sshMatch

    if (repoMatch) {
      const repo = repoMatch[1]
      return {
        githubUrl: `https://github.com/${repo}`,
        downloadUrl: `https://github.com/${repo}/releases/tag/{version}`,
      }
    }
  } catch {
    // 忽略错误
  }
  return {}
}

// ============ 主逻辑 ============

function main() {
  const args = process.argv.slice(2)

  // --current: 只显示当前版本
  if (args.includes('--current')) {
    const pkg = readPackageJson()
    const v = parseVersion(pkg.version)
    if (!v) {
      console.error('❌ 无法解析当前版本号')
      process.exit(1)
    }
    console.log(formatVersion(v, true))
    return
  }

  // 检查 dry-run 标志
  const dryRunIdx = args.indexOf('--dry-run')
  const dryRun = dryRunIdx !== -1
  if (dryRun) args.splice(dryRunIdx, 1)

  // 解析参数: [type] [changelog...]
  const type = args[0]
  const changelogMessage = args.slice(1).join(' ').trim()

  if (!type || !['major', 'minor', 'patch'].includes(type)) {
    console.log(`
📦 Magicmail 版本管理工具

用法:
  node version.js <major|minor|patch> [changelog消息] [选项]
  node version.js --current          查看当前版本
  node version.js --help             显示帮助信息

选项:
  --dry-run                          预览变更，不写入文件

示例:
  node version.js patch "修复登录问题"
  node version.js minor "新增暗色主题"
  node version.js major "v2.0 重构发布"
`)
    return
  }

  // 读取当前版本
  const pkg = readPackageJson()
  const current = parseVersion(pkg.version)
  if (!current) {
    console.error(`❌ 无法解析当前版本号: ${pkg.version}`)
    process.exit(1)
  }

  // 计算新版本
  const next = bumpVersion(current, type)
  const newVersion = formatVersion(next)
  const newVersionTag = formatVersion(next, true)

  console.log('')
  console.log(`━━━ 版本更新 ━━━`)
  console.log(`  当前版本:  ${formatVersion(current, true)}`)
  console.log(`  目标版本:  ${newVersionTag} (${type})`)
  if (changelogMessage) console.log(`  更新日志:  ${changelogMessage}`)
  console.log('')

  if (dryRun) {
    console.log('⏭️  [DRY RUN] 以下文件将被修改（未实际执行）：')
    console.log('')
    console.log(`  📄 ${PACKAGE_JSON_PATH}`)
    console.log(`     "version": "${pkg.version}" → "${newVersion}"`)
    console.log('')
    console.log(`  📄 ${VERSION_JSON_PATH}`)
    console.log(`     "latest": "${readVersionJson().latest || '(新建)'}" → "${newVersionTag}"`)
    if (changelogMessage) {
      console.log('')
      console.log(`  📄 ${CHANGELOG_PATH}`)
      console.log(`     追加 [${newVersionTag}] - ${formatDate()} 条目`)
    }
    console.log('')
    return
  }

  // ====== 写入 package.json ======
  pkg.version = newVersion
  writePackageJson(pkg)

  // ====== 写入/更新 version.json ======
  const vJson = readVersionJson()
  vJson.latest = newVersionTag
  vJson.releaseDate = formatDate()

  // 自动检测 GitHub URLs（如果尚未配置）
  const detected = detectGitHubUrls()
  if (!vJson.githubUrl && detected.githubUrl) {
    vJson.githubUrl = detected.githubUrl
    console.log(`🔗 自动检测 GitHub 仓库: ${detected.githubUrl}`)
  }
  // 使用硬编码的默认值作为兜底
  if (!vJson.githubUrl) {
    vJson.githubUrl = PROJECT_CONFIG.githubUrl
  }
  if (!vJson.homepageUrl) {
    vJson.homepageUrl = PROJECT_CONFIG.homepageUrl
  }
  if (!vJson.versionApiUrl) {
    vJson.versionApiUrl = PROJECT_CONFIG.versionApiUrl
  }
  // downloadUrl 始终按模板重建：即便存量 version.json 里是写死的旧链接也能纠正
  vJson.downloadUrl = toDownloadUrlTemplate(
    vJson.downloadUrl || detected.downloadUrl,
    vJson.githubUrl
  ).replace('{version}', newVersionTag)
  console.log(`🔗 downloadUrl → ${vJson.downloadUrl}`)

  writeVersionJson(vJson)

  // ====== 更新 CHANGELOG.md ======
  if (changelogMessage) {
    updateChangelog(newVersionTag, changelogMessage)
  }

  console.log('')
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
  console.log(`🎉 版本已升至 ${newVersionTag}`)
  console.log('')
  console.log('后续操作:')
  console.log(`  1. git add -A && git commit -m "release: ${newVersionTag}${changelogMessage ? ' - ' + changelogMessage : ''}"`)
  console.log(`  2. git tag ${newVersionTag}`)
  console.log(`  3. git push && git push --tags`)
  console.log(`  4. 将 version.json 部署到 EdgeOne Pages`)
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━')
  console.log('')
}

main()
