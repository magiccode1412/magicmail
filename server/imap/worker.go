// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package imap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"magicmail/config"
	"magicmail/models"
	"magicmail/notifier"
	"magicmail/sse"
	pop3pkg "magicmail/pop3"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"gorm.io/gorm"
)

// errIDLENotAdvertised 服务器未在 CAPABILITY 中声明 IDLE（RFC 2177 要求支持 IDLE
// 的服务器必须声明）。这是服务器自报的能力缺失，重试无意义，应立即转轮询。
//
// 实测：189.cn（Coremail/21cn）的能力集只有 IMAP4 / IMAP4rev1 / ID / XLIST，
// 没有 IDLE，但它在 IDLE 握手时会返回畸形响应而非直接 BAD，靠错误文本识别
// 要等超时或解析失败后才反应过来，靠能力探测则一次就能判定。
var errIDLENotAdvertised = errors.New("服务器未在 CAPABILITY 中声明 IDLE")

// WorkerPool 管理所有邮箱账号的后台同步协程
type WorkerPool struct {
	db           *gorm.DB
	config       *config.Config
	workers      map[uint]*AccountWorker // accountID -> worker
	mu           sync.RWMutex
	shutdown     int32 // 原子标志：1=关闭中
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
	sem          chan struct{} // 并发信号量
}

var globalPool *WorkerPool

// IDLE 失败退避与降级阈值
//
// 背景：早期实现在 IDLE 报错后固定休眠 30 秒再重试，且只有错误文本命中
// "BAD"/"not support"/"not allowed" 才会拉黑。网络抖动、连接被服务端踢掉这类
// 瞬时故障不会命中，于是 Worker 每 30 秒重连一次 IDLE，单账号约 120 次/小时，
// 既刷日志又容易被服务商限流。
const (
	idleBackoffBase       = 30 * time.Second // 第 1 次失败后的退避时长，之后逐次翻倍
	idleBackoffMax        = 15 * time.Minute // 退避时长上限
	idleMaxFailures       = 4                // 连续失败达到该次数后，本 Worker 生命周期内不再尝试 IDLE
	manualSyncWaitTimeout = 30 * time.Second // 手动同步等待并发令牌的最长时间
	idleLoopTimeout       = 25 * time.Minute // IDLE 单次监听时长（IMAP 规定最长 29 分钟）
	idleHeartbeatMin      = 60 * time.Second // IDLE 兜底同步间隔下限，避免把 IDLE 退化成高频轮询
)

// idleVerifiedServers 已确认支持 IMAP IDLE 的服务器白名单
// 仅用于启动时的信息性提示，不做准入控制：未知服务器仍会尝试一次 IDLE，
// 由 isIDLEUnsupportedErr 在失败时学习并写入黑名单。
var idleVerifiedServers = map[string]bool{
	"imap.gmail.com":        true, // Gmail - RFC2177 标准实现
	"imap.mail.yahoo.com":   true, // Yahoo Mail - 支持IDLE
	"outlook.office365.com": true, // Outlook/Exchange Online - 支持IDLE
	"imap.qq.com":           true, // QQ邮箱 - 支持IDLE (需确认)
}

// idleKnownBrokenSuffixes 已知 IDLE 实现不合规的服务器域名后缀，直接跳过 IDLE 尝试。
//
// ⚠️ 这是**兜底手段**，第一道防线是 idleLoop 里的 CAPABILITY 探测：服务器若没有
// 声明 IDLE 能力，根本不会走到这里。本名单只用于收拾"嘴上声明支持、实际响应
// 违反 RFC 2177"的服务器——它们能躲过能力探测，却在 IDLE 握手时返回畸形响应，
// 让 go-imap/v2 在协议解析层（imapwire）直接失败，且失败是确定性的。
// 典型错误：IDLE 命令失败: in response: cannot read tag: imapwire: expected atom, got "("
//
// 与 idleLearnedUnsupported（运行时学习）的区别：本名单是人工确认的静态名单，
// 命中后连"首次尝试"都不做，避免每个账号每次启动都白跑一轮登录 + IDLE 握手。
//
// 用后缀而非精确匹配：邮箱服务商不同时期给出的主机别名较多
// （如 imap.189.cn / imap.mail.189.cn），精确匹配容易漏网。
var idleKnownBrokenSuffixes = []string{
	"189.cn",  // 天翼邮箱（Coremail）：IDLE 响应不合规
	"139.com", // 移动和彩云（Coremail）：同一套实现
}

// isKnownBrokenServer 判断 IMAP 主机是否命中静态黑名单（域名后缀匹配，大小写不敏感）
func isKnownBrokenServer(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, suffix := range idleKnownBrokenSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// idleParseErrKeywords go-imap/v2 协议解析层（imapwire）报错的特征词。
//
// 出现这些词说明服务端返回了违反 IMAP 语法的响应，属于能力/兼容性问题，
// 而非网络抖动，因此与"服务器明确不支持"同等对待：直接加入黑名单，
// 而不是按瞬时故障退避重试（后者会白试 idleMaxFailures 次才降级）。
var idleParseErrKeywords = []string{
	"imapwire",        // go-imap/v2 的 IMAP 语法解析层
	"cannot read tag", // 响应行读不到合法 tag
	"expected atom",   // 期望原子却拿到其它 token（如 "("）
	"malformed",       // 畸形响应
}

// idleLearnedUnsupported 运行时学习到的、已确认不支持IDLE的服务器（全局共享）
var (
	idleLearnedUnsupported = make(map[string]bool)
	idleLearnMu            sync.RWMutex // 保护并发访问
)

// GlobalPool exposes the worker pool for external packages (services/handlers)
// Returns nil if workers haven't been started yet
func GlobalPool() *WorkerPool {
	return globalPool
}

// GlobalWorkerMode 返回指定账号 Worker 的当前同步模式（idle/polling/syncing/stopped）。
// 无对应 Worker（账号已停用、未启动或进程刚启动）时返回空串，调用方应回退为 "unknown"。
func GlobalWorkerMode(accountID uint) string {
	if globalPool == nil {
		return ""
	}
	globalPool.mu.RLock()
	defer globalPool.mu.RUnlock()
	if w, ok := globalPool.workers[accountID]; ok {
		return w.Mode()
	}
	return ""
}

// StartWorkers 启动所有活跃邮箱的后台同步 Worker（程序启动时调用）
func StartWorkers(db *gorm.DB, cfg *config.Config) {
	pool := &WorkerPool{
		db:         db,
		config:     cfg,
		workers:    make(map[uint]*AccountWorker),
		shutdownCh: make(chan struct{}),
		sem:        make(chan struct{}, cfg.IMAP.MaxConcurrent),
	}
	globalPool = pool

	// 查询所有活跃的邮箱账号
	var accounts []models.MailAccount
	if err := db.Where("status = ?", "active").Find(&accounts).Error; err != nil {
		log.Printf("❌ 查询邮箱账号失败: %v", err)
		return
	}

	if len(accounts) == 0 {
		log.Println("📭 没有活跃的邮箱账号")
		return
	}

	log.Printf("🚀 启动 %d 个邮箱同步 Worker...", len(accounts))

	for i := range accounts {
		pool.StartWorker(&accounts[i])
	}
}

// StopWorkers 优雅关闭所有 Worker
func StopWorkers() {
	if globalPool == nil {
		return
	}
	atomic.StoreInt32(&globalPool.shutdown, 1)
	close(globalPool.shutdownCh)

	globalPool.mu.RLock()
	for _, w := range globalPool.workers {
		w.Stop()
	}
	globalPool.mu.RUnlock()

	globalPool.wg.Wait()
	log.Println("🛑 所有 IMAP Worker 已停止")
}

// StartWorker 为单个邮箱账号启动同步协程
func (p *WorkerPool) StartWorker(account *models.MailAccount) *AccountWorker {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已有该账号的 Worker，先停掉旧的
	if existing, ok := p.workers[account.ID]; ok {
		existing.Stop()
	}

	w := NewAccountWorker(account, p.db, p.config, p.sem, p.shutdownCh)
	p.workers[account.ID] = w

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		w.Run()
	}()

	log.Printf("▶️  Worker 启动: %s (%s)", account.Email, account.Name)
	return w
}

// StopWorker 停止指定账号的 Worker
func (p *WorkerPool) StopWorker(accountID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.workers[accountID]; ok {
		w.Stop()
		delete(p.workers, accountID)
	}
}

// RestartWorker 重启指定账号的 Worker（配置变更后调用）
// 重启视为一次"用户触发的同步"，完成后推送 account.sync_* 进度事件
//
// ⚠️ 该方法会断开正在进行的 IDLE 长连接并重连，代价较大，仅适用于
// 账号新建/修改/启用等"配置必须重新加载"的场景。
// 用户点击"立即同步"请改用 WakeWorker，它不会打断 IDLE。
func (p *WorkerPool) RestartWorker(account *models.MailAccount) {
	w := p.StartWorker(account)
	if w != nil {
		w.manualSync.Store(true)
	}
}

// WakeWorker 唤醒指定账号的 Worker 立即执行一次同步（用户点击"立即同步"时调用）。
//
// 相比 RestartWorker 的优势：
//   - 不销毁 Worker、不断开 IDLE 长连接，因此不会丢失推送上下文；
//   - 阻塞在 IDLE / 轮询等待 / 失败退避中的 Worker 都会被立刻唤醒，秒级响应；
//   - 已在同步中的 Worker 会在本轮结束后补一次同步，不会并发重入。
//
// 返回 false 表示该账号当前没有运行中的 Worker（调用方应回退为 RestartWorker）。
func (p *WorkerPool) WakeWorker(accountID uint) bool {
	p.mu.RLock()
	w, ok := p.workers[accountID]
	p.mu.RUnlock()

	if !ok || w == nil {
		return false
	}

	// ⚠️ 关键：表中存在 ≠ 协程还活着。Run 返回后表项不会被自动清理，
	// 此时 Wake() 投递的信号永远无人消费，用户点击"立即同步"表现为彻底无响应、日志也一片安静。
	// 这里先做存活判定，失效则清理表项并返回 false，让调用方回退为重启 Worker。
	if w.IsDone() {
		p.mu.Lock()
		if cur, exists := p.workers[accountID]; exists && cur == w {
			delete(p.workers, accountID)
		}
		p.mu.Unlock()
		log.Printf("♻️  检测到已退出的 Worker，已从表中移除（将重启）: account_id=%d", accountID)
		return false
	}

	w.Wake()
	return true
}

// AccountWorker 单个邮箱账号的同步 Worker
type AccountWorker struct {
	account         *models.MailAccount
	db              *gorm.DB
	config          *config.Config
	sem             chan struct{}
	shutdownCh      chan struct{}
	stopCh          chan struct{} // 该 Worker 的独立停止通道（缓冲 1：Stop 时若 Worker 正忙也不会丢信号）
	wakeCh          chan struct{} // 手动同步唤醒通道（缓冲 1：用户连点只会合并为一次，不会堆积）
	doneCh          chan struct{} // Run 返回时关闭，供 WakeWorker 判断该 Worker 是否还活着
	idleUnsupported bool          // 标记该账号是否不支持IDLE（避免重复尝试）
	idleFailures    int           // 连续 IDLE 失败次数（仅由 Worker 自身协程读写，无需加锁）
	manualSync      atomic.Bool   // 标记本次同步是否由用户手动触发（用于推送同步进度事件）
	lastStatus      string        // 上一次上报的账号健康状态（用于去重，仅状态变化时推送 account.health）
	mode            atomic.Value  // 当前同步模式: idle/polling/syncing/stopped（切换时推送 SSE，供前端实时展示）
}

// NewAccountWorker 创建新的账号 Worker
func NewAccountWorker(account *models.MailAccount, db *gorm.DB, cfg *config.Config, sem chan struct{}, shutdownCh chan struct{}) *AccountWorker {
	w := &AccountWorker{
		account:    account,
		db:         db,
		config:     cfg,
		sem:        sem,
		shutdownCh: shutdownCh,
		stopCh:     make(chan struct{}, 1),
		wakeCh:     make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
	
	// 策略：默认尝试 IDLE，除非该服务器已知不可用（静态黑名单 + 运行时学习）
	switch {
	case isKnownBrokenServer(account.ImapHost):
		log.Printf("ℹ️  %s (%s) 已知 IDLE 实现不合规，直接使用轮询模式",
			account.Email, account.ImapHost)
	case !idleVerifiedServers[account.ImapHost]:
		// 不在白名单的服务器也会首次尝试IDLE，失败后自动标记为不支持
		log.Printf("ℹ️  %s (%s) 未在IDLE支持列表中，将首次尝试IDLE（失败后将自动禁用）",
			account.Email, account.ImapHost)
	}
	
	return w
}

// Run 启动 Worker 主循环：先做一次全量同步，然后进入 IDLE（仅 IMAP）或轮询模式
func (w *AccountWorker) Run() {
	// 关闭 doneCh 标记本 Worker 生命周期结束：
	// 否则 WakeWorker 会向一个已退出的 Worker 投递唤醒信号，用户点击"立即同步"将永远无响应。
	defer close(w.doneCh)
	defer log.Printf("⏹️  Worker 退出: %s", w.account.Email)

	ticker := time.NewTicker(time.Duration(w.config.IMAP.PollInterval) * time.Second)
	defer ticker.Stop()

	// 首次全量同步
	w.setMode("syncing")
	w.syncOnce()

	for {
		select {
		case <-w.stopCh:
			return
		case <-w.shutdownCh:
			return
		case <-ticker.C:
			// 定时轮询同步（非 IDLE 模式下主要由该分支驱动）
			log.Printf("⏱️  定时同步触发: %s", w.account.Email)
			w.setMode("syncing")
			w.syncOnce()

		case <-w.wakeCh:
			// 用户点击"立即同步"：无需等待下一个 tick，立即执行
			// ⚠️ 日志必须保留：若本行都不出现，说明唤醒信号根本没送达 Worker
			// （Worker 卡在 syncOnce/idleLoop 的阻塞 IO 中，或 Worker 已退出），
			// 这是"点了同步没反应"时最关键的定位依据。
			log.Printf("🔔 收到手动同步请求: %s", w.account.Email)
			w.setMode("syncing")
			w.syncOnce()

		default:
			// 仅 IMAP 协议支持 IDLE 实时监听；POP3 不支持 IDLE，直接等待下次轮询
			if w.isIMAP() && w.config.IMAP.IDLEEnabled && !w.isIDLEUnsupported() {
				w.setMode("idle")
				err := w.idleLoop()
				if err == nil {
					// idleLoop 正常返回说明检测到新邮件或超时，立即同步
					// 连续失败计数归零：IDLE 链路已恢复正常
					w.idleFailures = 0
					w.setMode("syncing")
					w.syncOnce()
					continue
				}

				// 服务器自报不支持 IDLE：已写入黑名单，直接转轮询，无需退避重试
				if errors.Is(err, errIDLENotAdvertised) {
					continue
				}

				// 区分"服务器明确不支持 IDLE"与"网络抖动等瞬时故障"
				if isIDLEUnsupportedErr(err) {
					// 能力问题：写入全局黑名单，同服务器的其他账号也不再白费力气
					w.markIDLEUnsupported()
					log.Printf("⚠️  %s (%s) 不支持 IDLE，已记录到全局黑名单（将使用轮询模式）: %v",
						w.account.Email, w.account.ImapHost, err)
					continue // 立即转入轮询分支
				}

				// 瞬时故障：指数退避重试，达到阈值后才降级，避免 30 秒无限重连
				w.idleFailures++
				if w.idleFailures >= idleMaxFailures {
					// 只降级当前 Worker，不污染全局黑名单：连续失败更可能是本机网络/账号问题，
					// 不应让同服务器的其他账号被牵连；Worker 重启后会重新尝试 IDLE。
					w.degradeToPolling()
					log.Printf("⚠️  %s IDLE 连续失败 %d 次（最后错误: %v），已降级为轮询模式",
						w.account.Email, w.idleFailures, err)
					continue
				}

				backoff := idleBackoff(w.idleFailures)
				log.Printf("⚠️  IDLE 异常 (%s) 第 %d/%d 次: %v，%v 后重试",
					w.account.Email, w.idleFailures, idleMaxFailures, err, backoff)
				w.setMode("polling")
				select {
				case <-time.After(backoff):
				case <-w.wakeCh:
					// 手动同步可以打断退避等待，不必干等到退避结束
					log.Printf("🔔 收到手动同步请求（打断 IDLE 退避）: %s", w.account.Email)
					w.setMode("syncing")
					w.syncOnce()
				case <-w.stopCh:
					return
				case <-w.shutdownCh:
					return
				}
			} else {
				// POP3 或未启用 IDLE：轮询等待下一次定时触发
				w.setMode("polling")
				select {
				case <-ticker.C:
					w.setMode("syncing")
					w.syncOnce()
				case <-w.wakeCh:
					// 手动同步：轮询模式下同样立即生效，无需等待下一个 tick
					log.Printf("🔔 收到手动同步请求（轮询模式）: %s", w.account.Email)
					w.setMode("syncing")
					w.syncOnce()
				case <-w.stopCh:
					return
				case <-w.shutdownCh:
					return
				}
			}
		}
	}
}

// syncOnce 执行单次完整同步
func (w *AccountWorker) syncOnce() {
	// 本次是否为用户手动触发的同步（用于推送同步进度事件）
	// 必须在获取并发令牌之前读取：手动同步即使因并发满被跳过，也必须给前端明确反馈，
	// 否则用户点了"立即同步"后界面会一直卡在同步中。
	wasManual := w.manualSync.Swap(false)
	manualAbort := func(msg string) {
		if wasManual {
			sse.PublishAccountSyncError(w.account.UserID, w.account.ID, w.account.Email, msg)
		}
	}
	fail := func(msg string) {
		w.updateAccountStatus("error", msg)
		manualAbort(msg)
	}

	// 获取并发令牌
	if wasManual {
		// 手动同步：有限等待而非直接放弃，避免用户点击后没有任何反馈
		select {
		case w.sem <- struct{}{}:
		case <-time.After(manualSyncWaitTimeout):
			log.Printf("⏳ 并发已满，手动同步等待超时: %s", w.account.Email)
			manualAbort("同步任务繁忙，请稍后重试")
			return
		case <-w.stopCh:
			return
		case <-w.shutdownCh:
			return
		}
	} else {
		select {
		case w.sem <- struct{}{}:
		default:
			log.Printf("⏳ 并发已满，跳过本次同步: %s", w.account.Email)
			return
		}
	}
	defer func() { <-w.sem }()

	start := time.Now()
	// 同步期间（连服务器、认证、逐封拉邮件）此前没有任何日志，一旦某一步被服务器"吊住"，
	// 终端就完全是静默的，看起来像"点了没反应"。这里给出起止日志 + 耗时，便于定位卡在哪一步。
	manualTag := ""
	if wasManual {
		manualTag = " [手动触发]"
	}
	log.Printf("🔄 开始同步 (%s)%s", w.account.Email, manualTag)
	defer func() {
		log.Printf("🏁 同步结束 (%s)%s: 耗时 %v", w.account.Email, manualTag, time.Since(start))
	}()

	// 重新从数据库获取最新账号信息（密码可能被更新）
	var fresh models.MailAccount
	if err := w.db.First(&fresh, w.account.ID).Error; err != nil {
		log.Printf("❌ 无法获取账号信息 (ID=%d): %v", w.account.ID, err)
		return
	}
	w.account = &fresh

	// 根据协议创建对应的邮件客户端
	client, err := NewMailClient(w.account, w.config)
	if err != nil {
		fail(err.Error())
		return
	}
	defer client.Close()

	// 认证
	if err := client.Authenticate(); err != nil {
		fail(err.Error())
		return
	}

	// 持久化刷新得到的 OAuth2 Token（RefreshToken 轮换后需落库，否则进程重启后会用已失效的旧 RefreshToken）
	if ic, ok := client.(*IMAPClient); ok {
		if err := ic.persistOAuthTokens(w.db); err != nil {
			log.Printf("⚠️ 保存刷新后的 OAuth2 Token 失败 (%s): %v", w.account.Email, err)
		}
	}

	// 根据协议选择对应的拉取器执行同步
	var count int
	var syncedIDs []uint // 本次同步成功入库的邮件ID（webhook精确推送用）
	if w.isIMAP() {
		// IMAP 同步（仅收件箱 INBOX；已发送由应用发送时本地落库，不从邮箱拉取）
		imapClient := client.(*IMAPClient)
		fetcher := NewFetcher(w.db, w.config)
		count, err = fetcher.SyncMailbox(imapClient)
		syncedIDs = fetcher.SyncedMailIDs // IMAP: 从 Fetcher 获取精确ID
	} else {
		// POP3 同步
		pop3Client := client.(*pop3pkg.POP3Client)
		pop3Fetcher := pop3pkg.NewPOP3Fetcher(w.db, w.config)
		count, err = pop3Fetcher.SyncMailbox(pop3Client)
		syncedIDs = pop3Fetcher.SyncedMailIDs // POP3: 从 POP3Fetcher 获取精确ID
	}

	if err != nil {
		fail(err.Error())
		return
	}

	// 同步成功，更新状态和时间
	now := time.Now()
	w.db.Model(&models.MailAccount{}).Where("id = ?", w.account.ID).
		Updates(map[string]interface{}{
			"last_sync_at": now,
			"status":      "active",
			"error_msg":   "",
		})

	if wasManual {
		sse.PublishAccountSyncDone(w.account.UserID, w.account.ID, w.account.Email, count)
	}

	if count > 0 {
		log.Printf("📬 %s 同步完成: 新增 %d 封邮件", w.account.Email, count)

		// ⭐ 使用精确的邮件ID列表查询（而非 Limit(count) 近似查询）
		// 解决 webhook 重复/漏发问题：count 可能是 INBOX+Sent 的总和，且按 sent_at 排序不可靠
		if len(syncedIDs) == 0 {
			log.Printf("⚠️  %s 有新增邮件但无ID记录，跳过 webhook", w.account.Email)
			return
		}

		log.Printf("🔍 [WEBHOOK] 准备为 %d 个邮件ID发送通知: %v", len(syncedIDs), syncedIDs)

		// 验证：检查DB中这些ID是否真实存在
		var dbCount int64
		w.db.Table("mails").Where("id IN ?", syncedIDs).Count(&dbCount)
		log.Printf("🔍 [WEBHOOK] DB中实际存在的邮件数: %d (计划查 %d 封)", dbCount, len(syncedIDs))

		var latestMails []struct {
			ID       uint      `gorm:"column:id"`
			Subject  string    `json:"subject"`
			From     string    `json:"from"`
			To       string    `gorm:"column:to"`       // 收件人
			Cc       string    `gorm:"column:cc"`       // 抄送
			SentAt   time.Time `json:"sent_at"`
			TextBody string    `gorm:"column:text_body"` // 纯文本正文
			HTMLBody string    `gorm:"column:html_body"` // HTML 正文（fallback）
		}
		w.db.Table("mails").
			Select("id, subject, `from`, `to`, cc, sent_at, text_body, html_body").
			Where("id IN ?", syncedIDs).
			Order("sent_at DESC").
			Find(&latestMails)

		log.Printf("🔍 [WEBHOOK] DB查询返回 %d 封邮件 (预期 %d)", len(latestMails), len(syncedIDs))

		mailList := make([]map[string]interface{}, len(latestMails))
		for i, m := range latestMails {
			// 预览：优先纯文本，其次 HTML 去标签，截取前 200 字符
			preview := m.TextBody
			if preview == "" && m.HTMLBody != "" {
				preview = stripHTML(m.HTMLBody)
			}
			if len(preview) > 200 { preview = preview[:200] + "..." }
			mailList[i] = map[string]interface{}{
				"subject":   m.Subject,
				"from":      m.From,
				"to":        m.To,
				"cc":        m.Cc,
				"sent_at":   m.SentAt.Format("2006-01-02 15:04:05"),
				"preview":   preview,
				"text_body": m.TextBody,
				"html_body": m.HTMLBody,
			}
		}

		// 触发 Webhook 通知（每封邮件独立触发一次，仅当前用户配置的 Webhook）
		nowTs := fmt.Sprintf("%d", time.Now().Unix())
		for _, mail := range mailList {
			notifier.TriggerByEvent(w.db, "mail.received", map[string]interface{}{
				"account_id":    w.account.ID,
				"account_email": w.account.Email,
				"account_name":  w.account.Name,
				"protocol":      w.account.Protocol,
				"subject":       mail["subject"],
				"from":          mail["from"],
				"to":            mail["to"],
				"cc":            mail["cc"],
				"sent_at":       mail["sent_at"],
				"preview":       mail["preview"],
				"text_body":     mail["text_body"],
				"html_body":     mail["html_body"],
				"timestamp":     nowTs,
			}, w.account.UserID)
		}

	// 推送 SSE 实时事件给前端（仅当前用户）
	sse.PublishMailReceived(w.account.UserID, w.account.ID, w.account.Email, count, mailList)
	sse.PublishStatsUpdated(w.account.UserID, w.account.ID, w.account.Email)

	// 发送 Web Push 离线推送通知（通过 notifier 包桥接，避免循环依赖，仅当前用户）
		notifier.SendPushNotification(
			w.account.UserID,
			fmt.Sprintf("📧 您有 %d 封新邮件", count),
			fmt.Sprintf("来自 %s", w.account.Email),
			map[string]interface{}{"account_id": w.account.ID},
		)
	}
}

// isIMAP 判断当前账号是否为 IMAP 协议
func (w *AccountWorker) isIMAP() bool {
	return w.account.Protocol != "pop3" && w.account.Protocol != "pop3-no-ssl"
}

// isIDLEUnsupported 检查该账号的服务器是否已知不支持IDLE（本地降级 + 静态黑名单 + 运行时学习）
func (w *AccountWorker) isIDLEUnsupported() bool {
	if w.idleUnsupported {
		return true
	}
	host := w.account.ImapHost

	// 静态黑名单：已知实现不合规，连首次尝试都跳过
	if isKnownBrokenServer(host) {
		return true
	}

	// 检查全局学习列表
	idleLearnMu.RLock()
	defer idleLearnMu.RUnlock()
	return idleLearnedUnsupported[host]
}

// degradeToPolling 仅将当前 Worker 降级为轮询，不写入全局黑名单。
//
// 与 markIDLEUnsupported 的区别：连续失败更可能是本机网络抖动、账号被限流等
// "环境"问题，而非服务器能力问题，不应牵连同服务器的其他账号。
// 该降级只在当前 Worker 生命周期内有效，Worker 重启后会重新尝试 IDLE。
func (w *AccountWorker) degradeToPolling() {
	w.idleUnsupported = true
}

// isIDLEUnsupportedErr 判断错误是否为"该服务器无法使用 IDLE"，分两类：
//
//  1. 服务器明确声明不支持：BAD / not support / not allowed
//  2. 协议解析失败：服务端返回了 go-imap/v2 读不懂的响应（imapwire 层报错）
//
// 第 2 类同样属于确定性的能力问题——重试多少次都会得到同样的畸形响应，
// 因此按"不支持"处理，直接写入黑名单，不再白白退避重试 4 次。
//
// 网络断开、EOF、超时等属于瞬时故障，返回 false，走退避重试而非直接拉黑。
func isIDLEUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	for _, kw := range []string{"BAD", "not support", "not allowed"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	for _, kw := range idleParseErrKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// idleBackoff 计算第 n 次 IDLE 失败后的退避时长：30s → 60s → 120s … 上限 idleBackoffMax
func idleBackoff(failures int) time.Duration {
	d := idleBackoffBase
	for i := 1; i < failures && d < idleBackoffMax; i++ {
		d *= 2
	}
	if d > idleBackoffMax {
		return idleBackoffMax
	}
	return d
}

// markIDLEUnsupported 将该服务器标记为不支持IDLE（写入全局共享内存）
func (w *AccountWorker) markIDLEUnsupported() {
	w.idleUnsupported = true
	
	// 写入全局学习列表，让其他同服务器的Worker也能受益
	idleLearnMu.Lock()
	defer idleLearnMu.Unlock()
	
	if !idleLearnedUnsupported[w.account.ImapHost] {
		idleLearnedUnsupported[w.account.ImapHost] = true
		log.Printf("📝 已将 %s 加入 IDLE 不支持黑名单", w.account.ImapHost)
	}
}

// idleHeartbeatInterval 返回 IDLE 期间的兜底同步间隔。
//
// 僵尸 IDLE 场景：连接看起来一切正常（既不报错也不断开），但服务器从不推送 EXISTS。
// go-imap/v2 在 IDLE 期间的读超时被显式设为 0（imapclient.idleReadTimeout = 0，不设 deadline），
// 因此"半开连接"——对端静默失联、NAT 会话超时、链路中断但未发 FIN——库永远感知不到，
// Wait() 不会返回，只能干等到 idleLoopTimeout（25 分钟）超时。心跳兜底同步把这个窗口
// 压缩到分钟级：即使推送链路完全失效，仍然按固定节奏主动拉取一次。
//
// 未显式配置时跟随 PollInterval；下限 idleHeartbeatMin，避免把 IDLE 退化成高频轮询。
func (w *AccountWorker) idleHeartbeatInterval() time.Duration {
	seconds := w.config.IMAP.IdleHeartbeat
	if seconds <= 0 {
		seconds = w.config.IMAP.PollInterval
	}
	if d := time.Duration(seconds) * time.Second; d > idleHeartbeatMin {
		return d
	}
	return idleHeartbeatMin
}

// idleLoop 进入 IDLE 监听循环，等待服务器推送新邮件通知
// 
// go-imap/v2 的 IDLE 实现机制:
//   - Idle().Wait() 只在连接断开或手动 Close() 时返回
//   - 收到 EXISTS (新邮件) 不会让 Wait() 返回
//   - 必须通过 UnilateralDataHandler 回调检测新邮件到达
//   - IDLE 期间 idleReadTimeout = 0，半开连接不会被超时机制发现
//
// 本函数采用"IDLE + 心跳兜底"策略:
//   1. 启动 IDLE 并设置 Mailbox handler 监听 EXISTS 事件
//   2. 收到 EXISTS 后立即结束 IDLE，返回主循环执行 syncOnce()
//   3. 每 idleHeartbeatInterval 主动同步一次（不中断 IDLE、不重连），兜底僵尸连接
//   4. IDLE 命令自身终止（连接被关闭）时报错，交由主循环退避重连
//   5. 25 分钟无事件则重启 IDLE 重建连接（IMAP 规定最长 29 分钟）
func (w *AccountWorker) idleLoop() error {
	// 创建带 UnilateralDataHandler 的 IMAP 客户端
	// 用于接收服务器的单方面数据推送 (EXISTS, FETCH, EXPUNGE 等)
	mailboxCh := make(chan struct{}, 1) // 非缓冲: 收到 EXISTS 时通知
	
	client, err := w.newIMAPClientWithHandler(mailboxCh)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Authenticate(); err != nil {
		return err
	}

	// ⭐ 能力探测：必须在认证之后读取——部分服务器登录前后的 CAPABILITY 不同，
	// 登录前只有最小能力集，登录后才会暴露 IDLE 等扩展。
	//
	// 这是最可靠的判定方式：服务器自己声明的能力，比事后靠错误文本猜、也比
	// 维护域名黑名单都准。命中后写入全局黑名单，同服务器的其他账号直接受益。
	if !client.Client.Caps().Has(imap.CapIdle) {
		w.markIDLEUnsupported()
		log.Printf("ℹ️  %s (%s) 服务器未声明 IDLE 能力，直接使用轮询模式（能力集: %v）",
			w.account.Email, w.account.ImapHost, client.Client.Caps())
		return errIDLENotAdvertised
	}

	_, err = client.SelectINBOX()
	if err != nil {
		return err
	}

	// 进入 IDLE 模式
	log.Printf("🔄 IDLE 监听中: %s", w.account.Email)

	idleCmd, err := client.Client.Idle()
	if err != nil {
		return fmt.Errorf("IDLE 命令失败: %w", err)
	}
	// 保证任何返回路径都会关闭 IDLE 命令：
	// 否则监控 Wait() 的协程会一直阻塞在等待服务端 DONE 响应上。
	// 注意 defer 的 LIFO 顺序：本语句后于 client.Close() 注册，故先于它执行，
	// 即"先写 DONE 结束 IDLE，再 LOGOUT 关闭连接"。重复调用是安全的（库内用 atomic 保护）。
	defer idleCmd.Close()

	// 监控 IDLE 命令自身的生命周期：服务端断开（FIN/RST）时 Wait() 立即返回，
	// 让主循环走"IDLE 异常 → 退避 → 重连/降级"流程，而不是干等 25 分钟超时。
	// 返回值写进缓冲为 1 的通道：其它分支已经 return 时该协程也不会泄漏。
	idleDone := make(chan error, 1)
	go func() { idleDone <- idleCmd.Wait() }()

	// 僵尸 IDLE 兜底：按固定节奏主动同步一次。
	// syncOnce 使用独立的 IMAP 连接，与当前 IDLE 长连接互不干扰，
	// 因此不需要关闭 IDLE、也不需要重连，代价仅是一次正常的收信往返。
	heartbeat := time.NewTicker(w.idleHeartbeatInterval())
	defer heartbeat.Stop()

	// 单次 IDLE 监听的总时长上限（IMAP 规定最长 29 分钟）
	// 用 Timer 而非 time.After：提前返回时可显式回收，避免定时器滞留 25 分钟
	timeout := time.NewTimer(idleLoopTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-mailboxCh:
			// ⭐ 收到服务器推送的新邮件通知！
			log.Printf("📬 IDLE 收到新邮件通知: %s", w.account.Email)
			return nil // 返回主循环执行 syncOnce()

		case <-heartbeat.C:
			// ⭐ 僵尸 IDLE 兜底同步：IDLE 保持不断，仅用独立连接拉一次
			log.Printf("💓 IDLE 心跳兜底同步: %s", w.account.Email)
			w.syncOnce()
			// 刚刚同步过：丢弃期间累积的 EXISTS 信号，避免紧接着又同步一轮。
			// 若期间真的来了新邮件，上面这次 syncOnce 已经收进来了。
			select {
			case <-mailboxCh:
			default:
			}

		case waitErr := <-idleDone:
			// 连接被服务端关闭：交给主循环按 IDLE 异常处理（退避后重连，连续失败则降级为轮询）
			if waitErr == nil {
				waitErr = errors.New("IDLE 命令已终止")
			}
			log.Printf("🔌 IDLE 连接中断 (%s): %v", w.account.Email, waitErr)
			return fmt.Errorf("IDLE 连接中断: %w", waitErr)

		case <-timeout.C:
			// 超时保底（IMAP 规定最长29分钟）
			log.Printf("⏰ IDLE 超时重启: %s", w.account.Email)
			return nil // 返回主循环重新进入 IDLE

		case <-w.wakeCh:
			// 用户点击"立即同步"：立刻结束 IDLE，返回主循环执行 syncOnce()
			log.Printf("⚡ IDLE 被手动同步唤醒: %s", w.account.Email)
			return nil

		case <-w.stopCh:
			log.Printf("⏹️  IDLE 停止信号: %s", w.account.Email)
			return nil

		case <-w.shutdownCh:
			return nil
		}
	}
}

// newIMAPClientWithHandler 创建带有 UnilateralDataHandler 的 IMAP 客户端
// 用于 IDLE 模式下接收服务器的实时推送
func (w *AccountWorker) newIMAPClientWithHandler(mailboxCh chan struct{}) (*IMAPClient, error) {
	host := w.account.ImapHost
	port := w.account.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	// TLS 配置（复用 client.go 中的配置逻辑）
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	var imapClient *imapclient.Client
	var err error

	// 直连方式创建客户端（与 NewIMAPClient 保持一致）
	imapClient, err = imapclient.DialTLS(addr, &imapclient.Options{
		TLSConfig: tlsConfig,
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			// ⭐ 关键: 监听 Mailbox 状态变化 (EXISTS/EXPUNGE 等)
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					// 收件箱邮件数量变化 → 新邮件到达或删除
					// 向 channel 发送信号（非阻塞，防止重复触发）
					select {
					case mailboxCh <- struct{}{}:
						log.Printf("📥 [%s] 检测到邮箱状态变更 (当前 %d 封)", 
							w.account.Email, *data.NumMessages)
					default:
						// 已有待处理的通知，忽略
					}
				}
			},
			// 可选: 监听新邮件的详细数据
			Fetch: func(msg *imapclient.FetchMessageData) {
				// 通常 EXISTS 之后会跟随 FETCH 数据
				// 这里可以进一步处理邮件详情
				log.Printf("📧 [%s] 收到 FETCH 推送 (seq=%d)", 
					w.account.Email, msg.SeqNum)
			},
		},
	})
	
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %w", addr, err)
	}

	return &IMAPClient{
		Client:  imapClient,
		Account: w.account,
		config:  w.config,
	}, nil
}

// Wake 唤醒 Worker 立即执行一次同步（用户点击"立即同步"时调用）。
//
// 先置 manualSync 标志再发信号，保证 syncOnce 中 Swap(false) 一定能读到；
// 通道缓冲为 1 且非阻塞发送，用户连点只会合并成一次，不会堆积成"同步风暴"。
func (w *AccountWorker) Wake() {
	w.manualSync.Store(true)
	select {
	case w.wakeCh <- struct{}{}:
	default:
		// 已有待处理的唤醒信号，本次请求直接复用
	}
}

// IsDone 判断 Worker 主协程是否已退出（Run 已返回）。
//
// 用途：Worker 退出后 workers 表中仍可能保留它的引用，
// 直接 Wake 会让信号永远无人消费；调用方据此决定清理表项并重启。
func (w *AccountWorker) IsDone() bool {
	if w == nil {
		return true
	}
	select {
	case <-w.doneCh:
		return true
	default:
		return false
	}
}

// Stop 停止此 Worker
func (w *AccountWorker) Stop() {
	w.setMode("stopped")
	select {
	case w.stopCh <- struct{}{}:
	default:
	}
}

// setMode 更新当前同步模式，仅在状态切换时推送 SSE 事件（供前端实时展示 idle/polling）
func (w *AccountWorker) setMode(mode string) {
	if mode == "" {
		return
	}
	if old, _ := w.mode.Load().(string); old != mode {
		w.mode.Store(mode)
		sse.PublishAccountMode(w.account.UserID, w.account.ID, w.account.Email, mode)
	}
}

// Mode 返回当前同步模式（idle/polling/syncing/stopped），未启动时为空串
func (w *AccountWorker) Mode() string {
	if w == nil {
		return ""
	}
	if v, ok := w.mode.Load().(string); ok {
		return v
	}
	return ""
}

// updateAccountStatus 更新账号状态到数据库，并在状态发生切换时推送 account.health 事件
func (w *AccountWorker) updateAccountStatus(status, errMsg string) {
	w.db.Model(&models.MailAccount{}).Where("id = ?", w.account.ID).
		Updates(map[string]interface{}{
			"status":    status,
			"error_msg": errMsg,
		})
	if status == "error" {
		log.Printf("❌ 同步错误 (%s): %s", w.account.Email, errMsg)
	}

	// 仅在健康状态发生切换时推送，避免每次轮询都刷屏（P3-1：账号连接健康实时可见）
	if w.lastStatus != status {
		w.lastStatus = status
		sse.PublishAccountHealth(w.account.UserID, w.account.ID, w.account.Email, status, errMsg)
	}
}

// isIdleClosed 判断错误是否为 IDLE 正常结束
func isIdleClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "idle terminated") ||
		contains(msg, "connection closed") ||
		contains(msg, "EOF")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// stripHTML 移除 HTML 标签，返回纯文本（用于 webhook preview fallback）
// 会完整移除 <style> 和 <script> 标签及其内部内容，避免 CSS/JS 代码泄漏到预览文本中
func stripHTML(html string) string {
	s := html

	// 1. 先移除 <style>...</style> 和 <script>...</script> 块（大小写不敏感）
	for _, tag := range []string{"style", "script"} {
		for {
			lower := strings.ToLower(s)
			startTag := "<" + tag
			endTag := "</" + tag + ">"

			startIdx := strings.Index(lower, startTag)
			if startIdx == -1 {
				break
			}
			// 找到起始标签的 '>' 位置
			startClose := strings.Index(s[startIdx:], ">")
			if startClose == -1 {
				break
			}
			contentStart := startIdx + startClose + 1
			endIdx := strings.Index(strings.ToLower(s[contentStart:]), endTag)
			if endIdx == -1 {
				// 没有闭合标签，只移除到末尾
				s = s[:startIdx]
				break
			}
			s = s[:startIdx] + s[contentStart+endIdx+len(endTag):]
		}
	}

	// 2. 移除剩余 HTML 标签
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// 3. 清理多余空白：将连续空白替换为单个空格
	raw := result.String()
	var cleaned strings.Builder
	prevSpace := false
	for _, r := range raw {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				cleaned.WriteRune(' ')
				prevSpace = true
			}
		} else {
			cleaned.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(cleaned.String())
}
