package dpi

import (
	"fmt"
	"strings"
)

type AppInfo struct {
	App      string
	Category string
	Color    string
}

type rule struct {
	keyword  string
	app      string
	category string
	color    string
}

var domainRules = []rule{
	// 视频娱乐
	{"bilivideo", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"bilibili", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"hdslb", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"douyin", "抖音", "视频娱乐", "#fe2c55"},
	{"amemv", "抖音", "视频娱乐", "#fe2c55"},
	{"iesdouyin", "抖音", "视频娱乐", "#fe2c55"},
	{"pstatp", "字节/头条", "视频娱乐", "#ef4444"},
	{"byteimg", "字节/抖音", "视频娱乐", "#fe2c55"},
	{"bytedance", "字节跳动", "视频娱乐", "#fe2c55"},
	{"v.qq.com", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"video.qq", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"iqiyi", "爱奇艺", "视频娱乐", "#00be06"},
	{"qy.net", "爱奇艺", "视频娱乐", "#00be06"},
	{"youku", "优酷", "视频娱乐", "#00a0e9"},
	{"kuaishou", "快手", "视频娱乐", "#ff4906"},
	{"gifshow", "快手", "视频娱乐", "#ff4906"},
	{"xiaohongshu", "小红书", "视频娱乐", "#ff2442"},
	{"xhscdn", "小红书", "视频娱乐", "#ff2442"},
	{"youtube", "YouTube", "视频娱乐", "#ff0000"},
	{"googlevideo", "YouTube", "视频娱乐", "#ff0000"},
	{"ytimg", "YouTube", "视频娱乐", "#ff0000"},
	{"netflix", "Netflix", "视频娱乐", "#e50914"},
	{"nflxvideo", "Netflix", "视频娱乐", "#e50914"},
	{"twitch", "Twitch", "视频娱乐", "#9146ff"},
	{"spotify", "Spotify", "视频娱乐", "#1db954"},
	{"music.163", "网易云音乐", "视频娱乐", "#d43c33"},
	{"y.qq.com", "QQ音乐", "视频娱乐", "#31c27c"},

	// 社交通讯
	{"weixin", "微信", "社交通讯", "#07c160"},
	{"wechat", "微信", "社交通讯", "#07c160"},
	{"wx.qq.com", "微信", "社交通讯", "#07c160"},
	{"qpic.cn", "QQ/微信", "社交通讯", "#12b7f5"},
	{"qq.com", "QQ/腾讯", "社交通讯", "#12b7f5"},
	{"weibo", "微博", "社交通讯", "#e6162d"},
	{"sinaimg", "微博", "社交通讯", "#e6162d"},
	{"telegram", "Telegram", "社交通讯", "#26a5e4"},
	{"t.me", "Telegram", "社交通讯", "#26a5e4"},
	{"discord", "Discord", "社交通讯", "#5865f2"},
	{"whatsapp", "WhatsApp", "社交通讯", "#25d366"},
	{"feishu", "飞书", "社交通讯", "#00d6b9"},
	{"dingtalk", "钉钉", "社交通讯", "#0089ff"},

	// 购物生活
	{"taobao", "淘宝", "购物生活", "#ff5000"},
	{"tmall", "天猫", "购物生活", "#ff0036"},
	{"alicdn", "阿里/淘宝", "购物生活", "#ff6a00"},
	{"alipay", "支付宝", "购物生活", "#1677ff"},
	{"jd.com", "京东", "购物生活", "#e1251b"},
	{"360buyimg", "京东", "购物生活", "#e1251b"},
	{"pinduoduo", "拼多多", "购物生活", "#e02e24"},
	{"yangkeduo", "拼多多", "购物生活", "#e02e24"},
	{"meituan", "美团", "购物生活", "#ffc300"},
	{"dianping", "大众点评", "购物生活", "#ff6633"},
	{"amap", "高德地图", "购物生活", "#2f89fc"},

	// 游戏平台
	{"steam", "Steam", "游戏平台", "#1b2838"},
	{"valvesoftware", "Steam", "游戏平台", "#1b2838"},
	{"epicgames", "Epic Games", "游戏平台", "#3b82f6"},
	{"riotgames", "拳头游戏", "游戏平台", "#d13639"},
	{"blizzard", "暴雪战网", "游戏平台", "#00aeff"},
	{"mihoyo", "米哈游", "游戏平台", "#38bdf8"},
	{"hoyoverse", "HoYoverse", "游戏平台", "#38bdf8"},
	{"nintendo", "Nintendo", "游戏平台", "#e60012"},
	{"playstation", "PlayStation", "游戏平台", "#0070d1"},
	{"xbox", "Xbox", "游戏平台", "#107c10"},

	// 办公开发
	{"github", "GitHub", "办公开发", "#6e5494"},
	{"githubusercontent", "GitHub", "办公开发", "#6e5494"},
	{"openai", "OpenAI", "办公开发", "#10a37f"},
	{"chatgpt", "OpenAI", "办公开发", "#10a37f"},
	{"anthropic", "Claude", "办公开发", "#d97757"},
	{"claude.ai", "Claude", "办公开发", "#d97757"},
	{"microsoft", "Microsoft", "办公开发", "#00a4ef"},
	{"office", "Office 365", "办公开发", "#d83b01"},
	{"windowsupdate", "Windows更新", "办公开发", "#0078d4"},
	{"docker", "Docker", "办公开发", "#2496ed"},

	// 云与 CDN / 设备服务
	{"apple.com", "Apple/AppStore", "云与CDN", "#a3aaae"},
	{"icloud.com", "iCloud", "云与CDN", "#3693f3"},
	{"mzstatic", "Apple CDN", "云与CDN", "#a3aaae"},
	{"aliyun", "阿里云", "云与CDN", "#ff6a00"},
	{"tencent", "腾讯云", "云与CDN", "#0052d9"},
	{"myqcloud", "腾讯云", "云与CDN", "#0052d9"},
	{"cloudflare", "Cloudflare", "云与CDN", "#f38020"},
	{"amazonaws", "AWS", "云与CDN", "#ff9900"},
	{"baidu", "百度", "云与CDN", "#2932e1"},
	{"bdstatic", "百度", "云与CDN", "#2932e1"},
	{"google", "Google", "云与CDN", "#4285f4"},
	{"1e100.net", "Google", "云与CDN", "#4285f4"},
	{"gstatic", "Google", "云与CDN", "#4285f4"},
	{"mi.com", "小米服务", "云与CDN", "#ff6700"},
	{"xiaomi", "小米服务", "云与CDN", "#ff6700"},
	{"huawei", "华为云", "云与CDN", "#cf0a2c"},
	{"nju.edu.cn", "南京大学/教育网", "云与CDN", "#8b5cf6"},
}

var ispRules = []rule{
	{"bilibili", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"bytedance", "字节跳动", "视频娱乐", "#fe2c55"},
	{"byteplus", "字节跳动", "视频娱乐", "#fe2c55"},
	{"tencent", "腾讯系", "社交通讯", "#12b7f5"},
	{"alibaba", "阿里系", "云与CDN", "#ff6a00"},
	{"aliyun", "阿里云", "云与CDN", "#ff6a00"},
	{"taobao", "淘宝/阿里", "购物生活", "#ff5000"},
	{"baidu", "百度", "云与CDN", "#2932e1"},
	{"cloudflare", "Cloudflare", "云与CDN", "#f38020"},
	{"google", "Google", "云与CDN", "#4285f4"},
	{"apple", "Apple 服务", "云与CDN", "#a3aaae"},
	{"microsoft", "Microsoft", "办公开发", "#00a4ef"},
	{"amazon", "AWS", "云与CDN", "#ff9900"},
	{"valve", "Steam", "游戏平台", "#1b2838"},
	{"akamai", "Akamai CDN", "云与CDN", "#009bde"},
	{"fastly", "Fastly CDN", "云与CDN", "#ff282d"},
	{"cernet", "教育网测速/资源", "云与CDN", "#8b5cf6"},
	{"nanjing", "教育网/南京节点", "云与CDN", "#8b5cf6"},
}

var appCatCache = func() map[string]AppInfo {
	m := make(map[string]AppInfo)
	for _, r := range domainRules {
		m[r.app] = AppInfo{App: r.app, Category: r.category, Color: r.color}
	}
	for _, r := range ispRules {
		m[r.app] = AppInfo{App: r.app, Category: r.category, Color: r.color}
	}
	return m
}()

func CategoryOf(app string) string {
	if info, ok := appCatCache[app]; ok {
		return info.Category
	}
	switch {
	case strings.Contains(app, "VPN") || strings.Contains(app, "BitTorrent"):
		return "VPN与P2P"
	case app == "HTTPS" || app == "HTTP" || app == "QUIC/HTTP3":
		return "网页浏览"
	case app == "SSH" || app == "RDP 远程桌面" || app == "SMB 文件共享" || app == "WinBox" || app == "RouterOS API":
		return "办公开发"
	case app == "DNS" || app == "NTP 对时" || app == "ICMP/Ping":
		return "基础网络"
	default:
		return "其他流量"
	}
}

func Classify(proto string, dstPort int, domain string, ispText string) AppInfo {
	d := strings.ToLower(domain)
	if d != "" {
		for _, r := range domainRules {
			if strings.Contains(d, r.keyword) {
				return AppInfo{App: r.app, Category: r.category, Color: r.color}
			}
		}
	}

	ispLower := strings.ToLower(ispText)
	if ispLower != "" {
		for _, r := range ispRules {
			if strings.Contains(ispLower, r.keyword) {
				return AppInfo{App: r.app, Category: r.category, Color: r.color}
			}
		}
	}

	p := strings.ToLower(proto)
	if p == "icmp" || p == "icmpv6" || p == "ipv6-icmp" {
		return AppInfo{App: "ICMP/Ping", Category: "基础网络", Color: "#64748b"}
	}

	switch dstPort {
	case 443:
		if p == "udp" {
			return AppInfo{App: "QUIC/HTTP3", Category: "网页浏览", Color: "#0ea5e9"}
		}
		return AppInfo{App: "HTTPS", Category: "网页浏览", Color: "#38bdf8"}
	case 80, 8080:
		return AppInfo{App: "HTTP", Category: "网页浏览", Color: "#0284c7"}
	case 53, 853:
		return AppInfo{App: "DNS", Category: "基础网络", Color: "#94a3b8"}
	case 22:
		return AppInfo{App: "SSH", Category: "办公开发", Color: "#10b981"}
	case 3389:
		return AppInfo{App: "RDP 远程桌面", Category: "办公开发", Color: "#14b8a6"}
	case 445:
		return AppInfo{App: "SMB 文件共享", Category: "办公开发", Color: "#06b6d4"}
	case 51820, 9900:
		return AppInfo{App: "WireGuard VPN", Category: "VPN与P2P", Color: "#a855f7"}
	case 4500, 500:
		return AppInfo{App: "IPsec VPN", Category: "VPN与P2P", Color: "#9333ea"}
	case 1194:
		return AppInfo{App: "OpenVPN", Category: "VPN与P2P", Color: "#7e22ce"}
	case 8291:
		return AppInfo{App: "WinBox", Category: "办公开发", Color: "#f59e0b"}
	case 8728, 8729:
		return AppInfo{App: "RouterOS API", Category: "办公开发", Color: "#f59e0b"}
	case 123:
		return AppInfo{App: "NTP 对时", Category: "基础网络", Color: "#64748b"}
	case 5353:
		return AppInfo{App: "mDNS", Category: "基础网络", Color: "#64748b"}
	case 25, 465, 587, 993, 995:
		return AppInfo{App: "邮件服务", Category: "办公开发", Color: "#22c55e"}
	}

	if dstPort >= 6881 && dstPort <= 6889 {
		return AppInfo{App: "BitTorrent/P2P", Category: "VPN与P2P", Color: "#ec4899"}
	}
	if dstPort > 0 {
		return AppInfo{App: fmt.Sprintf("%s:%d", strings.ToUpper(p), dstPort), Category: "其他流量", Color: "#6366f1"}
	}
	return AppInfo{App: strings.ToUpper(p), Category: "其他流量", Color: "#6366f1"}
}
