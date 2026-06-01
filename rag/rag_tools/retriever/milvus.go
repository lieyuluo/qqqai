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
	log.Printf("[Retriever][Milvus] Retrieve called: actualRetrieverNil=%t query=%q opts=%d", s.actualRetriever == nil, query, len(opts))
	if s.actualRetriever == nil {
		log.Println("[Retriever] Milvus 集合不存在或刚被清理，跳过检索 (请上传文件以构建知识库)")
		return []*schema.Document{}, nil
	}
	docs, err := s.actualRetriever.Retrieve(ctx, query, opts...)
	if err != nil {
		log.Printf("[Retriever][Milvus] actualRetriever.Retrieve failed: %v", err)
		return nil, err
	}
	log.Printf("[Retriever][Milvus] actualRetriever.Retrieve succeeded: docs=%d", len(docs))
	return docs, nil
}

func initMilvus() {
	registerRetriever("milvus", func(ctx context.Context) (retriever.Retriever, error) {
		// ==========================================
		// 🚨 新增：醒目打印当前使用的集合名称
		// ==========================================
		currentCollection := config.GlobalConfig.MilvusConf.CollectionName
		log.Println("======================================================")
		log.Printf("🔥 [Retriever][Milvus] 当前正在使用的表名: 【 %s 】", currentCollection)
		log.Println("======================================================")

		log.Printf("[Retriever][Milvus] factory start: collection=%s topK=%s embeddingModel=%s", currentCollection, config.GlobalConfig.MilvusConf.TopK, config.GlobalConfig.EmbeddingModelType)

		topK, err := strconv.Atoi(config.GlobalConfig.MilvusConf.TopK)
		if err != nil || topK <= 0 {
			topK = 10
		}
		log.Printf("[Retriever][Milvus] resolved topK=%d", topK)

		emb, err := embedding_model.GetEmbeddingModel(context.Background(), config.GlobalConfig.EmbeddingModelType)
		if err != nil {
			log.Printf("[Retriever][Milvus] GetEmbeddingModel failed: %v", err)
			return nil, err
		}

		// 1. 获取维度
		dim := getEmbeddingDim(ctx)
		log.Printf("[Retriever][Milvus] embedding dimension detected: dim=%d", dim)

		// 始终执行 schema 检查。如果 dim 为 0，我们只跳过维度比对，
		// 但依然会严格验证 'content' 和 'metadata' 字段是否存在。
		log.Printf("[Retriever][Milvus] schema check start")
		if err := checkAndDropIfDimMismatch(ctx, config.GlobalConfig.MilvusConf.CollectionName, dim); err != nil {
			log.Printf("[Retriever][Milvus] schema check failed: %v", err)
		} else {
			log.Printf("[Retriever][Milvus] schema check finished")
		}

		// 2. 检查集合是否还存在（可能刚被自动删除或系统刚启动还没传文件）
		exists, err := db.Milvus.HasCollection(ctx, config.GlobalConfig.MilvusConf.CollectionName)
		if err != nil {
			log.Printf("[Retriever][Milvus] HasCollection failed after schema check: %v", err)
			return nil, err
		}
		if !exists {
			log.Printf("[Retriever][Milvus] collection missing, returning empty safe retriever")
			return &safeMilvusRetriever{actualRetriever: nil}, nil
		}

		// ==========================================
		// 3. 强制释放并全量加载集合 (解决 Partial Load 导致的 "extra output fields" 报错)
		// ==========================================
		log.Printf("[Retriever][Milvus] 准备重新全量加载集合以清除脏缓存: %s", config.GlobalConfig.MilvusConf.CollectionName)

		// 先释放内存中的集合
		_ = db.Milvus.ReleaseCollection(ctx, config.GlobalConfig.MilvusConf.CollectionName)

		// 再强制全量加载 (false 表示同步加载，等待加载完毕)
		err = db.Milvus.LoadCollection(ctx, config.GlobalConfig.MilvusConf.CollectionName, false)
		if err != nil {
			log.Printf("[Retriever][Milvus] LoadCollection 失败 (可能是新集合尚未上传文件建索引): %v", err)
			log.Printf("[Retriever] 集合尚未就绪，降级为空检索器，保障普通聊天正常进行")
			return &safeMilvusRetriever{actualRetriever: nil}, nil
		}
		log.Printf("[Retriever][Milvus] 全量加载集合成功！")
		// ==========================================

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
			log.Printf("[Retriever][Milvus] milvus.NewRetriever failed: %v", err)
			return nil, err
		}

		// 包装一层返回，确保安全
		log.Printf("[Retriever][Milvus] milvus.NewRetriever succeeded: outputFields=%v", outputFields)
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
	log.Printf("[Retriever][Milvus] checkAndDropIfDimMismatch called: collection=%s expectedDim=%d", collectionName, expectedDim)
	exists, err := db.Milvus.HasCollection(ctx, collectionName)
	if err != nil || !exists {
		log.Printf("[Retriever][Milvus] schema check collection existence result: exists=%t err=%v", exists, err)
		return err
	}

	coll, err := db.Milvus.DescribeCollection(ctx, collectionName)
	if err != nil {
		log.Printf("[Retriever][Milvus] DescribeCollection failed: %v", err)
		return err
	}

	schemaMatch := true
	hasId, hasContent, hasMetadata := false, false, false
	fieldNames := make([]string, 0, len(coll.Schema.Fields))

	for _, field := range coll.Schema.Fields {
		fieldNames = append(fieldNames, field.Name)
		log.Printf("[Retriever][Milvus] schema field: name=%s type=%v typeParams=%v", field.Name, field.DataType, field.TypeParams)
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
	log.Printf("[Retriever][Milvus] schema check result before drop condition: fields=%v schemaMatch=%t hasId=%t hasContent=%t hasMetadata=%t", fieldNames, schemaMatch, hasId, hasContent, hasMetadata)

	if !schemaMatch || !hasId || !hasContent || !hasMetadata {
		log.Printf("[Retriever][Milvus] drop condition entered: schemaMatch=%t hasId=%t hasContent=%t hasMetadata=%t", schemaMatch, hasId, hasContent, hasMetadata)
		log.Printf("[Retriever] 检测到集合 Schema 缺失必要字段或维度错误，准备清理旧集合: %s", collectionName)
		if err := db.Milvus.ReleaseCollection(ctx, collectionName); err != nil {
			log.Printf("Release 集合失败 (可忽略): %v", err)
		}
		if err := db.Milvus.DropCollection(ctx, collectionName); err != nil {
			log.Fatalf("🚨 致命错误：Drop 集合失败！旧表依然存在，请手动去 Attu 面板删除。错误: %v", err)
		}

		// 等待删除生效
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			exists, _ := db.Milvus.HasCollection(ctx, collectionName)
			if !exists {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		log.Printf("[Retriever][Milvus] drop condition skipped: schema is considered valid")
	}
	return nil
}
