package server

import (
	"NetworkAuth/public"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 聚合注册所有路由
func RegisterRoutes(r *gin.Engine) {
	// 1. 所有接口路由基于 /api
	apiGroup := r.Group("/api")
	RegisterInstallRoutes(apiGroup)
	RegisterDefaultRoutes(apiGroup)
	RegisterAdminRoutes(apiGroup)

	// 2. 注册前端静态资源及兜底路由
	registerFrontendRoutes(r)
}

// registerFrontendRoutes 注册前端静态资源及兜底路由
func registerFrontendRoutes(r *gin.Engine) {
	// 提取嵌入的 dist 目录
	distFS, err := fs.Sub(public.Public, "dist")
	if err != nil {
		panic("Failed to initialize embedded static files: " + err.Error())
	}

	// 挂载静态资源目录 (如 assets)
	// 根据 Vue 构建产物，通常有 /assets 或 /static 目录，这里我们直接把整个 distFS 映射到根路由
	// 但为了避免与 /api 冲突，我们可以使用中间件和 NoRoute 来处理兜底

	// 提供静态文件服务器
	fileServer := http.FileServer(http.FS(distFS))

	// 拦截并处理静态资源请求
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 如果是 API 请求，直接放行
		if strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		// 检查静态文件中是否存在该路径
		// 移除开头的 "/"
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// 尝试在嵌入的文件系统中查找文件
		if _, err := fs.Stat(distFS, cleanPath); err == nil {
			// 文件存在，交由 FileServer 处理
			// 设置一些常见的缓存头
			if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		c.Next()
	})

	// SPA 前端路由兜底 (处理 History 模式)
	r.NoRoute(func(c *gin.Context) {
		// 如果是 API 请求找不到路由，返回 404 JSON
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{
				"code": 404,
				"msg":  "API Not Found",
			})
			return
		}

		// 其他所有非 API 请求，都返回 index.html 交给前端 Vue Router 处理
		c.Header("Content-Type", "text/html; charset=utf-8")
		indexFile, err := distFS.Open("index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load index.html")
			return
		}
		defer indexFile.Close()

		stat, _ := indexFile.Stat()
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
	})
}
