package gotype

import (
	"reflect"
	"sync"
	"unsafe"

	structform "github.com/urso/go-structform"
)

type Unfolder struct {
	unfoldCtx
}

type unfoldCtx struct {
	opts options

	buf buffer

	unfolder unfolderStack
	value    reflectValueStack
	baseType structformTypeStack
	ptr      ptrStack
	key      keyStack
	idx      idxStack

	keyCache symbolCache
}

type ptrUnfolder interface {
	initState(*unfoldCtx, unsafe.Pointer)
}

type reflUnfolder interface {
	initState(*unfoldCtx, reflect.Value)
}

type unfolder interface {
	// primitives
	OnNil(*unfoldCtx) error
	OnBool(*unfoldCtx, bool) error
	OnString(*unfoldCtx, string) error
	OnStringRef(*unfoldCtx, []byte) error
	OnInt8(*unfoldCtx, int8) error
	OnInt16(*unfoldCtx, int16) error
	OnInt32(*unfoldCtx, int32) error
	OnInt64(*unfoldCtx, int64) error
	OnInt(*unfoldCtx, int) error
	OnByte(*unfoldCtx, byte) error
	OnUint8(*unfoldCtx, uint8) error
	OnUint16(*unfoldCtx, uint16) error
	OnUint32(*unfoldCtx, uint32) error
	OnUint64(*unfoldCtx, uint64) error
	OnUint(*unfoldCtx, uint) error
	OnFloat32(*unfoldCtx, float32) error
	OnFloat64(*unfoldCtx, float64) error

	// array types
	OnArrayStart(*unfoldCtx, int, structform.BaseType) error
	OnArrayFinished(*unfoldCtx) error
	OnChildArrayDone(*unfoldCtx) error

	// object types
	OnObjectStart(*unfoldCtx, int, structform.BaseType) error
	OnObjectFinished(*unfoldCtx) error
	OnKey(*unfoldCtx, string) error
	OnKeyRef(*unfoldCtx, []byte) error
	OnChildObjectDone(*unfoldCtx) error
}

type typeUnfoldRegistry struct {
	mu sync.RWMutex
	m  map[reflect.Type]reflUnfolder
}

var unfoldRegistry = newTypeUnfoldRegistry()

func NewUnfolder(to interface{}) (*Unfolder, error) {
	u := &Unfolder{}
	u.opts = options{tag: "struct"}

	u.unfolder.init(&unfolderNoTarget{})
	u.value.init(reflect.Value{})
	u.ptr.init()
	u.key.init()
	u.idx.init()
	u.baseType.init()

	// TODO: make allocation buffer size configurable
	u.buf.init(1024)

	if to != nil {
		err := u.SetTarget(to)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}

func (u *Unfolder) EnableKeyCache(max int) {
	u.keyCache.init(max)
}

func (u *Unfolder) SetTarget(to interface{}) error {
	ctx := &u.unfoldCtx

	if ptr, u := lookupGoTypeUnfolder(to); u != nil {
		u.initState(ctx, ptr)
		return nil
	}

	t := reflect.TypeOf(to)
	if t.Kind() != reflect.Ptr {
		return errRequiresPointer
	}

	ru, err := lookupReflUnfolder(&u.unfoldCtx, t)
	if err != nil {
		return err
	}
	if ru != nil {
		ru.initState(ctx, reflect.ValueOf(to))
		return nil
	}

	return errUnsupported
}

func (u *unfoldCtx) OnObjectStart(len int, baseType structform.BaseType) error {
	return u.unfolder.current.OnObjectStart(u, len, baseType)
}

func (u *unfoldCtx) OnObjectFinished() error {
	lBefore := len(u.unfolder.stack) + 1

	if err := u.unfolder.current.OnObjectFinished(u); err != nil {
		return err
	}

	lAfter := len(u.unfolder.stack) + 1
	if old := u.unfolder.current; lAfter > 1 && lBefore != lAfter {
		return old.OnChildObjectDone(u)
	}

	return nil
}

func (u *unfoldCtx) OnKey(s string) error {
	return u.unfolder.current.OnKey(u, s)
}

func (u *unfoldCtx) OnKeyRef(s []byte) error {
	return u.unfolder.current.OnKeyRef(u, s)
}

func (u *unfoldCtx) OnArrayStart(len int, baseType structform.BaseType) error {
	return u.unfolder.current.OnArrayStart(u, len, baseType)
}

func (u *unfoldCtx) OnArrayFinished() error {
	lBefore := len(u.unfolder.stack) + 1

	if err := u.unfolder.current.OnArrayFinished(u); err != nil {
		return err
	}

	lAfter := len(u.unfolder.stack) + 1
	if old := u.unfolder.current; lAfter > 1 && lBefore != lAfter {
		return old.OnChildArrayDone(u)
	}

	return nil
}

func (u *unfoldCtx) OnNil() error {
	return u.unfolder.current.OnNil(u)
}

func (u *unfoldCtx) OnBool(b bool) error {
	return u.unfolder.current.OnBool(u, b)
}

func (u *unfoldCtx) OnString(s string) error {
	return u.unfolder.current.OnString(u, s)
}

func (u *unfoldCtx) OnStringRef(s []byte) error {
	return u.unfolder.current.OnStringRef(u, s)
}

func (u *unfoldCtx) OnInt8(i int8) error {
	return u.unfolder.current.OnInt8(u, i)
}

func (u *unfoldCtx) OnInt16(i int16) error {
	return u.unfolder.current.OnInt16(u, i)
}

func (u *unfoldCtx) OnInt32(i int32) error {
	return u.unfolder.current.OnInt32(u, i)
}

func (u *unfoldCtx) OnInt64(i int64) error {
	return u.unfolder.current.OnInt64(u, i)
}

func (u *unfoldCtx) OnInt(i int) error {
	return u.unfolder.current.OnInt(u, i)
}

func (u *unfoldCtx) OnByte(b byte) error {
	return u.unfolder.current.OnByte(u, b)
}

func (u *unfoldCtx) OnUint8(v uint8) error {
	return u.unfolder.current.OnUint8(u, v)
}

func (u *unfoldCtx) OnUint16(v uint16) error {
	return u.unfolder.current.OnUint16(u, v)
}

func (u *unfoldCtx) OnUint32(v uint32) error {
	return u.unfolder.current.OnUint32(u, v)
}

func (u *unfoldCtx) OnUint64(v uint64) error {
	return u.unfolder.current.OnUint64(u, v)
}

func (u *unfoldCtx) OnUint(v uint) error {
	return u.unfolder.current.OnUint(u, v)
}

func (u *unfoldCtx) OnFloat32(f float32) error {
	return u.unfolder.current.OnFloat32(u, f)
}

func (u *unfoldCtx) OnFloat64(f float64) error {
	return u.unfolder.current.OnFloat64(u, f)
}

func newTypeUnfoldRegistry() *typeUnfoldRegistry {
	return &typeUnfoldRegistry{m: map[reflect.Type]reflUnfolder{}}
}

func (r *typeUnfoldRegistry) find(t reflect.Type) reflUnfolder {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.m[t]
}

func (r *typeUnfoldRegistry) set(t reflect.Type, f reflUnfolder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[t] = f
}
