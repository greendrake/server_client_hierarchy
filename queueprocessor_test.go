package server_client_hierarchy

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestQP(t *testing.T) {
	assert := assert.New(t)
	counter := 0
	qp := &QueueProcessor{
		chunkHandler: func(chunk any) {
			counter += chunk.(int)
		},
	}
	qp.Put(5)
	qp.Put(2)
	qp.Put(1)
	qp.Put(1)
	qp.Put(1)
	qp.Put(1)
	qp.Put(1)
	qp.Wait()
	assert.Equal(12, counter)
}
