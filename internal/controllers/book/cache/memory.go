package cache

import (
	"container/list"
	"sync"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

type MemoryCache interface {
	GetBook(key string) (*models.Book, bool)
	SetBook(key string, value *models.Book)
	GetBookList(key string) ([]models.Book, bool)
	SetBookList(key string, value []models.Book)
	Reset()
}

type entry struct {
	key   string
	book  *models.Book
	books []models.Book
}

type LRUCache struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*list.Element
	order   *list.List
}

func NewLRUCache(maxSize int) *LRUCache {
	if maxSize <= 0 {
		maxSize = 1000
	}

	return &LRUCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
	}
}

func (c *LRUCache) GetBook(key string) (*models.Book, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)

	cached := elem.Value.(entry)
	if cached.book == nil {
		return nil, false
	}

	bookCopy := *cached.book
	return &bookCopy, true
}

func (c *LRUCache) SetBook(key string, value *models.Book) {
	if value == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	bookCopy := *value
	c.set(key, entry{key: key, book: &bookCopy})
}

func (c *LRUCache) GetBookList(key string) ([]models.Book, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)

	cached := elem.Value.(entry)
	if cached.books == nil {
		return nil, false
	}

	copySlice := make([]models.Book, len(cached.books))
	copy(copySlice, cached.books)
	return copySlice, true
}

func (c *LRUCache) SetBookList(key string, value []models.Book) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copySlice := make([]models.Book, len(value))
	copy(copySlice, value)
	c.set(key, entry{key: key, books: copySlice})
}

func (c *LRUCache) set(key string, value entry) {
	if elem, ok := c.items[key]; ok {
		elem.Value = value
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(value)
	c.items[key] = elem

	if c.order.Len() <= c.maxSize {
		return
	}

	oldest := c.order.Back()
	if oldest == nil {
		return
	}

	c.order.Remove(oldest)
	delete(c.items, oldest.Value.(entry).key)
}

func (c *LRUCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element, c.maxSize)
	c.order.Init()
}
