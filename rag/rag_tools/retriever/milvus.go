package retriever

import (
	"context"
	"encoding/json"
	"log"
	"maps"
	"qqqai/config"
	"qqqai/model/embedding_model"
	"qqqai/rag/rag_tools/db"
	"strconv"
	"time"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// safeMilvusRetriever 是一个安全包装器
// 用于在集合被删除或尚未创建时，优雅地返回空结果，防止聊天主流程崩溃
type safeMilvusRetriever struct {
	actualRetriever retriever.Retriever
}

func (s *safeMilvusRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if s.actualRetriever == nil {
		log.Println("[Retriever] Milvus 集合不存在或刚被清理，跳过检索 (请上传文件以构建知识库)")
		return []*schema.Document{}, nil
	}
	return s.actualRetriever.Retrieve(ctx, query, opts...)
}

func initMilvus() {
	registerRetriever("milvus", func(ctx context.Context) (retriever.Retriever, error) {
		topK, err := strconv.Atoi(config.GlobalConfig.MilvusConf.TopK)
		if err != nil || topK <= 0 {
			topK = 10
		}

		emb, err := embedding_model.GetEmbeddingModel(context.Background(), config.GlobalConfig.EmbeddingModelType)
		if err != nil {
			return nil, err
		}

		// 1. 获取维度并检查旧集合，如果 Schema 错误（无 content 字段）则自动删除
		dim := getEmbeddingDim(ctx)
		if dim > 0 {
			_ = checkAndDropIfDimMismatch(ctx, config.GlobalConfig.MilvusConf.CollectionName, dim)
		}

		// 2. 检查集合是否还存在（可能刚被自动删除或系统刚启动还没传文件）
		exists, err := db.Milvus.HasCollection(ctx, config.GlobalConfig.MilvusConf.CollectionName)
		if err != nil {
			return nil, err
		}
		if !exists {
			// 如果集合不存在，返回一个安全的空 Retriever，聊天可以正常继续，只是没有 RAG 知识补充
			return &safeMilvusRetriever{actualRetriever: nil}, nil
		}

		sp, _ := entity.NewIndexAUTOINDEXSearchParam(1)
		outputFields := []string{"content", "metadata"}

		ret, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
			Client:       db.Milvus,
			Embedding:    emb,
			TopK:         topK,
			Collection:   config.GlobalConfig.MilvusConf.CollectionName,
			VectorField:  "vector",
			OutputFields: outputFields,
			MetricType:   entity.COSINE,
			Sp:           sp,
			VectorConverter: func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
				vecs := make([]entity.Vector, 0, len(vectors))
				for _, v := range vectors {
					v32 := make([]float32, len(v))
					for i, val := range v {
						v32[i] = float32(val)
					}
					vecs = append(vecs, entity.FloatVector(v32))
				}
				return vecs, nil
			},
			DocumentConverter: func(ctx context.Context, result client.SearchResult) ([]*schema.Document, error) {
				docs := make([]*schema.Document, result.IDs.Len())
				for i := range docs {
					docs[i] = &schema.Document{MetaData: map[string]any{}}
					id, err := result.IDs.GetAsString(i)
					if err != nil {
						return nil, err
					}
					docs[i].ID = id
				}

				for _, field := range result.Fields {
					switch field.Name() {
					case "content":
						for i := range docs {
							content, err := field.GetAsString(i)
							if err != nil {
								return nil, err
							}
							docs[i].Content = content
						}
					case "metadata":
						for i := range docs {
							raw, err := field.Get(i)
							if err != nil {
								return nil, err
							}
							mergeMetadata(docs[i].MetaData, raw)
						}
					}
				}

				for i := range docs {
					if i < len(result.Scores) {
						distance := float64(result.Scores[i])
						docs[i].MetaData["distance"] = distance
						docs[i].WithScore(1 - distance)
					}
				}

				return docs, nil
			},
		})
		if err != nil {
			return nil, err
		}

		// 包装一层返回，确保安全
		return &safeMilvusRetriever{actualRetriever: ret}, nil
	})
}

func mergeMetadata(metadata map[string]any, raw any) {
	switch value := raw.(type) {
	case []byte:
		_ = json.Unmarshal(value, &metadata)
	case string:
		_ = json.Unmarshal([]byte(value), &metadata)
	case map[string]any:
		maps.Copy(metadata, value)
	}
}

// ========== 辅助函数（用于自动清理脏数据） ==========

func getEmbeddingDim(ctx context.Context) int {
	emb, err := embedding_model.GetEmbeddingModel(context.Background(), config.GlobalConfig.EmbeddingModelType)
	if err != nil {
		return 0
	}
	vecs, err := emb.EmbedStrings(ctx, []string{"dim"})
	if err != nil || len(vecs) != 1 || len(vecs[0]) == 0 {
		return 0
	}
	return len(vecs[0])
}

func checkAndDropIfDimMismatch(ctx context.Context, collectionName string, expectedDim int) error {
	exists, err := db.Milvus.HasCollection(ctx, collectionName)
	if err != nil || !exists {
		return err
	}

	coll, err := db.Milvus.DescribeCollection(ctx, collectionName)
	if err != nil {
		return err
	}

	schemaMatch := true
	hasId, hasContent, hasMetadata := false, false, false

	for _, field := range coll.Schema.Fields {
		switch field.Name {
		case "id":
			hasId = true
		case "content":
			hasContent = true
		case "metadata":
			hasMetadata = true
		}

		if field.DataType == entity.FieldTypeFloatVector {
			dimStr, ok := field.TypeParams["dim"]
			if ok {
				dim, err := strconv.Atoi(dimStr)
				if err == nil && dim != expectedDim {
					schemaMatch = false
				}
			}
		}
	}

	if !schemaMatch || !hasId || !hasContent || !hasMetadata {
		log.Printf("[Retriever] 检测到集合 Schema 缺失必要字段或维度错误，准备清理旧集合: %s", collectionName)
		_ = db.Milvus.ReleaseCollection(ctx, collectionName)
		_ = db.Milvus.DropCollection(ctx, collectionName)

		// 等待删除生效
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			exists, _ := db.Milvus.HasCollection(ctx, collectionName)
			if !exists {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}
