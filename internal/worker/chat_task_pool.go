package worker

import (
	"context"
	"fmt"
	"sync"

	"qqqai/internal/service"
)

type ChatTask struct {
	Request service.ChatRequest
	Result  chan ChatResult
	Ctx     context.Context
}

type ChatResult struct {
	Response service.ChatResponse
	Error    error
}

type ChatTaskPool struct {
	service *service.ChatService
	tasks   chan ChatTask
	wg      sync.WaitGroup
}

func NewChatTaskPool(chatService *service.ChatService, workers, queueSize int) *ChatTaskPool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 64
	}
	pool := &ChatTaskPool{
		service: chatService,
		tasks:   make(chan ChatTask, queueSize),
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

func (p *ChatTaskPool) Submit(ctx context.Context, req service.ChatRequest) (service.ChatResponse, error) {
	result := make(chan ChatResult, 1)
	task := ChatTask{Request: req, Result: result, Ctx: ctx}
	select {
	case p.tasks <- task:
	case <-ctx.Done():
		return service.ChatResponse{}, ctx.Err()
	default:
		return service.ChatResponse{}, fmt.Errorf("chat task queue is full")
	}

	select {
	case res := <-result:
		return res.Response, res.Error
	case <-ctx.Done():
		return service.ChatResponse{}, ctx.Err()
	}
}

func (p *ChatTaskPool) Close() {
	close(p.tasks)
	p.wg.Wait()
}

func (p *ChatTaskPool) worker() {
	defer p.wg.Done()
	for task := range p.tasks {
		resp, err := p.service.Chat(task.Ctx, task.Request)
		select {
		case task.Result <- ChatResult{Response: resp, Error: err}:
		case <-task.Ctx.Done():
		}
	}
}
