package excel

import (
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportData 导出数据配置
type ExportData struct {
	Headers []string    // 表头显示名称列表
	Fields  []string    // 对应结构体字段名或Map键名
	Data    interface{} // 数据源（必须是切片类型）
	Sheet   string      // 工作表名称，默认为 Sheet1
}

// Export 生成Excel文件
func Export(config ExportData) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := config.Sheet
	if sheet == "" {
		sheet = "Sheet1"
	}

	// 如果不是默认Sheet1，则创建新Sheet
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}

	// 设置表头
	for i, header := range config.Headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, header)
	}

	// 设置表头样式（加粗、背景色）
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	f.SetRowStyle(sheet, 1, 1, style)

	// 处理数据
	val := reflect.ValueOf(config.Data)
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("data must be a slice")
	}

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		// 如果是指针，获取其指向的值
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		rowNum := i + 2 // 从第2行开始（第1行是表头）

		for j, field := range config.Fields {
			cellName, _ := excelize.CoordinatesToCellName(j+1, rowNum)
			var cellValue interface{}

			// 尝试从结构体或Map中获取值
			if item.Kind() == reflect.Struct {
				fieldVal := item.FieldByName(field)
				if fieldVal.IsValid() {
					cellValue = fieldVal.Interface()
				}
			} else if item.Kind() == reflect.Map {
				key := reflect.ValueOf(field)
				mapVal := item.MapIndex(key)
				if mapVal.IsValid() {
					cellValue = mapVal.Interface()
				}
			}

			// 特殊类型处理
			switch v := cellValue.(type) {
			case time.Time:
				if !v.IsZero() {
					f.SetCellValue(sheet, cellName, v.Format("2006-01-02 15:04:05"))
				}
			case *time.Time:
				if v != nil && !v.IsZero() {
					f.SetCellValue(sheet, cellName, v.Format("2006-01-02 15:04:05"))
				}
			default:
				f.SetCellValue(sheet, cellName, v)
			}
		}
	}

	f.SetActiveSheet(index)
	// 如果创建了新Sheet且名字不叫Sheet1，删除默认的Sheet1
	if sheet != "Sheet1" {
		f.DeleteSheet("Sheet1")
	}

	return f, nil
}

// Parse 读取Excel文件内容，返回二维字符串数组
func Parse(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 获取第一个工作表
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
