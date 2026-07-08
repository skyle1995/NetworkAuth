package selfupdate

import (
	"NetworkAuth/utils"
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ============================================================================
// 更新准备（下载/校验/替换/重启）
// ============================================================================

// Prepare 下载并准备指定版本的更新包
// 输入参数：
//   - ctx: 上下文
//   - version: 目标版本号
//   - downloadURL: 下载链接
//   - expectedSHA256: 期望的 SHA256 哈希值
//
// 返回值：
//   - SelfUpdateStatus: 当前更新状态
func (m *SelfUpdateManager) Prepare(ctx context.Context, version, downloadURL, expectedSHA256 string) SelfUpdateStatus {
	m.mu.Lock()
	m.ensureLoadedLocked()
	if m.running {
		st := m.GetStatus()
		m.mu.Unlock()
		return st
	}
	m.running = true
	m.clearPreparedLocked()
	m.last.DownloadProgress = 0
	m.last.PreparedVersion = strings.TrimSpace(version)
	m.persistLocked(context.Background())
	m.mu.Unlock()

	go func() {
		err := m.prepare(context.Background(), version, downloadURL, expectedSHA256)

		m.mu.Lock()
		if err != nil {
			m.last.PrepareError = err.Error()
		} else {
			m.last.PrepareError = ""
		}
		m.running = false
		_ = selfUpdateSetSettingString(context.Background(), settingSelfUpdatePreparedAt, fmt.Sprintf("%d", time.Now().Unix()), "自更新准备时间戳")
		m.persistLocked(context.Background())
		m.mu.Unlock()
	}()

	return m.GetStatus()
}

// prepare 下载、校验、解压、替换二进制的实际执行逻辑
func (m *SelfUpdateManager) prepare(ctx context.Context, version, downloadURL, expectedSHA256 string) error {
	url := strings.TrimSpace(downloadURL)
	if url == "" {
		return errors.New("无可用下载地址")
	}
	expected := strings.TrimSpace(expectedSHA256)
	if expected == "" {
		return errors.New("缺少 SHA256，无法校验更新包")
	}

	workDir, err := ensureSelfUpdateWorkDir()
	if err != nil {
		return err
	}

	safeVer := selfUpdateSanitizeFilename(version)
	if safeVer == "" {
		safeVer = "unknown"
	}
	zipPath := filepath.Join(workDir, fmt.Sprintf("update-%s-%s-%s.zip", safeVer, runtime.GOOS, runtime.GOARCH))
	stagingDir := filepath.Join(workDir, "staging", safeVer)
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return fmt.Errorf("创建 staging 目录失败: %w", err)
	}

	// 下载 + SHA256 校验
	if err := selfUpdateDownloadWithSHA256(ctx, url, zipPath, expected, 0, m); err != nil {
		_ = os.Remove(zipPath)
		return err
	}

	// 解压提取二进制
	exeName := selfAppName
	if runtime.GOOS == "windows" {
		exeName = selfAppName + ".exe"
	}
	outBin := filepath.Join(stagingDir, exeName)
	if err := selfUpdateUnzipSingleFile(zipPath, exeName, outBin); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(outBin, 0755)
	}

	m.mu.Lock()
	m.last.Prepared = true
	m.last.PreparedVersion = version
	m.last.PreparedZip = zipPath
	m.last.StagingBinary = outBin
	m.last.PrepareError = ""
	m.last.AutoReplaceTried = false
	m.last.AutoReplaceOK = false
	m.last.AutoReplaceError = ""
	m.mu.Unlock()

	// Windows: 生成 PowerShell 脚本
	if runtime.GOOS == "windows" {
		m.writeUpdateScripts(workDir, outBin)
		return nil
	}

	// Linux/Darwin: 尝试自动替换
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		_ = m.tryAutoReplaceUnix(outBin)
		m.mu.Lock()
		replaceOk := m.last.AutoReplaceOK
		m.mu.Unlock()
		if !replaceOk {
			m.writeUpdateScripts(workDir, outBin)
		}
		// 替换成功后不再自动重启，改由前端「立即重启」按钮手动触发
	}
	return nil
}

// ============================================================================
// 工作目录
// ============================================================================

// ensureSelfUpdateWorkDir 确保更新缓存目录存在
func ensureSelfUpdateWorkDir() (string, error) {
	root := utilsGetRootDir()
	dir := filepath.Join(root, "data", "update-cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建更新缓存目录失败: %w", err)
	}
	return dir, nil
}

// selfUpdateSanitizeFilename 清理版本号使其可安全用作文件名
func selfUpdateSanitizeFilename(s string) string {
	v := strings.TrimSpace(s)
	if v == "" {
		return "unknown"
	}
	v = strings.ReplaceAll(v, string(os.PathSeparator), "_")
	v = strings.ReplaceAll(v, "..", "_")
	return v
}

// ============================================================================
// 下载 + SHA256 校验
// ============================================================================

// selfUpdateProgressWriter 下载进度追踪器
type selfUpdateProgressWriter struct {
	total    int64
	written  int64
	manager  *SelfUpdateManager
	lastTick time.Time
}

func (pw *selfUpdateProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	if pw.total > 0 && time.Since(pw.lastTick) > 500*time.Millisecond {
		pw.lastTick = time.Now()
		progress := int(float64(pw.written) / float64(pw.total) * 100)
		if progress > 100 {
			progress = 100
		}
		pw.manager.mu.Lock()
		pw.manager.last.DownloadProgress = progress
		pw.manager.mu.Unlock()
	}
	return n, nil
}

// selfUpdateDownloadWithSHA256 下载文件并进行流式 SHA256 校验
func selfUpdateDownloadWithSHA256(ctx context.Context, url, dstPath, expectedHex string, totalSize int64, m *SelfUpdateManager) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	tmpPath := dstPath + ".part"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	pw := &selfUpdateProgressWriter{total: totalSize, manager: m, lastTick: time.Now()}
	if _, err := io.Copy(io.MultiWriter(f, h, pw), resp.Body); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("写入下载文件失败: %w", err)
	}
	m.mu.Lock()
	m.last.DownloadProgress = 100
	m.mu.Unlock()

	sum := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(strings.TrimSpace(expectedHex), sum) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("SHA256 校验失败，期望 %s，实际 %s", expectedHex, sum)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("保存下载文件失败: %w", err)
	}
	return nil
}

// ============================================================================
// 解压
// ============================================================================

// selfUpdateUnzipSingleFile 从 ZIP 包中提取指定文件
func selfUpdateUnzipSingleFile(zipPath, wantName, outPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开更新包失败: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.EqualFold(filepath.Base(f.Name), wantName) {
			rc, zipErr := f.Open()
			if zipErr != nil {
				return fmt.Errorf("读取更新包文件失败: %w", zipErr)
			}
			defer rc.Close()

			if mkdirErr := os.MkdirAll(filepath.Dir(outPath), 0755); mkdirErr != nil {
				return fmt.Errorf("创建输出目录失败: %w", mkdirErr)
			}

			tmp := outPath + ".part"
			out, createErr := os.Create(tmp)
			if createErr != nil {
				return fmt.Errorf("创建输出文件失败: %w", createErr)
			}
			if _, copyErr := io.Copy(out, rc); copyErr != nil {
				out.Close()
				_ = os.Remove(tmp)
				return fmt.Errorf("写出二进制失败: %w", copyErr)
			}
			out.Close()
			if renameErr := os.Rename(tmp, outPath); renameErr != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("保存二进制失败: %w", renameErr)
			}
			return nil
		}
	}
	return fmt.Errorf("更新包中未找到文件: %s", wantName)
}

// ============================================================================
// 自动替换
// ============================================================================

// tryAutoReplaceUnix 尝试在 Linux/macOS 上自动替换当前二进制
func (m *SelfUpdateManager) tryAutoReplaceUnix(stagingBin string) error {
	m.mu.Lock()
	m.last.AutoReplaceTried = true
	m.mu.Unlock()

	exePath, err := os.Executable()
	if err != nil {
		m.mu.Lock()
		m.last.AutoReplaceOK = false
		m.last.AutoReplaceError = err.Error()
		m.mu.Unlock()
		return nil
	}
	exePath = filepath.Clean(exePath)

	if selfUpdateIsTempExecutable(exePath) {
		m.mu.Lock()
		m.last.AutoReplaceOK = false
		m.last.AutoReplaceError = "当前为临时运行路径，跳过自动安装"
		m.mu.Unlock()
		return nil
	}

	if err := selfUpdateAtomicReplaceFile(exePath, stagingBin); err != nil {
		m.mu.Lock()
		m.last.AutoReplaceOK = false
		m.last.AutoReplaceError = err.Error()
		m.mu.Unlock()
		return nil
	}

	m.mu.Lock()
	m.last.AutoReplaceOK = true
	m.last.AutoReplaceError = ""
	m.mu.Unlock()
	return nil
}

// selfUpdateIsTempExecutable 判断当前可执行文件是否在临时目录下运行
func selfUpdateIsTempExecutable(exePath string) bool {
	tmp := filepath.Clean(os.TempDir())
	p := filepath.Clean(exePath)
	if tmp == "" {
		return false
	}
	return strings.HasPrefix(p, tmp+string(os.PathSeparator))
}

// selfUpdateAtomicReplaceFile 原子替换二进制文件
func selfUpdateAtomicReplaceFile(targetPath, newPath string) error {
	targetDir := filepath.Dir(targetPath)
	newTmp := filepath.Join(targetDir, filepath.Base(targetPath)+".new")
	bak := targetPath + ".bak"

	in, err := os.Open(newPath)
	if err != nil {
		return fmt.Errorf("打开 staging 文件失败: %w", err)
	}
	defer in.Close()

	out, err := os.Create(newTmp)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(newTmp)
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	out.Close()

	if err := os.Chmod(newTmp, 0755); err != nil {
		_ = os.Remove(newTmp)
		return fmt.Errorf("设置执行权限失败: %w", err)
	}

	_ = os.Remove(bak)
	if err := os.Rename(targetPath, bak); err != nil {
		_ = os.Remove(newTmp)
		return fmt.Errorf("备份旧二进制失败: %w", err)
	}

	if err := os.Rename(newTmp, targetPath); err != nil {
		_ = os.Rename(bak, targetPath)
		_ = os.Remove(newTmp)
		return fmt.Errorf("替换二进制失败: %w", err)
	}
	return nil
}

// ============================================================================
// 更新脚本生成
// ============================================================================

// writeUpdateScripts 生成手动更新脚本
func (m *SelfUpdateManager) writeUpdateScripts(outDir, stagingBin string) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath = filepath.Clean(exePath)

	if runtime.GOOS == "windows" {
		ps1Path := filepath.Join(outDir, "update.ps1")
		if err := os.WriteFile(ps1Path, []byte(selfUpdateBuildPS1(stagingBin, exePath)), 0644); err == nil {
			m.mu.Lock()
			m.last.ScriptPowerShellPath = fmt.Sprintf(`powershell -ExecutionPolicy Bypass -File "%s"`, ps1Path)
			m.mu.Unlock()
		}
	} else {
		shPath := filepath.Join(outDir, "update.sh")
		if err := os.WriteFile(shPath, []byte(selfUpdateBuildSh(stagingBin, exePath)), 0644); err == nil {
			_ = os.Chmod(shPath, 0755)
			m.mu.Lock()
			m.last.ScriptShellPath = fmt.Sprintf(`sh "%s"`, shPath)
			m.mu.Unlock()
		}
	}
}

// selfUpdateBuildSh 生成 Shell 更新脚本
func selfUpdateBuildSh(stagingBin, targetBin string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

STAGING_BIN=%q
TARGET_BIN=%q
SERVICE_NAME=${SERVICE_NAME:-}

if [ -n "$SERVICE_NAME" ] && command -v systemctl >/dev/null 2>&1; then
  systemctl stop "$SERVICE_NAME" || true
fi

if [ ! -f "$STAGING_BIN" ]; then
  echo "staging binary not found: $STAGING_BIN"
  exit 1
fi

if [ -f "$TARGET_BIN" ]; then
  cp -f "$TARGET_BIN" "$TARGET_BIN.bak"
fi

cp "$STAGING_BIN" "$TARGET_BIN"
chmod +x "$TARGET_BIN" || true

if [ -n "$SERVICE_NAME" ] && command -v systemctl >/dev/null 2>&1; then
  systemctl start "$SERVICE_NAME"
fi

echo "update ok"
`, stagingBin, targetBin)
}

// selfUpdateBuildPS1 生成 PowerShell 更新脚本
func selfUpdateBuildPS1(stagingBin, targetBin string) string {
	return fmt.Sprintf(`param(
  [string]$ServiceName = ""
)

$StagingBin = %q
$TargetBin = %q

if (!(Test-Path $StagingBin)) { throw "staging binary not found: $StagingBin" }

if ($ServiceName -ne "") {
  Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
}

if (Test-Path $TargetBin) {
  Copy-Item -Path $TargetBin -Destination ($TargetBin + ".bak") -Force
}
Copy-Item -Path $StagingBin -Destination $TargetBin -Force

if ($ServiceName -ne "") {
  Start-Service -Name $ServiceName
}

Write-Output "update ok"
`, stagingBin, targetBin)
}

// ============================================================================
// 自动重启
// ============================================================================

// RestartNow 由管理员手动触发：延迟 1 秒后重启进程以加载已安装的新二进制（确保 HTTP 响应先返回）。
func (m *SelfUpdateManager) RestartNow() {
	go selfUpdateTriggerAutoRestart(1 * time.Second)
}

// selfUpdateTriggerAutoRestart 在自动替换二进制成功后延迟重启进程
func selfUpdateTriggerAutoRestart(delay time.Duration) {
	time.Sleep(delay)

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	selfUpdateSetProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return
	}

	os.Exit(0)
}

// ============================================================================
// 辅助
// ============================================================================

// utilsGetRootDir 获取项目根目录
func utilsGetRootDir() string {
	return utils.GetRootDir()
}
