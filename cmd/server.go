package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"NetworkAuth/database"
	"NetworkAuth/middleware"
	"NetworkAuth/server"
	"NetworkAuth/services"
	"NetworkAuth/utils"
	"NetworkAuth/utils/logger"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverCmd 代表服务器命令
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 NetworkAuth 系统服务器",
	Long:  `启动 NetworkAuth 系统 HTTP 服务器，监听配置文件中指定的端口，提供 Web 管理界面和 API 服务。`,
	Run:   runServer,
}

func init() {
	// 将服务器命令添加到根命令
	rootCmd.AddCommand(serverCmd)

	// 添加服务器特定的标志
	serverCmd.Flags().StringP("host", "H", "", "服务器监听地址 (覆盖配置文件)")
	serverCmd.Flags().IntP("port", "p", 0, "服务器监听端口 (覆盖配置文件)")
}

// runServer 运行HTTP服务器
func runServer(cmd *cobra.Command, args []string) {
	// 获取配置
	host := getServerHost(cmd)
	port := getServerPort(cmd)
	addr := fmt.Sprintf("%s:%d", host, port)

	// 获取全局日志实例
	logger := logger.GetLogger()
	logger.LogServerStart(host, port)

	// 重定向 Gin 框架内部日志到 Logrus
	// 这将捕获 [GIN-debug] 路由注册日志和其他框架级输出
	gin.DefaultWriter = logger.WriterLevel(logrus.DebugLevel)
	gin.DefaultErrorWriter = logger.WriterLevel(logrus.ErrorLevel)

	// 设置 Gin 模式
	if !viper.GetBool("server.dev_mode") {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化Redis（如果配置存在，失败不致命）
	utils.InitRedis()

	// 初始化数据库（根据 viper 配置选择 SQLite 或 MySQL）
	// 如果初始化失败（例如 MySQL 连不上），则打印错误并退出
	db, err := database.Init()
	if err != nil {
		logrus.WithError(err).Fatal("数据库初始化失败，请检查配置或确认是否已安装")
	}

	if db != nil {
		// 检查系统是否已安装
		isInstalled := services.GetSettingsService().GetString("is_installed", "0")
		if isInstalled == "1" {
			// 执行自动迁移（确保表结构存在）
			if err := database.AutoMigrate(); err != nil {
				logrus.WithError(err).Fatal("数据库自动迁移失败")
			}
			// 初始化默认系统设置
			if err := database.SeedDefaultSettings(); err != nil {
				logrus.WithError(err).Fatal("默认系统设置初始化失败")
			}
			if err := database.SeedDefaultPortalNavigation(); err != nil {
				logrus.WithError(err).Fatal("默认门户导航初始化失败")
			}

			// 初始化加密管理器
			// 从数据库设置中获取加密密钥
			encryptionKey := services.GetSettingsService().GetEncryptionKey()
			if err := utils.InitEncryption(encryptionKey); err != nil {
				logrus.WithError(err).Fatal("加密管理器初始化失败")
			}

			// 启动日志清理定时任务
			services.StartLogCleanupTask()
		} else {
			logrus.Info("系统尚未安装 (is_installed=0)，跳过核心组件初始化")
		}
	} else {
		logrus.Info("系统处于未初始化状态，跳过数据库自动迁移和设置加载")
	}

	// 创建HTTP服务器
	server := createHTTPServer(addr)

	// 启动服务器
	startServer(server)
}

// getServerHost 获取服务器监听地址
func getServerHost(cmd *cobra.Command) string {
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		return host
	}
	return viper.GetString("server.host")
}

// getServerPort 获取服务器监听端口
func getServerPort(cmd *cobra.Command) int {
	if port, _ := cmd.Flags().GetInt("port"); port != 0 {
		return port
	}
	return viper.GetInt("server.port")
}

// createHTTPServer 创建HTTP服务器
func createHTTPServer(addr string) *http.Server {
	// 创建 Gin 引擎
	r := gin.New()

	// 使用默认的 Recovery 中间件
	r.Use(gin.Recovery())

	// 启用 CORS 中间件，支持前后端分离
	r.Use(middleware.CorsMiddleware())

	// 添加日志中间件
	// 默认为 true，只有显式设置为 false 才关闭
	enableAccessLog := true
	if viper.IsSet("server.access_log") {
		enableAccessLog = viper.GetBool("server.access_log")
	}
	if enableAccessLog {
		r.Use(middleware.WrapHandler())
	}

	// 添加开发模式中间件（统一管理开发模式功能）
	r.Use(middleware.DevModeMiddleware())

	// 添加安装检查中间件
	r.Use(middleware.InstallCheckMiddleware())

	// 添加维护模式中间件
	r.Use(middleware.MaintenanceMiddleware())

	// 注册路由
	registerRoutes(r)

	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

// registerRoutes 注册HTTP路由
func registerRoutes(r *gin.Engine) {
	// 使用server包中的路由注册函数
	server.RegisterRoutes(r)
}

// startServer 启动服务器并处理优雅关闭
func startServer(server *http.Server) {
	// 获取全局日志实例
	logger := logger.GetLogger()

	// 创建一个通道来接收操作系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在goroutine中启动服务器
	go func() {
		logger.WithField("addr", server.Addr).Info("HTTP服务器已启动")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogError(err, "服务器启动失败")
			os.Exit(1)
		}
	}()

	// 等待中断信号
	<-sigChan
	logger.Info("收到关闭信号，正在优雅关闭服务器...")

	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 优雅关闭服务器
	if err := server.Shutdown(ctx); err != nil {
		logger.LogError(err, "服务器关闭时出错")
	} else {
		logger.LogServerStop()
	}
}
