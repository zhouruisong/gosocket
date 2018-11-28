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

	//delete all connect
	DisAllConnection(room string)
}

var (
	bLogger      *f1.FileLogger
	newBroadcast = newBroadcastDefault
	userMap      sync.Map
)

const groupNumber = 500

type broadcast struct {
	m map[string]map[string]Socket
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

func (b *broadcast) DisAllConnection(room string) {
	b.Lock()
	defer b.Unlock()

	sockets, _ := b.m[room]
	for id, _ := range sockets {
		sockets[id].Leave(room)
		sockets[id].Disconnect()
	}
	b.m[room] = nil
	userMap.Store(room, 0)
	//bLogger.I(3, "room:%v,current len:%v", room, b.n)
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
		bLogger.I(3, "exist in map join soid=%v", soid)
		return nil
	}
	sockets[soid] = socket
	userMap.Store(room, len(sockets))
	b.m[room] = sockets

	n, ok := userMap.Load(room)
	if !ok {
		bLogger.W(3, "can not load key=%v", room)
		return nil
	}
	if n == nil {
		bLogger.I(3, "===n is null")
		return nil
	}

	bLogger.I(3, "===join len=%v", len(sockets))
	bLogger.I(3, "===join num=%v,room=%v", n.(int), room)
	return nil
}

func (b *broadcast) Leave(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()

	soid := socket.Id()
	sockets, ok := b.m[room]
	if !ok {
		userMap.Store(room, 0)
		return nil
	}
	_, ok1 := sockets[soid]
	if !ok1 {
		n, ok := userMap.Load(room)
		if !ok {
			bLogger.W(3, "can not load key=%v", room)
			return nil
		}
		if n == nil {
			bLogger.W(3, "n is null key=%v", room)
			userMap.Store(room, 0)
			return nil
		}

		num := n.(int)
		if num == 0 {
			userMap.Store(room, 0)
		} else {
			num--
			userMap.Store(room, num)
		}
		//bLogger.I(3, "not in map,soid:%v,current n:%v,len:%v,room:%v", soid, b.n, len(sockets), room)
		return nil
	}
	delete(sockets, soid)
	userMap.Store(room, len(sockets))
	if len(sockets) == 0 {
		delete(b.m, room)
		userMap.Store(room, 0)
		return nil
	}
	b.m[room] = sockets
	return nil
}

func (b *broadcast) Send(ignore Socket, room, event string, args ...interface{}) error {
	b.RLock()
	defer b.RUnlock()
	sockets := b.m[room]

	// 开始时间
	start := time.Now()
	var sliceSo []Socket
	for id, _ := range sockets {
		if ignore != nil && ignore.Id() == id {
			continue
		}
		sliceSo = append(sliceSo, sockets[id])
	}

	n, ok := userMap.Load(":" + room)
	if !ok {
		bLogger.W(3, "Load key:%v failed", ":"+room)
		return nil
	}
	if n == nil {
		bLogger.W(3, "Load key:%v failed", ":"+room)
		return nil
	}

	num := n.(int)
	group := num / groupNumber
	left := num % groupNumber
	var st int
	var ed int
	var wt sync.WaitGroup
	if group > 0 {
		for i := 0; i < group; i++ {
			wt.Add(1)
			st = i * groupNumber
			ed = st + groupNumber
			go func(s, e int, w *sync.WaitGroup) {
				defer wt.Done()
				//bLogger.I(3, "index:%v,start:%v,end:%v", i, st, ed)
				for j := s; j < e; j++ {
					sliceSo[j].Emit(event, args...)
				}
			}(st, ed, &wt)
		}
	}

	stleft := group * groupNumber
	edleft := stleft + left
	//bLogger.I(3, "group:%v,left:%v,stleft:%v,edleft:%v",
	//	group, left, stleft, edleft)
	for i := stleft; i < edleft; i++ {
		sliceSo[i].Emit(event, args...)
	}
	wt.Wait()
	// 结束时间
	end := time.Now()
	latency := end.Sub(start) // us
	//消耗时间
	bLogger.I(3, "cost: %s", latency)
	return nil
}

func (b *broadcast) Len(room string) int {
	//bLogger.I(3, "room=%v", room)
	n, ok := userMap.Load(":" + room)
	if !ok {
		bLogger.W(3, "Load key:%v failed", ":"+room)
	}
	if n == nil {
		bLogger.W(3, "Load key:%v failed", ":"+room)
		return 0
	}
	bLogger.I(3, "room len:%v", n)
	return n.(int)
}
