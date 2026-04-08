package server

import (
	"NetworkAuth/public"
	"NetworkAuth/utils"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
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
	distConfig := viper.GetString("server.dist")
	var fileServer http.Handler

	// 判断是否配置了外部 dist (支持 http 反向代理或本地目录)
	if distConfig != "" {
		if strings.HasPrefix(distConfig, "http://") || strings.HasPrefix(distConfig, "https://") {
			// 反向代理到前端开发服务器
			r.Use(func(c *gin.Context) {
				if !strings.HasPrefix(c.Request.URL.Path, "/api") {
					proxy := httputil.NewSingleHostReverseProxy(&url.URL{
						Scheme: strings.Split(distConfig, "://")[0],
						Host:   strings.TrimPrefix(distConfig, strings.Split(distConfig, "://")[0]+"://"),
					})
					proxy.ServeHTTP(c.Writer, c.Request)
					c.Abort()
				}
			})
			return // 反向代理接管了所有非 API 路由，直接返回
		} else {
			// 使用本地外部目录
			if !filepath.IsAbs(distConfig) {
				distConfig = filepath.Join(utils.GetRootDir(), distConfig)
			}
			fileServer = http.FileServer(http.Dir(distConfig))

			// 拦截并处理静态资源请求
			r.Use(func(c *gin.Context) {
				path := c.Request.URL.Path
				if strings.HasPrefix(path, "/api") {
					c.Next()
					return
				}

				cleanPath := strings.TrimPrefix(path, "/")
				if cleanPath == "" {
					cleanPath = "index.html"
				}

				fullPath := distConfig + "/" + cleanPath
				if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
					if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") {
						c.Header("Cache-Control", "public, max-age=31536000")
					}
					fileServer.ServeHTTP(c.Writer, c.Request)
					c.Abort()
					return
				}
				c.Next()
			})

			// SPA 前端路由兜底
			r.NoRoute(func(c *gin.Context) {
				if strings.HasPrefix(c.Request.URL.Path, "/api") {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "API Not Found"})
					return
				}
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.File(distConfig + "/index.html")
			})
			return
		}
	}

	// 提取嵌入的 dist 目录 (默认方式)
	distFS, err := fs.Sub(public.Public, "dist")
	if err != nil {
		panic("Failed to initialize embedded static files: " + err.Error())
	}

	// 提供静态文件服务器
	fileServer = http.FileServer(http.FS(distFS))

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
