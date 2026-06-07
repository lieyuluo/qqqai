package worker

import (
	"context"
	"fmt"
	"log"
	"sync"

	"qqqai/internal/dao"
	"qqqai/internal/entity"
	"qqqai/rag/rag_flow"

	"github.com/cloudwego/eino/components/document"
)

type FileIndexTask struct {
	FileID int64
	Path   string
}

type FileIndexWorker struct {
	tasks chan FileIndexTask
	ctx   context.Context
	wg    sync.WaitGroup
}

func NewFileIndexWorker(ctx context.Context, workers, queueSize int) *FileIndexWorker {
	if workers <= 0 {
		workers = 2
	}
	if queueSize <= 0 {
		queueSize = 32
	}
	w := &FileIndexWorker{
		tasks: make(chan FileIndexTask, queueSize),
		ctx:   ctx,
	}
	for i := 0; i < workers; i++ {
		w.wg.Add(1)
		go w.worker()
	}
	return w
}

func (w *FileIndexWorker) Submit(task FileIndexTask) error {
	select {
	case w.tasks <- task:
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	default:
		return fmt.Errorf("file index queue is full")
	}
}

func (w *FileIndexWorker) Close() {
	close(w.tasks)
	w.wg.Wait()
}

func (w *FileIndexWorker) worker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case task, ok := <-w.tasks:
			if !ok {
				return
			}
			w.index(task)
		}
	}
}

func (w *FileIndexWorker) index(task FileIndexTask) {
	ctx := w.ctx
	if err := dao.UpdateFileStatus(ctx, task.FileID, entity.FileStatusIndexing, "", 0); err != nil {
		log.Printf("update file indexing status failed: %v", err)
	}
	runnable, err := rag_flow.GetIndexingGraph()
	if err != nil {
		_ = dao.UpdateFileStatus(ctx, task.FileID, entity.FileStatusFailed, err.Error(), 0)
		return
	}
	ids, err := runnable.Invoke(ctx, document.Source{URI: task.Path})
	if err != nil {
		_ = dao.UpdateFileStatus(ctx, task.FileID, entity.FileStatusFailed, err.Error(), 0)
		return
	}
	_ = dao.UpdateFileStatus(ctx, task.FileID, entity.FileStatusIndexed, "", len(ids))
}
