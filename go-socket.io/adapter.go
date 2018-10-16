package socketio

import (
	"sync"
	"time"
	"fmt"
)

// BroadcastAdaptor is the adaptor to handle broadcasts.
type BroadcastAdaptor interface {
	// Join causes the socket to join a room.
	Join(room string, socket Socket) error

	// Leave causes the socket to leave a room.
	Leave(room string, socket Socket) error

	// Send will send an event with args to the room. If "ignore" is not nil, the event will be excluded from being sent to "ignore".
	Send(ignore Socket, room, event string, args ...interface{}) error

	//Len socket in room
	Len(room string) int
}

const consumeRoutine = 10

var newBroadcast = newBroadcastDefault
var sendCh = make(chan *sendParameter, 5000)

type sendParameter struct {
	s     Socket
	event string
	args  interface{}
}

type broadcast struct {
	m map[string]map[string]Socket
	n int
	sync.RWMutex
}

func (b *broadcast) writeCh(s *sendParameter) int {
	select {
	case sendCh <- s:
		fmt.Printf("writeCh data:%+v",*s)
		return 0
	case <-time.After(time.Second * 2): //写入超时
		return -1
	}
	return 0
}

func (b *broadcast) readCh() {
	for {
		d, isClose := <-sendCh
		if !isClose {
			break
		}
		fmt.Printf("readCh data:%+v",d)
		d.s.Emit(d.event, d.args)
	}
	return
}

func newBroadcastDefault() BroadcastAdaptor {
	b := &broadcast{
		m: make(map[string]map[string]Socket),
	}
	for i := 0; i < consumeRoutine; i++ {
		go b.readCh()
	}
	return b
}

func (b *broadcast) Join(room string, socket Socket) error {
	b.Lock()
	sockets, ok := b.m[room]
	if !ok {
		sockets = make(map[string]Socket)
	}
	sockets[socket.Id()] = socket
	b.m[room] = sockets
	b.n = len(b.m[room])
	b.Unlock()
	return nil
}

func (b *broadcast) Leave(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()
	sockets, ok := b.m[room]
	if !ok {
		return nil
	}
	delete(sockets, socket.Id())
	if len(sockets) == 0 {
		delete(b.m, room)
		b.n = len(b.m[room])
		return nil
	}
	b.m[room] = sockets
	b.n = len(b.m[room])
	return nil
}

func (b *broadcast) Send(ignore Socket, room, event string, args ...interface{}) error {
	b.RLock()
	sockets := b.m[room]
	for id, s := range sockets {
		if ignore != nil && ignore.Id() == id {
			continue
		}
		m := &sendParameter{
			s:     s,
			event: event,
			args:  args,
		}
		//写入管道
		b.writeCh(m)
		//s.Emit(event, args...)
	}
	b.RUnlock()
	return nil
}

func (b *broadcast) Len(room string) int {
	b.RLock()
	defer b.RUnlock()
	return b.n
}
