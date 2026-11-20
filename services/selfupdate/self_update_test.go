package selfupdate

import (
	"NetworkAuth/utils/storage"
	"runtime"
	"testing"
	"time"
)

// TestFilterPlatformZIP 验证当前平台只选择发布包而不选择摘要或其他平台。
func TestFilterPlatformZIP(t *testing.T) {
	name := "NetworkAuth-" + runtime.GOOS + "-" + runtime.GOARCH + "-v1.zip"
	files := []storage.FileInfo{{Name: name + ".sha256"}, {Name: "NetworkAuth-other-platform-v1.zip"}, {Name: name}}
	got := selfUpdateFilterByPlatform(files)
	if len(got) != 1 || got[0].Name != name {
		t.Fatalf("更新包筛选不符: %v", got)
	}
}

// TestPrepareWhileRunning 验证重复准备请求直接返回正在运行的状态。
func TestPrepareWhileRunning(t *testing.T) {
	m := SelfUpdateManager{loaded: true, running: true}
	done := make(chan SelfUpdateStatus, 1)
	go func() { done <- m.Prepare("1.0.0") }()
	select {
	case st := <-done:
		if !st.Running {
			t.Fatal("应保留正在运行的状态")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("重复请求未返回")
	}
}
