package common

import "sync"

type VoidFunc func(interface{})
type BoolFunc func(interface{}) bool

type funcHolder struct {
	id int
	fn interface{}
}
type event struct {
	nextID int
	funcs  map[int]*funcHolder
}

func (th *event) add(fn interface{}) int {
	th.nextID++
	if th.funcs == nil {
		th.funcs = map[int]*funcHolder{}
	}
	th.funcs[th.nextID] = &funcHolder{
		id: th.nextID,
		fn: fn,
	}
	return th.nextID
}
func (th *event) Remove(id int) {
	delete(th.funcs, id)
}
func (th *event) Emit(arg interface{}) {
	for id, h := range th.funcs {
		switch h.fn.(type) {
		case VoidFunc:
			h.fn.(VoidFunc)(arg)
		case BoolFunc:
			if !h.fn.(BoolFunc)(arg) {
				th.Remove(id)
			}
		}
	}
}

type SafeEvent struct {
	l sync.RWMutex
	event
}

func (th *SafeEvent) Add(h VoidFunc) int {
	th.l.Lock()
	defer th.l.Unlock()
	return th.add(h)
}
func (th *SafeEvent) AddBool(h BoolFunc) int {
	th.l.Lock()
	defer th.l.Unlock()
	return th.add(h)
}
func (th *SafeEvent) EmitSafe(arg interface{}) {
	th.l.Lock()
	defer th.l.Unlock()
	th.Emit(arg)
}

type UnSafeEvent struct {
	event
}

func (th *UnSafeEvent) Add(h VoidFunc) int {
	return th.add(h)
}
func (th *UnSafeEvent) AddBool(h BoolFunc) int {
	return th.add(h)
}
