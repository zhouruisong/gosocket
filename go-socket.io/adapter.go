package socketio

import (
	"sync"
	"time"
	f1 "github.com/zhouruisong/fileLogger"
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

	//inject logger
	InjectLogger(l *f1.FileLogger)
}

var bLogger *f1.FileLogger
var newBroadcast = newBroadcastDefault

type broadcast struct {
	m map[string]map[string]Socket
	n int
	sync.RWMutex
}

func newBroadcastDefault() BroadcastAdaptor {
	b := &broadcast{
		m: make(map[string]map[string]Socket),
	}
	return b
}

func (b *broadcast) InjectLogger(l *f1.FileLogger) {
	bLogger = l
}

func (b *broadcast) Join(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()

	soid := socket.Id()
	sockets, ok := b.m[room]
	if !ok {
		sockets = make(map[string]Socket)
	}
	_, ok1 := sockets[soid]
	if ok1 {
		//bLogger.I(3, "exist in map join soid:%v", soid)
		return nil
	}
	sockets[soid] = socket
	b.n = len(sockets)
	b.m[room] = sockets
	bLogger.I(3, "join soid:%v", soid)
	return nil
}

func (b *broadcast) Leave(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()

	soid := socket.Id()
	sockets, ok := b.m[room]
	if !ok {
		b.n = 0
		return nil
	}
	_, ok1 := sockets[soid]
	if !ok1 {
		if b.n == 0 {
			b.n = 0
		} else {
			b.n--
		}
		//bLogger.I(3, "not in map,soid:%v,current n:%v,len:%v,room:%v", soid, b.n, len(sockets), room)
		return nil
	}
	delete(sockets, soid)
	b.n = len(sockets)
	if len(sockets) == 0 {
		delete(b.m, room)
		b.n = 0
		return nil
	}
	b.m[room] = sockets
	bLogger.I(3, "leave soid:%v,current n:%v", soid, b.n)
	return nil
}

func (b *broadcast) Send(ignore Socket, room, event string, args ...interface{}) error {
	b.RLock()
	defer b.RUnlock()

	sockets := b.m[room]
	// 开始时间
	start := time.Now()
	for id, _ := range sockets {
		if ignore != nil && ignore.Id() == id {
			continue
		}

		//发送失败
		if err := sockets[id].Emit(event, args...); err != nil {
			bLogger.W(3, "Emit failed,soid:%v,err:%v,event:%+v,args:%v",
				id, err, event, args)
		}
	}
	// 结束时间
	end := time.Now()
	latency := end.Sub(start) // us
	//消耗时间大于300ms打印日志
	if latency.Nanoseconds()/1000000 > 400 {
		bLogger.I(3, "=====time cost bigger 300ms,cost : %s ====", latency)
	}

	return nil
}

func (b *broadcast) Len(room string) int {
	b.RLock()
	defer b.RUnlock()
	return b.n
}