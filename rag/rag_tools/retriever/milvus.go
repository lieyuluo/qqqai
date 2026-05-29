package retriever

import (
	"context"
	"encoding/json"
	"maps"
	"qqqai/config"
	"qqqai/model/embedding_model"
	"qqqai/rag/rag_tools/db"
	"strconv"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func initMilvus() {
	registerRetriever("milvus", func(ctx context.Context) (retriever.Retriever, error) {
		topK, err := strconv.Atoi(config.GlobalConfig.MilvusConf.TopK)
		if err != nil || topK <= 0 {
			topK = 10
		}
		sp, _ := entity.NewIndexAUTOINDEXSearchParam(1)
		emb, err := embedding_model.GetEmbeddingModel(context.Background(), config.GlobalConfig.EmbeddingModelType)
		if err != nil {
			return nil, err
		}
		outputFields, err := existingMilvusOutputFields(ctx, config.GlobalConfig.MilvusConf.CollectionName)
		if err != nil {
			return nil, err
		}
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

		return ret, nil
	})
}

func existingMilvusOutputFields(ctx context.Context, collectionName string) ([]string, error) {
	wanted := []string{"content", "metadata"}
	exists, err := db.Milvus.HasCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return wanted, nil
	}

	collection, err := db.Milvus.DescribeCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]struct{}, len(collection.Schema.Fields))
	for _, field := range collection.Schema.Fields {
		fields[field.Name] = struct{}{}
	}

	outputFields := make([]string, 0, len(wanted))
	for _, field := range wanted {
		if _, ok := fields[field]; ok {
			outputFields = append(outputFields, field)
		}
	}
	return outputFields, nil
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
