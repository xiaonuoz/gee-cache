package lru

import "container/list"

// 最近最少使用算法，
// 相对于仅考虑时间因素的 FIFO(先进先出) 和仅考虑访问频率的 LFU (最少使用)，LRU 算法可以认为是相对平衡的一种淘汰算法。
// LRU 认为，如果数据最近被访问过，那么将来被访问的概率也会更高。
// LRU 算法的实现非常简单，维护一个队列，如果某条记录被访问了，则移动到队尾，那么队首则是最近最少访问的数据，淘汰该条记录即可。

// 缓存是LRU缓存。它对并发访问不安全。
// LRU中有一个存储key和链表指针的map和一个双向链表，链表存储具体数据
// 链表的作用就是来删除最近最少使用的key,所以链表存储的数据类型中要有map的key来方便删除数据。
type Cache struct {
	// 允许使用的最大内存
	maxBytes int64
	// 当前已使用的内存
	nbytes int64
	// Go 语言标准库实现的双向链表 list.List
	ll *list.List
	// 键是字符串，值是双向链表中对应节点的指针
	cache map[string]*list.Element
	// cache中记录被移除时的回调函数，可以为 nil
	OnEvicted func(key string, value Value)
}

// 双向链表节点的数据类型，在链表中仍保存每个值对应的 key 的好处在于，淘汰队首节点时，需要用 key 从字典中删除对应的映射。
type entry struct {
	key   string
	value Value
}

// 为了通用性，设计值是实现了 Value 接口的任意类型，该接口只包含了一个方法 Len() int，用于返回值所占用的内存大小。
type Value interface {
	Len() int
}

// 构造函数
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// 查找键的值，如果键对应的链表节点存在，则将对应节点移动到队尾，并返回查找到的值。
func (c *Cache) Get(key string) (value Value, ok bool) {
	// 从字典中找到对应的双向链表的节点
	if ele, ok := c.cache[key]; ok {
		// 双向链表作为队列，队首队尾是相对的，在这里约定 front 为队尾
		// 将该节点移动到队尾
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// 新增或者修改
func (c *Cache) Add(key string, value Value) {
	// 如果键存在，则更新对应节点的值，并将该节点移到队尾。
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		// 因为强转成entry指针，所以直接修改对应的值即可
		kv := ele.Value.(*entry)
		kv.value = value
		// 新增内存是新存入的value内存减去原value内存
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
	} else {
		// 不存在则是新增场景，首先队尾添加新节点 &entry{key, value},
		ele := c.ll.PushFront(&entry{key, value})
		// 并在字典中添加 key 和节点的映射关系
		c.cache[key] = ele
		// 更新所用内存
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	// 如果超过了设定的最大值 c.maxBytes，则移除最少访问的节点。
	// 类似于for死循环中判断最大内存是否小于所占内存，一直删除，直到不符合条件再退出for循环
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// 删除，实际上是缓存淘汰。即移除最近最少访问的节点（队首）
func (c *Cache) RemoveOldest() {
	// 返回列表ll的最后一个元素，如果列表为空则返回nil
	ele := c.ll.Back()
	if ele != nil {
		// 从链表中删除
		c.ll.Remove(ele)
		// 获取map的key,从map中删除
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		// 更新当前所用的内存
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 如果回调函数 OnEvicted 不为 nil，则调用回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 返回 添加了多少条数据
func (c *Cache) Len() int {
	return c.ll.Len()
}
