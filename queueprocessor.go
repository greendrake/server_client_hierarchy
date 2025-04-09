package server_client_hierarchy

import (
	"github.com/greendrake/mutexqueue"
	"sync"
)

type ChunkHandler func(chunk any)

type QueueProcessor struct {
	queue        *mutexqueue.MutexQueue
	chunkHandler ChunkHandler
	wg           sync.WaitGroup
	m            sync.Mutex
	isProcessing bool
}

func (q *QueueProcessor) Put(chunk any) {
	// Do not queue or do anything unless there is a handler
	if q.chunkHandler != nil {
		if q.queue == nil {
			q.queue = mutexqueue.New()
		}
		q.wg.Add(1)
		q.queue.Put(chunk)
		if !q.isProcessing {
			go q.processChunks()
		}
	}
}

func (q *QueueProcessor) processChunks() {
	q.m.Lock()
	q.isProcessing = true
	defer q.m.Unlock()
	for !q.queue.IsEmpty() {
		q.chunkHandler(q.queue.Get())
		q.wg.Done()
	}
	q.isProcessing = false
}

func (q *QueueProcessor) Wait() {
	q.wg.Wait()
}
