package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// TemplatesFS 嵌入模板的文件系统
//
//go:embed template/*/*.html
var templatesFS embed.FS

// StaticFS 嵌入静态资源的文件系统（包含 CSS/JS 的 static 与 图片/字体等资源的 assets）
//
//go:embed static/* assets/*
var staticFS embed.FS

// getDistRootFS 获取基于 server.dist 的本地文件系统
// 当 server.dist 非空且路径存在时，返回对应的本地只读 FS；否则返回 nil
func getDistRootFS() fs.FS {
	// 从配置中读取 server.dist
	distPath := viper.GetString("server.dist")
	if distPath == "" {
		return nil
	}
	// 归一化路径，兼容相对/绝对
	absPath := distPath
	if !filepath.IsAbs(distPath) {
		if p, err := filepath.Abs(distPath); err == nil {
			absPath = p
		}
	}
	// 检查目录是否存在
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		return os.DirFS(absPath)
	}
	log.Printf("server.dist 路径无效或不可访问：%s，将回退使用嵌入资源", distPath)
	return nil
}

// ParseTemplates 解析模板
// 优先从 server.dist 指定目录加载（当配置非空且有效），否则回退到嵌入模板
func ParseTemplates() (*template.Template, error) { // Go 顶级函数不支持箭头写法
	if distFS := getDistRootFS(); distFS != nil {
		// 期望 dist 目录下存在 template 与 template/admin 结构
		// 如：{dist}/template/*.html 与 {dist}/template/admin/*.html
		return template.ParseFS(distFS, "template/*/*.html")
	}
	// 默认：使用嵌入模板
	return template.ParseFS(templatesFS, "template/*/*.html")
}

// GetStaticFS 返回静态资源文件系统（包含 static 与 assets 目录）
// 优先使用 server.dist 指定的本地目录；否则回退到嵌入静态资源
func GetStaticFS() (fs.FS, error) { // Go 顶级函数不支持箭头写法
	if distFS := getDistRootFS(); distFS != nil {
		// 直接返回以 dist 根为起点的 FS，routes 中会再基于此 FS Sub 出 static 与 assets
		return distFS, nil
	}
	return staticFS, nil
}
