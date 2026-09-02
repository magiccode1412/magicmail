#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const https = require('https');
const jsYaml = require('js-yaml');
const Ajv = require('ajv').default;
const addFormats = require('ajv-formats');

const CNB_WEB_PROTOCOL = process.env.CNB_WEB_PROTOCOL || 'https';
const CNB_WEB_HOST = process.env.CNB_WEB_HOST || 'cnb.cool';
const CACHE_TTL_MS = 24 * 60 * 60 * 1000; // 24h

// ─── CLI args ─────────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
const filePath = args.find(a => !a.startsWith('-'));
const forceRefresh = args.includes('--refresh');

if (!filePath || args.includes('-h') || args.includes('--help')) {
  console.log(`
CNB Pipeline Config Validator

Usage:
  node validate.js <yml-file-path> [options]

支持的文件：
  .cnb.yml              流水线配置（YAML 语法 + 语义校验 + Schema）
  .cnb/web_trigger.yml  Web 触发器配置（YAML 语法 + 目录约束 + Schema）

Options:
  --refresh   Force re-download the Schema (ignore cache)
  -h, --help  Show this help

Examples:
  node validate.js .cnb.yml
  node validate.js .cnb/web_trigger.yml
  node validate.js /path/to/.cnb.yml --refresh
`);
  process.exit(filePath ? 0 : 1);
}

// ─── File type detection ──────────────────────────────────────────────────────
// 按文件名自动识别校验类型：basename 匹配 web_trigger.yml(.yaml) 走 web-trigger schema，
// 其余一律按 .cnb.yml 流水线配置处理。
const isWebTrigger = /(^|[._-])web_trigger\.ya?ml$/i.test(path.basename(filePath));
const SCHEMA_URL = isWebTrigger
  ? `${CNB_WEB_PROTOCOL}://docs.${CNB_WEB_HOST}/web-trigger-schema-zh.json`
  : `${CNB_WEB_PROTOCOL}://docs.${CNB_WEB_HOST}/conf-schema-zh.json`;
const SCHEMA_CACHE = path.join(__dirname,
  isWebTrigger ? '.web-trigger-schema-cache.json' : '.schema-cache.json');

// ─── Step 1: YAML parse ───────────────────────────────────────────────────────

// !reference 是 CNB 扩展的自定义 YAML 标签，语法为：
//   !reference [key, path, ...]
// js-yaml 默认不识别自定义标签，需注册 Type 让解析器接受并保留原始值。
// 解析后用 { $ref: [...] } 占位，Schema 校验阶段对含有 $ref 的字段跳过类型检查。
const referenceType = new jsYaml.Type('!reference', {
  kind: 'sequence',
  construct: (data) => ({ $ref: data }),
});
const CNB_SCHEMA = jsYaml.DEFAULT_SCHEMA.extend([referenceType]);

// web_trigger.yml 为可选配置，仓库未配置时直接跳过校验（视为通过）。
// 注意：.cnb.yml 不在此跳过——其缺失仍会在下方读取时报错。
if (isWebTrigger && !fs.existsSync(path.resolve(filePath))) {
  console.log(`ℹ️  未找到 ${filePath}，仓库未配置 web_trigger.yml，跳过校验`);
  process.exit(0);
}

let doc;
try {
  const raw = fs.readFileSync(path.resolve(filePath), 'utf8');
  doc = jsYaml.load(raw, { schema: CNB_SCHEMA });
  console.log('✅ YAML 语法校验通过');
} catch (e) {
  console.error('❌ YAML 语法错误:', e.message);
  process.exit(1);
}

// ─── Step 1.1: web_trigger 目录约束校验 ────────────────────────────────────────

// web_trigger.yml 必须位于仓库的 .cnb/ 目录下（即 .cnb/web_trigger.yml）才会被平台识别。
// Schema 不校验文件位置，这里按父目录名做一次约束检查。
if (isWebTrigger) {
  const parentDir = path.basename(path.dirname(path.resolve(filePath)));
  if (parentDir !== '.cnb') {
    console.error('\n❌ 目录约束校验失败:');
    console.error(`  web_trigger.yml 必须放在仓库的 .cnb/ 目录下（即 .cnb/web_trigger.yml），当前位于 "${parentDir}/" 目录，平台将无法识别`);
    process.exit(1);
  }
  console.log('✅ 目录约束校验通过');
}

// ─── Step 1.5: Semantic validation ────────────────────────────────────────────

const semanticErrors = [];
const semanticWarnings = [];

// ── 1) Branch-event placement rules ──

// 仓库级事件只能放在 "$" 兜底分支下，不能放在具体分支名下。
const REPO_LEVEL_EVENT_PREFIXES = ['issue.', 'tag_deploy.'];
const REPO_LEVEL_EVENT_EXACT = new Set(['tag_push', 'auto_tag', 'vscode']);

// crontab 事件必须放在具体分支名下，不能放在 $ 兜底分支或通配符分支下
const CRONTAB_EVENT_PREFIXES = ['crontab:', 'crontab '];

// web_trigger / api_trigger 建议放在 $ 兜底分支下（仅 warning，不强制）
const TRIGGER_EVENT_PREFIXES = ['web_trigger', 'api_trigger'];

function getBaseEvent(eventKey) {
  return eventKey.split('@')[0];
}

function isRepoLevelEvent(eventKey) {
  const baseEvent = getBaseEvent(eventKey);
  if (REPO_LEVEL_EVENT_EXACT.has(baseEvent)) return true;
  return REPO_LEVEL_EVENT_PREFIXES.some(prefix => baseEvent.startsWith(prefix));
}

function isCrontabEvent(eventKey) {
  const baseEvent = getBaseEvent(eventKey);
  return CRONTAB_EVENT_PREFIXES.some(prefix => baseEvent.startsWith(prefix));
}

function isTriggerEvent(eventKey) {
  const baseEvent = getBaseEvent(eventKey);
  return TRIGGER_EVENT_PREFIXES.some(prefix => baseEvent.startsWith(prefix));
}

// 判断分支 key 是否为通配符/glob 模式（包含 *、**、!、| 等特殊字符）
function isWildcardBranch(branchKey) {
  return /[*?!|()\[\]]/.test(branchKey);
}

// ── 2) CNB_ built-in environment variables ──

// All 82 official CNB_ prefixed built-in environment variables
const CNB_BUILTIN_ENV_VARS = new Set([
  // 基础变量
  'CNB',
  'CNB_WEB_PROTOCOL',
  'CNB_WEB_HOST',
  'CNB_WEB_ENDPOINT',
  'CNB_API_ENDPOINT',
  'CNB_GROUP_SLUG',
  'CNB_GROUP_SLUG_LOWERCASE',
  'CNB_GROUP_ID',
  'CNB_ROOT_SLUG',
  'CNB_ROOT_SLUG_LOWERCASE',
  'CNB_ROOT_ID',
  'CNB_EVENT',
  'CNB_EVENT_URL',
  'CNB_BRANCH',
  'CNB_BRANCH_SHA',
  'CNB_DEFAULT_BRANCH',
  'CNB_TOKEN_USER_NAME',
  'CNB_TOKEN',
  'CNB_IS_CRONEVENT',
  'CNB_DOCKER_REGISTRY',
  'CNB_DOCKER_MODEL_REGISTRY',
  'CNB_HELM_REGISTRY',
  'CNB_HAS_LFS_FILES',
  // 提交类变量
  'CNB_BEFORE_SHA',
  'CNB_COMMIT',
  'CNB_COMMIT_SHORT',
  'CNB_COMMIT_MESSAGE',
  'CNB_COMMIT_MESSAGE_TITLE',
  'CNB_COMMITTER',
  'CNB_COMMITTER_EMAIL',
  'CNB_NEW_COMMITS_COUNT',
  'CNB_IS_TAG',
  'CNB_TAG_MESSAGE',
  'CNB_TAG_RELEASE_TITLE',
  'CNB_TAG_RELEASE_DESC',
  'CNB_TAG_IS_RELEASE',
  'CNB_TAG_IS_PRE_RELEASE',
  'CNB_IS_NEW_BRANCH',
  'CNB_IS_NEW_BRANCH_WITH_UPDATE',
  // 仓库类变量
  'CNB_REPO_SLUG',
  'CNB_REPO_SLUG_LOWERCASE',
  'CNB_REPO_NAME',
  'CNB_REPO_NAME_LOWERCASE',
  'CNB_REPO_ID',
  'CNB_REPO_URL_HTTPS',
  'CNB_FORK_FROM_REPO_SLUG',
  // 构建类变量
  'CNB_BUILD_ID',
  'CNB_BUILD_WEB_URL',
  'CNB_BUILD_START_TIME',
  'CNB_BUILD_USER',
  'CNB_BUILD_USER_NICKNAME',
  'CNB_BUILD_USER_EMAIL',
  'CNB_BUILD_USER_ID',
  'CNB_BUILD_USER_NPC_SLUG',
  'CNB_BUILD_USER_NPC_NAME',
  'CNB_BUILD_STAGE_NAME',
  'CNB_BUILD_JOB_NAME',
  'CNB_BUILD_JOB_KEY',
  'CNB_BUILD_WORKSPACE',
  'CNB_BUILD_FAILED_MSG',
  'CNB_BUILD_FAILED_STAGE_NAME',
  'CNB_PIPELINE_NAME',
  'CNB_PIPELINE_KEY',
  'CNB_PIPELINE_ID',
  'CNB_PIPELINE_DOCKER_IMAGE',
  'CNB_PIPELINE_STATUS',
  'CNB_PIPELINE_MAX_RUN_TIME',
  'CNB_RUNNER_IP',
  'CNB_CPUS',
  'CNB_MEMORY',
  'CNB_IS_RETRY',
  // 合并类变量
  'CNB_PULL_REQUEST',
  'CNB_PULL_REQUEST_LIKE',
  'CNB_PULL_REQUEST_PROPOSER',
  'CNB_PULL_REQUEST_TITLE',
  'CNB_PULL_REQUEST_DESCRIPTION',
  'CNB_PULL_REQUEST_BRANCH',
  'CNB_PULL_REQUEST_SHA',
  'CNB_PULL_REQUEST_TARGET_SHA',
  'CNB_PULL_REQUEST_MERGE_SHA',
  'CNB_PULL_REQUEST_SLUG',
  'CNB_PULL_REQUEST_ACTION',
  'CNB_PULL_REQUEST_ID',
  'CNB_PULL_REQUEST_IID',
  'CNB_PULL_REQUEST_REVIEWERS',
  'CNB_PULL_REQUEST_REVIEW_STATE',
  'CNB_REVIEW_REVIEWED_BY',
  'CNB_REVIEW_LAST_REVIEWED_BY',
  'CNB_PULL_REQUEST_IS_WIP',
  // 云原生开发类变量
  'CNB_VSCODE_WEB_URL',
  'CNB_VSCODE_MAX_RUN_TIME',
  // Issue 类变量
  'CNB_ISSUE_ID',
  'CNB_ISSUE_IID',
  'CNB_ISSUE_TITLE',
  'CNB_ISSUE_DESCRIPTION',
  'CNB_ISSUE_OWNER',
  'CNB_ISSUE_STATE',
  'CNB_ISSUE_IS_RESOLVED',
  'CNB_ISSUE_ASSIGNEES',
  'CNB_ISSUE_LABELS',
  'CNB_ISSUE_PRIORITY',
  // 评论类变量
  'CNB_COMMENT_ID',
  'CNB_COMMENT_BODY',
  'CNB_COMMENT_TYPE',
  'CNB_COMMENT_FILE_PATH',
  'CNB_COMMENT_RANGE',
  'CNB_REVIEW_ID',
  'CNB_REVIEW_DESCRIPTION',
  // NPC 类变量
  'CNB_NPC_SLUG',
  'CNB_NPC_SLUG_LOWERCASE',
  'CNB_NPC_NAME',
  'CNB_NPC_SHA',
  'CNB_NPC_PROMPT',
  'CNB_NPC_SLOGAN',
  'CNB_NPC_AVATAR',
  'CNB_NPC_ENABLE_THINKING',
  'CNB_NPC_ENABLE_WORKMODE',
  'CNB_NPC_TRIGGER_CONTENT',
]);

// Extract $VAR or ${VAR} references from a string
function extractVarRefs(str) {
  if (typeof str !== 'string') return [];
  const refs = [];
  // Match $VAR and ${VAR}, but not \$VAR
  const regex = /(?<!\\)\$\{([A-Za-z_][A-Za-z0-9_]*)\}|(?<!\\)\$([A-Za-z_][A-Za-z0-9_]*)/g;
  let match;
  while ((match = regex.exec(str)) !== null) {
    refs.push(match[1] || match[2]);
  }
  return refs;
}

// Recursively find all CNB_ variable references that are NOT in the built-in list
function findInvalidCNBVars(node, path = '') {
  if (node === null || node === undefined) return;
  if (typeof node === 'string') {
    const refs = extractVarRefs(node);
    for (const ref of refs) {
      if (ref.startsWith('CNB_') && !CNB_BUILTIN_ENV_VARS.has(ref)) {
        semanticErrors.push(
          `${path || '(root)'}: 使用了非内置的 CNB_ 前缀变量 "$${ref}"。所有 CNB_ 开头的变量为系统内置只读变量，请检查拼写或使用其他前缀`
        );
      }
    }
    return;
  }
  if (Array.isArray(node)) {
    node.forEach((item, i) => findInvalidCNBVars(item, `${path}[${i}]`));
    return;
  }
  if (typeof node === 'object') {
    for (const [k, v] of Object.entries(node)) {
      // Skip keys that are structural (branch names, event names, etc.) — only check values
      // Also skip the 'env' key's own keys (those are variable definitions, not references)
      if (k === 'env' && typeof v === 'object' && v !== null && !Array.isArray(v)) {
        // Check env values for CNB_ references, and also check env KEY names
        for (const [envKey, envVal] of Object.entries(v)) {
          if (envKey.startsWith('CNB_') && !CNB_BUILTIN_ENV_VARS.has(envKey)) {
            semanticErrors.push(
              `${path}.env.${envKey}: 自定义了非内置的 CNB_ 前缀变量 "${envKey}"。所有 CNB_ 开头的变量为系统内置只读变量，不允许覆盖或定义同名变量，请使用其他前缀`
            );
          }
          findInvalidCNBVars(envVal, `${path}.env.${envKey}`);
        }
      } else {
        findInvalidCNBVars(v, path ? `${path}.${k}` : k);
      }
    }
  }
}

// ── 3) Built-in task event support validation ──

// Map of built-in task types to their supported events
// "ALL" means all events are supported
const BUILTIN_TASK_EVENTS = {
  'docker:cache': 'ALL',
  'cnb:await': 'ALL',
  'cnb:resolve': 'ALL',
  'cnb:apply': ['push', 'commit.add', 'branch.create', 'pull_request.target', 'pull_request.mergeable', 'tag_push', 'pull_request.merged', 'api_trigger', 'web_trigger', 'crontab', 'tag_deploy'],
  'cnb:trigger': 'ALL',
  'cnb:read-file': 'ALL',
  'cnb:destroy-token': 'ALL',
  'vscode:go': ['vscode', 'branch.create', 'api_trigger', 'web_trigger'],
  'git:auto-merge': ['pull_request.mergeable'],
  'git:reviewer': ['pull_request', 'pull_request.target', 'pull_request.update'],
  'git:issue-update': 'ALL',
  'git:release': ['push', 'commit.add', 'branch.create', 'tag_push', 'pull_request.merged', 'api_trigger', 'web_trigger', 'tag_deploy'],
  'git:pr-update': 'ALL',
  'git:pr-commit-message-preset': ['pull_request', 'pull_request.target', 'pull_request.update', 'pull_request.mergeable', 'pull_request.approved', 'pull_request.changes_requested'],
  'testing:coverage': 'ALL',
  'artifact:remove-tag': ['push', 'commit.add', 'tag_push', 'tag_deploy', 'pull_request.merged', 'api_trigger', 'web_trigger', 'crontab', 'branch.create'],
  'tapd:status-update': 'ALL',
  'tapd:comment': 'ALL',
  'knowledge:update': 'ALL',
  'npc:go': 'ALL',
};

// Get the effective event key, stripping NPC @suffix
function getEffectiveEvent(eventKey) {
  return eventKey.split('@')[0];
}

// Check if an event matches a supported event pattern
// Handles crontab events, web_trigger_* prefixes, api_trigger_* prefixes, etc.
function isEventSupported(actualEvent, supportedEvents) {
  if (supportedEvents === 'ALL') return true;
  if (supportedEvents.includes(actualEvent)) return true;

  // Handle prefixed events: web_trigger_* matches web_trigger, api_trigger_* matches api_trigger
  if (actualEvent.startsWith('web_trigger')) {
    return supportedEvents.includes('web_trigger');
  }
  if (actualEvent.startsWith('api_trigger')) {
    return supportedEvents.includes('api_trigger');
  }
  if (actualEvent.startsWith('tag_deploy.')) {
    return supportedEvents.includes('tag_deploy');
  }
  // crontab events
  if (actualEvent.startsWith('crontab:') || actualEvent.startsWith('crontab ')) {
    return supportedEvents.includes('crontab');
  }

  return false;
}

// Recursively find all built-in task usages and validate their event context
function findBuiltinTaskEventMismatches(node, currentBranch, currentEvent, path = '') {
  if (node === null || node === undefined) return;
  if (typeof node !== 'object' || Array.isArray(node)) {
    if (Array.isArray(node)) {
      node.forEach((item, i) => findBuiltinTaskEventMismatches(item, currentBranch, currentEvent, `${path}[${i}]`));
    }
    return;
  }

  // Check if this node is a job with a 'type' field (built-in task)
  const taskType = node.type;
  if (typeof taskType === 'string' && BUILTIN_TASK_EVENTS[taskType]) {
    const supportedEvents = BUILTIN_TASK_EVENTS[taskType];
    const effectiveEvent = getEffectiveEvent(currentEvent);
    if (!isEventSupported(effectiveEvent, supportedEvents)) {
      const supportedDesc = supportedEvents === 'ALL' ? '所有事件' : supportedEvents.join(', ');
      semanticErrors.push(
        `${path || '(root)'}: 内置任务 "${taskType}" 不支持在 "${currentEvent}" 事件下使用。支持的事件: ${supportedDesc}`
      );
    }
  }

  // Recurse into child structures
  for (const [k, v] of Object.entries(node)) {
    if (v === null || v === undefined) continue;
    findBuiltinTaskEventMismatches(v, currentBranch, currentEvent, path ? `${path}.${k}` : k);
  }
}

// ── 4) System dependency install detection ──

// Patterns for detecting manual system package installation in scripts
const APT_INSTALL_PATTERN = /\bapt(-get)?\s+install\b/;
const YUM_INSTALL_PATTERN = /\byum\s+install\b/;
const APK_ADD_PATTERN = /\bapk\s+add\b/;
const DNF_INSTALL_PATTERN = /\bdnf\s+install\b/;
const PACMAN_INSTALL_PATTERN = /\bpacman\s+-S\b/;
const EMERGE_PATTERN = /\bemerge\b/;

function detectSystemDepsInstall(scriptStr) {
  if (typeof scriptStr !== 'string') return null;
  // Skip if it's just a comment line
  const lines = scriptStr.split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    if (APT_INSTALL_PATTERN.test(trimmed)) return 'apt install';
    if (YUM_INSTALL_PATTERN.test(trimmed)) return 'yum install';
    if (APK_ADD_PATTERN.test(trimmed)) return 'apk add';
    if (DNF_INSTALL_PATTERN.test(trimmed)) return 'dnf install';
    if (PACMAN_INSTALL_PATTERN.test(trimmed)) return 'pacman -S';
    if (EMERGE_PATTERN.test(trimmed)) return 'emerge';
  }
  return null;
}

// Recursively check scripts for system dependency installation
function findSystemDepsInstall(node, path = '', hasDockerBuild = false) {
  if (node === null || node === undefined) return;
  if (typeof node !== 'object' || Array.isArray(node)) {
    if (Array.isArray(node)) {
      node.forEach((item, i) => findSystemDepsInstall(item, `${path}[${i}]`, hasDockerBuild));
    }
    return;
  }

  // Check if pipeline level has docker.build (custom Dockerfile)
  let localHasDockerBuild = hasDockerBuild;
  if (node.docker && typeof node.docker === 'object' && node.docker.build) {
    localHasDockerBuild = true;
  }

  // Check script/commands fields
  const scriptFields = ['script', 'commands'];
  for (const field of scriptFields) {
    if (node[field]) {
      const scripts = Array.isArray(node[field]) ? node[field] : [node[field]];
      for (const s of scripts) {
        const detected = detectSystemDepsInstall(s);
        if (detected && !localHasDockerBuild) {
          semanticWarnings.push(
            `${path}.${field}: 检测到使用 "${detected}" 手动安装系统依赖。建议通过指定自定义构建环境 Dockerfile（docker.build）的方式预装依赖，避免每次流水线执行时重复安装。参考: https://docs.cnb.cool/zh/build/build-env.md`
          );
        }
      }
    }
  }

  // Recurse into child structures
  for (const [k, v] of Object.entries(node)) {
    if (v === null || v === undefined) continue;
    if (k === 'script' || k === 'commands') continue; // already checked above
    findSystemDepsInstall(v, path ? `${path}.${k}` : k, localHasDockerBuild);
  }
}

// ── Run all semantic validations ──
// web_trigger.yml 结构为 { branch: [...] }，与 .cnb.yml 的「分支→事件」结构完全不同，
// 以下语义规则均不适用，整体跳过，仅做 YAML 语法 + 目录约束 + Schema 校验。
if (!isWebTrigger) {
if (doc && typeof doc === 'object' && !Array.isArray(doc)) {
  for (const [branchKey, branchValue] of Object.entries(doc)) {
    // 跳过非分支 key：include 声明、以 . 开头的 YAML 锚点定义
    if (branchKey === 'include' || branchKey.startsWith('.')) continue;

    const isDollar = branchKey === '$';
    const isWildcard = isWildcardBranch(branchKey);

    if (branchValue && typeof branchValue === 'object' && !Array.isArray(branchValue)) {
      for (const [eventKey, eventValue] of Object.entries(branchValue)) {
        // Branch-event placement rules
        if (isRepoLevelEvent(eventKey) && !isDollar) {
          semanticErrors.push(
            `分支 "${branchKey}" 下不允许使用仓库级事件 "${eventKey}"，请将其移至 "$" 兜底分支下`
          );
        }
        if (isCrontabEvent(eventKey) && (isDollar || isWildcard)) {
          semanticErrors.push(
            `${isDollar ? '"$" 兜底分支' : `通配符分支 "${branchKey}"`} 下不允许使用 crontab 事件 "${eventKey}"，请将其移至具体的分支名下`
          );
        }
        if (isTriggerEvent(eventKey) && !isDollar) {
          semanticWarnings.push(
            `分支 "${branchKey}" 下的 "${eventKey}" 建议移至 "$" 兜底分支下`
          );
        }

        // Built-in task event support validation
        if (Array.isArray(eventValue)) {
          eventValue.forEach((pipeline, i) => {
            findBuiltinTaskEventMismatches(pipeline, branchKey, eventKey, `${branchKey}.${eventKey}[${i}]`);
          });
        }

        // System dependency install detection
        if (Array.isArray(eventValue)) {
          eventValue.forEach((pipeline, i) => {
            const pipelineHasDockerBuild = pipeline && pipeline.docker && pipeline.docker.build;
            findSystemDepsInstall(pipeline, `${branchKey}.${eventKey}[${i}]`, !!pipelineHasDockerBuild);
          });
        }
      }
    }
  }

  // CNB_ env var validation (doc-level scan)
  findInvalidCNBVars(doc);
}

if (semanticWarnings.length > 0) {
  console.warn('\n⚠️  语义校验警告:');
  semanticWarnings.forEach(w => console.warn(' ', w));
}

if (semanticErrors.length > 0) {
  console.error('\n❌ 语义校验失败:');
  semanticErrors.forEach(e => console.error(' ', e));
  process.exit(1);
}
console.log('✅ 语义校验通过');
}

// ─── Step 2: load schema (cache-first) ────────────────────────────────────────

function loadCachedSchema() {
  if (forceRefresh) return null;
  try {
    const stat = fs.statSync(SCHEMA_CACHE);
    if (Date.now() - stat.mtimeMs < CACHE_TTL_MS) {
      return JSON.parse(fs.readFileSync(SCHEMA_CACHE, 'utf8'));
    }
  } catch (_) {}
  return null;
}

function fetchSchema(url, redirectCount = 0) {
  return new Promise((resolve, reject) => {
    if (redirectCount > 5) {
      reject(new Error('重定向次数过多'));
      return;
    }
    https.get(url, (res) => {
      // 跟随重定向
      if (res.statusCode === 301 || res.statusCode === 302 || res.statusCode === 307 || res.statusCode === 308) {
        const location = res.headers['location'];
        if (!location) {
          reject(new Error(`HTTP ${res.statusCode} 但无 Location 头`));
          return;
        }
        res.resume();
        resolve(fetchSchema(location, redirectCount + 1));
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`HTTP ${res.statusCode}`));
        return;
      }
      let data = '';
      res.on('data', c => data += c);
      res.on('end', () => {
        try {
          const schema = JSON.parse(data);
          fs.writeFileSync(SCHEMA_CACHE, data, 'utf8');
          resolve(schema);
        } catch (e) {
          reject(new Error('Schema JSON 解析失败: ' + e.message));
        }
      });
    }).on('error', reject);
  });
}

// ─── Step 3: validate ─────────────────────────────────────────────────────────

// 递归将文档中所有 !reference 占位符（{ $ref: [...] }）替换为占位字符串，
// 同时记录被替换节点的 JSON Pointer 路径，校验时跳过这些路径及其祖先路径上的错误。
const refPaths = new Set();

// RFC 6901 JSON Pointer: key 中的 ~ 转义为 ~0，/ 转义为 ~1
function escapeJsonPointer(key) {
  return key.replace(/~/g, '~0').replace(/\//g, '~1');
}

// 占位字符串：对于期望 string 的字段（如 options.key）能通过类型检查，
// 对于期望 object/array 的字段通过路径过滤处理。
const REF_PLACEHOLDER = '__CNB_REFERENCE__';

function resolveReferences(node, pointer = '') {
  if (node === null || typeof node !== 'object') return node;
  if (Array.isArray(node)) {
    const _REF_SENTINEL = Symbol('ref');
    return node
      .map((item, i) => {
        if (item && typeof item === 'object' && !Array.isArray(item) &&
            Object.prototype.hasOwnProperty.call(item, '$ref') && Array.isArray(item.$ref)) {
          return _REF_SENTINEL;
        }
        return resolveReferences(item, `${pointer}/${i}`);
      })
      .filter(item => item !== _REF_SENTINEL);
  }
  if (Object.prototype.hasOwnProperty.call(node, '$ref') && Array.isArray(node.$ref)) {
    refPaths.add(pointer);
    return REF_PLACEHOLDER;
  }
  const result = {};
  for (const [k, v] of Object.entries(node)) {
    result[k] = resolveReferences(v, `${pointer}/${escapeJsonPointer(k)}`);
  }
  return result;
}

// 检查一个错误路径是否与任何 !reference 路径相关：
// - 错误在 !reference 路径上或其子路径
// - 错误在 !reference 的祖先路径（如 options 级别的错误，而 options.key 是 !reference）
function isRefRelated(errorPath) {
  for (const rp of refPaths) {
    if (errorPath === rp || errorPath.startsWith(rp + '/')) return true;
    if (rp.startsWith(errorPath + '/')) return true;
  }
  return false;
}

function formatError(e) {
  const loc = e.instancePath || '(root)';
  switch (e.keyword) {
    case 'additionalProperties':
      return `${loc}: 不允许的字段 "${e.params.additionalProperty}"`;
    case 'required':
      return `${loc}: 缺少必填字段 "${e.params.missingProperty}"`;
    case 'enum':
      return `${loc}: 值必须是以下之一: ${JSON.stringify(e.params.allowedValues)}`;
    case 'type':
      return `${loc}: 类型错误，期望 ${e.params.type}`;
    case 'minItems':
      return `${loc}: 数组至少需要 ${e.params.limit} 个元素`;
    case 'minLength':
      return `${loc}: 字符串不能为空`;
    case 'pattern':
      return `${loc}: 格式不符合要求 (${e.params.pattern})`;
    default:
      return `${loc}: ${e.message}`;
  }
}

function validate(schema) {
  const ajv = new Ajv({ strict: false, allErrors: true });
  addFormats(ajv);
  const validateFn = ajv.compile(schema);
  const valid = validateFn(resolveReferences(doc));

  if (valid) {
    console.log('✅ Schema 校验通过');
    return;
  }

  const allErrors = validateFn.errors || [];

  // 过滤掉 oneOf/anyOf/if/else 的容器级噪音，只保留具体字段错误；
  // 同时跳过来自 !reference 占位路径及其祖先路径的误报
  const useful = allErrors.filter(e =>
    e.keyword !== 'oneOf' &&
    e.keyword !== 'anyOf' &&
    e.keyword !== 'if' &&
    e.keyword !== 'else' &&
    !isRefRelated(e.instancePath)
  );

  // 去重
  const seen = new Set();
  const deduped = useful.filter(e => {
    const key = `${e.instancePath}|${e.keyword}|${JSON.stringify(e.params)}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });

  if (deduped.length === 0) {
    console.log('✅ Schema 校验通过');
    return;
  }

  // 按路径深度降序排列，优先显示最深（最具体）的错误
  deduped.sort((a, b) => {
    const depthA = (a.instancePath.match(/\//g) || []).length;
    const depthB = (b.instancePath.match(/\//g) || []).length;
    return depthB - depthA;
  });

  // 如果有深层路径的错误（depth >= 2），只展示深层错误，屏蔽根级噪音
  const maxDepth = deduped.length
    ? (deduped[0].instancePath.match(/\//g) || []).length
    : 0;
  const toShow = (maxDepth >= 2
    ? deduped.filter(e => (e.instancePath.match(/\//g) || []).length >= 2)
    : deduped
  ).slice(0, 15);

  const fallback = toShow.length ? toShow : allErrors.slice(0, 10);

  const totalCount = deduped.length;
  const shownCount = fallback.length;
  const countDesc = totalCount > shownCount ? `共 ${totalCount} 个问题，展示前 ${shownCount} 个` : `共 ${shownCount} 个问题`;
  console.error(`\n❌ Schema 校验失败（${countDesc}）:`);
  fallback.forEach(e => console.error(' ', formatError(e)));

  process.exit(1);
}

// ─── Main ─────────────────────────────────────────────────────────────────────

const cached = loadCachedSchema();
if (cached) {
  validate(cached);
} else {
  fetchSchema(SCHEMA_URL)
    .then(validate)
    .catch(err => {
      console.error('❌ 无法获取 Schema:', err.message);
      process.exit(1);
    });
}
