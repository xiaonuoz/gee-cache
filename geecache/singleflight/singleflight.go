package singleflight

import "sync"

// call 代表正在进行中，或已经结束的请求
// 使用 sync.WaitGroup 锁避免重入
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// 管理不同 key 的请求(call)
type Group struct {
	mu sync.Mutex // protects m
	m  map[string]*call
}

// 针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误。
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 加锁，同一时刻只有一个请求进行
	g.mu.Lock()

	// 延迟初始化，提高内存使用效率
	if g.m == nil {
		g.m = make(map[string]*call)
	}

	// 如果key对应的请求存在,则直接解锁并且等待第一次请求结束
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	// 不存在则表示它是第一次请求,赋值然后解锁
	c := new(call)
	c.wg.Add(1)
	// 添加到 g.m，表明 key 已经有对应的请求在处理
	g.m[key] = c
	g.mu.Unlock()

	// 根据互斥锁的逻辑，除了第一次请求会走到此处，其他相同key的请求都会在上方Wait()处等待
	c.val, c.err = fn()
	// 调用结束后,其他请求直接进行取值
	c.wg.Done()

	// 当前key不能一直存在,Do只是针对相同key的瞬时多次请求调用一次fn,不删除它在任意时刻都返回第一次调用的数据
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
