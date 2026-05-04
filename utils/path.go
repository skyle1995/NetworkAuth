package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetRootDir 获取当前程序运行的真实根目录
// 能跨平台地识别是在生产环境执行二进制文件，还是在开发阶段使用 go run/test 乃至 IDE 调试运行
func GetRootDir() string {
	var baseDir string

	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	execPath, err := os.Executable()
	if err != nil {
		return workDir
	}

	realExecPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realExecPath
	}
	execDir := filepath.Dir(execPath)

	realTempDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		realTempDir = os.TempDir()
	}

	isGoRun := false
	rel, err := filepath.Rel(realTempDir, execDir)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		isGoRun = true
	} else if strings.Contains(execPath, "go-build") || strings.Contains(execPath, "__debug_bin") || strings.HasSuffix(execPath, ".test") {
		isGoRun = true
	} else if strings.Contains(filepath.Base(execPath), "___go_build") || strings.HasPrefix(filepath.Base(execPath), "dlv") {
		isGoRun = true
	}

	if isGoRun {
		// 开发模式下，利用 runtime 获取 utils/path.go 所在的绝对路径
		// 向上两级即可得到项目的真实绝对根目录，避免因终端 CWD 不同导致的配置读取和连接失败
		_, b, _, _ := runtime.Caller(0)
		baseDir = filepath.Dir(filepath.Dir(b))
	} else {
		// 生产模式下（正式编译的独立二进制），返回可执行文件所在目录
		baseDir = execDir
	}

	return baseDir
}

// DisplayPath 返回适合日志展示的路径。
// 对项目根目录内的文件保留相对路径；其他路径退化为文件名，避免泄露绝对安装目录。
func DisplayPath(path string) string {
	if path == "" {
		return ""
	}

	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return cleanPath
	}

	rootDir := filepath.Clean(GetRootDir())
	rel, err := filepath.Rel(rootDir, cleanPath)
	if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return rel
	}

	return filepath.Base(cleanPath)
}
