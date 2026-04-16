package cmd

import (
	"NetworkAuth/config"
	"NetworkAuth/utils"
	"NetworkAuth/utils/logger"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

var cfgFile string

// rootCmd 代表没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "NetworkAuth",
	Short: "网络授权服务命令行工具",
	Long: `网络授权服务 (NetworkAuth) 是一个专注于应用鉴权、接口管理和动态逻辑分发的后端系统。
本命令行工具用于启动服务器、管理配置和执行维护任务。`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 在加载配置前配置logrus用于非HTTP日志

		setupLogrusForNonHTTP()

	},
}

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

	// 初始化配置（优先使用命令行参数，否则默认 config.json）
	// 注意：如果文件不存在，配置系统将在内存中生成默认配置
	if cfgFile != "" {
		config.Init(cfgFile)
	} else {
		config.Init("config.json")
	}

	// 根据配置文件进一步配置logrus
	setupLogrusFromConfig()

	// 初始化HTTP日志处理器
	logger.InitLogger()

	// 记录配置加载完成，使用相对路径或文件名保持一致性
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fileName := utils.DisplayPath(configFile)
		logrus.WithField("file", fileName).Info("配置文件加载完成")
	} else {
		logrus.Info("配置加载完成(内存默认配置)")
	}
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
		// 统一转换为绝对路径，避免不同系统或启动目录下出现日志落点不一致。
		absPath := filepath.Clean(logFile)
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(utils.GetRootDir(), absPath)
		}
		if normalizedPath, err := filepath.Abs(absPath); err == nil {
			absPath = normalizedPath
		}

		// 确保日志目录存在
		logDir := filepath.Dir(absPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logrus.WithError(err).Error("创建日志目录失败")
			return
		}

		// 配置lumberjack日志轮转
		lumberjackLogger := &lumberjack.Logger{
			Filename:   absPath,
			MaxSize:    viper.GetInt("log.max_size"),    // MB
			MaxBackups: viper.GetInt("log.max_backups"), // 保留的旧日志文件数量
			MaxAge:     viper.GetInt("log.max_age"),     // 天数
			Compress:   true,                            // 压缩旧日志文件
		}

		// 同时输出到控制台和文件（带日志切割）
		multiWriter := io.MultiWriter(os.Stdout, lumberjackLogger)
		logrus.SetOutput(multiWriter)

		logrus.WithFields(logrus.Fields{
			"file":        utils.DisplayPath(absPath),
			"max_size":    viper.GetInt("log.max_size"),
			"max_backups": viper.GetInt("log.max_backups"),
			"max_age":     viper.GetInt("log.max_age"),
		}).Info("日志切割功能已启用")
	}
	// 当日志文件路径为空时，保持默认输出到控制台，不创建任何目录
}
