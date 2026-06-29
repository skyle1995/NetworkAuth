package cmd

import (
	"NetworkAuth/database"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// dbCmd 数据库维护命令组
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "数据库维护命令",
	Long:  "数据库维护相关子命令，如按结构体整理表结构（删除多余列、调整列顺序）。",
}

// dbTidyCmd 按结构体整理表结构
var dbTidyCmd = &cobra.Command{
	Use:   "tidy",
	Short: "按结构体整理数据表（删除多余列 / 调整列顺序）",
	Long: `按模型结构体整理数据库表结构：
  --prune    删除数据库中、结构体已不存在的多余列（不可逆，请先备份）
  --reorder  将表的物理列顺序调整为与结构体一致（列顺序仅影响观感，无功能影响）
  --apply    实际执行；不加该标志时仅预览(dry-run)，只打印计划不改库

示例：
  NetworkAuth db tidy                      # 预览：同时检查多余列与列顺序
  NetworkAuth db tidy --prune --apply      # 仅删除多余列并执行
  NetworkAuth db tidy --reorder --apply    # 仅调整列顺序并执行
  NetworkAuth db tidy --prune --reorder --apply  # 删列+排序并执行

说明：SQLite 通过整表重建实现排序（事务内原子完成）；MySQL 通过 ALTER ... MODIFY AFTER 实现。`,
	Run: runDBTidy,
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbTidyCmd)

	dbTidyCmd.Flags().Bool("prune", false, "删除结构体中不存在的多余列")
	dbTidyCmd.Flags().Bool("reorder", false, "按结构体顺序重排列")
	dbTidyCmd.Flags().Bool("apply", false, "实际执行（默认仅预览 dry-run）")
}

func runDBTidy(cmd *cobra.Command, args []string) {
	prune, _ := cmd.Flags().GetBool("prune")
	reorder, _ := cmd.Flags().GetBool("reorder")
	apply, _ := cmd.Flags().GetBool("apply")

	// 未指定任何操作时，默认两项都检查（仍为 dry-run，除非加 --apply）
	if !prune && !reorder {
		prune, reorder = true, true
		logrus.Info("[结构整理] 未指定 --prune/--reorder，默认两项都检查")
	}

	// 初始化数据库连接（配置已由根命令 PersistentPreRun 加载）
	db, err := database.Init()
	if err != nil {
		logrus.WithError(err).Fatal("[结构整理] 数据库初始化失败")
		return
	}
	if db == nil {
		logrus.Fatal("[结构整理] 数据库未连接：系统可能尚未安装，或数据库配置缺失")
		return
	}

	if err := database.TidyTables(database.TidyOptions{
		Prune:   prune,
		Reorder: reorder,
		Apply:   apply,
	}); err != nil {
		logrus.WithError(err).Fatal("[结构整理] 执行失败")
		return
	}

	if !apply {
		logrus.Info("[结构整理] 以上为预览结果；确认无误后加 --apply 实际执行")
	}
}
