package dpi

import (
	"fmt"
	"net"
	"strings"
)

type AppInfo struct {
	App       string `json:"app"`
	Category  string `json:"category"`
	Color     string `json:"color"`
	IsRealApp bool   `json:"is_real_app"`
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
	{"biliapi", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"douyin", "抖音", "视频娱乐", "#1c1c1e"},
	{"amemv", "抖音", "视频娱乐", "#1c1c1e"},
	{"iesdouyin", "抖音", "视频娱乐", "#1c1c1e"},
	{"snssdk", "抖音", "视频娱乐", "#1c1c1e"},
	{"byteimg", "抖音", "视频娱乐", "#1c1c1e"},
	{"bytedance", "抖音", "视频娱乐", "#1c1c1e"},
	{"pstatp", "今日头条", "视频娱乐", "#ed4040"},
	{"toutiao", "今日头条", "视频娱乐", "#ed4040"},
	{"v.qq.com", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"video.qq", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"vv.video.qq", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"iqiyi", "爱奇艺", "视频娱乐", "#00be06"},
	{"qy.net", "爱奇艺", "视频娱乐", "#00be06"},
	{"youku", "优酷", "视频娱乐", "#00a0e9"},
	{"ykimg", "优酷", "视频娱乐", "#00a0e9"},
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
	{"126.net", "网易云音乐", "视频娱乐", "#d43c33"},
	{"y.qq.com", "QQ音乐", "视频娱乐", "#31c27c"},

	// 社交通讯
	{"weixin", "微信", "社交通讯", "#07c160"},
	{"wechat", "微信", "社交通讯", "#07c160"},
	{"wx.qq.com", "微信", "社交通讯", "#07c160"},
	{"servicewechat", "微信", "社交通讯", "#07c160"},
	{"tenpay", "微信", "社交通讯", "#07c160"},
	{"qpic.cn", "QQ", "社交通讯", "#12b7f5"},
	{"qlogo.cn", "QQ", "社交通讯", "#12b7f5"},
	{"qq.com", "QQ", "社交通讯", "#12b7f5"},
	{"weibo", "微博", "社交通讯", "#e6162d"},
	{"sinaimg", "微博", "社交通讯", "#e6162d"},
	{"weibocdn", "微博", "社交通讯", "#e6162d"},
	{"zhihu", "知乎", "社交通讯", "#0066ff"},
	{"zhimg", "知乎", "社交通讯", "#0066ff"},
	{"telegram", "Telegram", "社交通讯", "#26a5e4"},
	{"t.me", "Telegram", "社交通讯", "#26a5e4"},
	{"discord", "Discord", "社交通讯", "#5865f2"},
	{"whatsapp", "WhatsApp", "社交通讯", "#25d366"},
	{"feishu", "飞书", "办公开发", "#00d6b9"},
	{"dingtalk", "钉钉", "办公开发", "#0089ff"},

	// 购物与金融、智能家居
	{"taobao", "淘宝", "购物金融", "#ff5000"},
	{"tmall", "天猫", "购物金融", "#ff0036"},
	{"alicdn", "淘宝", "购物金融", "#ff5000"},
	{"tbcdn", "淘宝", "购物金融", "#ff5000"},
	{"alipay", "支付宝", "购物金融", "#1677ff"},
	{"alipayobjects", "支付宝", "购物金融", "#1677ff"},
	{"jd.com", "京东", "购物金融", "#e1251b"},
	{"360buyimg", "京东", "购物金融", "#e1251b"},
	{"pinduoduo", "拼多多", "购物金融", "#e02e24"},
	{"yangkeduo", "拼多多", "购物金融", "#e02e24"},
	{"meituan", "美团", "购物金融", "#ffc300"},
	{"sankuai", "美团", "购物金融", "#ffc300"},
	{"amap", "高德地图", "购物金融", "#2f89fc"},
	{"icbc", "工商银行", "购物金融", "#c8161d"},
	{"cmbchina", "招商银行", "购物金融", "#c8161d"},
	{"ccb.com", "建设银行", "购物金融", "#0066b3"},
	{"ys7.com", "萤石云", "智能家居", "#2563eb"},
	{"ezviz", "萤石云", "智能家居", "#2563eb"},
	{"mi.com", "米家", "智能家居", "#ff6700"},
	{"xiaomi", "米家", "智能家居", "#ff6700"},
	{"io.mi.com", "米家", "智能家居", "#ff6700"},
	{"home-assistant", "HomeAssistant", "智能家居", "#18bcf2"},

	// 游戏平台
	{"ds.163.com", "网易大神", "游戏平台", "#1e293b"},
	{"netease", "网易大神", "游戏平台", "#d43c33"},
	{"steam", "Steam", "游戏平台", "#1b2838"},
	{"valvesoftware", "Steam", "游戏平台", "#1b2838"},
	{"epicgames", "Epic Games", "游戏平台", "#2a2a2a"},
	{"riotgames", "拳头游戏", "游戏平台", "#d13639"},
	{"blizzard", "暴雪战网", "游戏平台", "#00aeff"},
	{"mihoyo", "米哈游", "游戏平台", "#38bdf8"},
	{"hoyoverse", "HoYoverse", "游戏平台", "#38bdf8"},
	{"playstation", "PlayStation", "游戏平台", "#0070d1"},

	// 办公与开发
	{"github", "GitHub", "办公开发", "#24292f"},
	{"githubusercontent", "GitHub", "办公开发", "#24292f"},
	{"openai", "OpenAI", "办公开发", "#10a37f"},
	{"chatgpt", "OpenAI", "办公开发", "#10a37f"},
	{"anthropic", "Claude", "办公开发", "#d97757"},
	{"claude.ai", "Claude", "办公开发", "#d97757"},
	{"microsoft", "Microsoft", "办公开发", "#00a4ef"},
	{"office", "Office 365", "办公开发", "#d83b01"},
	{"windowsupdate", "Microsoft", "办公开发", "#0078d4"},
	{"docker", "Docker", "办公开发", "#2496ed"},

	// 云与 Apple/Google
	{"itunes.apple", "AppStore", "云与终端", "#0d84ff"},
	{"apps.apple", "AppStore", "云与终端", "#0d84ff"},
	{"mzstatic", "AppStore", "云与终端", "#0d84ff"},
	{"icloud", "iCloud", "云与终端", "#3693f3"},
	{"apple.com", "Apple", "云与终端", "#555555"},
	{"aliyun", "阿里云", "云与终端", "#ff6a00"},
	{"tencent", "腾讯云", "云与终端", "#0052d9"},
	{"myqcloud", "腾讯云", "云与终端", "#0052d9"},
	{"cloudflare", "Cloudflare", "云与终端", "#f38020"},
	{"amazonaws", "AWS", "云与终端", "#ff9900"},
	{"baidu", "百度", "云与终端", "#2932e1"},
	{"bdstatic", "百度", "云与终端", "#2932e1"},
	{"google", "Google", "云与终端", "#4285f4"},
	{"1e100.net", "Google", "云与终端", "#4285f4"},
	{"gstatic", "Google", "云与终端", "#4285f4"},
	{"nju.edu.cn", "南京大学", "云与终端", "#8b5cf6"},
}

var ispASNRules = []rule{
	{"as137718", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"bilibili", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"as138699", "抖音", "视频娱乐", "#1c1c1e"},
	{"as396986", "抖音", "视频娱乐", "#1c1c1e"},
	{"bytedance", "抖音", "视频娱乐", "#1c1c1e"},
	{"byteplus", "抖音", "视频娱乐", "#1c1c1e"},
	{"as45090", "QQ", "社交通讯", "#12b7f5"},
	{"as132203", "QQ", "社交通讯", "#12b7f5"},
	{"tencent", "腾讯云", "云与终端", "#0052d9"},
	{"as37963", "淘宝", "购物金融", "#ff5000"},
	{"as45102", "阿里云", "云与终端", "#ff6a00"},
	{"alibaba", "淘宝", "购物金融", "#ff5000"},
	{"aliyun", "阿里云", "云与终端", "#ff6a00"},
	{"as55967", "百度", "云与终端", "#2932e1"},
	{"baidu", "百度", "云与终端", "#2932e1"},
	{"as62041", "Telegram", "社交通讯", "#26a5e4"},
	{"as62014", "Telegram", "社交通讯", "#26a5e4"},
	{"telegram", "Telegram", "社交通讯", "#26a5e4"},
	{"as13335", "Cloudflare", "云与终端", "#f38020"},
	{"cloudflare", "Cloudflare", "云与终端", "#f38020"},
	{"as15169", "Google", "云与终端", "#4285f4"},
	{"google", "Google", "云与终端", "#4285f4"},
	{"as714", "Apple", "云与终端", "#555555"},
	{"apple", "Apple", "云与终端", "#555555"},
	{"as8075", "Microsoft", "办公开发", "#00a4ef"},
	{"microsoft", "Microsoft", "办公开发", "#00a4ef"},
	{"as32590", "Steam", "游戏平台", "#1b2838"},
	{"valve", "Steam", "游戏平台", "#1b2838"},
	{"as16509", "AWS", "云与终端", "#ff9900"},
	{"amazon", "AWS", "云与终端", "#ff9900"},
	{"as4538", "教育网", "云与终端", "#8b5cf6"},
	{"cernet", "教育网", "云与终端", "#8b5cf6"},
	{"netease", "网易云音乐", "游戏平台", "#d43c33"},
}

type cidrRule struct {
	net  *net.IPNet
	info AppInfo
}

var appCIDRs []cidrRule

func init() {
	defs := []struct {
		cidr, app, cat, color string
	}{
		{"17.0.0.0/8", "Apple", "云与终端", "#555555"},
		{"149.154.160.0/20", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.4.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.8.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.12.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.16.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.56.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"2001:b28:f23d::/48", "Telegram", "社交通讯", "#26a5e4"},
		{"2001:67c:4e8::/48", "Telegram", "社交通讯", "#26a5e4"},
		{"1.1.1.0/24", "Cloudflare", "云与终端", "#f38020"},
		{"1.0.0.0/24", "Cloudflare", "云与终端", "#f38020"},
		{"104.16.0.0/12", "Cloudflare", "云与终端", "#f38020"},
		{"8.8.8.0/24", "Google", "云与终端", "#4285f4"},
		{"8.8.4.0/24", "Google", "云与终端", "#4285f4"},
		{"119.29.29.0/24", "腾讯云", "云与终端", "#0052d9"},
		{"223.5.5.0/24", "阿里云", "云与终端", "#ff6a00"},
	}
	for _, d := range defs {
		if _, n, err := net.ParseCIDR(d.cidr); err == nil {
			appCIDRs = append(appCIDRs, cidrRule{
				net:  n,
				info: AppInfo{App: d.app, Category: d.cat, Color: d.color, IsRealApp: true},
			})
		}
	}
}

var appMetaMap = func() map[string]AppInfo {
	m := make(map[string]AppInfo)
	for _, r := range domainRules {
		m[r.app] = AppInfo{App: r.app, Category: r.category, Color: r.color, IsRealApp: true}
	}
	for _, r := range ispASNRules {
		m[r.app] = AppInfo{App: r.app, Category: r.category, Color: r.color, IsRealApp: true}
	}
	return m
}()

func SignatureCount() int {
	return len(domainRules) + len(ispASNRules) + len(appCIDRs)
}

func CategoryOf(app string) string {
	if info, ok := appMetaMap[app]; ok {
		return info.Category
	}
	switch {
	case strings.Contains(app, "VPN") || strings.Contains(app, "BitTorrent"):
		return "VPN与P2P"
	case app == "HTTPS" || app == "HTTP" || app == "QUIC/HTTP3":
		return "网页浏览"
	case app == "SSH" || app == "RDP 远程桌面" || app == "SMB 文件共享" || app == "WinBox" || app == "RouterOS API":
		return "办公与管理"
	case app == "DNS" || app == "NTP 对时" || app == "ICMP/Ping" || app == "mDNS":
		return "基础网络"
	default:
		return "其他协议"
	}
}

func Classify(proto string, dstPort int, domain, remoteIP, ispAndASN string) AppInfo {
	d := strings.ToLower(domain)
	if d != "" {
		for _, r := range domainRules {
			if strings.Contains(d, r.keyword) {
				return AppInfo{App: r.app, Category: r.category, Color: r.color, IsRealApp: true}
			}
		}
	}

	if remoteIP != "" {
		if ip := net.ParseIP(remoteIP); ip != nil {
			for _, cr := range appCIDRs {
				if cr.net.Contains(ip) {
					return cr.info
				}
			}
		}
	}

	meta := strings.ToLower(ispAndASN)
	if meta != "" {
		for _, r := range ispASNRules {
			if strings.Contains(meta, r.keyword) {
				return AppInfo{App: r.app, Category: r.category, Color: r.color, IsRealApp: true}
			}
		}
	}

	p := strings.ToLower(proto)
	if p == "icmp" || p == "icmpv6" || p == "ipv6-icmp" {
		return AppInfo{App: "ICMP/Ping", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	}

	switch dstPort {
	case 443:
		if p == "udp" {
			return AppInfo{App: "QUIC/HTTP3", Category: "网页浏览", Color: "#0ea5e9", IsRealApp: false}
		}
		return AppInfo{App: "HTTPS", Category: "网页浏览", Color: "#38bdf8", IsRealApp: false}
	case 80, 8080:
		return AppInfo{App: "HTTP", Category: "网页浏览", Color: "#0284c7", IsRealApp: false}
	case 53, 853:
		return AppInfo{App: "DNS", Category: "基础网络", Color: "#94a3b8", IsRealApp: false}
	case 22:
		return AppInfo{App: "SSH", Category: "办公与管理", Color: "#10b981", IsRealApp: true}
	case 3389:
		return AppInfo{App: "RDP 远程桌面", Category: "办公与管理", Color: "#14b8a6", IsRealApp: true}
	case 445:
		return AppInfo{App: "SMB 文件共享", Category: "办公与管理", Color: "#06b6d4", IsRealApp: true}
	case 51820, 9900:
		return AppInfo{App: "WireGuard VPN", Category: "VPN与P2P", Color: "#a855f7", IsRealApp: true}
	case 4500, 500:
		return AppInfo{App: "IPsec VPN", Category: "VPN与P2P", Color: "#9333ea", IsRealApp: true}
	case 1194:
		return AppInfo{App: "OpenVPN", Category: "VPN与P2P", Color: "#7e22ce", IsRealApp: true}
	case 8291:
		return AppInfo{App: "WinBox", Category: "办公与管理", Color: "#f59e0b", IsRealApp: false}
	case 8728, 8729:
		return AppInfo{App: "RouterOS API", Category: "办公与管理", Color: "#f59e0b", IsRealApp: false}
	case 123:
		return AppInfo{App: "NTP 对时", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	case 5353:
		return AppInfo{App: "mDNS", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	}

	if dstPort >= 6881 && dstPort <= 6889 {
		return AppInfo{App: "BitTorrent/P2P", Category: "VPN与P2P", Color: "#ec4899", IsRealApp: true}
	}
	if dstPort > 0 {
		return AppInfo{App: fmt.Sprintf("%s:%d", strings.ToUpper(p), dstPort), Category: "其他协议", Color: "#6366f1", IsRealApp: false}
	}
	return AppInfo{App: strings.ToUpper(p), Category: "其他协议", Color: "#6366f1", IsRealApp: false}
}
