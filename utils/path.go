package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// GetRootDir 获取当前程序运行的真实根目录
// 能够智能、跨平台地识别是编译后的可执行文件运行，还是通过 `go run` 运行（通常在临时目录下）
func GetRootDir() string {
	var baseDir string

	// 首先尝试获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	// 获取程序可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取可执行文件路径失败，使用当前工作目录
		return workDir
	}

	// 解析软链接，获取真实物理路径（macOS 下 /tmp 经常是 /private/tmp 的软链）
	realExecPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realExecPath
	}
	execDir := filepath.Dir(execPath)

	realTempDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		realTempDir = os.TempDir()
	}

	// 跨平台安全地判断 execDir 是否在 realTempDir 内部
	// 使用 filepath.Rel 可以避免直接 HasPrefix 带来的大小写、路径分隔符以及部分目录名重合的问题
	rel, err := filepath.Rel(realTempDir, execDir)
	isGoRun := false
	if err == nil {
		// 如果 rel 不以 ".." 开头，说明 execDir 在 TempDir 内部，即为 go run 模式
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			isGoRun = true
		}
	} else {
		// fallback: 如果 Rel 失败（例如跨盘符），则退回简单的 HasPrefix 判断（带上分隔符防误判）
		cleanTemp := filepath.Clean(realTempDir) + string(os.PathSeparator)
		cleanExec := filepath.Clean(execDir) + string(os.PathSeparator)
		if strings.HasPrefix(strings.ToLower(cleanExec), strings.ToLower(cleanTemp)) {
			isGoRun = true
		}
	}

	if isGoRun {
		baseDir = workDir
	} else {
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
