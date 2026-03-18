package server

import (
	"NetworkAuth/web"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 聚合注册所有路由
func RegisterRoutes(r *gin.Engine) {
	registerStaticRoutes(r)
	registerFaviconRoute(r)
	RegisterInstallRoutes(r)
	RegisterDefaultRoutes(r)
	RegisterAdminRoutes(r)
}

// registerStaticRoutes 注册静态资源路由
// 静态资源服务，将 /static/ 和 /assets/ 映射到嵌入的文件系统
func registerStaticRoutes(r *gin.Engine) {
	if fsys, err := web.GetStaticFS(); err == nil {
		// 为 /static/ 路径创建子文件系统
		if staticSubFS, staticErr := fs.Sub(fsys, "static"); staticErr == nil {
			r.StaticFS("/static", http.FS(staticSubFS))
		} else {
			log.Printf("创建静态资源子文件系统失败: %v", staticErr)
		}
		// 为 /assets/ 路径创建子文件系统
		if assetsSubFS, assetsErr := fs.Sub(fsys, "assets"); assetsErr == nil {
			r.StaticFS("/assets", http.FS(assetsSubFS))
		} else {
			log.Printf("创建资产资源子文件系统失败: %v", assetsErr)
		}
	} else {
		log.Printf("初始化静态资源文件系统失败: %v", err)
	}
}

// registerFaviconRoute 注册favicon路由
func registerFaviconRoute(r *gin.Engine) {
	// 将 /favicon.ico 重定向到 /assets/favicon.svg
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/assets/favicon.svg")
	})
}
