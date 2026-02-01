package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thirdlf03/kire/internal/cache"
	"github.com/thirdlf03/kire/internal/model"
)

func TestJSONCache_PutGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}

	emb := model.Embedding{BlockIndex: 0, Vector: []float64{1, 2, 3}}
	if err := c.Put("key1", emb); err != nil {
		t.Fatal(err)
	}

	got, ok, err := c.Get("key1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got.Vector) != 3 || got.Vector[0] != 1 {
		t.Errorf("unexpected vector: %v", got.Vector)
	}
}

func TestJSONCache_Miss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := c.Get("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected cache miss")
	}
}

func TestJSONCache_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.cache.json")

	// Write cache
	c1, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	emb := model.Embedding{BlockIndex: 5, Vector: []float64{0.5, 0.6}}
	if err := c1.Put("persist-key", emb); err != nil {
		t.Fatal(err)
	}
	if err := c1.Close(); err != nil {
		t.Fatal(err)
	}

	// Read cache in new instance
	c2, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	got, ok, err := c2.Get("persist-key")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cache hit after persistence")
	}
	if got.BlockIndex != 5 {
		t.Errorf("expected BlockIndex 5, got %d", got.BlockIndex)
	}
}

func TestJSONCache_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	emb := model.Embedding{BlockIndex: 0, Vector: []float64{1, 2}}
	if err := c.Put("key", emb); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error(".tmp file should not remain after successful Close")
	}

	// Verify cache file exists and is valid
	c2, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()
	_, ok, _ := c2.Get("key")
	if !ok {
		t.Error("expected cache hit after atomic write")
	}
}

func TestCacheKey_CollisionResistance(t *testing.T) {
	// "ab"+"c" vs "a"+"bc" should produce different keys
	k1 := cache.CacheKey("ab", "c", "task", 0)
	k2 := cache.CacheKey("a", "bc", "task", 0)
	if k1 == k2 {
		t.Errorf("expected different keys for different field boundaries, got same: %s", k1)
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := cache.CacheKey("hello", "model-1", "SEMANTIC_SIMILARITY", 768)
	k2 := cache.CacheKey("hello", "model-1", "SEMANTIC_SIMILARITY", 768)
	if k1 != k2 {
		t.Errorf("keys should be identical")
	}

	k3 := cache.CacheKey("world", "model-1", "SEMANTIC_SIMILARITY", 768)
	if k1 == k3 {
		t.Errorf("different text should produce different keys")
	}
}
