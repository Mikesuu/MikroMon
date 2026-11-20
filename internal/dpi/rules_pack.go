package dpi

import "strings"

type suffixSpec struct {
	domain, app, cat, color string
}

var suffixRules = make(map[string]AppInfo)

func init() {
	packs := []suffixSpec{
		// === Google / YouTube / Android ===
		{"gvt1.com", "Google", "云与CDN", "#4285f4"},
		{"gvt2.com", "Google", "云与CDN", "#4285f4"},
		{"gvt3.com", "Google", "云与CDN", "#4285f4"},
		{"ggpht.com", "YouTube", "视频娱乐", "#ff0000"},
		{"googleusercontent.com", "Google", "云与CDN", "#4285f4"},
		{"googleapis.com", "Google", "云与CDN", "#4285f4"},
		{"google.com", "Google", "云与CDN", "#4285f4"},
		{"google.cn", "Google", "云与CDN", "#4285f4"},
		{"gstatic.com", "Google", "云与CDN", "#4285f4"},
		{"youtube.com", "YouTube", "视频娱乐", "#ff0000"},
		{"youtu.be", "YouTube", "视频娱乐", "#ff0000"},
		{"googlevideo.com", "YouTube", "视频娱乐", "#ff0000"},
		{"ytimg.com", "YouTube", "视频娱乐", "#ff0000"},
		{"android.com", "Google", "云与CDN", "#4285f4"},
		{"play.google.com", "Google Play", "云与CDN", "#01875f"},
		{"firebaseio.com", "Firebase", "云与CDN", "#ffca28"},
		{"flutter.dev", "Google", "办公协同", "#02569b"},
		{"golang.org", "Go", "办公协同", "#00add8"},
		{"go.dev", "Go", "办公协同", "#00add8"},

		// === Apple / iCloud / AppStore ===
		{"aaplimg.com", "Apple", "云与CDN", "#555555"},
		{"cdn-apple.com", "Apple", "云与CDN", "#555555"},
		{"apple-dns.net", "Apple", "云与CDN", "#555555"},
		{"apple.com", "Apple", "云与CDN", "#555555"},
		{"icloud.com", "iCloud", "云与CDN", "#3693f3"},
		{"icloud-content.com", "iCloud", "云与CDN", "#3693f3"},
		{"mzstatic.com", "AppStore", "云与CDN", "#0d84ff"},
		{"itunes.apple.com", "AppStore", "云与CDN", "#0d84ff"},
		{"apps.apple.com", "AppStore", "云与CDN", "#0d84ff"},

		// === Huawei / HarmonyOS ===
		{"dbankcloud.cn", "华为云", "云与CDN", "#cf0a2c"},
		{"dbankcloud.com", "华为云", "云与CDN", "#cf0a2c"},
		{"hicloud.com", "华为云", "云与CDN", "#cf0a2c"},
		{"huaweicloud.com", "华为云", "云与CDN", "#cf0a2c"},
		{"huawei.com", "华为", "智能家居", "#cf0a2c"},
		{"vmall.com", "华为商城", "购物电商", "#cf0a2c"},
		{"harmonyos.com", "华为", "智能家居", "#cf0a2c"},

		// === BitTorrent / Trackers ===
		{"demonii.com", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},
		{"opentrackr.org", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},
		{"openbittorrent.com", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},
		{"publicbt.com", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},
		{"nyaa.si", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},
		{"qbittorrent.org", "BitTorrent/P2P", "VPN与P2P", "#ec4899"},

		// === Cloud & CDN (Domestic & Global) ===
		{"ksyuncdn.com", "金山云", "云与CDN", "#2563eb"},
		{"ksyun.com", "金山云", "云与CDN", "#2563eb"},
		{"ks-cdn.com", "金山云", "云与CDN", "#2563eb"},
		{"aliyun.com", "阿里云", "云与CDN", "#ff6a00"},
		{"aliyuncs.com", "阿里云", "云与CDN", "#ff6a00"},
		{"alicdn.com", "阿里云", "云与CDN", "#ff6a00"},
		{"alibabacloud.com", "阿里云", "云与CDN", "#ff6a00"},
		{"qcloud.com", "腾讯云", "云与CDN", "#0052d9"},
		{"myqcloud.com", "腾讯云", "云与CDN", "#0052d9"},
		{"tencent-cloud.net", "腾讯云", "云与CDN", "#0052d9"},
		{"cloud.tencent.com", "腾讯云", "云与CDN", "#0052d9"},
		{"volces.com", "火山引擎", "云与CDN", "#1e293b"},
		{"volcengine.com", "火山引擎", "云与CDN", "#1e293b"},
		{"bytecdn.cn", "火山引擎", "云与CDN", "#1e293b"},
		{"baidubce.com", "百度云", "云与CDN", "#2932e1"},
		{"bcebos.com", "百度云", "云与CDN", "#2932e1"},
		{"jdcloud.com", "京东云", "云与CDN", "#e1251b"},
		{"jcloudcs.com", "京东云", "云与CDN", "#e1251b"},
		{"ctyun.cn", "天翼云", "云与CDN", "#ff6600"},
		{"ucloud.cn", "UCloud", "云与CDN", "#2563eb"},
		{"qiniucdn.com", "七牛云", "云与CDN", "#00aae7"},
		{"qbox.me", "七牛云", "云与CDN", "#00aae7"},
		{"upyun.com", "又拍云", "云与CDN", "#0ea5e9"},
		{"upaiyun.com", "又拍云", "云与CDN", "#0ea5e9"},
		{"wangsu.com", "网宿CDN", "云与CDN", "#0284c7"},
		{"chinanetcenter.com", "网宿CDN", "云与CDN", "#0284c7"},
		{"cloudflare.com", "Cloudflare", "云与CDN", "#f38020"},
		{"cloudflare.net", "Cloudflare", "云与CDN", "#f38020"},
		{"workers.dev", "Cloudflare", "云与CDN", "#f38020"},
		{"pages.dev", "Cloudflare", "云与CDN", "#f38020"},
		{"amazonaws.com", "AWS", "云与CDN", "#ff9900"},
		{"aws.amazon.com", "AWS", "云与CDN", "#ff9900"},
		{"cloudfront.net", "AWS", "云与CDN", "#ff9900"},
		{"azure.com", "Azure", "云与CDN", "#0078d4"},
		{"azureedge.net", "Azure", "云与CDN", "#0078d4"},
		{"windows.net", "Azure", "云与CDN", "#0078d4"},
		{"oraclecloud.com", "Oracle Cloud", "云与CDN", "#f80000"},
		{"digitalocean.com", "DigitalOcean", "云与CDN", "#0080ff"},
		{"vultr.com", "Vultr", "云与CDN", "#007bfc"},
		{"linode.com", "Linode", "云与CDN", "#02b159"},
		{"hetzner.com", "Hetzner", "云与CDN", "#d50c2d"},
		{"ovh.net", "OVHcloud", "云与CDN", "#123f6d"},
		{"akamai.net", "Akamai CDN", "云与CDN", "#009bde"},
		{"akamaized.net", "Akamai CDN", "云与CDN", "#009bde"},
		{"fastly.net", "Fastly CDN", "云与CDN", "#ff282d"},
		{"bunnycdn.com", "BunnyCDN", "云与CDN", "#fd8d32"},
		{"vercel.app", "Vercel", "云与CDN", "#111111"},
		{"vercel.com", "Vercel", "云与CDN", "#111111"},
		{"netlify.app", "Netlify", "云与CDN", "#00c7b7"},
		{"heroku.com", "Heroku", "云与CDN", "#430098"},
		{"supabase.co", "Supabase", "云与CDN", "#3ecf8e"},

		// === AI 大模型与开发者服务 ===
		{"openai.com", "OpenAI", "AI大模型", "#10a37f"},
		{"chatgpt.com", "OpenAI", "AI大模型", "#10a37f"},
		{"oaistatic.com", "OpenAI", "AI大模型", "#10a37f"},
		{"oaiusercontent.com", "OpenAI", "AI大模型", "#10a37f"},
		{"anthropic.com", "Claude", "AI大模型", "#d97757"},
		{"claude.ai", "Claude", "AI大模型", "#d97757"},
		{"gemini.google.com", "Gemini", "AI大模型", "#4285f4"},
		{"generativelanguage.googleapis.com", "Gemini", "AI大模型", "#4285f4"},
		{"aistudio.google.com", "Gemini", "AI大模型", "#4285f4"},
		{"deepseek.com", "DeepSeek", "AI大模型", "#4d6bfe"},
		{"moonshot.cn", "Kimi", "AI大模型", "#1e293b"},
		{"kimi.com", "Kimi", "AI大模型", "#1e293b"},
		{"doubao.com", "豆包", "AI大模型", "#2563eb"},
		{"tongyi.aliyun.com", "通义千问", "AI大模型", "#6366f1"},
		{"qwen.ai", "通义千问", "AI大模型", "#6366f1"},
		{"yiyan.baidu.com", "文心一言", "AI大模型", "#2932e1"},
		{"zhipuai.cn", "智谱清言", "AI大模型", "#3b82f6"},
		{"bigmodel.cn", "智谱清言", "AI大模型", "#3b82f6"},
		{"perplexity.ai", "Perplexity", "AI大模型", "#22b8cf"},
		{"poe.com", "Poe", "AI大模型", "#5f4dee"},
		{"midjourney.com", "Midjourney", "AI大模型", "#1e293b"},
		{"huggingface.co", "HuggingFace", "AI大模型", "#ff9d00"},
		{"civitai.com", "Civitai", "AI大模型", "#3b82f6"},
		{"ollama.com", "Ollama", "AI大模型", "#1e293b"},
		{"cursor.sh", "Cursor", "AI大模型", "#1e293b"},
		{"cursor.com", "Cursor", "AI大模型", "#1e293b"},
		{"codeium.com", "Windsurf", "AI大模型", "#09b6a2"},
		{"windsurf.com", "Windsurf", "AI大模型", "#09b6a2"},
		{"suno.com", "Suno AI", "AI大模型", "#1e293b"},
		{"elevenlabs.io", "ElevenLabs", "AI大模型", "#1e293b"},
		{"runwayml.com", "Runway", "AI大模型", "#1e293b"},
		{"grok.com", "Grok", "AI大模型", "#14171a"},
		{"x.ai", "Grok", "AI大模型", "#14171a"},
		{"coze.cn", "扣子 Coze", "AI大模型", "#4f46e5"},
		{"coze.com", "Coze", "AI大模型", "#4f46e5"},
		{"dify.ai", "Dify", "AI大模型", "#155eef"},
		{"siliconflow.cn", "硅基流动", "AI大模型", "#6366f1"},
		{"minimax.io", "MiniMax", "AI大模型", "#ec4899"},
		{"minimaxi.com", "MiniMax", "AI大模型", "#ec4899"},

		// === DevOps / Code / Tools ===
		{"github.com", "GitHub", "办公协同", "#24292f"},
		{"githubusercontent.com", "GitHub", "办公协同", "#24292f"},
		{"githubassets.com", "GitHub", "办公协同", "#24292f"},
		{"gitlab.com", "GitLab", "办公协同", "#fc6d26"},
		{"gitee.com", "Gitee", "办公协同", "#c71d23"},
		{"bitbucket.org", "Bitbucket", "办公协同", "#0052cc"},
		{"docker.com", "Docker", "办公协同", "#2496ed"},
		{"docker.io", "Docker", "办公协同", "#2496ed"},
		{"npmjs.org", "NPM", "办公协同", "#cb3837"},
		{"pypi.org", "PyPI", "办公协同", "#3775a9"},
		{"pythonhosted.org", "PyPI", "办公协同", "#3775a9"},
		{"crates.io", "Rust Crates", "办公协同", "#dea584"},
		{"maven.org", "Maven", "办公协同", "#c71a36"},
		{"nuget.org", "NuGet", "办公协同", "#004880"},
		{"jetbrains.com", "JetBrains", "办公协同", "#fe315d"},
		{"visualstudio.com", "VS Code", "办公协同", "#007acc"},
		{"vscode-cdn.net", "VS Code", "办公协同", "#007acc"},
		{"stackoverflow.com", "StackOverflow", "办公协同", "#f48024"},
		{"huggingface.com", "HuggingFace", "AI大模型", "#ff9d00"},
		{"ubuntu.com", "Ubuntu", "安全与运维", "#e95420"},
		{"debian.org", "Debian", "安全与运维", "#a80030"},
		{"alpinelinux.org", "Alpine", "安全与运维", "#0d597f"},
		{"archlinux.org", "Arch Linux", "安全与运维", "#1793d1"},
		{"fedoraproject.org", "Fedora", "安全与运维", "#51a2da"},
		{"redhat.com", "RedHat", "安全与运维", "#ee0000"},
		{"mikrotik.com", "MikroTik", "安全与运维", "#293239"},
		{"openwrt.org", "OpenWrt", "安全与运维", "#00b5e2"},
		{"ui.com", "Ubiquiti", "安全与运维", "#0559c9"},
		{"sentry.io", "Sentry", "安全与运维", "#362d59"},
		{"datadoghq.com", "Datadog", "安全与运维", "#632ca6"},
		{"grafana.com", "Grafana", "安全与运维", "#f46800"},

		// === Social / Video / Streaming ===
		{"tiktok.com", "TikTok", "视频娱乐", "#1c1c1e"},
		{"tiktokv.com", "TikTok", "视频娱乐", "#1c1c1e"},
		{"tiktokcdn.com", "TikTok", "视频娱乐", "#1c1c1e"},
		{"byteoversea.com", "TikTok", "视频娱乐", "#1c1c1e"},
		{"instagram.com", "Instagram", "社交通讯", "#e4405f"},
		{"cdninstagram.com", "Instagram", "社交通讯", "#e4405f"},
		{"facebook.com", "Facebook", "社交通讯", "#1877f2"},
		{"fbcdn.net", "Facebook", "社交通讯", "#1877f2"},
		{"whatsapp.com", "WhatsApp", "社交通讯", "#25d366"},
		{"whatsapp.net", "WhatsApp", "社交通讯", "#25d366"},
		{"threads.net", "Threads", "社交通讯", "#1c1c1e"},
		{"twitter.com", "X (Twitter)", "社交通讯", "#14171a"},
		{"x.com", "X (Twitter)", "社交通讯", "#14171a"},
		{"twimg.com", "X (Twitter)", "社交通讯", "#14171a"},
		{"telegram.org", "Telegram", "社交通讯", "#26a5e4"},
		{"t.me", "Telegram", "社交通讯", "#26a5e4"},
		{"discord.com", "Discord", "社交通讯", "#5865f2"},
		{"discordapp.com", "Discord", "社交通讯", "#5865f2"},
		{"discord.gg", "Discord", "社交通讯", "#5865f2"},
		{"reddit.com", "Reddit", "社交通讯", "#ff4500"},
		{"redd.it", "Reddit", "社交通讯", "#ff4500"},
		{"pinterest.com", "Pinterest", "社交通讯", "#e60023"},
		{"linkedin.com", "LinkedIn", "办公协同", "#0a66c2"},
		{"snapchat.com", "Snapchat", "社交通讯", "#fffc00"},
		{"twitch.tv", "Twitch", "视频娱乐", "#9146ff"},
		{"ttvnw.net", "Twitch", "视频娱乐", "#9146ff"},
		{"netflix.com", "Netflix", "视频娱乐", "#e50914"},
		{"nflxvideo.net", "Netflix", "视频娱乐", "#e50914"},
		{"disneyplus.com", "Disney+", "视频娱乐", "#113ccf"},
		{"hulu.com", "Hulu", "视频娱乐", "#1ce783"},
		{"hbomax.com", "HBO Max", "视频娱乐", "#5822b4"},
		{"max.com", "Max", "视频娱乐", "#002be7"},
		{"primevideo.com", "Prime Video", "视频娱乐", "#00a8e1"},
		{"spotify.com", "Spotify", "视频娱乐", "#1db954"},
		{"scdn.co", "Spotify", "视频娱乐", "#1db954"},
		{"soundcloud.com", "SoundCloud", "视频娱乐", "#ff5500"},
		{"bilibili.com", "哔哩哔哩", "视频娱乐", "#fb7299"},
		{"bilivideo.com", "哔哩哔哩", "视频娱乐", "#fb7299"},
		{"hdslb.com", "哔哩哔哩", "视频娱乐", "#fb7299"},
		{"douyin.com", "抖音", "视频娱乐", "#1c1c1e"},
		{"douyincdn.com", "抖音", "视频娱乐", "#1c1c1e"},
		{"kuaishou.com", "快手", "视频娱乐", "#ff4906"},
		{"xiaohongshu.com", "小红书", "视频娱乐", "#ff2442"},
		{"xhscdn.com", "小红书", "视频娱乐", "#ff2442"},

		// === Gaming ===
		{"steampowered.com", "Steam", "游戏娱乐", "#1b2838"},
		{"steamstatic.com", "Steam", "游戏娱乐", "#1b2838"},
		{"steamcontent.com", "Steam", "游戏娱乐", "#1b2838"},
		{"steamcommunity.com", "Steam", "游戏娱乐", "#1b2838"},
		{"epicgames.com", "Epic Games", "游戏娱乐", "#2a2a2a"},
		{"unrealengine.com", "Epic Games", "游戏娱乐", "#2a2a2a"},
		{"riotgames.com", "拳头游戏", "游戏娱乐", "#d13639"},
		{"leagueoflegends.com", "拳头游戏", "游戏娱乐", "#d13639"},
		{"playvalorant.com", "拳头游戏", "游戏娱乐", "#d13639"},
		{"blizzard.com", "暴雪战网", "游戏娱乐", "#00aeff"},
		{"battle.net", "暴雪战网", "游戏娱乐", "#00aeff"},
		{"hoyoverse.com", "米哈游", "游戏娱乐", "#38bdf8"},
		{"mihoyo.com", "米哈游", "游戏娱乐", "#38bdf8"},
		{"playstation.com", "PlayStation", "游戏娱乐", "#0070d1"},
		{"playstation.net", "PlayStation", "游戏娱乐", "#0070d1"},
		{"xbox.com", "Xbox", "游戏娱乐", "#107c10"},
		{"xboxlive.com", "Xbox", "游戏娱乐", "#107c10"},
		{"nintendo.com", "Nintendo", "游戏娱乐", "#e60012"},
		{"nintendo.net", "Nintendo", "游戏娱乐", "#e60012"},
		{"ea.com", "EA游戏", "游戏娱乐", "#ff4747"},
		{"ubisoft.com", "育碧 Ubisoft", "游戏娱乐", "#0070ff"},
		{"rockstargames.com", "Rockstar", "游戏娱乐", "#f59e0b"},
		{"roblox.com", "Roblox", "游戏娱乐", "#1e293b"},
		{"minecraft.net", "Minecraft", "游戏娱乐", "#3c8527"},
		{"mojang.com", "Minecraft", "游戏娱乐", "#3c8527"},

		// === 车联网 / 新能源汽车 ===
		{"tesla.com", "Tesla", "出行生活", "#cc0000"},
		{"teslamotors.com", "Tesla", "出行生活", "#cc0000"},
		{"nio.com", "蔚来汽车", "出行生活", "#00bebe"},
		{"xiaopeng.com", "小鹏汽车", "出行生活", "#2563eb"},
		{"lixiang.com", "理想汽车", "出行生活", "#004d40"},
		{"byd.com", "比亚迪", "出行生活", "#d7191a"},
		{"zeekrlife.com", "极氪", "出行生活", "#1e293b"},
		{"xiaomi-ev.com", "小米汽车", "出行生活", "#ff6700"},

		// === Smart Home / NAS / IoT / Remote ===
		{"aqara.com", "Aqara绿米", "智能家居", "#2563eb"},
		{"tuya.com", "涂鸦智能", "智能家居", "#ff4800"},
		{"tuyacn.com", "涂鸦智能", "智能家居", "#ff4800"},
		{"mi.com", "米家", "智能家居", "#ff6700"},
		{"xiaomi.com", "小米", "智能家居", "#ff6700"},
		{"yeelight.com", "Yeelight", "智能家居", "#ff5a5f"},
		{"roborock.com", "石头智能", "智能家居", "#1e293b"},
		{"ecovacs.com", "科沃斯", "智能家居", "#0284c7"},
		{"dreame.tech", "追觅", "智能家居", "#d97706"},
		{"synology.com", "群晖 Synology", "智能家居", "#0084d6"},
		{"qnap.com", "威联通 QNAP", "智能家居", "#0055a5"},
		{"home-assistant.io", "HomeAssistant", "智能家居", "#18bcf2"},
		{"ui.direct", "Ubiquiti", "智能家居", "#0559c9"},
		{"tailscale.com", "Tailscale", "VPN与P2P", "#242424"},
		{"zerotier.com", "ZeroTier", "VPN与P2P", "#ffb441"},
		{"wireguard.com", "WireGuard VPN", "VPN与P2P", "#a855f7"},
		{"todesk.com", "ToDesk", "办公协同", "#0066ff"},
		{"anydesk.com", "AnyDesk", "办公协同", "#ef443b"},
		{"rustdesk.com", "RustDesk", "办公协同", "#0284c7"},
		{"teamviewer.com", "TeamViewer", "办公协同", "#00458f"},
		{"oray.com", "向日葵远程", "办公协同", "#e11d48"},

		// === Productivity / Design / SaaS ===
		{"microsoft.com", "Microsoft", "办公协同", "#00a4ef"},
		{"live.com", "Microsoft", "办公协同", "#00a4ef"},
		{"office.com", "Microsoft", "办公协同", "#d83b01"},
		{"office365.com", "Microsoft", "办公协同", "#d83b01"},
		{"msn.com", "Microsoft", "新闻资讯", "#00a4ef"},
		{"bing.com", "Bing", "网页浏览", "#008373"},
		{"notion.so", "Notion", "办公协同", "#1e293b"},
		{"notion.site", "Notion", "办公协同", "#1e293b"},
		{"figma.com", "Figma", "办公协同", "#f24e1e"},
		{"canva.com", "Canva", "办公协同", "#00c4cc"},
		{"adobe.com", "Adobe", "办公协同", "#ff0000"},
		{"slack.com", "Slack", "办公协同", "#4a154b"},
		{"zoom.us", "Zoom", "办公协同", "#2d8cff"},
		{"atlassian.com", "Atlassian", "办公协同", "#0052cc"},
		{"feishu.cn", "飞书", "办公协同", "#00d6b9"},
		{"dingtalk.com", "钉钉", "办公协同", "#0089ff"},
		{"wps.cn", "WPS", "办公协同", "#ea3323"},
		{"wps.com", "WPS", "办公协同", "#ea3323"},

		// === E-Commerce / Shopping ===
		{"taobao.com", "淘宝", "购物电商", "#ff5000"},
		{"tmall.com", "天猫", "购物电商", "#ff0036"},
		{"jd.com", "京东", "购物电商", "#e1251b"},
		{"pinduoduo.com", "拼多多", "购物电商", "#e02e24"},
		{"amazon.com", "Amazon", "购物电商", "#ff9900"},
		{"ebay.com", "eBay", "购物电商", "#e53238"},
		{"shopify.com", "Shopify", "购物电商", "#96bf48"},
		{"temu.com", "Temu", "购物电商", "#fb7701"},
		{"shein.com", "SHEIN", "购物电商", "#1e293b"},
		{"aliexpress.com", "AliExpress", "购物电商", "#e62e04"},
	}
	for _, p := range packs {
		suffixRules[strings.ToLower(p.domain)] = AppInfo{
			App: p.app, Category: p.cat, Color: p.color, IsRealApp: true,
		}
	}
}

var customRules = make(map[string]AppInfo)

func SetCustomRules(list []struct {
	Domain, App, Category, Color string
}) {
	next := make(map[string]AppInfo, len(list))
	for _, r := range list {
		d := strings.ToLower(strings.TrimSpace(r.Domain))
		if d == "" || r.App == "" {
			continue
		}
		cat := r.Category
		if cat == "" {
			cat = "其他协议"
		}
		col := r.Color
		if col == "" {
			col = "#0099cc"
		}
		next[d] = AppInfo{App: r.App, Category: cat, Color: col, IsRealApp: true}
	}
	customRules = next
}

func matchSuffixMap(d string, m map[string]AppInfo) (AppInfo, bool) {
	if d == "" || len(m) == 0 {
		return AppInfo{}, false
	}
	cur := d
	for {
		if info, ok := m[cur]; ok {
			return info, true
		}
		idx := strings.IndexByte(cur, '.')
		if idx < 0 || idx+1 >= len(cur) {
			break
		}
		cur = cur[idx+1:]
	}
	return AppInfo{}, false
}
