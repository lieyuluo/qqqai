package rag_flow

import (
	"context"
	"fmt"
	"qqqai/rag/rag_tools/indexer"
	document2 "qqqai/tool/document"
	"sync"

	"github.com/cloudwego/eino/components/document"

	"github.com/cloudwego/eino/compose"
)

const (
	Milvus   = "Milvus"
	ES       = "ES"
	Splitter = "Splitter"
	Parser   = "Parser"
	Loader   = "Loader"
)

var (
	cachedIndexingGraph  compose.Runnable[document.Source, []string]
	indexingGraphOnce    sync.Once
	indexingGraphInitErr error
)

// InitIndexingGraph 在应用启动时编译并缓存索引图
func InitIndexingGraph(ctx context.Context) error {
	indexingGraphOnce.Do(func() {
		cachedIndexingGraph, indexingGraphInitErr = buildIndexingGraph(ctx)
	})
	return indexingGraphInitErr
}

func GetIndexingGraph() (compose.Runnable[document.Source, []string], error) {
	if cachedIndexingGraph == nil {
		return nil, fmt.Errorf("IndexingGraph 未初始化，请先调用 InitIndexingGraph")
	}
	return cachedIndexingGraph, nil
}

// buildIndexingGraph 创建索引图
func buildIndexingGraph(ctx context.Context) (compose.Runnable[document.Source, []string], error) {
	// 创建图
	g := compose.NewGraph[document.Source, []string]()

	milvus, err := indexer.GetIndexer(ctx, "milvus")
	if err != nil {
		return nil, err
	}
	es, err := indexer.GetIndexer(ctx, "es")
	if err != nil {
		return nil, err
	}

	// 添加节点
	if err := g.AddLoaderNode(Loader, document2.Loader); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode(Parser, compose.InvokableLambda(BuildParseNode)); err != nil {
		return nil, err
	}
	if err := g.AddDocumentTransformerNode(Splitter, document2.Splitter); err != nil {
		return nil, err
	}
	if err := g.AddIndexerNode(Milvus, milvus, compose.WithOutputKey("milvus_res")); err != nil {
		return nil, err
	}
	if err := g.AddIndexerNode(ES, es, compose.WithOutputKey("es_res")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("Merge", compose.InvokableLambda(func(ctx context.Context, input map[string]any) ([]string, error) {
		var allIDs []string
		for _, ids := range input {
			i, _ := ids.([]string)
			allIDs = append(allIDs, i...)
		}
		return allIDs, nil
	})); err != nil {
		return nil, err
	}

	// 设置边
	if err := g.AddEdge(compose.START, Loader); err != nil {
		return nil, err
	}
	if err := g.AddEdge(Loader, Parser); err != nil {
		return nil, err
	}
	if err := g.AddEdge(Parser, Splitter); err != nil {
		return nil, err
	}
	if err := g.AddEdge(Splitter, Milvus); err != nil {
		return nil, err
	}
	if err := g.AddEdge(Splitter, ES); err != nil {
		return nil, err
	}
	if err := g.AddEdge(Milvus, "Merge"); err != nil {
		return nil, err
	}
	if err := g.AddEdge(ES, "Merge"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("Merge", compose.END); err != nil {
		return nil, err
	}

	// 编译图
	r, err := g.Compile(
		ctx,
		compose.WithGraphName("RAGIndexing"),
		compose.WithNodeTriggerMode(compose.AllPredecessor),
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}
