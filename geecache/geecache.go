package geecache

import (
	"fmt"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

// 函数类型实现某一个接口，称之为接口型函数，
// 方便使用者在调用时既能够传入具体的函数作为参数，也能够传入实现了该接口的结构体作为参数。
// 既可以直接走GetterFunc这一条路，它的Get函数就是调用自身。
// 也可以另定义一个结构体，其中实现了该接口，因此传参是合法的，它会走结构体对应实现的Get方法，
// 但是传结构体就意味着需要先实例化一个实现了该接口的对象，与这个比起来，直接传一个函数会更便捷。
// func GetFromSource(getter Getter, key string) []byte
// 这种类型其对应的接口必须有且只有一个函数
type GetterFunc func(key string) ([]byte, error)

// 定义一个函数类型 F，并且实现接口 A 的方法，然后在这个方法中调用自己。
// 这是 Go 语言中将其他函数转换为接口 A 的常用技巧。
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// 一个 Group 可以认为是一个缓存的命名空间，每个 Group 拥有一个唯一的名称 name。
type Group struct {
	name string
	// 缓存未命中时获取源数据的回调(callback)，由用户自己添加回调，从某文件或某库中获取数据添加至缓存中
	getter    Getter
	mainCache cache
	peers     PeerPicker
}

var (
	// 这里使用了只读锁RWMutex，因为不涉及任何冲突变量的写操作。
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup create a new instance of Group
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g
}

// GetGroup returns the named group previously created with NewGroup, or
// nil if there's no such group.
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get value for a key from cache
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	// 缓存不存在，则调用 load 方法，load 调用 getLocally（分布式场景下会调用 getFromPeer 从其他节点获取），
	// getLocally 调用用户回调函数 g.getter.Get() 获取源数据，并且将源数据添加到缓存 mainCache 中（通过 populateCache 方法）
	return g.load(key)
}

func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) load(key string) (value ByteView, err error) {
	// 先从远端获取数据
	if g.peers != nil {
		// 获取http请求，其中存有远端url地址
		if peer, ok := g.peers.PickPeer(key); ok {
			// 获取远端值，获取不到从本地获取
			if value, err = g.getFromPeer(peer, key); err == nil {
				return value, nil
			}
			log.Println("[GeeCache] Failed to get from peer", err)
		}
	}

	return g.getLocally(key)
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

// 从远端获取key
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}
