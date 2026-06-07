package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"qqqai/config"
	"qqqai/tool/sql_tools"
)

type SQLService struct{}

func NewSQLService() *SQLService {
	return &SQLService{}
}

func (s *SQLService) Generate(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	return sql_tools.SQLGenerate(ctx, prompt)
}

func (s *SQLService) Execute(ctx context.Context, sqlText string, confirm bool) (string, string, error) {
	if !confirm {
		return "", "", fmt.Errorf("confirm must be true before executing SQL")
	}
	cleaned, err := ValidateSelectSQL(sqlText)
	if err != nil {
		return "", "", err
	}
	wrapped := fmt.Sprintf("SELECT * FROM (%s) AS qqqai_safe LIMIT %d", cleaned, config.GetSQLMaxRows())
	result, err := sql_tools.SQLExecute(ctx, wrapped)
	return wrapped, result, err
}

var (
	lineCommentRE  = regexp.MustCompile(`(?m)--[^\n\r]*`)
	blockCommentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)
	spaceRE        = regexp.MustCompile(`\s+`)
	dangerSQLRE    = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|grant|revoke|call|load|replace|merge|outfile|dumpfile)\b|for\s+update`)
)

func ValidateSelectSQL(sqlText string) (string, error) {
	cleaned := strings.TrimSpace(sqlText)
	if cleaned == "" {
		return "", fmt.Errorf("sql is required")
	}
	cleaned = blockCommentRE.ReplaceAllString(cleaned, " ")
	cleaned = lineCommentRE.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	if strings.HasSuffix(cleaned, ";") {
		cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, ";"))
	}
	if strings.Contains(cleaned, ";") {
		return "", fmt.Errorf("only one SELECT statement is allowed")
	}
	normalized := strings.TrimSpace(spaceRE.ReplaceAllString(cleaned, " "))
	if !strings.HasPrefix(strings.ToLower(normalized), "select ") && !strings.EqualFold(normalized, "select") {
		return "", fmt.Errorf("only SELECT statements are allowed")
	}
	if dangerSQLRE.MatchString(normalized) {
		return "", fmt.Errorf("dangerous SQL keyword is not allowed")
	}
	return normalized, nil
}
