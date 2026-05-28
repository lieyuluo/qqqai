package sql_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"qqqai/config"
	"sync"

	"github.com/cloudwego/eino-ext/components/tool/mcp/officialmcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	cachedMCPTools []tool.BaseTool
	mcpOnce        sync.Once
	mcpInitErr     error
)

// InitMCPTools 初始化MCP连接并缓存工具实例
func InitMCPTools(ctx context.Context) error {
	mcpOnce.Do(func() {
		cachedMCPTools, mcpInitErr = connectMCP(ctx)
	})
	return mcpInitErr
}

// connectMCP 创建MCP连接并获取工具列表
func connectMCP(ctx context.Context) ([]tool.BaseTool, error) {
	cli := mcp.NewClient(&mcp.Implementation{
		Name:    "qqqai-client",
		Version: "1.0.0",
	}, nil)

	cmd := exec.Command("npx", "-y", "mcp-server-mysql")
	cmd.Env = append(os.Environ(),
		"MYSQL_HOST="+config.GlobalConfig.MySQLConf.Host,
		"MYSQL_PORT="+config.GlobalConfig.MySQLConf.Port,
		"MYSQL_USER="+config.GlobalConfig.MySQLConf.Username,
		"MYSQL_PASS="+config.GlobalConfig.MySQLConf.Password,
		"MYSQL_DB="+config.GlobalConfig.MySQLConf.Database,
	)

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	session, err := cli.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("MCP 客户端连接失败: %w", err)
	}

	return officialmcp.GetTools(ctx, &officialmcp.Config{
		Cli:          session,
		ToolNameList: []string{"mysql_query", "list_tables", "describe_table"},
	})
}

// GetMCPTool 返回缓存的 MCP 工具列表
func GetMCPTool(ctx context.Context) ([]tool.BaseTool, error) {
	if cachedMCPTools == nil {
		return nil, fmt.Errorf("MCP 工具未初始化，请先调用 InitMCPTools")
	}
	return cachedMCPTools, nil
}

func SQLExecute(ctx context.Context, sql string) (string, error) {
	mcpTool, err := GetMCPTool(ctx)
	if err != nil {
		return "", err
	}

	var targetTool tool.InvokableTool
	var toolNames []string
	for _, t := range mcpTool {
		if invokable, ok := t.(tool.InvokableTool); ok {
			info, _ := invokable.Info(ctx)
			toolNames = append(toolNames, info.Name)
			if info.Name == "mysql_query" {
				targetTool = invokable
				break
			}
		}
	}

	if targetTool == nil {
		return "", fmt.Errorf("未找到指定的数据库执行工具, 当前可用工具: %v", toolNames)
	}

	// 使用 json.Marshal 自动处理 SQL 中的换行符和特殊字符转义
	params := map[string]string{"sql": sql}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化 MCP 参数失败: %w", err)
	}

	// 使用InvokableRun通过MCP协议发送请求到Server
	result, err := targetTool.InvokableRun(ctx, string(paramsJSON))
	if err != nil {
		return "", fmt.Errorf("MCP 工具执行失败: %w", err)
	}

	_ = compose.ProcessState[*SQLState](ctx, func(ctx context.Context, state *SQLState) error {
		state.Result = result
		return nil
	})

	return result, nil
}
