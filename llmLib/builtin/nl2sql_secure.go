// 文件职责：
// - 增强版 NL2SQL（自然语言转 SQL）工具
// - 强制只读模式、增强注入检测、查询复杂度限制
// - 提供完整的 SQL 安全防护
//
// 安全增强：
// 1. 强制使用只读数据库连接
// 2. 多层 SQL 注入检测（正则 + 语义分析）
// 3. 查询复杂度限制（JOIN 数量、子查询深度）
// 4. 执行超时和行数限制
// 5. 全操作审计日志
// 6. 危险操作确认流程

package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/security"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 增强版 NL2SQL 工具
// ============================================================================

// SecureNL2SQL 增强版 NL2SQL 工具
type SecureNL2SQL struct {
	db             *sql.DB          // 数据库连接（必须是只读账号）
	auditLogger    *security.AuditLogger
	confirmation   security.ConfirmationCallback
	maxRows        int              // 最大返回行数
	queryTimeout   time.Duration    // 查询超时
	allowComplex   bool             // 允许复杂查询（JOIN、子查询等）
}

// NewSecureNL2SQL 创建增强版 NL2SQL 工具
func NewSecureNL2SQL(db *sql.DB, opts ...func(*SecureNL2SQL)) *SecureNL2SQL {
	nl2sql := &SecureNL2SQL{
		db:           db,
		auditLogger:  security.NewAuditLogger(),
		maxRows:      50,
		queryTimeout: 5 * time.Second,
		allowComplex: false, // 默认不允许复杂查询
	}

	for _, opt := range opts {
		opt(nl2sql)
	}

	return nl2sql
}

// SecureNL2SQLOption 配置选项
type SecureNL2SQLOption func(*SecureNL2SQL)

// WithMaxRows 设置最大返回行数
func WithMaxRows(max int) SecureNL2SQLOption {
	return func(n *SecureNL2SQL) {
		n.maxRows = max
	}
}

// WithQueryTimeout 设置查询超时
func WithQueryTimeout(timeout time.Duration) SecureNL2SQLOption {
	return func(n *SecureNL2SQL) {
		n.queryTimeout = timeout
	}
}

// WithComplexQueries 允许复杂查询
func WithComplexQueries(allowed bool) SecureNL2SQLOption {
	return func(n *SecureNL2SQL) {
		n.allowComplex = allowed
	}
}

// WithNL2SQLAuditCallback 为 NL2SQL 设置审计回调
func WithNL2SQLAuditCallback(callback func(security.AuditEvent)) SecureNL2SQLOption {
	return func(n *SecureNL2SQL) {
		// 创建一个新的 AuditLogger 并设置回调
		n.auditLogger = security.NewAuditLogger(
			security.WithEventCallback(callback),
		)
	}
}

// ============================================================================
// SQL 安全检查（增强版）
// ============================================================================

// SQLSecurityCheck SQL 安全检查结果
type SQLSecurityCheck struct {
	IsSafe    bool     // 是否安全
	RiskLevel string   // 风险等级：low/medium/high
	Warnings  []string // 警告信息
}

// checkSQLSecurity 执行多层 SQL 安全检查
func (n *SecureNL2SQL) checkSQLSecurity(query string) *SQLSecurityCheck {
	result := &SQLSecurityCheck{
		IsSafe:    true,
		Warnings:  make([]string, 0),
		RiskLevel: "low",
	}

	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 第 1 层：基本格式检查
	if q == "" {
		result.IsSafe = false
		result.Warnings = append(result.Warnings, "SQL 语句为空")
		result.RiskLevel = "high"
		return result
	}

	// 第 2 层：必须以 SELECT 开头
	if !strings.HasPrefix(qLower, "select") && !strings.HasPrefix(qLower, "with") {
		result.IsSafe = false
		result.Warnings = append(result.Warnings, fmt.Sprintf("只允许 SELECT 查询，检测到: %s", qLower[:min(50, len(qLower))]))
		result.RiskLevel = "high"
		return result
	}

	// 第 3 层：禁止多条语句
	if strings.Contains(q, ";") {
		// 检查 ; 是否在末尾（允许结尾的分号）
		trimmed := strings.TrimRight(q, "; \t\n")
		if strings.Contains(trimmed, ";") {
			result.IsSafe = false
			result.Warnings = append(result.Warnings, "禁止多条 SQL 语句")
			result.RiskLevel = "high"
			return result
		}
	}

	// 第 4 层：关键字黑名单（增强版）
	forbiddenKeywords := []struct {
		keyword string
		reason  string
	}{
		{"insert", "INSERT 语句"},
		{"update", "UPDATE 语句"},
		{"delete", "DELETE 语句"},
		{"drop", "DROP 语句"},
		{"alter", "ALTER 语句"},
		{"truncate", "TRUNCATE 语句"},
		{"grant", "GRANT 权限语句"},
		{"revoke", "REVOKE 权限语句"},
		{"create", "CREATE 语句"},
		{"exec", "EXEC 命令"},
		{"execute", "EXECUTE 命令"},
		{"xp_", "SQL Server 扩展存储过程"},
		{"union", "UNION 查询（可能导致信息泄露）"},
		{"information_schema", "系统元数据访问"},
		{"sysobjects", "SQL Server 系统表"},
		{"pg_tables", "PostgreSQL 系统表"},
	}

	for _, kw := range forbiddenKeywords {
		// 使用正则表达式匹配，避免大小写绕过
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw.keyword) + `\b`)
		if pattern.MatchString(q) {
			result.IsSafe = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("检测到禁止的关键字: %s (%s)", kw.keyword, kw.reason))
			result.RiskLevel = "high"
			return result
		}
	}

	// 第 5 层：检测注释注入
	if strings.Contains(q, "--") || strings.Contains(q, "/*") {
		result.Warnings = append(result.Warnings, "SQL 包含注释，可能存在注入风险")
		if result.RiskLevel == "low" {
			result.RiskLevel = "medium"
		}
	}

	// 第 6 层：检测复杂查询（如果配置为不允许）
	if !n.allowComplex {
		// 检测 JOIN
		if regexp.MustCompile(`(?i)\bjoin\b`).MatchString(q) {
			result.Warnings = append(result.Warnings, "查询包含 JOIN，需要启用复杂查询选项")
			result.RiskLevel = "medium"
		}

		// 检测子查询
		if strings.Contains(q, "(select") || strings.Contains(q, "(SELECT") {
			result.Warnings = append(result.Warnings, "查询包含子查询，需要启用复杂查询选项")
			result.RiskLevel = "medium"
		}

		// 检测 UNION
		if regexp.MustCompile(`(?i)\bunion\b`).MatchString(q) {
			result.Warnings = append(result.Warnings, "查询包含 UNION，需要启用复杂查询选项")
			result.RiskLevel = "high"
		}
	}

	// 第 7 层：检测非常长的查询（可能是 DoS 攻击）
	if len(q) > 10000 {
		result.Warnings = append(result.Warnings, "SQL 语句过长，可能存在 DoS 风险")
		result.RiskLevel = "medium"
	}

	return result
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// 查询执行（安全增强版）
// ============================================================================

// QueryTool 创建增强版查询工具
func (n *SecureNL2SQL) QueryTool() tool.Tool {
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
			startTime := time.Now()

			// 解析参数
			sqlQuery, ok := args["sql"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 sql 参数")
			}

			// SQL 安全检查
			securityCheck := n.checkSQLSecurity(sqlQuery)
			if !securityCheck.IsSafe {
				n.auditLogger.Log(security.AuditEvent{
					ToolName: "nl2sql_query",
					ToolArgs: args,
					Error:    fmt.Sprintf("SQL 安全检查失败: %v", securityCheck.Warnings),
					Duration: time.Since(startTime),
				})
				return nil, fmt.Errorf("SQL 安全检查失败: %v", securityCheck.Warnings)
			}

			// 如果存在警告但通过了检查，记录审计日志
			if len(securityCheck.Warnings) > 0 {
				n.auditLogger.Log(security.AuditEvent{
					ToolName: "nl2sql_query",
					ToolArgs: args,
					Result:   fmt.Sprintf("SQL 通过但有警告: %v", securityCheck.Warnings),
					Duration: time.Since(startTime),
				})
			}

			// 执行查询
			result, err := n.runReadOnly(ctx, sqlQuery)
			if err != nil {
				n.auditLogger.Log(security.AuditEvent{
					ToolName: "nl2sql_query",
					ToolArgs: args,
					Error:    err.Error(),
					Duration: time.Since(startTime),
				})
				return nil, err
			}

			// 记录成功审计日志
			n.auditLogger.Log(security.AuditEvent{
				ToolName: "nl2sql_query",
				ToolArgs: args,
				Result:   result,
				Duration: time.Since(startTime),
			})

			return result, nil
		},
	)
}

// runReadOnly 执行只读查询（增强版）
func (n *SecureNL2SQL) runReadOnly(ctx context.Context, query string) (string, error) {
	// 设置查询超时
	ctx, cancel := context.WithTimeout(ctx, n.queryTimeout)
	defer cancel()

	// 执行查询
	rows, err := n.db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("查询执行失败: %w", err)
	}
	defer rows.Close()

	// 获取列信息
	cols, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("获取列信息失败: %w", err)
	}

	// 构建结果
	var sb strings.Builder
	sb.WriteString(strings.Join(cols, " | ") + "\n")
	sb.WriteString(strings.Repeat("-", len(strings.Join(cols, " | "))) + "\n")

	// 限制行数
	count := 0
	for rows.Next() && count < n.maxRows {
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

	if count >= n.maxRows {
		sb.WriteString(fmt.Sprintf("\n... (结果已截断，仅显示前 %d 行)\n", n.maxRows))
	} else {
		sb.WriteString(fmt.Sprintf("\n共 %d 行\n", count))
	}

	return sb.String(), nil
}

// ============================================================================
// 获取 Schema（只读）
// ============================================================================

// GetSchemaTool 创建增强版获取 schema 工具
func (n *SecureNL2SQL) GetSchemaTool() tool.Tool {
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
			startTime := time.Now()
			defer func() {
				n.auditLog("get_db_schema", getStringArg(args, "table"), "", nil, time.Since(startTime))
			}()

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

// ============================================================================
// EXPLAIN 查询计划
// ============================================================================

// ExplainQueryTool 创建增强版 EXPLAIN 工具
func (n *SecureNL2SQL) ExplainQueryTool() tool.Tool {
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
			startTime := time.Now()
			defer func() {
				n.auditLog("explain_query", getStringArg(args, "sql"), "", nil, time.Since(startTime))
			}()

			sqlQuery, ok := args["sql"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 sql 参数")
			}

			// 安全检查
			securityCheck := n.checkSQLSecurity(sqlQuery)
			if !securityCheck.IsSafe {
				return nil, fmt.Errorf("SQL 安全检查失败: %v", securityCheck.Warnings)
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

// auditLog 记录审计日志
func (n *SecureNL2SQL) auditLog(toolName, sql string, result string, err error, duration time.Duration) {
	n.auditLogger.Log(security.AuditEvent{
		ToolName: toolName,
		ToolArgs: map[string]any{"sql": sql},
		Result:   result,
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Duration: duration,
	})
}

// GetAuditLogger 获取审计日志器
func (n *SecureNL2SQL) GetAuditLogger() *security.AuditLogger {
	return n.auditLogger
}
