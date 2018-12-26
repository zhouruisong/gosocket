package socketio

import (
	"sync"
	//"time"
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
	//userMap      sync.Map
)

const groupNumber = 500

type broadcast struct {
	m    map[string]map[string]Socket
	user map[string]int
	sync.RWMutex
}

func newBroadcastDefault() BroadcastAdaptor {
	b := &broadcast{
		m:    make(map[string]map[string]Socket),
		user: make(map[string]int),
	}
	return b
}

func (b *broadcast) InjectLogger(l *f1.FileLogger) {
	bLogger = l
}

func (b *broadcast) DisAllConnection(room string) {
	bLogger.I(3, "DisAllConnection room=%v", room)
	r := ":" + room
	sockets, _ := b.m[r]
	bLogger.I(3, "Leave room=%v", room)
	for id, _ := range sockets {
		//bLogger.I(3, "=====leave id:%v,room=%v", id, room)
		sockets[id].Leave(room)
		//sockets[id].Disconnect()
	}

	bLogger.I(3, "DisAllConnection room=%v", room)
	//此处代码不能打开，会出现挂机情况，还未修改
	//for id, _ := range sockets {
	//	//bLogger.I(3, "=====leave id:%v,room=%v", id, room)
	//	sockets[id].Disconnect()
	//}
	//bLogger.I(3, "DisAllConnection room=%v", room)

	b.Lock()
	defer b.Unlock()
	b.m[r] = nil
	b.user[r] = 0
	bLogger.I(3, "DisAllConnection ok room=%v", room)
}

func (b *broadcast) Join(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()
	//bLogger.I(3, "Join room=%v", room)

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
	b.user[room] = len(sockets)
	b.m[room] = sockets

	//bLogger.I(3, "===join len=%v", len(sockets))
	//bLogger.I(3, "===join num=%v,room=%v", b.user[room], room)
	return nil
}

func (b *broadcast) Leave(room string, socket Socket) error {
	b.Lock()
	defer b.Unlock()
	//bLogger.I(3, "Leave room=%v", room)

	soid := socket.Id()
	sockets, ok := b.m[room]
	if !ok {
		b.user[room] = 0
		return nil
	}
	_, ok1 := sockets[soid]
	if !ok1 {
		num := b.user[room]
		if num == 0 {
			b.user[room] = 0
		} else {
			num--
			b.user[room] = num
		}
		//bLogger.I(3, "not in map,soid:%v,current n:%v,len:%v,room:%v", soid, b.n, len(sockets), room)
		return nil
	}
	delete(sockets, soid)
	b.user[room] = len(sockets)
	if len(sockets) == 0 {
		delete(b.m, room)
		b.user[room] = 0
		return nil
	}
	b.m[room] = sockets
	return nil
}

func (b *broadcast) Send(ignore Socket, room, event string, args ...interface{}) error {
	b.RLock()
	defer b.RUnlock()
	sockets := b.m[room]

	//bLogger.I(3, "room=%v", room)

	// 开始时间
	//start := time.Now()
	var sliceSo []Socket
	for id, _ := range sockets {
		if ignore != nil && ignore.Id() == id {
			continue
		}
		sliceSo = append(sliceSo, sockets[id])
	}

	num := b.user[room]
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
	//end := time.Now()
	//latency := end.Sub(start) // us
	////消耗时间
	//bLogger.I(3, "cost: %s", latency)
	return nil
}

func (b *broadcast) Len(room string) int {
	b.RLock()
	defer b.RUnlock()
	//bLogger.I(3, "room=%v", room)
	n := b.user[":"+room]
	//bLogger.I(3, "room len:%v", n)
	return n
}
