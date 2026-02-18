// Package cache provides benchmarks for Ristretto L1 cache.
package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// BenchmarkL1CacheGet benchmarks L1 cache Get performance
func BenchmarkL1CacheGet(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cache, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()
	// Pre-fill cache
	for i := 0; i < 1000; i++ {
		key := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
		cache.Set(ctx, key, []byte("test-data"))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
			cache.Get(ctx, key)
			i++
		}
	})
}

// BenchmarkL1CacheSet benchmarks L1 cache Set performance
func BenchmarkL1CacheSet(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cache, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
			cache.Set(ctx, key, []byte("test-data"))
			i++
		}
	})
}

// BenchmarkL1CacheGetOrCompute benchmarks GetOrCompute performance
func BenchmarkL1CacheGetOrCompute(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cache, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()
	var computeMu sync.Mutex
	computeCount := 0

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
			cache.GetOrCompute(ctx, key, func() ([]byte, error) {
				computeMu.Lock()
				computeCount++
				computeMu.Unlock()
				return []byte("computed-data"), nil
			})
			i++
		}
	})
}

// BenchmarkSemanticCache benchmarks semantic cache operations
func BenchmarkSemanticCache(b *testing.B) {
	logger := zaptest.NewLogger(b)
	l1, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer l1.Close()

	sc := NewSemanticCache(l1)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			namespace := "ns1"
			query := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
			sc.SetSimilarity(ctx, namespace, query, 0.85)
			sc.GetSimilarity(ctx, namespace, query)
			i++
		}
	})
}

// BenchmarkContextCache benchmarks context cache operations
func BenchmarkContextCache(b *testing.B) {
	logger := zaptest.NewLogger(b)
	l1, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer l1.Close()

	cc := NewContextCache(l1)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			convID := string(rune(i%10 + '0'))
			context := "test-context-data-" + string(rune(i%26+'a'))
			cc.SetContext(ctx, convID, context)
			cc.GetContext(ctx, convID)
			i++
		}
	})
}

// BenchmarkConcurrentAccess benchmarks concurrent cache access
func BenchmarkConcurrentAccess(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cache, err := NewL1Cache(10000, 5*time.Minute, nil, logger)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i%26 + 'a')) + string(rune((i/26)%26 + 'a'))
			switch i % 3 {
			case 0:
				cache.Get(ctx, key)
			case 1:
				cache.Set(ctx, key, []byte("data"))
			case 2:
				cache.Delete(ctx, key)
			}
			i++
		}
	})
}
