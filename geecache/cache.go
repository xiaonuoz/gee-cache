package geecache

import (
	"sync"

	"gee-cache/geecache/lru"
)

// 实例化lru,增加互斥锁，封装add和get方法
type cache struct {
	// 这个锁定义在结构体里面，每一个结构体都有一个它自己的特有的锁
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 判断 c.lru 是否为 nil，如果等于 nil 再创建实例。
	// 这种方法称之为延迟初始化，一个对象的延迟初始化意味着该对象的创建将会延迟至第一次使用该对象时。
	// 主要用于提高性能，并减少程序内存要求。
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return
}
