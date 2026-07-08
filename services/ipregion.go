package services

import (
	"strings"
	"sync"

	ip2location "github.com/ip2location/ip2location-go/v9"
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// IP 地区识别（双库可选：ip2region / IP2Location LITE）
// ============================================================================
//
// 后台可选用 ip2region(国内省市优、中文) 或 IP2Location LITE(全球、英文省市)。
// 解析结果统一为 (province, city)，供市级/省级 IP 验证使用。库缺失/关闭时优雅降级：
// 解析返回空，IP 验证退回精确匹配。设置变更后可调用 InitIPRegion 热重载。

// 运营模式设置键
//   ip_region_provider: "ip2region" | "ip2location" | ""(关闭)
//   ip2region_db / ip2location_db: 各自库文件路径

var (
	ipMu      sync.RWMutex
	ipResolve func(ip string) (province, city string) // nil=未启用
)

// InitIPRegion 依据系统设置(提供方 + 库路径)加载地区库。可重复调用以热重载。
func InitIPRegion() {
	s := GetSettingsService()
	provider := strings.TrimSpace(s.GetString("ip_region_provider", "ip2region"))

	var resolver func(string) (string, string)
	switch provider {
	case "ip2location":
		resolver = loadIP2Location(s.GetString("ip2location_db", "data/IP2LOCATION-LITE.BIN"))
	case "ip2region":
		resolver = loadIP2Region(s.GetString("ip2region_db", "data/ip2region.xdb"))
	default:
		resolver = nil // 关闭
	}

	ipMu.Lock()
	ipResolve = resolver
	ipMu.Unlock()
}

// ResolveIPRegion 解析 IP 的省份与城市；库未就绪或解析失败返回空串。
func ResolveIPRegion(ip string) (province, city string) {
	ipMu.RLock()
	r := ipResolve
	ipMu.RUnlock()
	if r == nil {
		return "", ""
	}
	return r(strings.TrimSpace(ip))
}

// loadIP2Region 加载 ip2region xdb，返回解析闭包；失败返回 nil。
func loadIP2Region(path string) func(string) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	content, err := xdb.LoadContentFromFile(path)
	if err != nil {
		logrus.WithError(err).Warnf("ip2region 库加载失败(%s)，地区级IP验证将退回精确匹配", path)
		return nil
	}
	searcher, err := xdb.NewWithBuffer(xdb.IPv4, content)
	if err != nil {
		logrus.WithError(err).Warn("ip2region 库初始化失败")
		return nil
	}
	logrus.Infof("IP地区库已加载: ip2region (%s)", path)
	return func(ip string) (string, string) {
		region, err := searcher.Search(strings.TrimSpace(ip))
		if err != nil || region == "" {
			return "", ""
		}
		return parseRegion(region)
	}
}

// loadIP2Location 加载 IP2Location LITE BIN，返回解析闭包；失败返回 nil。
func loadIP2Location(path string) func(string) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	db, err := ip2location.OpenDB(path)
	if err != nil {
		logrus.WithError(err).Warnf("IP2Location 库加载失败(%s)，地区级IP验证将退回精确匹配", path)
		return nil
	}
	logrus.Infof("IP地区库已加载: IP2Location (%s)", path)
	return func(ip string) (string, string) {
		rec, err := db.Get_all(strings.TrimSpace(ip))
		if err != nil {
			return "", ""
		}
		return normLoc(rec.Region), normLoc(rec.City)
	}
}

// parseRegion 解析 ip2region v3 的 "国家|省份|城市|ISP|国家码"，取省份与城市。
func parseRegion(region string) (province, city string) {
	parts := strings.Split(region, "|")
	norm := func(i int) string {
		if i >= len(parts) {
			return ""
		}
		v := strings.TrimSpace(parts[i])
		if v == "0" {
			return ""
		}
		return v
	}
	return norm(1), norm(2)
}

// normLoc 归一化 IP2Location 字段：占位/无效值统一为空串。
func normLoc(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "-" ||
		strings.HasPrefix(v, "This parameter") ||
		strings.HasPrefix(v, "Invalid") {
		return ""
	}
	return v
}
