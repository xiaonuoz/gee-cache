package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash maps bytes to uint32
type Hash func(data []byte) uint32

// Map constains all hashed keys
type Map struct {
	// 采取依赖注入的方式，允许用于替换成自定义的 Hash 函数，也方便测试时替换，默认为 crc32.ChecksumIEEE 算法
	hash Hash
	// 虚拟节点倍数,一个真实节点采用几个虚拟节点，避免key数据倾斜的问题 [环上大部分数据在少数几个节点上，负载不均衡，这就是数据倾斜]
	replicas int
	// 哈希环，通过sort排序来实现环的结构
	keys []int // Sorted
	// 虚拟节点与真实节点的映射表 hashMap，键是虚拟节点的哈希值，值是真实节点的名称
	hashMap map[int]string
}

// New creates a Map instance
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// 添加节点
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 对应创建 m.replicas 个虚拟节点
		for i := 0; i < m.replicas; i++ {
			// 获取这个节点的hash值并放到环上，增加映射关系
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	// 排序形成有序的环
	sort.Ints(m.keys)
}

// 获取对应真实节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	// 获取key的hash值，并顺时针寻找距离此key最近的虚拟节点
	hash := int(m.hash([]byte(key)))
	// Search采用二分查找法,其对应的slice需要是有序的，如果没查询到符合条件的数据，会返回len(s)
	// 搜索返回第一个真索引。如果没有这样的索引，Search返回n
	// the first true index. If there is no such index, Search returns n.
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	// 取余是当出现没有符合条件的值，即这个key比所有节点的hash都大，那么它会顺时针分配到keys[0]节点
	// 根据映射关系找到真实节点返回
	return m.hashMap[m.keys[idx%len(m.keys)]]
}

// 删除节点，至于删除之后的节点上的数据会在第二次获取时访问db分配给其他附近节点
func (m *Map) Remove(key string) {
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
		idx := sort.SearchInts(m.keys, hash)
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		delete(m.hashMap, hash)
	}
}
