package services

import (
	"os"
	"testing"
)

func TestParseRegion(t *testing.T) {
	// v3 格式：国家|省份|城市|ISP|国家码
	p, c := parseRegion("中国|广东省|深圳市|电信|CN")
	if p != "广东省" || c != "深圳市" {
		t.Fatalf("want 广东省/深圳市, got %s/%s", p, c)
	}
	// 缺省字段 "0" 归一化为空
	if p, c := parseRegion("中国|0|0|内网IP|CN"); p != "" || c != "" {
		t.Fatalf("zero fields should be empty, got %s/%s", p, c)
	}
	// 空串安全
	if p, c := parseRegion(""); p != "" || c != "" {
		t.Fatalf("empty region should be empty")
	}
}

func TestIP2RegionLoaderLookup(t *testing.T) {
	const dbPath = "../data/ip2region.xdb"
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("ip2region.xdb 不存在，跳过地区解析集成测试")
	}
	resolve := loadIP2Region(dbPath)
	if resolve == nil {
		t.Fatalf("loadIP2Region 返回 nil")
	}
	province, city := resolve("114.114.114.114")
	if province == "" {
		t.Fatalf("应能解析出省份，得到空")
	}
	t.Logf("114.114.114.114 → 省=%q 市=%q", province, city)

	// 无效 IP 返回空、不 panic
	if p, c := resolve("not-an-ip"); p != "" || c != "" {
		t.Fatalf("invalid ip should resolve to empty")
	}
}

func TestIPResolverDisabled(t *testing.T) {
	// 缺路径的 loader 应返回 nil；未启用时 ResolveIPRegion 返回空
	if loadIP2Region("") != nil || loadIP2Location("") != nil {
		t.Fatalf("empty path loader should be nil")
	}
	ipMu.Lock()
	ipResolve = nil
	ipMu.Unlock()
	if p, c := ResolveIPRegion("114.114.114.114"); p != "" || c != "" {
		t.Fatalf("disabled resolver should return empty")
	}
}
