package server_client_hierarchy

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type TestServerNode struct {
	Node
}

func newTestServerNode() *TestServerNode {
	n := &TestServerNode{}
	n.SetTask(func(ch chan bool) {
		for {
			select {
			case <-ch:
				return
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
	})
	return n
}

type TestClientNode struct {
	Node
}

func newTestClientNode() *TestClientNode {
	n := &TestClientNode{}
	n.Node.SetPrincipallyClient(true)
	return n
}

func TestContext(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := newTestServerNode()
	w.SetContextWaiter(ctx)
	assert.Equal(false, w.IsRunning())
	w.Start()
	assert.Equal(true, w.IsRunning())
	cancel()
	w.Wait()
	assert.Equal(false, w.IsRunning())
}

func TestStandaloneTask(t *testing.T) {
	assert := assert.New(t)
	w := newTestServerNode()
	testInput := "foo"
	var chunkHolder string
	w.SetOChunkHandler(func(chunk any) {
		chunkHolder = chunk.(string)
	})
	assert.Equal(false, w.IsRunning())
	w.Start()
	assert.Equal(true, w.IsRunning())
	w.input(testInput)
	time.Sleep(time.Millisecond)
	w.Stop()
	assert.Equal(false, w.IsRunning())
	assert.Equal(testInput, chunkHolder)
}

func TestBasicHierarchy(t *testing.T) {
	assert := assert.New(t)
	L0 := newTestServerNode()
	assert.Equal(false, L0.IsRunning())
	L1_1 := newTestServerNode()

	// 2 levels
	L0.AddClient(L1_1)
	assert.Equal(true, L0.IsRunning())
	L1_2 := newTestServerNode()
	L0.AddClient(L1_2)
	assert.Equal(true, L0.IsRunning())
	L0.RemoveClient(L1_2)
	assert.Equal(true, L0.IsRunning())
	L0.RemoveClient(L1_1)
	assert.Equal(false, L0.IsRunning())

	// 3 levels, leafs
	L2_1 := newTestClientNode()
	L0.AddClient(L1_1)
	assert.Equal(true, L0.IsRunning())
	assert.Equal(true, L1_1.IsRunning())
	assert.Equal(false, L2_1.IsRunning())
	L1_1.AddClient(L2_1)
	assert.Equal(true, L1_1.IsRunning())
	assert.Equal(true, L2_1.IsRunning())
	L1_1.RemoveClient(L2_1)
	assert.Equal(false, L1_1.IsRunning())
	assert.Equal(false, L2_1.IsRunning())
	assert.Equal(false, L0.IsRunning())
	L1_1.AddClient(L2_1)
	assert.Equal(true, L1_1.IsRunning())
	assert.Equal(true, L2_1.IsRunning())
	L1_1.Stop()
	assert.Equal(false, L0.IsRunning())
	assert.Equal(false, L1_1.IsRunning())
	assert.Equal(false, L2_1.IsRunning())
}

func TestChunkPassFromServerToClient(t *testing.T) {
	assert := assert.New(t)
	s := newTestServerNode()
	testInput := "foo"
	var chunkHolder string
	c := newTestClientNode()
	c.SetOChunkHandler(func(chunk any) {
		chunkHolder = chunk.(string)
	})
	s.AddClient(c)
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	s.input(testInput)
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	time.Sleep(time.Millisecond)
	s.Stop()
	assert.Equal(false, s.IsRunning())
	assert.Equal(testInput, chunkHolder)
}

func Test3LevelChunkPassFromServerToClient(t *testing.T) {
	assert := assert.New(t)
	s := newTestServerNode()
	s2 := newTestServerNode()
	testInput := "foo"
	var chunkHolder string
	c := newTestClientNode()
	c.SetOChunkHandler(func(chunk any) {
		chunkHolder = chunk.(string)
	})
	s.AddClient(s2)
	s2.AddClient(c)
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	s.input(testInput)
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	time.Sleep(time.Millisecond)
	s.Stop()
	assert.Equal(false, s.IsRunning())
	assert.Equal(testInput, chunkHolder)
}

var testCounterValue int = 100

func newTestServerFeederNode() *TestServerNode {
	n := &TestServerNode{}
	// This task will produce chunks of work and feed them to the client
	n.task = func(ch chan bool) {
		counter := testCounterValue
		for {
			select {
			case <-ch:
				return
			default:
				n.Output(counter)                // push the produced data to the output queue
				time.Sleep(2 * time.Millisecond) // imitate some I/O business
				counter++
			}
		}
	}
	return n
}

func TestServer2ClientQueue(t *testing.T) {
	assert := assert.New(t)
	s := newTestServerFeederNode()
	s2 := newTestServerNode()
	var chunkHolder int
	handled := false
	c := newTestClientNode()
	c.SetOChunkHandler(func(chunk any) {
		if handled == false {
			handled = true
			chunkHolder = chunk.(int)
		}
	})
	s2.AddClient(c)
	s.AddClient(s2)
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	assert.Equal(1, len(s.Clients))
	assert.Equal(true, s.IsRunning())
	time.Sleep(time.Millisecond) // allow for the server's task to perform some work before shutting it down
	s.Stop()
	assert.Equal(false, s.IsRunning())
	assert.Equal(testCounterValue, chunkHolder)
}
