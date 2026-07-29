// 文件职责：
// - 实现带只读防护的 NL2SQL（自然语言转 SQL）工具
// - 通过多层安全防护防止 SQL 注入和误操作
// - 包括代码层关键字过滤、超时控制、行数限制等

package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// NL2SQL 是带只读防护的 NL2SQL 工具
type NL2SQL struct {
	db *sql.DB // 数据库连接（必须是只读账号）
}

// NewNL2SQL 创建一个新的 NL2SQL 工具
// db: 数据库连接（建议使用只读账号）
func NewNL2SQL(db *sql.DB) *NL2SQL {
	return &NL2SQL{db: db}
}

// isSelectOnly 是代码层的粗校验：只允许 SELECT 查询
// 这是纵深防御的一环，不能替代只读账号
func isSelectOnly(query string) error {
	q := strings.TrimSpace(strings.ToLower(query))

	// 必须是以 select 开头
	if !strings.HasPrefix(q, "select") {
		return fmt.Errorf("只允许 SELECT 查询，检测到: %s", q)
	}

	// 禁止多条语句（防止 ; 后面的恶意语句）
	// 但允许 select 末尾的分号
	if strings.Contains(q, ";") && !strings.HasSuffix(q, ";") {
		return fmt.Errorf("禁止多条语句")
	}

	// 关键字黑名单（辅助防护）
	forbiddenKeywords := []string{
		"insert", "update", "delete", "drop", "alter", "truncate",
		"grant", "revoke", "create", "exec", "execute", "xp_",
		"union ", "union\n", "--", "/*", "*/",
	}
	for _, kw := range forbiddenKeywords {
		if strings.Contains(q, kw) {
			return fmt.Errorf("检测到禁止的关键字: %s", kw)
		}
	}

	return nil
}

// runReadOnly 在只读连接上执行查询，带超时与行数限制
func runReadOnly(ctx context.Context, db *sql.DB, query string) (string, error) {
	// 第一层：代码层关键字校验
	if err := isSelectOnly(query); err != nil {
		return "", fmt.Errorf("SQL 安全检查失败: %w", err)
	}

	// 第二层：设置查询超时（防止慢查询拖垮服务）
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 第三层：执行查询
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("查询执行失败: %w", err)
	}
	defer rows.Close()

	// 第四层：限制返回行数
	cols, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("获取列信息失败: %w", err)
	}

	var sb strings.Builder
	// 写入表头
	sb.WriteString(strings.Join(cols, " | ") + "\n")
	sb.WriteString(strings.Repeat("-", len(strings.Join(cols, " | "))) + "\n")

	// 限制行数
	count := 0
	const maxRows = 50
	for rows.Next() && count < maxRows {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("扫描行数据失败: %w", err)
		}

		cells := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				cells[i] = "NULL"
			} else {
				cells[i] = fmt.Sprintf("%v", v)
			}
		}
		sb.WriteString(strings.Join(cells, " | ") + "\n")
		count++
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("遍历结果集失败: %w", err)
	}

	if count >= maxRows {
		sb.WriteString(fmt.Sprintf("\n... (结果已截断，仅显示前 %d 行)\n", maxRows))
	} else {
		sb.WriteString(fmt.Sprintf("\n共 %d 行\n", count))
	}

	return sb.String(), nil
}

// QueryTool 创建 NL2SQL 查询工具
// 该工具将自然语言问题转换为 SQL 查询并执行（需要模型先理解如何转换）
func (n *NL2SQL) QueryTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"sql": {
				"type": "string",
				"description": "要执行的 SELECT 查询语句（必须是只读查询）"
			}
		},
		"required": ["sql"]
	}`)

	return tool.NewJSONSchemaTool(
		"nl2sql_query",
		"执行只读 SQL 查询。仅支持 SELECT 语句，禁止 INSERT/UPDATE/DELETE/DROP 等写操作。支持超时和行数限制。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			// 解析参数
			sqlQuery, ok := args["sql"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 sql 参数")
			}

			// 执行只读查询
			result, err := runReadOnly(ctx, n.db, sqlQuery)
			if err != nil {
				return nil, err
			}

			return result, nil
		},
	)
}

// GetSchemaTool 创建获取数据库 schema 的工具
// 帮助模型了解可用的表结构，从而生成正确的 SQL
func (n *NL2SQL) GetSchemaTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"table": {
				"type": "string",
				"description": "可选：指定表名，为空则返回所有表"
			}
		}
	}`)

	return tool.NewJSONSchemaTool(
		"get_db_schema",
		"获取数据库的表结构信息。不传 table 参数则返回所有表列表，传入 table 则返回该表的详细列信息。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			var tableName string
			if args["table"] != nil {
				tableArg, ok := args["table"].(string)
				if !ok {
					return nil, fmt.Errorf("table 参数类型错误")
				}
				tableName = tableArg
			}

			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			var result strings.Builder

			if tableName == "" {
				// 返回所有表
				rows, err := n.db.QueryContext(ctx,
					"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name")
				if err != nil {
					return nil, fmt.Errorf("查询表列表失败: %w", err)
				}
				defer rows.Close()

				result.WriteString("数据库表列表:\n")
				for rows.Next() {
					var name string
					if err := rows.Scan(&name); err != nil {
						return nil, err
					}
					result.WriteString(fmt.Sprintf("  - %s\n", name))
				}
			} else {
				// 返回指定表的列信息
				rows, err := n.db.QueryContext(ctx,
					`SELECT column_name, data_type, is_nullable, column_default
					 FROM information_schema.columns
					 WHERE table_name = ? AND table_schema = DATABASE()
					 ORDER BY ordinal_position`,
					tableName)
				if err != nil {
					return nil, fmt.Errorf("查询表结构失败: %w", err)
				}
				defer rows.Close()

				result.WriteString(fmt.Sprintf("表 %s 的结构:\n", tableName))
				for rows.Next() {
					var colName, dataType, isNullable string
					var defaultValue sql.NullString
					if err := rows.Scan(&colName, &dataType, &isNullable, &defaultValue); err != nil {
						return nil, err
					}
					nullable := "YES"
					if isNullable == "NO" {
						nullable = "NO"
					}
					defaultVal := "NULL"
					if defaultValue.Valid {
						defaultVal = defaultValue.String
					}
					result.WriteString(fmt.Sprintf("  - %s: %s (nullable: %s, default: %s)\n", colName, dataType, nullable, defaultVal))
				}
			}

			return result.String(), nil
		},
	)
}

// ExplainQueryTool 创建 SQL 执行计划工具
// 帮助模型理解查询性能
func (n *NL2SQL) ExplainQueryTool() tool.Tool {
	paramsJSON := []byte(`{
		"type": "object",
		"properties": {
			"sql": {
				"type": "string",
				"description": "要分析的 SELECT 查询语句"
			}
		},
		"required": ["sql"]
	}`)

	return tool.NewJSONSchemaTool(
		"explain_query",
		"分析 SELECT 查询语句的执行计划，帮助理解查询性能和优化建议。",
		paramsJSON,
		func(ctx context.Context, args map[string]any) (any, error) {
			// 解析参数
			sqlQuery, ok := args["sql"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 sql 参数")
			}

			// 安全检查
			if err := isSelectOnly(sqlQuery); err != nil {
				return nil, fmt.Errorf("SQL 安全检查失败: %w", err)
			}

			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			// 执行 EXPLAIN
			rows, err := n.db.QueryContext(ctx, fmt.Sprintf("EXPLAIN %s", sqlQuery))
			if err != nil {
				return nil, fmt.Errorf("执行 EXPLAIN 失败: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return nil, fmt.Errorf("获取列信息失败: %w", err)
			}

			var sb strings.Builder
			sb.WriteString("查询执行计划:\n")
			sb.WriteString(strings.Join(cols, " | ") + "\n")
			sb.WriteString(strings.Repeat("=", len(strings.Join(cols, " | "))) + "\n")

			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return nil, err
				}
				cells := make([]string, len(vals))
				for i, v := range vals {
					if v == nil {
						cells[i] = "NULL"
					} else {
						cells[i] = fmt.Sprintf("%v", v)
					}
				}
				sb.WriteString(strings.Join(cells, " | ") + "\n")
			}

			return sb.String(), nil
		},
	)
}
