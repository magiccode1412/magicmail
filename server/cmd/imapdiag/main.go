// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

// Command imapdiag 诊断 IMAP 账号"认证成功但收件箱为空"的问题。
//
// 使用方法（在 server 目录下执行）：
//
//	go run ./cmd/imapdiag -host imap.189.cn -user xxx@189.cn -pass '授权码'
//
// 它会打印完整的 IMAP 协议交互（登录凭据自动脱敏），并分别用三条独立路径
// 统计邮件数量：STATUS / SELECT(EXISTS) / UID SEARCH ALL。三者不一致即可判定
// 是协议解析或权限问题，而不是服务器真的没有邮件。
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// 与 imap.NewIMAPClient 保持一致的密码套件，避免只因套件不匹配而连不上
var cipherSuites = []uint16{
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
}

// logConn 在 net.Conn 上记录原始字节流，用于观察服务器返回的真实响应。
// 这是判断"空收件箱"到底是服务端返回的还是客户端解析错的关键手段。
type logConn struct {
	net.Conn
}

func (c *logConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		fmt.Printf("<<< %q\n", string(p[:n]))
	}
	return n, err
}

func (c *logConn) Write(p []byte) (int, error) {
	s := string(p)
	if idx := strings.Index(strings.ToUpper(s), " LOGIN "); idx >= 0 {
		fmt.Printf(">>> %q  (登录凭据已脱敏)\n", s[:idx+len(" LOGIN ")]+"*** ***")
	} else {
		fmt.Printf(">>> %q\n", s)
	}
	return c.Conn.Write(p)
}

func section(title string) {
	fmt.Printf("\n========== %s ==========\n", title)
}

func main() {
	host := flag.String("host", "imap.189.cn", "IMAP 服务器主机名")
	port := flag.Int("port", 993, "IMAP 端口（SSL 通常为 993）")
	user := flag.String("user", "", "登录用户名（通常为完整邮箱地址）")
	pass := flag.String("pass", "", "登录密码或授权码")
	mailbox := flag.String("mailbox", "INBOX", "要诊断的邮箱目录名")
	all := flag.Bool("all", false, "扫描 LIST 出的所有目录并统计每个目录的邮件数（定位邮件是否落在别的文件夹）")
	noID := flag.Bool("no-id", false, "跳过 IMAP ID 命令（用于对比 ID 命令是否影响 SELECT 结果）")
	raw := flag.Bool("raw", true, "打印原始 IMAP 协议交互")
	flag.Parse()

	if *user == "" || *pass == "" {
		fmt.Println("❌ 必须提供 -user 和 -pass")
		flag.Usage()
		os.Exit(2)
	}

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	fmt.Printf("🔍 诊断目标: %s (邮箱目录: %s)\n", addr, *mailbox)

	section("1. TCP + TLS 连接")
	rawConn, err := net.DialTimeout("tcp", addr, 20*time.Second)
	if err != nil {
		fmt.Printf("❌ TCP 连接失败: %v\n", err)
		os.Exit(1)
	}
	defer rawConn.Close()

	var conn net.Conn = rawConn
	if *raw {
		conn = &logConn{Conn: rawConn}
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:   *host,
		MinVersion:   tls.VersionTLS12,
		CipherSuites: cipherSuites,
	})
	if err := tlsConn.Handshake(); err != nil {
		fmt.Printf("❌ TLS 握手失败: %v\n", err)
		os.Exit(1)
	}
	client := imapclient.New(tlsConn, &imapclient.Options{})
	defer client.Close()

	if err := client.WaitGreeting(); err != nil {
		fmt.Printf("❌ 等待服务器问候失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 连接成功，服务器能力: %v\n", client.Caps())

	section("2. IMAP ID 命令（RFC 2971）")
	if *noID {
		fmt.Println("⏭️  已按 -no-id 跳过")
	} else {
		idData := &imap.IDData{
			Name:    "MagicMail",
			Version: "1.0.0",
			Vendor:  "MagicCode",
		}
		if resp, err := client.ID(idData).Wait(); err != nil {
			fmt.Printf("⚠️  ID 命令失败（不阻塞登录）: %v\n", err)
		} else {
			fmt.Printf("✅ ID 成功，服务器自述: name=%q vendor=%q version=%q\n",
				resp.Name, resp.Vendor, resp.Version)
		}
	}

	section("3. 登录认证")
	if err := client.Login(*user, *pass).Wait(); err != nil {
		fmt.Printf("❌ 登录失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ 登录成功")
	fmt.Printf("   登录后服务器能力: %v\n", client.Caps())
	// 应用靠这个判定是否走 IDLE，诊断时单独列出便于对照
	if client.Caps().Has(imap.CapIdle) {
		fmt.Println("   IDLE 能力: ✅ 已声明（应用会走 IDLE 实时推送）")
	} else {
		fmt.Println("   IDLE 能力: ❌ 未声明（应用应直接走轮询）")
	}

	section("4. LIST 目录列表")
	mailboxes, err := client.List("", "*", nil).Collect()
	if err != nil {
		fmt.Printf("⚠️  LIST 失败: %v\n", err)
	} else {
		for i, m := range mailboxes {
			fmt.Printf("   [%d] %-30q delim=%q attrs=%v\n", i, m.Mailbox, string(m.Delim), m.Attrs)
		}
		if len(mailboxes) == 0 {
			fmt.Println("   ⚠️  LIST 返回空列表")
		}
	}

	// 全目录扫描：很多服务商（尤其 Coremail 系）会把邮件自动归类到
	// "我的账单"/"官方活动" 等目录，INBOX 为空不代表邮箱里没有邮件。
	if *all && len(mailboxes) > 0 {
		section("4b. 全目录邮件数统计")
		total := uint32(0)
		for _, m := range mailboxes {
			st, err := client.Status(m.Mailbox, &imap.StatusOptions{
				NumMessages: true,
				NumUnseen:   true,
				UIDNext:     true,
				UIDValidity: true,
			}).Wait()
			if err != nil {
				fmt.Printf("   %-30q STATUS 失败: %v\n", m.Mailbox, err)
				continue
			}
			var msgs uint32
			if st.NumMessages != nil {
				msgs = *st.NumMessages
			}
			total += msgs
			mark := "  "
			if msgs > 0 {
				mark = "⭐"
			}
			fmt.Printf("   %s %-30q MESSAGES=%-6d UNSEEN=%-6v UIDNEXT=%-8d UIDVALIDITY=%d\n",
				mark, m.Mailbox, msgs, derefU32(st.NumUnseen), st.UIDNext, st.UIDValidity)
		}
		fmt.Printf("   ---- 所有目录合计: %d 封 ----\n", total)
	}

	section("5. STATUS（不进入已选状态，独立计数）")
	statusCount := -1
	statusData, err := client.Status(*mailbox, &imap.StatusOptions{
		NumMessages: true,
		NumUnseen:   true,
		UIDNext:     true,
		UIDValidity: true,
	}).Wait()
	if err != nil {
		fmt.Printf("⚠️  STATUS 失败: %v\n", err)
	} else {
		if statusData.NumMessages != nil {
			statusCount = int(*statusData.NumMessages)
		}
		var unseen interface{} = "n/a"
		if statusData.NumUnseen != nil {
			unseen = *statusData.NumUnseen
		}
		fmt.Printf("   MESSAGES=%v UNSEEN=%v UIDNEXT=%d UIDVALIDITY=%d\n",
			derefU32(statusData.NumMessages), unseen, statusData.UIDNext, statusData.UIDValidity)
	}

	section("6. SELECT（进入已选状态）")
	sel, err := client.Select(*mailbox, nil).Wait()
	if err != nil {
		fmt.Printf("❌ SELECT 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   EXISTS(NumMessages)=%d UIDNEXT=%d UIDVALIDITY=%d\n",
		sel.NumMessages, sel.UIDNext, sel.UIDValidity)
	fmt.Printf("   Flags=%v\n   PermanentFlags=%v\n", sel.Flags, sel.PermanentFlags)

	section("7. UID SEARCH ALL（独立计数）")
	searchCount := -1
	searchData, err := client.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		fmt.Printf("⚠️  UID SEARCH 失败: %v\n", err)
	} else {
		setStr := "<nil>"
		if searchData.All != nil {
			setStr = searchData.All.String()
		}
		fmt.Printf("   结果集合: %s\n", setStr)
		fmt.Printf("   ESEARCH: UID=%v Min=%d Max=%d Count=%d\n",
			searchData.UID, searchData.Min, searchData.Max, searchData.Count)
		if searchData.Count > 0 {
			searchCount = int(searchData.Count)
		} else if searchData.All == nil {
			searchCount = 0 // 空集合：SEARCH 明确返回"无匹配邮件"，不是命令失败
		} else {
			searchCount = countFromSet(searchData.All)
		}
	}

	section("8. 抓取前 3 封信封")
	if sel.NumMessages == 0 {
		fmt.Println("   ⏭️  EXISTS=0，跳过")
	} else {
		seqSet := imap.SeqSet{}
		end := sel.NumMessages
		if end > 3 {
			end = 3
		}
		seqSet.AddRange(1, end)
		fetchCmd := client.Fetch(seqSet, &imap.FetchOptions{
			Envelope:     true,
			UID:          true,
			Flags:        true,
			InternalDate: true,
		})
		msgs, err := fetchCmd.Collect()
		fetchCmd.Close()
		if err != nil {
			fmt.Printf("   ⚠️  FETCH 失败: %v\n", err)
		} else {
			fmt.Printf("   FETCH 返回 %d 封\n", len(msgs))
			for _, m := range msgs {
				subj, from := "", ""
				if m.Envelope != nil {
					subj = m.Envelope.Subject
					if len(m.Envelope.From) > 0 {
						from = m.Envelope.From[0].Addr()
					}
				}
				fmt.Printf("   - UID=%d flags=%v date=%v subject=%q from=%q\n",
					m.UID, m.Flags, m.InternalDate.Format("2006-01-02 15:04"), subj, from)
			}
		}
	}

	section("诊断结论")
	fmt.Printf("STATUS 计数        : %s\n", fmtCount(statusCount))
	fmt.Printf("SELECT EXISTS 计数 : %d\n", sel.NumMessages)
	fmt.Printf("UID SEARCH 计数    : %s\n", fmtCount(searchCount))

	switch {
	case sel.NumMessages > 0:
		fmt.Println("✅ 服务器有邮件，且 SELECT 能正确读到数量。若应用里仍为空，问题在应用层（同步模式/天数过滤/去重）。")
	case statusCount > 0 || searchCount > 0:
		fmt.Println("⚠️  SELECT 读到 0，但 STATUS/SEARCH 有数 —— 是协议解析或 SELECT 兼容性问题，需针对性适配。")
	default:
		fmt.Println("📭 三条路径都是 0：服务器侧该目录确实没有邮件。请确认：")
		fmt.Println("   1) 网页端该邮箱是否真的有邮件（而不在「其他文件夹」/「垃圾邮件」里）")
		fmt.Println("   2) 服务商是否要求使用「授权码 / 客户端专用密码」而非网页登录密码")
		fmt.Println("   3) 网页端设置里是否已开启 IMAP 服务，以及是否限制了可收取的范围（如仅最近 N 天）")
	}
}

func derefU32(p *uint32) interface{} {
	if p == nil {
		return "n/a"
	}
	return *p
}

func fmtCount(n int) string {
	if n < 0 {
		return "未知（命令失败或服务器未返回）"
	}
	return strconv.Itoa(n)
}

// countFromSet 从 SEARCH 返回的消息号集合中统计数量。
// 服务器不支持 ESEARCH 时 Count 为 0，只能从集合本身解析；
// 集合是"区间列表"，元素个数不等于邮件数，必须按区间累加。
func countFromSet(set imap.NumSet) int {
	n := 0
	switch v := set.(type) {
	case imap.SeqSet:
		for _, r := range v {
			n += rangeLen(uint32(r.Start), uint32(r.Stop))
		}
	case imap.UIDSet:
		for _, r := range v {
			n += rangeLen(uint32(r.Start), uint32(r.Stop))
		}
	}
	return n
}

// rangeLen 计算单个闭区间 [start, stop] 的元素个数。
// stop 为 0 表示 IMAP 中的单个序号（如 "5"）或通配 "*"，按 1 计。
func rangeLen(start, stop uint32) int {
	if start == 0 {
		return 0
	}
	if stop == 0 || stop < start {
		return 1
	}
	return int(stop - start + 1)
}
