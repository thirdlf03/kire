package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/thirdlf03/kire/internal/model"
)

// Cache provides get/put for embeddings keyed by content hash.
type Cache interface {
	Get(key string) (model.Embedding, bool, error)
	Put(key string, emb model.Embedding) error
	Close() error
}

// JSONCache is a file-backed JSON cache.
type JSONCache struct {
	path    string
	mu      sync.RWMutex
	entries map[string]model.Embedding
	dirty   bool
}

func NewJSONCache(path string) (*JSONCache, error) {
	c := &JSONCache{
		path:    path,
		entries: make(map[string]model.Embedding),
	}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read cache: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &c.entries); err != nil {
				return nil, fmt.Errorf("parse cache: %w", err)
			}
		}
	}
	return c, nil
}

func (c *JSONCache) Get(key string) (model.Embedding, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	emb, ok := c.entries[key]
	return emb, ok, nil
}

func (c *JSONCache) Put(key string, emb model.Embedding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = emb
	c.dirty = true
	return nil
}

func (c *JSONCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	data, err := json.Marshal(c.entries)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	// Atomic write: write to temp file then rename
	tmpPath := c.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write cache temp: %w", err)
	}
	if err := os.Rename(tmpPath, c.path); err != nil {
		_ = os.Remove(tmpPath) // clean up on failure
		return fmt.Errorf("rename cache: %w", err)
	}
	c.dirty = false
	return nil
}

// CacheKey generates a deterministic key from text and embedding parameters.
func CacheKey(text, modelName, taskType string, dims int) string {
	h := sha256.New()
	h.Write([]byte(text))
	h.Write([]byte{0}) // null delimiter to prevent field collision
	h.Write([]byte(modelName))
	h.Write([]byte{0})
	h.Write([]byte(taskType))
	h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d", dims)
	return hex.EncodeToString(h.Sum(nil))
}
