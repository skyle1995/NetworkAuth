package database

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ============================================================================
// 数据表结构整理（手动维护命令使用，不在程序启动时自动执行）
//
// 功能：
//  1. 移除无效字段：删除数据库表中、结构体已不存在的列；
//  2. 字段排序：把表的物理列顺序调整为与结构体定义一致。
//
// 安全说明：
//  - 默认 dry-run（仅打印计划，不改库）；需 Apply=true 才真正执行。
//  - 列顺序对程序无功能影响，仅为观感整齐；删列不可逆，请先备份数据库。
//  - SQLite：DDL 支持事务，采用「改名→按结构体建新表→拷数据→删旧表」整表重建（原子）。
//  - MySQL：DDL 非事务，采用 ALTER TABLE DROP COLUMN（删列）+ MODIFY ... AFTER（排序），不重建。
// ============================================================================

// TidyOptions 整理选项
type TidyOptions struct {
	Prune   bool // 是否删除结构体中不存在的多余列
	Reorder bool // 是否按结构体顺序重排列
	Apply   bool // false=仅预览(dry-run)；true=实际执行
}

// tablePlan 单表的整理计划
type tablePlan struct {
	table       string
	model       any
	extra       []string // 多余列（数据库有、结构体无）
	expected    []string // 结构体顺序的列名
	current     []string // 当前物理顺序的列名
	needReorder bool     // 去除多余列后，公共列顺序是否与结构体不一致
}

// TidyTables 按结构体整理所有表的列（删多余列 + 排序）
func TidyTables(opts TidyOptions) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("数据库未连接（系统可能尚未安装，或数据库配置缺失）")
	}
	dialect := db.Dialector.Name() // "sqlite" / "mysql"

	mode := "预览(dry-run，不改库)"
	if opts.Apply {
		mode = "执行(将修改数据库)"
	}
	logrus.WithFields(logrus.Fields{
		"dialect": dialect, "prune": opts.Prune, "reorder": opts.Reorder,
	}).Infof("[结构整理] 开始 —— %s", mode)

	var changed, skipped int
	for _, model := range AllModels() {
		plan, err := buildTablePlan(db, model)
		if err != nil {
			logrus.WithError(err).Error("[结构整理] 解析表结构失败，跳过")
			continue
		}
		if plan == nil {
			continue // 表不存在
		}

		hasExtra := opts.Prune && len(plan.extra) > 0
		doReorder := opts.Reorder && plan.needReorder
		if !hasExtra && !doReorder {
			skipped++
			continue
		}

		// 打印计划
		if hasExtra {
			logrus.Warnf("[结构整理] 表 %s 待删除多余列: %v", plan.table, plan.extra)
		}
		if doReorder {
			logrus.Warnf("[结构整理] 表 %s 列顺序需调整 -> 结构体顺序", plan.table)
		}

		if !opts.Apply {
			changed++
			continue // dry-run 不执行
		}

		if err := applyTablePlan(db, dialect, plan, hasExtra, doReorder); err != nil {
			logrus.WithError(err).Errorf("[结构整理] 表 %s 执行失败", plan.table)
			return err
		}
		logrus.Infof("[结构整理] 表 %s 调整完成", plan.table)
		changed++
	}

	logrus.Infof("[结构整理] 完成：%d 张表%s，%d 张无需调整",
		changed, map[bool]string{true: "已调整", false: "待调整"}[opts.Apply], skipped)
	return nil
}

// buildTablePlan 计算单表的整理计划；表不存在时返回 (nil, nil)
func buildTablePlan(db *gorm.DB, model any) (*tablePlan, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return nil, err
	}
	table := stmt.Schema.Table
	if !db.Migrator().HasTable(model) {
		return nil, nil
	}

	expected := append([]string(nil), stmt.Schema.DBNames...) // 结构体顺序的列
	expectedSet := toSet(expected)

	colTypes, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		return nil, err
	}
	current := make([]string, 0, len(colTypes))
	for _, c := range colTypes {
		current = append(current, c.Name()) // 物理顺序
	}
	currentSet := toSet(current)

	// 多余列：当前有、结构体无
	var extra []string
	for _, c := range current {
		if !expectedSet[c] {
			extra = append(extra, c)
		}
	}

	// 公共列在「结构体顺序」与「当前顺序」下的序列，用于判断是否需要重排
	var desired, currentCommon []string
	for _, c := range expected {
		if currentSet[c] {
			desired = append(desired, c)
		}
	}
	for _, c := range current {
		if expectedSet[c] {
			currentCommon = append(currentCommon, c)
		}
	}

	return &tablePlan{
		table: table, model: model, extra: extra,
		expected: expected, current: current,
		needReorder: !equalSlice(desired, currentCommon),
	}, nil
}

// applyTablePlan 执行整理计划
func applyTablePlan(db *gorm.DB, dialect string, plan *tablePlan, doPrune, doReorder bool) error {
	// 需要重排时：SQLite 走整表重建（顺带丢弃多余列）；MySQL 走 MODIFY AFTER。
	if doReorder {
		if dialect == "sqlite" {
			// 重建时仅拷贝「结构体存在的列」，故无论是否 doPrune 都会移除多余列
			return rebuildSQLiteTable(db, plan)
		}
		// MySQL：先删多余列（若需要），再排序
		if doPrune {
			if err := dropExtraColumns(db, plan); err != nil {
				return err
			}
		}
		return reorderMySQLTable(db, plan)
	}
	// 仅删多余列（不重排）
	if doPrune {
		return dropExtraColumns(db, plan)
	}
	return nil
}

// dropExtraColumns 逐列删除多余字段（SQLite/MySQL 均支持 DROP COLUMN）
func dropExtraColumns(db *gorm.DB, plan *tablePlan) error {
	for _, c := range plan.extra {
		if err := db.Migrator().DropColumn(plan.model, c); err != nil {
			return fmt.Errorf("删除列 %s.%s 失败: %w", plan.table, c, err)
		}
		logrus.Infof("  - 已删除列 %s.%s", plan.table, c)
	}
	return nil
}

// rebuildSQLiteTable 通过整表重建实现 SQLite 的列排序（顺带移除多余列）
// SQLite DDL 支持事务，整个过程在事务内完成，失败自动回滚。
func rebuildSQLiteTable(db *gorm.DB, plan *tablePlan) error {
	table := plan.table
	tmp := table + "__tidy_tmp"

	// 拷贝列 = 结构体顺序里、当前表也存在的列
	currentSet := toSet(plan.current)
	var common []string
	for _, c := range plan.expected {
		if currentSet[c] {
			common = append(common, c)
		}
	}

	// 重建期间关闭外键检查（PRAGMA 不能在事务内生效，需在事务外设置）
	db.Exec("PRAGMA foreign_keys = OFF")
	defer db.Exec("PRAGMA foreign_keys = ON")

	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(tmp) {
			if err := tx.Migrator().DropTable(tmp); err != nil {
				return err
			}
		}
		// 0. 先删除旧表的显式索引：SQLite 中表改名后显式索引名不会跟着变，
		//    会与 CreateTable 重建的同名索引冲突。自动索引(sqlite_autoindex_*)随表改名，无需处理。
		idxNames, err := sqliteExplicitIndexNames(tx, table)
		if err != nil {
			return err
		}
		for _, idx := range idxNames {
			if err := tx.Exec("DROP INDEX IF EXISTS " + quoteIdent("sqlite", idx)).Error; err != nil {
				return fmt.Errorf("删除旧索引 %s 失败: %w", idx, err)
			}
		}
		// 1. 旧表改名为临时表
		if err := tx.Migrator().RenameTable(table, tmp); err != nil {
			return fmt.Errorf("重命名旧表失败: %w", err)
		}
		// 2. 按结构体重建新表（列顺序=结构体顺序，含索引/约束）
		if err := tx.Migrator().CreateTable(plan.model); err != nil {
			return fmt.Errorf("重建新表失败: %w", err)
		}
		// 3. 拷贝公共列数据
		if len(common) > 0 {
			cols := quoteList("sqlite", common)
			sql := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
				quoteIdent("sqlite", table), cols, cols, quoteIdent("sqlite", tmp))
			if err := tx.Exec(sql).Error; err != nil {
				return fmt.Errorf("拷贝数据失败: %w", err)
			}
		}
		// 4. 删除临时表
		if err := tx.Migrator().DropTable(tmp); err != nil {
			return fmt.Errorf("删除临时表失败: %w", err)
		}
		return nil
	})
}

// reorderMySQLTable 通过 ALTER TABLE ... MODIFY ... AFTER 调整 MySQL 列顺序
// 列定义取自 SHOW CREATE TABLE，避免手工拼 DDL 丢失类型/默认值/注释。
func reorderMySQLTable(db *gorm.DB, plan *tablePlan) error {
	defs, err := mysqlColumnDefs(db, plan.table)
	if err != nil {
		return err
	}
	// 仅排列结构体存在、且当前表也有的列
	currentSet := toSet(plan.current)
	prev := ""
	for _, col := range plan.expected {
		if !currentSet[col] {
			continue
		}
		def, ok := defs[col]
		if !ok {
			return fmt.Errorf("未能从 SHOW CREATE TABLE 解析到列 %s 的定义", col)
		}
		pos := "FIRST"
		if prev != "" {
			pos = "AFTER " + quoteIdent("mysql", prev)
		}
		sql := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s",
			quoteIdent("mysql", plan.table), def, pos)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("调整列 %s 顺序失败: %w", col, err)
		}
		prev = col
	}
	return nil
}

// mysqlColumnDefs 从 SHOW CREATE TABLE 解析每列的完整定义（不含末尾逗号）
func mysqlColumnDefs(db *gorm.DB, table string) (map[string]string, error) {
	var name, ddl string
	row := db.Raw("SHOW CREATE TABLE " + quoteIdent("mysql", table)).Row()
	if err := row.Scan(&name, &ddl); err != nil {
		return nil, err
	}
	defs := map[string]string{}
	for _, line := range strings.Split(ddl, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ",")
		if !strings.HasPrefix(line, "`") { // 列定义行才以 `列名` 开头，索引/约束行不是
			continue
		}
		end := strings.Index(line[1:], "`")
		if end < 0 {
			continue
		}
		col := line[1 : 1+end]
		defs[col] = line // 形如 `col` varchar(64) NOT NULL DEFAULT '' COMMENT '...'
	}
	return defs, nil
}

// sqliteExplicitIndexNames 返回某表上由 CREATE INDEX 显式创建的索引名
// （排除 UNIQUE/PK 约束自动生成的 sqlite_autoindex_* —— 它们随表改名，且不可单独 DROP）
func sqliteExplicitIndexNames(tx *gorm.DB, table string) ([]string, error) {
	rows, err := tx.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=? AND name NOT LIKE 'sqlite_autoindex_%'",
		table,
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// ---- 小工具 ----

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func quoteIdent(dialect, id string) string {
	if dialect == "mysql" {
		return "`" + id + "`"
	}
	return `"` + id + `"`
}

func quoteList(dialect string, cols []string) string {
	qs := make([]string, len(cols))
	for i, c := range cols {
		qs[i] = quoteIdent(dialect, c)
	}
	return strings.Join(qs, ", ")
}
