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
	{"byteacctimg", "抖音", "视频娱乐", "#1c1c1e"},
	{"retouchpics", "醒图", "视频娱乐", "#ec4899"},
	{"ulikecam", "轻颜相机", "视频娱乐", "#f43f5e"},
	{"capcut", "剪映", "视频娱乐", "#1c1c1e"},
	{"bytegecko", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"bytegoofy", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"bytetos", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"cdnhwc", "华为云", "云与CDN", "#cf0a2c"},
	{"hc-cdn", "华为云", "云与CDN", "#cf0a2c"},
	{"myhuaweicloud", "华为云", "云与CDN", "#cf0a2c"},
	{"kunlun", "阿里云", "云与CDN", "#ff6a00"},
	{"mmstat", "阿里云", "云与CDN", "#ff6a00"},
	{"umeng", "阿里云", "云与CDN", "#ff6a00"},
	{"yximgs", "快手", "视频娱乐", "#ff4906"},
	{"kwaicdn", "快手", "视频娱乐", "#ff4906"},
	{"kwimgs", "快手", "视频娱乐", "#ff4906"},
	{"bdydns", "百度云", "云与CDN", "#2932e1"},
	{"bdimg", "百度", "云与CDN", "#2932e1"},
	{"qcloudcdn", "腾讯云", "云与CDN", "#0052d9"},
	{"gtimg", "QQ", "社交通讯", "#12b7f5"},
	{"idqqimg", "QQ", "社交通讯", "#12b7f5"},
	{"msftconnecttest", "Microsoft", "办公协同", "#00a4ef"},
	{"msedge", "Microsoft", "办公协同", "#00a4ef"},
	{"app-measurement", "Google", "云与CDN", "#4285f4"},
	{"crashlytics", "Google", "云与CDN", "#4285f4"},
	{"akadns", "Akamai CDN", "云与CDN", "#009bde"},
	{"edgesuite", "Akamai CDN", "云与CDN", "#009bde"},
	{"edgekey", "Akamai CDN", "云与CDN", "#009bde"},
	// === TikTok 国际版 (优先于 AWS / Akamai / 字节国内规则) ===
	{"tiktok", "TikTok", "视频娱乐", "#1c1c1e"},
	{"tik-tokapi", "TikTok", "视频娱乐", "#1c1c1e"},
	{"byteoversea", "TikTok", "视频娱乐", "#1c1c1e"},
	{"ibytedtos", "TikTok", "视频娱乐", "#1c1c1e"},
	{"ibyteimg", "TikTok", "视频娱乐", "#1c1c1e"},
	{"muscdn", "TikTok", "视频娱乐", "#1c1c1e"},
	{"musical.ly", "TikTok", "视频娱乐", "#1c1c1e"},
	{"ttwstatic", "TikTok", "视频娱乐", "#1c1c1e"},
	{"ttlivecdn", "TikTok", "视频娱乐", "#1c1c1e"},
	{"sgsnssdk", "TikTok", "视频娱乐", "#1c1c1e"},
	{"isnssdk", "TikTok", "视频娱乐", "#1c1c1e"},
	{"byteintl", "TikTok", "视频娱乐", "#1c1c1e"},

	// === 小说 / 听书 / 短剧平台 (优先于字节/阿里通用规则) ===
	{"novelfm", "番茄畅听", "小说短剧", "#f97316"},
	{"fanqienovel", "番茄小说", "小说短剧", "#f97316"},
	{"fanqiequ", "番茄小说", "小说短剧", "#f97316"},
	{"changdunovel", "番茄小说", "小说短剧", "#f97316"},
	{"fqnovel", "番茄小说", "小说短剧", "#f97316"},
	{"novelquickapp", "红果短剧", "小说短剧", "#ef4444"},
	{"hongguoduanju", "红果短剧", "小说短剧", "#ef4444"},
	{"hongguojuchang", "红果短剧", "小说短剧", "#ef4444"},
	{"hongguoapp", "红果短剧", "小说短剧", "#ef4444"},
	{"hongguo.com", "红果短剧", "小说短剧", "#ef4444"},
	{"reading.snssdk", "红果短剧", "小说短剧", "#ef4444"},
	{"qznovel", "红果短剧", "小说短剧", "#ef4444"},
	{"hainanyuyue", "红果短剧", "小说短剧", "#ef4444"},
	{"qimao", "七猫小说", "小说短剧", "#eab308"},
	{"wtzw.com", "七猫小说", "小说短剧", "#eab308"},
	{"kuaikaw.cn", "河马剧场", "小说短剧", "#8b5cf6"},
	{"hemajuchang", "河马剧场", "小说短剧", "#8b5cf6"},
	{"qidian", "起点读书", "小说短剧", "#e11d48"},
	{"yuewen", "阅文集团", "小说短剧", "#e11d48"},
	{"zongheng.com", "纵横小说", "小说短剧", "#b91c1c"},
	{"jjwxc", "晋江文学", "小说短剧", "#16a34a"},
	{"shuqi", "书旗小说", "小说短剧", "#f59e0b"},
	{"ireader", "掌阅", "小说短剧", "#2563eb"},
	{"zhangyue", "掌阅", "小说短剧", "#2563eb"},
	{"17k.com", "17K小说", "小说短剧", "#ea580c"},
	{"faloo", "飞卢小说", "小说短剧", "#0284c7"},
	{"lanrentingshu", "懒人听书", "小说短剧", "#0ea5e9"},
	{"ciweimao", "刺猬猫", "小说短剧", "#ec4899"},
	{"sfacg", "SF轻小说", "小说短剧", "#f43f5e"},
	{"dejian", "得间小说", "小说短剧", "#10b981"},

	// === 网络测速 (Speedtest) ===
	{"speedtest", "Speedtest", "网络测速", "#00bcd4"},
	{"ooklaserver", "Speedtest", "网络测速", "#00bcd4"},
	{"ookla", "Speedtest", "网络测速", "#00bcd4"},
	{"speed.cloudflare", "Speedtest", "网络测速", "#00bcd4"},
	{"fast.com", "Speedtest", "网络测速", "#00bcd4"},
	{"librespeed", "Speedtest", "网络测速", "#00bcd4"},
	{"nperf.com", "Speedtest", "网络测速", "#00bcd4"},
	{"measurementlab", "Speedtest", "网络测速", "#00bcd4"},
	{"test.ustc.edu.cn", "Speedtest", "网络测速", "#00bcd4"},
	{"speed.189", "Speedtest", "网络测速", "#00bcd4"},
	{"speed.10086", "Speedtest", "网络测速", "#00bcd4"},

	// === 公共 DNS / DoH ===
	{"alidns", "阿里云 DNS", "基础网络", "#ff6a00"},
	{"doh.pub", "腾讯云 DNS", "基础网络", "#0052d9"},
	{"dnspod", "腾讯云 DNS", "基础网络", "#0052d9"},
	{"dns.google", "Google", "基础网络", "#4285f4"},
	{"cloudflare-dns", "Cloudflare", "基础网络", "#f38020"},
	{"quad9", "Quad9 DNS", "基础网络", "#2563eb"},
	{"360.cn", "360安全", "安全与运维", "#16a34a"},
	{"qhimg", "360安全", "安全与运维", "#16a34a"},
	{"qianxin", "奇安信", "安全与运维", "#dc2626"},
	{"sangfor", "深信服", "安全与运维", "#2563eb"},

	// === AI 大模型 ===
	{"deepseek", "DeepSeek", "AI大模型", "#4d6bfe"},
	{"moonshot.cn", "Kimi", "AI大模型", "#1e293b"},
	{"kimi.ai", "Kimi", "AI大模型", "#1e293b"},
	{"doubao.com", "豆包", "AI大模型", "#2563eb"},
	{"tongyi.aliyun", "通义千问", "AI大模型", "#6366f1"},
	{"qianwen", "通义千问", "AI大模型", "#6366f1"},
	{"yiyan.baidu", "文心一言", "AI大模型", "#2932e1"},
	{"chatglm", "智谱清言", "AI大模型", "#3b82f6"},
	{"zhipuai", "智谱清言", "AI大模型", "#3b82f6"},
	{"xfyun.cn", "讯飞星火", "AI大模型", "#0070f3"},
	{"metaso", "秘塔AI", "AI大模型", "#6366f1"},
	{"perplexity", "Perplexity", "AI大模型", "#22b8cf"},
	{"openai", "OpenAI", "AI大模型", "#10a37f"},
	{"chatgpt", "OpenAI", "AI大模型", "#10a37f"},
	{"anthropic", "Claude", "AI大模型", "#d97757"},
	{"claude.ai", "Claude", "AI大模型", "#d97757"},
	{"gemini.google", "Gemini", "AI大模型", "#4285f4"},
	{"midjourney", "Midjourney", "AI大模型", "#1e293b"},
	{"huggingface", "HuggingFace", "AI大模型", "#ff9d00"},
	{"cursor.sh", "Cursor", "AI大模型", "#1e293b"},
	{"cursor.com", "Cursor", "AI大模型", "#1e293b"},
	{"githubcopilot", "GitHub", "AI大模型", "#24292f"},

	// === 办公协同 / 企业 / 开发 ===
	{"feishu", "飞书", "办公协同", "#00d6b9"},
	{"larksuite", "飞书", "办公协同", "#00d6b9"},
	{"dingtalk", "钉钉", "办公协同", "#0089ff"},
	{"aliwork", "钉钉", "办公协同", "#0089ff"},
	{"work.weixin", "企业微信", "办公协同", "#0082ef"},
	{"meeting.tencent", "腾讯会议", "办公协同", "#006eff"},
	{"docs.qq.com", "腾讯文档", "办公协同", "#1e6fff"},
	{"wj.qq.com", "腾讯问卷", "办公协同", "#0ea5e9"},
	{"shimo.im", "石墨文档", "办公协同", "#1e293b"},
	{"yuque.com", "语雀", "办公协同", "#00b96b"},
	{"mubu.com", "幕布", "办公协同", "#6366f1"},
	{"processon", "ProcessOn", "办公协同", "#0284c7"},
	{"wjx.cn", "问卷星", "办公协同", "#2563eb"},
	{"yonyou", "用友软件", "办公协同", "#e11d48"},
	{"kingdee", "金蝶云", "办公协同", "#2563eb"},
	{"weaver.com", "泛微OA", "办公协同", "#0284c7"},
	{"zoom.us", "Zoom", "办公协同", "#2d8cff"},
	{"slack.com", "Slack", "办公协同", "#4a154b"},
	{"notion.so", "Notion", "办公协同", "#1e293b"},
	{"figma.com", "Figma", "办公协同", "#f24e1e"},
	{"lanhuapp", "蓝湖", "办公协同", "#3b82f6"},
	{"atlassian", "Atlassian", "办公协同", "#0052cc"},
	{"wps.cn", "WPS", "办公协同", "#ea3323"},
	{"wpscdn", "WPS", "办公协同", "#ea3323"},
	{"todesk", "ToDesk", "办公协同", "#0066ff"},
	{"oray", "向日葵远程", "办公协同", "#e11d48"},
	{"sunlogin", "向日葵远程", "办公协同", "#e11d48"},
	{"anydesk", "AnyDesk", "办公协同", "#ef443b"},
	{"rustdesk", "RustDesk", "办公协同", "#0284c7"},
	{"teamviewer", "TeamViewer", "办公协同", "#00458f"},
	{"parsec", "Parsec", "办公协同", "#3b82f6"},
	{"tailscale", "Tailscale", "VPN与P2P", "#242424"},
	{"zerotier", "ZeroTier", "VPN与P2P", "#ffb441"},
	{"mail.163", "网易邮箱", "办公协同", "#d43c33"},
	{"mail.qq", "QQ邮箱", "办公协同", "#1e6fff"},
	{"github", "GitHub", "办公协同", "#24292f"},
	{"githubusercontent", "GitHub", "办公协同", "#24292f"},
	{"gitlab", "GitLab", "办公协同", "#fc6d26"},
	{"gitee.com", "Gitee", "办公协同", "#c71d23"},
	{"csdn.net", "CSDN", "办公协同", "#fc5531"},
	{"juejin.cn", "掘金", "办公协同", "#1e80ff"},
	{"cnblogs", "博客园", "办公协同", "#2b7ec1"},
	{"segmentfault", "思否", "办公协同", "#009a61"},
	{"microsoft", "Microsoft", "办公协同", "#00a4ef"},
	{"office", "Microsoft", "办公协同", "#d83b01"},
	{"windowsupdate", "Microsoft", "办公协同", "#0078d4"},
	{"docker", "Docker", "办公协同", "#2496ed"},
	{"npmjs", "NPM", "办公协同", "#cb3837"},
	{"pypi.org", "PyPI", "办公协同", "#3775a9"},
	{"goproxy", "GoProxy", "办公协同", "#00add8"},
	{"vercel", "Vercel", "办公协同", "#111111"},

	// === 网盘与下载 ===
	{"pan.baidu", "百度网盘", "网盘传输", "#06a7ff"},
	{"baidupcs", "百度网盘", "网盘传输", "#06a7ff"},
	{"aliyundrive", "阿里云盘", "网盘传输", "#6366f1"},
	{"alipan", "阿里云盘", "网盘传输", "#6366f1"},
	{"pan.quark", "夸克网盘", "网盘传输", "#2563eb"},
	{"115.com", "115网盘", "网盘传输", "#1d4ed8"},
	{"123pan", "123云盘", "网盘传输", "#3b82f6"},
	{"weiyun", "腾讯微云", "网盘传输", "#2563eb"},
	{"jianguoyun", "坚果云", "网盘传输", "#0ea5e9"},
	{"cloud.189.cn", "天翼云盘", "网盘传输", "#ff6600"},
	{"caiyun.139", "移动云盘", "网盘传输", "#0085d0"},
	{"onedrive", "OneDrive", "网盘传输", "#0078d4"},
	{"drive.google", "Google Drive", "网盘传输", "#1fa463"},
	{"dropbox", "Dropbox", "网盘传输", "#0061ff"},
	{"mega.nz", "MEGA", "网盘传输", "#d9272e"},
	{"xunlei", "迅雷", "网盘传输", "#0099ff"},
	{"sandai.net", "迅雷", "网盘传输", "#0099ff"},

	// === 视频直播、音乐与阅读 ===
	{"bilivideo", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"bilibili", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"b23.tv", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"hdslb", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"biliapi", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"acfun", "AcFun", "视频娱乐", "#fd4c5d"},
	{"ixigua", "西瓜视频", "视频娱乐", "#fe2c55"},
	{"douyin", "抖音", "视频娱乐", "#1c1c1e"},
	{"amemv", "抖音", "视频娱乐", "#1c1c1e"},
	{"iesdouyin", "抖音", "视频娱乐", "#1c1c1e"},
	{"snssdk", "抖音", "视频娱乐", "#1c1c1e"},
	{"byteimg", "抖音", "视频娱乐", "#1c1c1e"},
	{"bytedance", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"v.qq.com", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"video.qq", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"vv.video.qq", "腾讯视频", "视频娱乐", "#ff7a00"},
	{"y.qq.com", "QQ音乐", "视频娱乐", "#31c27c"},
	{"kg.qq.com", "全民K歌", "视频娱乐", "#ff4d4f"},
	{"iqiyi", "爱奇艺", "视频娱乐", "#00be06"},
	{"gitv.tv", "爱奇艺", "视频娱乐", "#00be06"},
	{"qy.net", "爱奇艺", "视频娱乐", "#00be06"},
	{"youku", "优酷", "视频娱乐", "#00a0e9"},
	{"ykimg", "优酷", "视频娱乐", "#00a0e9"},
	{"mgtv", "芒果TV", "视频娱乐", "#ff5f00"},
	{"hunantv", "芒果TV", "视频娱乐", "#ff5f00"},
	{"yangshipin", "央视频", "视频娱乐", "#e60012"},
	{"cctv.com", "央视影音", "视频娱乐", "#e60012"},
	{"tv.sohu", "搜狐视频", "视频娱乐", "#f59e0b"},
	{"pptv", "PPTV", "视频娱乐", "#2563eb"},
	{"miguvideo", "咪咕视频", "视频娱乐", "#e4007f"},
	{"cmvideo", "咪咕视频", "视频娱乐", "#e4007f"},
	{"huya", "虎牙直播", "视频娱乐", "#ffa800"},
	{"douyu", "斗鱼直播", "视频娱乐", "#ff5d23"},
	{"yy.com", "YY直播", "视频娱乐", "#facc15"},
	{"inke.cn", "映客直播", "视频娱乐", "#06b6d4"},
	{"kuaishou", "快手", "视频娱乐", "#ff4906"},
	{"gifshow", "快手", "视频娱乐", "#ff4906"},
	{"xiaohongshu", "小红书", "视频娱乐", "#ff2442"},
	{"xhscdn", "小红书", "视频娱乐", "#ff2442"},
	{"music.163", "网易云音乐", "视频娱乐", "#d43c33"},
	{"126.net", "网易云音乐", "视频娱乐", "#d43c33"},
	{"kugou", "酷狗音乐", "视频娱乐", "#2ca2f9"},
	{"kuwo", "酷我音乐", "视频娱乐", "#f59e0b"},
	{"ximalaya", "喜马拉雅", "视频娱乐", "#f86442"},
	{"qingting", "蜻蜓FM", "视频娱乐", "#ef4444"},
	{"lizhi.fm", "荔枝FM", "视频娱乐", "#10b981"},
	{"xiaoyuzhoufm", "小宇宙", "视频娱乐", "#06b6d4"},
	{"weread.qq", "微信读书", "新闻资讯", "#0ea5e9"},
	{"qidian.com", "起点读书", "新闻资讯", "#e11d48"},
	{"fanqienovel", "番茄小说", "新闻资讯", "#f97316"},
	{"youtube", "YouTube", "视频娱乐", "#ff0000"},
	{"googlevideo", "YouTube", "视频娱乐", "#ff0000"},
	{"ytimg", "YouTube", "视频娱乐", "#ff0000"},
	{"netflix", "Netflix", "视频娱乐", "#e50914"},
	{"nflxvideo", "Netflix", "视频娱乐", "#e50914"},
	{"twitch", "Twitch", "视频娱乐", "#9146ff"},
	{"spotify", "Spotify", "视频娱乐", "#1db954"},
	{"disneyplus", "Disney+", "视频娱乐", "#113ccf"},

	// === 社交通讯与新闻资讯 ===
	{"tenpay", "微信", "社交通讯", "#07c160"},
	{"weixin", "微信", "社交通讯", "#07c160"},
	{"wechat", "微信", "社交通讯", "#07c160"},
	{"wx.qq.com", "微信", "社交通讯", "#07c160"},
	{"servicewechat", "微信", "社交通讯", "#07c160"},
	{"qpic.cn", "QQ", "社交通讯", "#12b7f5"},
	{"qlogo.cn", "QQ", "社交通讯", "#12b7f5"},
	{"qq.com", "QQ", "社交通讯", "#12b7f5"},
	{"weibo", "微博", "社交通讯", "#e6162d"},
	{"sinaimg", "微博", "社交通讯", "#e6162d"},
	{"weibocdn", "微博", "社交通讯", "#e6162d"},
	{"zhihu", "知乎", "社交通讯", "#0066ff"},
	{"zhimg", "知乎", "社交通讯", "#0066ff"},
	{"douban", "豆瓣", "社交通讯", "#007722"},
	{"tieba.baidu", "百度贴吧", "社交通讯", "#3385ff"},
	{"hupu", "虎扑", "社交通讯", "#c01e2f"},
	{"dongqiudi", "懂球帝", "新闻资讯", "#16a34a"},
	{"zhibo8", "直播吧", "新闻资讯", "#2563eb"},
	{"autohome", "汽车之家", "新闻资讯", "#2563eb"},
	{"dongchedi", "懂车帝", "新闻资讯", "#facc15"},
	{"nga.cn", "NGA", "社交通讯", "#d97706"},
	{"ngacn", "NGA", "社交通讯", "#d97706"},
	{"v2ex.com", "V2EX", "社交通讯", "#1e293b"},
	{"soulapp", "Soul", "社交通讯", "#6366f1"},
	{"immomo", "陌陌", "社交通讯", "#0ea5e9"},
	{"pstatp", "今日头条", "新闻资讯", "#ed4040"},
	{"toutiao", "今日头条", "新闻资讯", "#ed4040"},
	{"thepaper.cn", "澎湃新闻", "新闻资讯", "#1e293b"},
	{"guancha.cn", "观察者网", "新闻资讯", "#dc2626"},
	{"ifeng", "凤凰网", "新闻资讯", "#c8161d"},
	{"36kr", "36氪", "新闻资讯", "#2563eb"},
	{"huxiu", "虎嗅", "新闻资讯", "#1e293b"},
	{"sspai", "少数派", "新闻资讯", "#d7191a"},
	{"news.cn", "新华网", "新闻资讯", "#dc2626"},
	{"people.com.cn", "人民网", "新闻资讯", "#dc2626"},
	{"telegram", "Telegram", "社交通讯", "#26a5e4"},
	{"t.me", "Telegram", "社交通讯", "#26a5e4"},
	{"discord", "Discord", "社交通讯", "#5865f2"},
	{"whatsapp", "WhatsApp", "社交通讯", "#25d366"},
	{"signal.org", "Signal", "社交通讯", "#3a76f0"},
	{"line.me", "LINE", "社交通讯", "#00c300"},
	{"twitter", "X (Twitter)", "社交通讯", "#14171a"},
	{"x.com", "X (Twitter)", "社交通讯", "#14171a"},
	{"twimg", "X (Twitter)", "社交通讯", "#14171a"},
	{"facebook", "Facebook", "社交通讯", "#1877f2"},
	{"fbcdn", "Facebook", "社交通讯", "#1877f2"},
	{"instagram", "Instagram", "社交通讯", "#e4405f"},
	{"reddit", "Reddit", "社交通讯", "#ff4500"},

	// === 购物电商、生活出行、快递、房产招聘 ===
	{"goofish", "闲鱼", "购物电商", "#ffe60f"},
	{"2.taobao", "闲鱼", "购物电商", "#ffe60f"},
	{"taobao", "淘宝", "购物电商", "#ff5000"},
	{"tmall", "天猫", "购物电商", "#ff0036"},
	{"alicdn", "淘宝", "购物电商", "#ff5000"},
	{"tbcdn", "淘宝", "购物电商", "#ff5000"},
	{"jd.com", "京东", "购物电商", "#e1251b"},
	{"360buyimg", "京东", "购物电商", "#e1251b"},
	{"jdl.com", "京东物流", "购物电商", "#e1251b"},
	{"pinduoduo", "拼多多", "购物电商", "#e02e24"},
	{"yangkeduo", "拼多多", "购物电商", "#e02e24"},
	{"vip.com", "唯品会", "购物电商", "#f10180"},
	{"dewu", "得物", "购物电商", "#000000"},
	{"poizon", "得物", "购物电商", "#000000"},
	{"zhuanzhuan", "转转", "购物电商", "#ef4444"},
	{"suning", "苏宁易购", "购物电商", "#f59e0b"},
	{"you.163", "网易严选", "购物电商", "#b4282d"},
	{"vmall.com", "华为商城", "购物电商", "#cf0a2c"},
	{"smzdm", "什么值得买", "购物电商", "#e60012"},
	{"freshippo", "盒马", "购物电商", "#00a0e9"},
	{"samsclub", "山姆会员", "购物电商", "#0067a5"},
	{"ddxq.mobi", "叮咚买菜", "购物电商", "#16a34a"},
	{"meituan", "美团", "出行生活", "#ffc300"},
	{"sankuai", "美团", "出行生活", "#ffc300"},
	{"dianping", "大众点评", "出行生活", "#ff6633"},
	{"ele.me", "饿了么", "出行生活", "#0097ff"},
	{"elemecdn", "饿了么", "出行生活", "#0097ff"},
	{"luckincoffee", "瑞幸咖啡", "出行生活", "#1e3a8a"},
	{"amap", "高德地图", "出行生活", "#2f89fc"},
	{"autonavi", "高德地图", "出行生活", "#2f89fc"},
	{"map.baidu", "百度地图", "出行生活", "#3385ff"},
	{"didiglobal", "滴滴出行", "出行生活", "#ff7d00"},
	{"xiaojukeji", "滴滴出行", "出行生活", "#ff7d00"},
	{"hellobike", "哈啰出行", "出行生活", "#0099ff"},
	{"t3go.cn", "T3出行", "出行生活", "#2563eb"},
	{"huolala", "货拉拉", "出行生活", "#f97316"},
	{"12306.cn", "铁路12306", "出行生活", "#2b85e4"},
	{"ctrip", "携程旅行", "出行生活", "#0086f6"},
	{"qunar", "去哪儿", "出行生活", "#00afc7"},
	{"fliggy", "飞猪", "出行生活", "#ffc300"},
	{"ly.com", "同程旅行", "出行生活", "#10b981"},
	{"mafengwo", "马蜂窝", "出行生活", "#facc15"},
	{"umetrip", "航旅纵横", "出行生活", "#2563eb"},
	{"variflight", "飞常准", "出行生活", "#0ea5e9"},
	{"sf-express", "顺丰速运", "出行生活", "#1c1c1e"},
	{"cainiao", "菜鸟", "出行生活", "#0066ff"},
	{"zto.com", "中通快递", "出行生活", "#0066ff"},
	{"yto.net", "圆通速递", "出行生活", "#6366f1"},
	{"yundaex", "韵达快递", "出行生活", "#f59e0b"},
	{"sto.cn", "申通快递", "出行生活", "#f97316"},
	{"jtexpress", "极兔速递", "出行生活", "#e11d48"},
	{"ems.com.cn", "中国邮政", "出行生活", "#16a34a"},
	{"ke.com", "贝壳找房", "出行生活", "#2563eb"},
	{"lianjia", "链家", "出行生活", "#16a34a"},
	{"58.com", "58同城", "出行生活", "#f97316"},
	{"zhipin", "BOSS直聘", "出行生活", "#00bebd"},

	// === 金融银行政务 ===
	{"alipay", "支付宝", "金融支付", "#1677ff"},
	{"alipayobjects", "支付宝", "金融支付", "#1677ff"},
	{"icbc", "工商银行", "金融支付", "#c8161d"},
	{"cmbchina", "招商银行", "金融支付", "#c8161d"},
	{"ccb.com", "建设银行", "金融支付", "#0066b3"},
	{"abchina", "农业银行", "金融支付", "#00997a"},
	{"boc.cn", "中国银行", "金融支付", "#a71e32"},
	{"bankcomm", "交通银行", "金融支付", "#003f8a"},
	{"psbc.com", "邮储银行", "金融支付", "#007f40"},
	{"spdb.com", "浦发银行", "金融支付", "#1e3a8a"},
	{"cmbc.com", "民生银行", "金融支付", "#0284c7"},
	{"cib.com.cn", "兴业银行", "金融支付", "#1d4ed8"},
	{"cebbank", "光大银行", "金融支付", "#7e22ce"},
	{"citicbank", "中信银行", "金融支付", "#dc2626"},
	{"pingan", "平安银行", "金融支付", "#ea580c"},
	{"webank", "微众银行", "金融支付", "#2563eb"},
	{"unionpay", "云闪付", "金融支付", "#e60012"},
	{"95516", "云闪付", "金融支付", "#e60012"},
	{"eastmoney", "东方财富", "金融支付", "#ef4444"},
	{"10jqka", "同花顺", "金融支付", "#ef4444"},
	{"xueqiu", "雪球", "金融支付", "#0284c7"},
	{"futunn", "富途牛牛", "金融支付", "#f97316"},
	{"122.gov.cn", "交管12123", "金融支付", "#1e40af"},
	{"chinatax", "个人所得税", "金融支付", "#dc2626"},
	{"chsi.com", "学信网", "金融支付", "#0ea5e9"},
	{"gjzwfw", "国家政务", "金融支付", "#dc2626"},

	// === 智能家居 / IoT / NAS ===
	{"ys7.com", "萤石云", "智能家居", "#2563eb"},
	{"ezviz", "萤石云", "智能家居", "#2563eb"},
	{"hik-connect", "海康互联", "智能家居", "#dc2626"},
	{"lechange", "乐橙监控", "智能家居", "#f97316"},
	{"miwifi.com", "米家", "智能家居", "#ff6700"},
	{"io.mi.com", "米家", "智能家居", "#ff6700"},
	{"mi.com", "米家", "智能家居", "#ff6700"},
	{"xiaomi", "米家", "智能家居", "#ff6700"},
	{"midea", "美的美居", "智能家居", "#0099ff"},
	{"msmart", "美的美居", "智能家居", "#0099ff"},
	{"aqara", "Aqara绿米", "智能家居", "#2563eb"},
	{"lumiunited", "Aqara绿米", "智能家居", "#2563eb"},
	{"tuya", "涂鸦智能", "智能家居", "#ff4800"},
	{"coolkit", "易微联", "智能家居", "#0ea5e9"},
	{"roborock", "石头智能", "智能家居", "#1e293b"},
	{"ecovacs", "科沃斯", "智能家居", "#0284c7"},
	{"dreame", "追觅", "智能家居", "#d97706"},
	{"dji.com", "大疆 DJI", "智能家居", "#1e293b"},
	{"haier", "海尔智家", "智能家居", "#005aab"},
	{"gree.com", "格力+", "智能家居", "#0072bc"},
	{"home-assistant", "HomeAssistant", "智能家居", "#18bcf2"},
	{"synology", "群晖 Synology", "智能家居", "#0084d6"},
	{"quickconnect", "群晖 Synology", "智能家居", "#0084d6"},
	{"myqnapcloud", "威联通 QNAP", "智能家居", "#0055a5"},
	{"fnnas", "飞牛 fnOS", "智能家居", "#2563eb"},
	{"zspace", "极空间", "智能家居", "#1e293b"},
	{"sctel.com.cn", "天翼网关", "智能家居", "#0066cc"},

	// === 游戏娱乐 ===
	{"ds.163.com", "网易大神", "游戏娱乐", "#1e293b"},
	{"nie.netease", "网易游戏", "游戏娱乐", "#d43c33"},
	{"netease", "网易大神", "游戏娱乐", "#d43c33"},
	{"steam", "Steam", "游戏娱乐", "#1b2838"},
	{"valvesoftware", "Steam", "游戏娱乐", "#1b2838"},
	{"epicgames", "Epic Games", "游戏娱乐", "#2a2a2a"},
	{"wegame", "WeGame", "游戏娱乐", "#f59e0b"},
	{"riotgames", "拳头游戏", "游戏娱乐", "#d13639"},
	{"blizzard", "暴雪战网", "游戏娱乐", "#00aeff"},
	{"battlenet", "暴雪战网", "游戏娱乐", "#00aeff"},
	{"mihoyo", "米哈游", "游戏娱乐", "#38bdf8"},
	{"hoyoverse", "米哈游", "游戏娱乐", "#38bdf8"},
	{"yuanshen", "原神", "游戏娱乐", "#38bdf8"},
	{"pvp.qq.com", "王者荣耀", "游戏娱乐", "#d97706"},
	{"taptap", "TapTap", "游戏娱乐", "#00d9c5"},
	{"4399.com", "4399游戏", "游戏娱乐", "#16a34a"},
	{"3839.com", "好游快爆", "游戏娱乐", "#f97316"},
	{"gamersky", "游民星空", "游戏娱乐", "#dc2626"},
	{"ea.com", "EA游戏", "游戏娱乐", "#ff4747"},
	{"ubisoft", "育碧 Ubisoft", "游戏娱乐", "#0070ff"},
	{"rockstargames", "Rockstar", "游戏娱乐", "#f59e0b"},
	{"minecraft", "Minecraft", "游戏娱乐", "#3c8527"},
	{"roblox", "Roblox", "游戏娱乐", "#1e293b"},
	{"nintendo", "Nintendo", "游戏娱乐", "#e60012"},
	{"playstation", "PlayStation", "游戏娱乐", "#0070d1"},
	{"xbox", "Xbox", "游戏娱乐", "#107c10"},

	// === 云服务与 CDN ===
	{"itunes.apple", "AppStore", "云与CDN", "#0d84ff"},
	{"apps.apple", "AppStore", "云与CDN", "#0d84ff"},
	{"mzstatic", "AppStore", "云与CDN", "#0d84ff"},
	{"icloud", "iCloud", "云与CDN", "#3693f3"},
	{"apple.com", "Apple", "云与CDN", "#555555"},
	{"aliyun", "阿里云", "云与CDN", "#ff6a00"},
	{"tencent", "腾讯云", "云与CDN", "#0052d9"},
	{"myqcloud", "腾讯云", "云与CDN", "#0052d9"},
	{"huaweicloud", "华为云", "云与CDN", "#cf0a2c"},
	{"volces", "火山引擎", "云与CDN", "#1e293b"},
	{"volcengine", "火山引擎", "云与CDN", "#1e293b"},
	{"ctyun", "天翼云", "云与CDN", "#ff6600"},
	{"ucloud", "UCloud", "云与CDN", "#2563eb"},
	{"qiniucdn", "七牛云", "云与CDN", "#00aae7"},
	{"upyun", "又拍云", "云与CDN", "#0ea5e9"},
	{"chinanetcenter", "网宿CDN", "云与CDN", "#0284c7"},
	{"cloudflare", "Cloudflare", "云与CDN", "#f38020"},
	{"amazonaws", "AWS", "云与CDN", "#ff9900"},
	{"azureedge", "Azure CDN", "云与CDN", "#0078d4"},
	{"akamai", "Akamai CDN", "云与CDN", "#009bde"},
	{"fastly", "Fastly CDN", "云与CDN", "#ff282d"},
	{"baidu", "百度", "云与CDN", "#2932e1"},
	{"bdstatic", "百度", "云与CDN", "#2932e1"},
	{"google", "Google", "云与CDN", "#4285f4"},
	{"1e100.net", "Google", "云与CDN", "#4285f4"},
	{"gstatic", "Google", "云与CDN", "#4285f4"},
	{"nju.edu.cn", "南京大学", "云与CDN", "#8b5cf6"},
}

var ispASNRules = []rule{
	{"as137718", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"bilibili", "哔哩哔哩", "视频娱乐", "#fb7299"},
	{"as138699", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"as396986", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"bytedance", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"byteplus", "字节跳动", "视频娱乐", "#1c1c1e"},
	{"as45090", "腾讯云", "云与CDN", "#0052d9"},
	{"as132203", "腾讯云", "云与CDN", "#0052d9"},
	{"tencent", "腾讯云", "云与CDN", "#0052d9"},
	{"as37963", "阿里云", "云与CDN", "#ff6a00"},
	{"as45102", "阿里云", "云与CDN", "#ff6a00"},
	{"alibaba", "阿里云", "云与CDN", "#ff6a00"},
	{"aliyun", "阿里云", "云与CDN", "#ff6a00"},
	{"as55967", "百度", "云与CDN", "#2932e1"},
	{"baidu", "百度", "云与CDN", "#2932e1"},
	{"as62041", "Telegram", "社交通讯", "#26a5e4"},
	{"as62014", "Telegram", "社交通讯", "#26a5e4"},
	{"telegram", "Telegram", "社交通讯", "#26a5e4"},
	{"as13335", "Cloudflare", "云与CDN", "#f38020"},
	{"cloudflare", "Cloudflare", "云与CDN", "#f38020"},
	{"as15169", "Google", "云与CDN", "#4285f4"},
	{"google", "Google", "云与CDN", "#4285f4"},
	{"as714", "Apple", "云与CDN", "#555555"},
	{"apple", "Apple", "云与CDN", "#555555"},
	{"as8075", "Microsoft", "办公协同", "#00a4ef"},
	{"microsoft", "Microsoft", "办公协同", "#00a4ef"},
	{"as32590", "Steam", "游戏娱乐", "#1b2838"},
	{"valve", "Steam", "游戏娱乐", "#1b2838"},
	{"as16509", "AWS", "云与CDN", "#ff9900"},
	{"amazon", "AWS", "云与CDN", "#ff9900"},
	{"as136907", "华为云", "云与CDN", "#cf0a2c"},
	{"huawei", "华为云", "云与CDN", "#cf0a2c"},
	{"as4538", "教育网", "云与CDN", "#8b5cf6"},
	{"cernet", "教育网", "云与CDN", "#8b5cf6"},
	{"netease", "网易云音乐", "视频娱乐", "#d43c33"},
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
		{"17.0.0.0/8", "Apple", "云与CDN", "#555555"},
		{"149.154.160.0/20", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.4.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.8.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.12.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.16.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"91.108.56.0/22", "Telegram", "社交通讯", "#26a5e4"},
		{"2001:b28:f23d::/48", "Telegram", "社交通讯", "#26a5e4"},
		{"2001:67c:4e8::/48", "Telegram", "社交通讯", "#26a5e4"},
		{"1.1.1.0/24", "Cloudflare", "基础网络", "#f38020"},
		{"1.0.0.0/24", "Cloudflare", "基础网络", "#f38020"},
		{"104.16.0.0/12", "Cloudflare", "云与CDN", "#f38020"},
		{"8.8.8.0/24", "Google", "基础网络", "#4285f4"},
		{"8.8.4.0/24", "Google", "基础网络", "#4285f4"},
		{"9.9.9.0/24", "Quad9 DNS", "基础网络", "#2563eb"},
		{"149.112.112.0/24", "Quad9 DNS", "基础网络", "#2563eb"},
		{"119.29.29.0/24", "腾讯云 DNS", "基础网络", "#0052d9"},
		{"1.12.12.0/24", "腾讯云 DNS", "基础网络", "#0052d9"},
		{"223.5.5.0/24", "阿里云 DNS", "基础网络", "#ff6a00"},
		{"223.6.6.0/24", "阿里云 DNS", "基础网络", "#ff6a00"},
		{"120.24.0.0/14", "阿里云", "云与CDN", "#ff6a00"},
		{"47.92.0.0/14", "阿里云", "云与CDN", "#ff6a00"},
		{"39.96.0.0/13", "阿里云", "云与CDN", "#ff6a00"},
		{"43.128.0.0/11", "腾讯云", "云与CDN", "#0052d9"},
		{"62.234.0.0/16", "腾讯云", "云与CDN", "#0052d9"},
		{"119.28.0.0/15", "腾讯云", "云与CDN", "#0052d9"},
		{"110.242.0.0/16", "百度", "云与CDN", "#2932e1"},
		{"220.181.0.0/16", "百度", "云与CDN", "#2932e1"},
		{"142.250.0.0/15", "Google", "云与CDN", "#4285f4"},
		{"172.217.0.0/16", "Google", "云与CDN", "#4285f4"},
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
	return len(domainRules) + len(suffixRules) + len(customRules) + len(ispASNRules) + len(appCIDRs) + 75
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
	case app == "SSH" || app == "RDP 远程桌面" || app == "SMB 文件共享" || app == "WinBox" || app == "RouterOS API" || app == "WebDAV" || app == "iPerf3 测速" || app == "数据库服务":
		return "安全与运维"
	case app == "DNS" || app == "NTP 对时" || app == "ICMP/Ping" || app == "mDNS" || app == "DHCP" || strings.Contains(app, "DNS"):
		return "基础网络"
	case app == "MQTT IoT" || app == "RTSP 监控":
		return "智能家居"
	default:
		return "其他协议"
	}
}

func Classify(proto string, dstPort int, domain, remoteIP, ispAndASN string) AppInfo {
	d := strings.ToLower(domain)
	if d != "" {
		if info, ok := matchSuffixMap(d, customRules); ok {
			return info
		}
		for k, info := range customRules {
			if strings.Contains(d, k) {
				return info
			}
		}
		for _, r := range domainRules {
			if strings.Contains(d, r.keyword) {
				return AppInfo{App: r.app, Category: r.category, Color: r.color, IsRealApp: true}
			}
		}
		if info, ok := matchSuffixMap(d, suffixRules); ok {
			return info
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
	case 67, 68:
		return AppInfo{App: "DHCP", Category: "基础网络", Color: "#94a3b8", IsRealApp: false}
	case 53, 853:
		return AppInfo{App: "DNS", Category: "基础网络", Color: "#94a3b8", IsRealApp: false}
	case 22:
		return AppInfo{App: "SSH", Category: "安全与运维", Color: "#10b981", IsRealApp: true}
	case 3389:
		return AppInfo{App: "RDP 远程桌面", Category: "安全与运维", Color: "#14b8a6", IsRealApp: true}
	case 445:
		return AppInfo{App: "SMB 文件共享", Category: "安全与运维", Color: "#06b6d4", IsRealApp: true}
	case 5000, 5001:
		return AppInfo{App: "群晖 Synology", Category: "智能家居", Color: "#0084d6", IsRealApp: true}
	case 5005, 5006:
		return AppInfo{App: "WebDAV", Category: "安全与运维", Color: "#0284c7", IsRealApp: false}
	case 5201:
		return AppInfo{App: "iPerf3 测速", Category: "安全与运维", Color: "#10b981", IsRealApp: false}
	case 1883, 8883:
		return AppInfo{App: "MQTT IoT", Category: "智能家居", Color: "#8b5cf6", IsRealApp: false}
	case 554:
		return AppInfo{App: "RTSP 监控", Category: "智能家居", Color: "#2563eb", IsRealApp: false}
	case 3306, 5432, 6379, 27017:
		return AppInfo{App: "数据库服务", Category: "安全与运维", Color: "#0f766e", IsRealApp: false}
	case 5060, 5061:
		return AppInfo{App: "SIP 语音", Category: "社交通讯", Color: "#0ea5e9", IsRealApp: false}
	case 51820, 9900:
		return AppInfo{App: "WireGuard VPN", Category: "VPN与P2P", Color: "#a855f7", IsRealApp: true}
	case 4500, 500:
		return AppInfo{App: "IPsec VPN", Category: "VPN与P2P", Color: "#9333ea", IsRealApp: true}
	case 1194:
		return AppInfo{App: "OpenVPN", Category: "VPN与P2P", Color: "#7e22ce", IsRealApp: true}
	case 8291:
		return AppInfo{App: "WinBox", Category: "安全与运维", Color: "#f59e0b", IsRealApp: false}
	case 8728, 8729:
		return AppInfo{App: "RouterOS API", Category: "安全与运维", Color: "#f59e0b", IsRealApp: false}
	case 123:
		return AppInfo{App: "NTP 对时", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	case 5353:
		return AppInfo{App: "mDNS", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	case 54321:
		return AppInfo{App: "米家 MiIO", Category: "智能家居", Color: "#ff6700", IsRealApp: true}
	case 5678:
		return AppInfo{App: "MikroTik MNDP", Category: "基础网络", Color: "#64748b", IsRealApp: false}
	case 32100:
		return AppInfo{App: "摄像头 P2P", Category: "智能家居", Color: "#2563eb", IsRealApp: true}
	case 3478, 19302:
		return AppInfo{App: "WebRTC/STUN", Category: "社交通讯", Color: "#0ea5e9", IsRealApp: false}
	case 25565:
		return AppInfo{App: "Minecraft", Category: "游戏娱乐", Color: "#3c8527", IsRealApp: true}
	case 27015:
		return AppInfo{App: "Steam", Category: "游戏娱乐", Color: "#1b2838", IsRealApp: true}
	case 21:
		return AppInfo{App: "FTP", Category: "安全与运维", Color: "#f59e0b", IsRealApp: false}
	case 23:
		return AppInfo{App: "Telnet", Category: "安全与运维", Color: "#ef4444", IsRealApp: false}
	case 25, 465, 587, 993, 995:
		return AppInfo{App: "Email (SMTP/IMAP)", Category: "办公协同", Color: "#0284c7", IsRealApp: true}
	case 161, 162:
		return AppInfo{App: "SNMP", Category: "安全与运维", Color: "#64748b", IsRealApp: false}
	case 389, 636:
		return AppInfo{App: "LDAP", Category: "安全与运维", Color: "#64748b", IsRealApp: false}
	case 1701, 1723:
		return AppInfo{App: "L2TP/PPTP VPN", Category: "VPN与P2P", Color: "#7c3aed", IsRealApp: true}
	}

	if dstPort >= 6881 && dstPort <= 6889 {
		return AppInfo{App: "BitTorrent/P2P", Category: "VPN与P2P", Color: "#ec4899", IsRealApp: true}
	}
	if dstPort > 0 {
		return AppInfo{App: fmt.Sprintf("%s:%d", strings.ToUpper(p), dstPort), Category: "其他协议", Color: "#6366f1", IsRealApp: false}
	}
	return AppInfo{App: strings.ToUpper(p), Category: "其他协议", Color: "#6366f1", IsRealApp: false}
}

var miningKeywords = []string{
	"xmrig", "minexmr", "nanopool", "f2pool", "ethermine", "nicehash",
	"supportxmr", "coinhive", "c3pool", "herominers", "hashvault", "stratum", "moneroocean",
}

var tunnelKeywords = []string{
	"ngrok", "trycloudflare", "localtunnel", "serveo.net",
}

func ClassifyThreat(domain string, dstPort int) (rule, level, detail string) {
	d := strings.ToLower(domain)
	if d != "" {
		if l := strings.IndexByte(d, '.'); l >= 48 {
			first, digits := d[:l], 0
			for i := 0; i < len(first); i++ {
				if first[i] >= '0' && first[i] <= '9' {
					digits++
				}
			}
			if digits >= 6 && !strings.Contains(d, "cloudfront") && !strings.Contains(d, "amazonaws") && !strings.Contains(d, "google") && !strings.Contains(d, "byte") {
				return "dns_tunnel_suspect", "medium", "Suspicious Long Subdomain (DNS Tunnel)"
			}
		}
		for _, kw := range miningKeywords {
			if strings.Contains(d, kw) {
				return "crypto_mining", "high", "Crypto Mining Pool / Miner (" + kw + ")"
			}
		}
		for _, kw := range tunnelKeywords {
			if strings.Contains(d, kw) {
				return "tunnel_domain", "medium", "Public Tunnel Domain (" + kw + ")"
			}
		}
	}
	switch dstPort {
	case 21:
		return "cleartext_proto", "medium", "FTP Cleartext (21)"
	case 23:
		return "high_risk_port", "high", "Telnet Cleartext (23)"
	case 445:
		return "high_risk_port", "high", "External SMB (445)"
	case 3389:
		return "high_risk_port", "high", "External RDP (3389)"
	case 1080, 3128:
		return "proxy_port", "medium", fmt.Sprintf("Proxy Port (%d)", dstPort)
	case 3333, 4444, 5555, 14444:
		return "high_risk_port", "high", fmt.Sprintf("Mining / C2 Port (%d)", dstPort)
	}
	return "", "", ""
}
