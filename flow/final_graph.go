package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"qqqai/ai"
	"qqqai/config"
	"qqqai/model/chat_model"
	"qqqai/tool"
	"qqqai/tool/analyst_tools"
	"qqqai/tool/sql_tools"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type FinalGraphRequest struct {
	Query     string `json:"query" binding:"required"`
	SessionID string `json:"session_id,omitempty"`
	SQL       string `json:"sql,omitempty"`
	Docs      string `json:"docs,omitempty"`
}

const (
	Trans_List    = "Trans_List"
	Intent_Model  = "Intent_Model"
	React         = "React"
	RAGChat       = "RAGChat"
	Chat          = "Chat"
	ChatToEnd     = "ChatToEnd"
	RAGToEnd      = "RAGToEnd"
	ToToolCall    = "ToToolCall"
	MCP           = "MCP"
	AnalystGraph  = "AnalystGraph"
	AnalysisToEnd = "AnalysisToEnd"
)

func init() {
	schema.Register[*FinalGraphRequest]()
}

var (
	cachedFinalGraph  compose.Runnable[FinalGraphRequest, []*schema.Message]
	finalGraphOnce    sync.Once
	finalGraphInitErr error
)

// InitFinalGraph compiles and caches the application-level routing graph.
func InitFinalGraph(ctx context.Context, store compose.CheckPointStore) error {
	finalGraphOnce.Do(func() {
		cachedFinalGraph, finalGraphInitErr = buildFinalGraph(ctx, store)
	})
	return finalGraphInitErr
}

// GetFinalGraph returns the cached application-level routing graph.
func GetFinalGraph() (compose.Runnable[FinalGraphRequest, []*schema.Message], error) {
	if cachedFinalGraph == nil {
		return nil, fmt.Errorf("FinalGraph 未初始化，请先调用 InitFinalGraph")
	}
	return cachedFinalGraph, nil
}

func buildFinalGraph(ctx context.Context, store compose.CheckPointStore) (compose.Runnable[FinalGraphRequest, []*schema.Message], error) {
	g := compose.NewGraph[FinalGraphRequest, []*schema.Message](
		compose.WithGenLocalState(func(ctx context.Context) *FinalGraphRequest {
			return &FinalGraphRequest{}
		}),
	)

	_ = g.AddLambdaNode(Intent_Model, compose.InvokableLambda(func(ctx context.Context, input FinalGraphRequest) (*schema.Message, error) {
		if err := compose.ProcessState[*FinalGraphRequest](ctx, func(ctx context.Context, state *FinalGraphRequest) error {
			*state = input
			return nil
		}); err != nil {
			return nil, err
		}

		intentTemp := prompt.FromMessages(schema.FString,
			schema.SystemMessage(`你是一个意图识别专家。请分析用户输入，只输出 SQL、RAG 或 Chat 三个标签之一，不要输出解释。

SQL：
用户在问数据库查询、SQL 生成、数据统计、指标分析、报表需求、结构化数据分析、业务数据计算。

RAG：
用户在问文件、文档、知识库、上传资料、已索引内容、历史资料，或者问题明显需要结合外部资料 / 项目资料 / 文档内容回答。

Chat：
普通聊天、闲聊、角色对话、写作、润色、翻译、常识问答、代码解释、无需数据库和文件检索的问题。`),
			schema.UserMessage("{query}"),
		)

		cm, err := chat_model.GetChatModel(ctx, config.GlobalConfig.IntentModelType)
		if err != nil {
			return nil, err
		}
		output, err := intentTemp.Format(ctx, map[string]any{
			"query": input.Query,
		})
		if err != nil {
			return nil, err
		}
		return cm.Generate(ctx, output)
	}))

	react, err := BuildReactGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("构建 React 子图失败: %w", err)
	}
	_ = g.AddGraphNode(React, react, compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *FinalGraphRequest) ([]*schema.Message, error) {
		return []*schema.Message{schema.UserMessage(state.Query)}, nil
	}))

	_ = g.AddLambdaNode(RAGChat, compose.InvokableLambda(func(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
		var query string
		var sessionID string
		if err := compose.ProcessState[*FinalGraphRequest](ctx, func(ctx context.Context, state *FinalGraphRequest) error {
			query = state.Query
			sessionID = state.SessionID
			return nil
		}); err != nil {
			return nil, err
		}
		if sessionID == "" {
			if value, ok := ctx.Value("session_id").(string); ok {
				sessionID = value
			}
		}
		if sessionID == "" {
			sessionID = "default-session"
		}

		ragRunner, err := GetRAGChatFlow()
		if err != nil {
			return nil, err
		}
		return ragRunner.Invoke(ctx, RAGChatInput{Query: query, SessionID: sessionID})
	}))

	_ = g.AddLambdaNode(Chat, compose.InvokableLambda(func(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
		var query string
		var sessionID string
		if err := compose.ProcessState[*FinalGraphRequest](ctx, func(ctx context.Context, state *FinalGraphRequest) error {
			query = state.Query
			sessionID = state.SessionID
			return nil
		}); err != nil {
			return nil, err
		}
		if sessionID == "" {
			if value, ok := ctx.Value("session_id").(string); ok {
				sessionID = value
			}
		}
		if sessionID == "" {
			sessionID = "default-session"
		}

		reply, err := ai.GenerateReply(ctx, sessionID, query)
		if err != nil {
			return nil, err
		}
		return schema.AssistantMessage(reply, nil), nil
	}))

	_ = g.AddLambdaNode(ChatToEnd, compose.InvokableLambda(tool.MsgToMsgs))
	_ = g.AddLambdaNode(RAGToEnd, compose.InvokableLambda(tool.MsgToMsgs))
	_ = g.AddLambdaNode(Trans_List, compose.InvokableLambda(tool.MsgToMsgs))
	_ = g.AddLambdaNode(AnalysisToEnd, compose.InvokableLambda(func(ctx context.Context, input *analyst_tools.AnalysisResult) ([]*schema.Message, error) {
		if input == nil {
			return []*schema.Message{schema.AssistantMessage("数据分析完成，但未生成可用的分析结果。", nil)}, nil
		}

		formatJSON := func(value any) string {
			data, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return fmt.Sprintf("%v", value)
			}
			return string(data)
		}

		parts := make([]string, 0, 3)
		if textAnalysis := strings.TrimSpace(input.TextAnalysis); textAnalysis != "" {
			parts = append(parts, "分析报告:\n"+textAnalysis)
		}
		if input.Statistics != nil {
			parts = append(parts, "统计结果:\n"+formatJSON(input.Statistics))
		}
		if input.ChartConfig != nil {
			parts = append(parts, "图表配置:\n"+formatJSON(input.ChartConfig))
		}

		if len(parts) == 0 {
			parts = append(parts, "数据分析完成，但分析报告、统计结果和图表配置均为空。")
		}

		return []*schema.Message{schema.AssistantMessage(strings.Join(parts, "\n\n"), nil)}, nil
	}))

	_ = g.AddBranch(Trans_List, compose.NewGraphBranch(func(ctx context.Context, input []*schema.Message) (endNode string, err error) {
		if len(input) == 0 {
			return Chat, nil
		}
		content := strings.ToUpper(strings.TrimSpace(input[len(input)-1].Content))
		if strings.Contains(content, "SQL") {
			return React, nil
		}
		if strings.Contains(content, "RAG") {
			return RAGChat, nil
		}
		return Chat, nil
	}, map[string]bool{
		React:   true,
		RAGChat: true,
		Chat:    true,
	}))

	_ = g.AddLambdaNode(ToToolCall, compose.InvokableLambda(func(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
		msg, err := tool.MsgsToMsg(ctx, input)
		if err != nil {
			return nil, err
		}
		msg.Content = msg.Content[5:]
		return tool.MsgToSQLToolCall(ctx, msg)
	}))

	tools, err := sql_tools.GetMCPTool(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 MCP 工具失败: %w", err)
	}
	mcpTool, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: tools,
	})
	if err != nil {
		return nil, err
	}
	_ = g.AddToolsNode(MCP, mcpTool)

	analystGraph, err := BuildAnalystGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("构建 AnalystGraph 子图失败: %w", err)
	}
	_ = g.AddGraphNode(AnalystGraph, analystGraph)

	_ = g.AddEdge(compose.START, Intent_Model)
	_ = g.AddEdge(Intent_Model, Trans_List)

	_ = g.AddEdge(React, ToToolCall)
	_ = g.AddEdge(ToToolCall, MCP)
	_ = g.AddEdge(MCP, AnalystGraph)
	_ = g.AddEdge(AnalystGraph, AnalysisToEnd)
	_ = g.AddEdge(AnalysisToEnd, compose.END)

	_ = g.AddEdge(RAGChat, RAGToEnd)
	_ = g.AddEdge(RAGToEnd, compose.END)

	_ = g.AddEdge(Chat, ChatToEnd)
	_ = g.AddEdge(ChatToEnd, compose.END)

	return g.Compile(ctx, compose.WithCheckPointStore(store))
}
