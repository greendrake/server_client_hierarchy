package server_client_hierarchy

import (
	"context"
	"github.com/greendrake/eventbus"
	"sync"
	"sync/atomic"
)

const (
	STATE_STOPPED  uint32 = 0
	STATE_STARTING uint32 = 1
	STATE_RUNNING  uint32 = 2
	STATE_STOPPING uint32 = 3
)

type ChunkOfWorkHandler func(chunk any)

type NodeInterface interface {
	eventbus.EventBusInterface
	GetNode() *Node
	AddClient(client NodeInterface)
	RemoveClient(client NodeInterface)
	removeAllClients()
	SetPrincipallyClient(isPrincipallyClient bool)
	setServer(server NodeInterface)
	SetContext(Ctx context.Context)
	SetIChunkHandler(ChunkHandler)
	SetOChunkHandler(ChunkHandler)
	Start()
	Stop()
	Wait()
	input(chunk any)
	Output(chunk any)
	flushQueues()
	SetTask(t NodeTask)
}

type NodeTask func(ch chan bool)

type Node struct {
	eventbus.EventBus
	ID string
	// Context is needed here for some network diallers using it as a means of interrupting/cancelling connection timeouts (which will otherwise block)
	Ctx context.Context
	// Clients need to be added/removed in a thread-safe manner
	clientAddRemoveMutex sync.Mutex
	// Reference to the server Node embedding struct, if any
	server  NodeInterface
	Clients []NodeInterface

	task                   NodeTask
	taskStopCommandChannel chan bool
	stopChan               chan bool

	// Input queue processor
	iQueue        *QueueProcessor
	iChunkHandler ChunkHandler

	// Output queue processor
	oQueue        *QueueProcessor
	oChunkHandler ChunkHandler

	// False by default i.e. a Node is principlally Server by default,
	// which means it starts/stops depending on whether it has clients.
	// Setting this TRUE makes it start/stop depending on whether it is attached to a server.
	isPrincipallyClient bool
	state               atomic.Uint32

	removeAllClientsInProgress bool
}

func (n *Node) SetTask(task NodeTask) {
	n.task = task
}

func (n *Node) SetIChunkHandler(chunkHandler ChunkHandler) {
	n.iChunkHandler = chunkHandler
}

func (n *Node) SetOChunkHandler(chunkHandler ChunkHandler) {
	n.oChunkHandler = chunkHandler
}

func (n *Node) input(chunk any) {
	if n.IsRunning() {
		if n.iQueue == nil {
			// By default, the input queue simply passes the chunks to the ouput
			if n.iChunkHandler == nil {
				n.SetIChunkHandler(n.Output)
			}
			n.iQueue = &QueueProcessor{
				chunkHandler: n.iChunkHandler,
			}
		}
		n.iQueue.Put(chunk)
	}
}

func (n *Node) Output(chunk any) {
	if n.IsRunning() {
		// When the Node is a server, we know how to handle chunks by default.
		// When the Node is a client, the chunk handler must be already set as there is no default handler.
		if n.oQueue == nil && (!n.isPrincipallyClient || n.oChunkHandler != nil) {
			if n.oChunkHandler == nil {
				n.SetOChunkHandler(func(chunk any) {
					for _, cl := range n.Clients {
						cl.input(chunk)
					}
				})
			}
			n.oQueue = &QueueProcessor{
				chunkHandler: n.oChunkHandler,
			}
		}
		// Client nodes must have output handler set up. Otherwise this method will do nothing.
		if n.oQueue != nil {
			n.oQueue.Put(chunk)
		}
	}
}

// The enbedding struct's New method is supposed to call this as it sees fit
func (n *Node) SetPrincipallyClient(isPrincipallyClient bool) {
	n.isPrincipallyClient = isPrincipallyClient
}

func (n *Node) setServer(server NodeInterface) {
	if server == nil {
		n.removeAllClients()
	}
	n.server = server
	if n.server == nil {
		n.Stop()
	} else {
		n.Start()
	}
}

func (n *Node) SetContext(Ctx context.Context) {
	n.Ctx = Ctx
}

func (n *Node) Wait() {
	if !n.isStopped() {
		waitChan := make(chan bool)
		n.On("stop", func(args ...any) {
			waitChan <- true
		})
		<-waitChan
	}
}

func (n *Node) SetContextWaiter(Ctx context.Context) {
	n.SetContext(Ctx)
	n.SetTask(func(ch chan bool) {
		select {
		case <-Ctx.Done():
			go n.Stop()
			<-ch
		case <-ch:
		}
	})
}

func (n *Node) AddClient(client NodeInterface) {
	n.clientAddRemoveMutex.Lock()
	defer n.clientAddRemoveMutex.Unlock()
	n.Clients = append(n.Clients, client)
	if n.Ctx != nil {
		client.SetContext(n.Ctx)
	}
	client.setServer(n)
	if !n.isPrincipallyClient {
		// Client added, start the server Node if not started already
		n.Start()
	}
}

func (n *Node) RemoveClient(client NodeInterface) {
	if !n.removeAllClientsInProgress {
		n.clientAddRemoveMutex.Lock()
		defer n.clientAddRemoveMutex.Unlock()
		for index, v := range n.Clients {
			if v.GetNode() == client.GetNode() {
				n.Clients = append(n.Clients[:index], n.Clients[index+1:]...)
				client.setServer(nil)
				if len(n.Clients) == 0 && !n.isPrincipallyClient {
					// Last client removed, stop
					// log.Printf("Last client removed stop: %v", n.ID)
					n.Stop()
				}
				return
			}
		}
	}
}

func (n *Node) removeAllClients() {
	if len(n.Clients) > 0 {
		n.removeAllClientsInProgress = true
		n.clientAddRemoveMutex.Lock()
		defer n.clientAddRemoveMutex.Unlock()
		for _, client := range n.Clients {
			client.setServer(nil)
		}
		n.Clients = nil
		if !n.isPrincipallyClient {
			// Last client removed, stop
			n.Stop()
		}
		n.removeAllClientsInProgress = false
	}
}

func (n *Node) Start() {
	if n.isStopped() {
		n.debug("starting...")
		n.state.Store(STATE_STARTING)
		if n.task != nil {
			if n.taskStopCommandChannel == nil {
				n.taskStopCommandChannel = make(chan bool)
				n.stopChan = make(chan bool)
			}
			go func() {
				n.task(n.taskStopCommandChannel) // supposed to be blocking
				n.stopChan <- true
			}()
		}
		n.state.Store(STATE_RUNNING)
		n.Trigger("start")
		n.debug("started")
	}
}

func (n *Node) IsRunning() bool {
	return n.state.Load() == STATE_RUNNING
}

func (n *Node) isStopped() bool {
	return n.state.Load() == STATE_STOPPED
}

func (n *Node) IsStopping() bool {
	return n.state.Load() == STATE_STOPPING
}

func (n *Node) debug(msg string) {
	// fmt.Printf("[%v] %v\n", n.ID, msg)
}

func (n *Node) Stop() {
	if n.IsRunning() {
		n.debug("stopping...")
		n.state.Store(STATE_STOPPING)
		if n.task != nil {
			n.taskStopCommandChannel <- true // command the task to stop
			<-n.stopChan                     // and now wait for it
		}
		n.flushQueues()
		n.removeAllClients()
		if n.server != nil {
			n.server.RemoveClient(n)
		}
		n.state.Store(STATE_STOPPED)
		n.Trigger("stop")
		n.debug("stopped")
	}
}

func (n *Node) flushQueues() {
	if n.iQueue != nil {
		n.iQueue.Wait()
		n.iQueue = nil
	}
	if n.oQueue != nil {
		n.oQueue.Wait()
		n.oQueue = nil
	}
	for _, cl := range n.Clients {
		cl.flushQueues()
	}
}

// Used by embedding structs to get the inner Node
// See https://stackoverflow.com/a/78615820/2470051
func (n *Node) GetNode() *Node {
	return n
}
