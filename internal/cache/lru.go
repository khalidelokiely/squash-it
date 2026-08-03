package cache

import (
	"context"
	"sync"
)

type node struct {
	key   string
	value string
	next  *node
	prev  *node
}

type LRUCache struct {
	items map[string]*node
	head  *node
	tail  *node
	size  int
	mu    sync.Mutex
}

func NewLRUCache(size int) *LRUCache {
	sentinelHead := &node{key: "", value: "", next: nil, prev: nil}
	sentinelTail := &node{key: "", value: "", next: nil, prev: nil}

	sentinelHead.next = sentinelTail
	sentinelTail.prev = sentinelHead

	return &LRUCache{
		items: make(map[string]*node, size),
		head:  sentinelHead,
		tail:  sentinelTail,
		size:  size,
	}
}

// removeNode detaches a node from the list safely by changing its next->prev and prev->next
// pointers
func (l *LRUCache) removeNode(node *node) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// addNode attaches a node to the head of the list (right next to the sentinel Head)
func (l *LRUCache) addNode(node *node) {
	node.prev = l.head
	node.next = l.head.next
	l.head.next.prev = node
	l.head.next = node
}

// evict evicts the least recently used node from the list - called upon len(items) == size
// upon Set of a node / key that doesn't exist in the list
func (l *LRUCache) evict() {
	evicted := l.tail.prev
	// Additional guard to protect nil pointer issues with removing head
	if evicted != l.head {
		l.removeNode(evicted)
		delete(l.items, evicted.key)
	}
}

func (l *LRUCache) Get(ctx context.Context, key string) (string, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	node, ok := l.items[key]
	if ok {
		if l.head.next != node {
			l.removeNode(node)
			l.addNode(node)
		}
		return node.value, true, nil
	}

	return "", false, nil
}

func (l *LRUCache) Set(ctx context.Context, key string, value string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Node exists already. Notice we don't check for capacity before checking
	// if the node exists so we don't evict pre-emptively
	if node, ok := l.items[key]; ok {
		// Node exists, so update its value and reposition it and return.
		node.value = value
		l.removeNode(node)
		l.addNode(node)
		return nil
	}

	// Node doesn't exist. So we check if we're capped and evict if we are
	if len(l.items) >= l.size {
		l.evict()
	}

	node := &node{key: key, value: value}
	l.items[key] = node
	// position the node within the list
	l.addNode(node)
	return nil
}
