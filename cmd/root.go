package cmd

import (
	"io"
	"networkDev/config"
	"networkDev/utils/logger"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ============================================================================
// 全局变量
// ============================================================================

var cfgFile string

// ============================================================================
// 命令定义
// ============================================================================

// rootCmd 代表没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "networkDev",
	Short: "一个基于Cobra的网络验证服务器应用",
	Long: `networkDev是一个使用Cobra CLI框架构建的网络验证服务器应用，
集成了Viper配置管理、Logrus日志记录和embed静态资源嵌入功能。`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 在加载配置前配置logrus用于非HTTP日志

		setupLogrusForNonHTTP()

	},
}

// ============================================================================
// 公共函数
// ============================================================================

// Execute 添加所有子命令到根命令并设置适当的标志
// 这由main.main()调用。只需要对rootCmd执行一次。
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// 在这里定义标志和配置设置
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认为 config.json)")
}

// ============================================================================
// 私有函数
// ============================================================================

// setupLogrusForNonHTTP 配置logrus用于非HTTP日志
// 在加载配置文件之前进行基本的logrus设置
func setupLogrusForNonHTTP() {
	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceColors:     false,
		DisableColors:   true,
	})

	// 设置默认日志级别
	logrus.SetLevel(logrus.InfoLevel)

	// 设置输出目标（稍后会根据配置文件调整）
	logrus.SetOutput(os.Stdout)
	if cfgFile != "" {
		config.Init(cfgFile)
	} else {
		config.Init("./config.json")
	}

	// 根据配置文件进一步配置logrus
	setupLogrusFromConfig()

	// 初始化HTTP日志处理器
	logger.InitLogger()

	// 记录配置加载完成
	logrus.Info("配置文件加载完成")
}

// initConfig 读取配置文件和环境变量
func initConfig() {

}

// setupLogrusFromConfig 根据配置文件进一步配置logrus
// 设置日志级别和输出目标，支持日志切割功能
func setupLogrusFromConfig() {
	// 设置日志级别
	if level := viper.GetString("log.level"); level != "" {
		if logLevel, err := logrus.ParseLevel(level); err == nil {
			logrus.SetLevel(logLevel)
		}
	}

	// 设置日志输出目标
	logFile := viper.GetString("log.file")
	if logFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logrus.WithError(err).Error("创建日志目录失败")
			return
		}

		// 配置lumberjack日志轮转
		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    viper.GetInt("log.max_size"),    // MB
			MaxBackups: viper.GetInt("log.max_backups"), // 保留的旧日志文件数量
			MaxAge:     viper.GetInt("log.max_age"),     // 天数
			Compress:   true,                            // 压缩旧日志文件
		}

		// 同时输出到控制台和文件（带日志切割）
		multiWriter := io.MultiWriter(os.Stdout, lumberjackLogger)
		logrus.SetOutput(multiWriter)

		logrus.WithFields(logrus.Fields{
			"file":        logFile,
			"max_size":    viper.GetInt("log.max_size"),
			"max_backups": viper.GetInt("log.max_backups"),
			"max_age":     viper.GetInt("log.max_age"),
		}).Info("日志切割功能已启用")
	}
	// 当日志文件路径为空时，保持默认输出到控制台，不创建任何目录
}
