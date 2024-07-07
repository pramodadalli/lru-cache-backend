package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type entry struct {
	key        string
	value      interface{}
	expiration time.Time
	next       *entry
	prev       *entry
}

type LRUCache struct {
	capacity int
	size     int
	cache    map[string]*entry
	head     *entry
	tail     *entry
	mutex    sync.Mutex
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*entry),
	}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if ent, ok := c.cache[key]; ok {
		if ent.expiration.After(time.Now()) {
			log.Printf("Cache HIT: Key %s", key)
			c.moveToFront(ent)
			return ent.value, true
		} else {
			log.Printf("Cache EXPIRED: Key %s", key)
			c.removeEntry(ent)
		}
	} else {
		log.Printf("Cache MISS: Key %s", key)
	}
	return nil, false
}

func (c *LRUCache) Set(key string, value interface{}, expiration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expirationTime := time.Now().Add(expiration)
	if ent, ok := c.cache[key]; ok {
		// Update existing entry
		log.Printf("Cache UPDATE: Key %s", key)
		ent.value = value
		ent.expiration = expirationTime
		c.moveToFront(ent)
	} else {
		// Add new entry
		log.Printf("Cache INSERT: Key %s", key)
		newEntry := &entry{
			key:        key,
			value:      value,
			expiration: expirationTime,
		}
		c.cache[key] = newEntry
		c.addToFront(newEntry)
		c.size++

		// Evict if cache exceeds capacity
		if c.size > c.capacity {
			c.evictOldest()
		}
	}
}

func (c *LRUCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if ent, ok := c.cache[key]; ok {
		log.Printf("Cache DELETE: Key %s", key)
		c.removeEntry(ent)
	} else {
		log.Printf("Cache DELETE FAILED: Key %s not found", key)
	}
}

func (c *LRUCache) removeEntry(ent *entry) {
	delete(c.cache, ent.key)
	c.removeNode(ent)
	c.size--
}

func (c *LRUCache) removeNode(ent *entry) {
	if ent.prev != nil {
		ent.prev.next = ent.next
	} else {
		c.head = ent.next
	}
	if ent.next != nil {
		ent.next.prev = ent.prev
	} else {
		c.tail = ent.prev
	}
}

func (c *LRUCache) moveToFront(ent *entry) {
	c.removeNode(ent)
	c.addToFront(ent)
}

func (c *LRUCache) addToFront(ent *entry) {
	ent.next = c.head
	ent.prev = nil
	if c.head != nil {
		c.head.prev = ent
	}
	c.head = ent
	if c.tail == nil {
		c.tail = ent
	}
}

func (c *LRUCache) evictOldest() {
	if c.tail != nil {
		log.Printf("Cache EVICT: Key %s", c.tail.key)
		c.removeEntry(c.tail)
	}
}

func cacheGetHandler(cache *LRUCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		key := params["key"]

		log.Printf("GET request received for key: %s", key)

		if value, ok := cache.Get(key); ok {
			json.NewEncoder(w).Encode(value)
		} else {
			http.Error(w, "Key not found", http.StatusNotFound)
		}
	}
}

func cacheSetHandler(cache *LRUCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		key := params["key"]
		var value interface{}

		err := json.NewDecoder(r.Body).Decode(&value)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		log.Printf("SET request received for key: %s", key)

		// Example: Custom expiration of 10 seconds
		cache.Set(key, value, 10*time.Second)
		w.WriteHeader(http.StatusCreated)
	}
}

func cacheDeleteHandler(cache *LRUCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		key := params["key"]

		log.Printf("DELETE request received for key: %s", key)

		cache.Delete(key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func main() {
	cache := NewLRUCache(1000)

	r := mux.NewRouter()
	r.HandleFunc("/cache/{key}", cacheGetHandler(cache)).Methods("GET")
	r.HandleFunc("/cache/{key}", cacheSetHandler(cache)).Methods("PUT")
	r.HandleFunc("/cache/{key}", cacheDeleteHandler(cache)).Methods("DELETE")

	// CORS middleware configuration
	corsHandler := handlers.CORS(
		handlers.AllowedHeaders([]string{"Content-Type"}),
		handlers.AllowedOrigins([]string{"http://localhost:3000"}), // Replace with your frontend URL
		handlers.AllowCredentials(),
	)

	// Apply CORS middleware to all routes
	http.Handle("/", corsHandler(r))

	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
