package dpi

import (
	"fmt"
	"strings"
)

type domainRule struct {
	keyword string
	app     string
}

var domainRules = []domainRule{
	{"bilivideo", "哔哩哔哩"},
	{"bilibili", "哔哩哔哩"},
	{"hdslb", "哔哩哔哩"},
	{"douyin", "抖音"},
	{"amemv", "抖音"},
	{"bytedance", "字节跳动"},
	{"byteimg", "字节跳动"},
	{"weixin", "微信"},
	{"wechat", "微信"},
	{"qpic.cn", "微信/QQ"},
	{"qq.com", "腾讯系"},
	{"tencent", "腾讯云/游戏"},
	{"googlevideo", "YouTube"},
	{"youtube", "YouTube"},
	{"ytimg", "YouTube"},
	{"netflix", "Netflix"},
	{"nflxvideo", "Netflix"},
	{"steam", "Steam"},
	{"valvesoftware", "Steam"},
	{"apple.com", "Apple 服务"},
	{"icloud.com", "iCloud"},
	{"mzstatic", "Apple CDN"},
	{"taobao", "淘宝"},
	{"tmall", "天猫"},
	{"alicdn", "阿里云CDN"},
	{"aliyun", "阿里云"},
	{"jd.com", "京东"},
	{"iqiyi", "爱奇艺"},
	{"youku", "优酷"},
	{"baidu", "百度"},
	{"github", "GitHub"},
	{"openai", "OpenAI"},
	{"chatgpt", "OpenAI"},
	{"anthropic", "Claude"},
	{"microsoft", "Microsoft"},
	{"windowsupdate", "Windows更新"},
	{"cloudflare", "Cloudflare"},
	{"amazonaws", "AWS"},
	{"google", "Google"},
}

func Classify(proto string, dstPort int, domain string) string {
	d := strings.ToLower(domain)
	if d != "" {
		for _, r := range domainRules {
			if strings.Contains(d, r.keyword) {
				return r.app
			}
		}
	}

	p := strings.ToLower(proto)
	if p == "icmp" || p == "icmpv6" {
		return "ICMP/Ping"
	}

	switch dstPort {
	case 443:
		if p == "udp" {
			return "QUIC/HTTP3"
		}
		return "HTTPS"
	case 80, 8080:
		return "HTTP"
	case 53, 853:
		return "DNS"
	case 22:
		return "SSH"
	case 3389:
		return "RDP 远程桌面"
	case 445:
		return "SMB 文件共享"
	case 51820:
		return "WireGuard VPN"
	case 4500, 500:
		return "IPsec VPN"
	case 1194:
		return "OpenVPN"
	case 8291:
		return "MikroTik WinBox"
	case 8728, 8729:
		return "RouterOS API"
	case 21, 20:
		return "FTP"
	case 25, 465, 587, 993, 995:
		return "邮件服务"
	case 123:
		return "NTP 对时"
	}

	if dstPort >= 6881 && dstPort <= 6889 {
		return "BitTorrent/P2P"
	}
	if dstPort > 0 {
		return fmt.Sprintf("%s:%d", strings.ToUpper(p), dstPort)
	}
	return strings.ToUpper(p)
}
