package cache

import (
	"os"
	"testing"
	"time"
)

func TestBuildKey_Deterministic(t *testing.T) {
	k1 := BuildKey("https://example.com", "json", "direct", map[string]string{"a": "1", "b": "2"})
	k2 := BuildKey("https://example.com", "json", "direct", map[string]string{"b": "2", "a": "1"})
	if k1 != k2 {
		t.Fatalf("BuildKey not deterministic: %q vs %q", k1, k2)
	}
}

func TestBuildKey_DifferentInputs(t *testing.T) {
	k1 := BuildKey("https://a.com", "json", "direct", nil)
	k2 := BuildKey("https://b.com", "json", "direct", nil)
	if k1 == k2 {
		t.Fatal("expected different keys for different URLs")
	}
}

func TestDiskCache_SetGet(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := BuildKey("url", "json", "direct", nil)

	var got string
	found, err := c.Get(key, &got)
	if err != nil || found {
		t.Fatalf("expected miss, got found=%v err=%v", found, err)
	}

	if err := c.Set(key, time.Minute, "hello"); err != nil {
		t.Fatal(err)
	}

	found, err = c.Get(key, &got)
	if err != nil || !found || got != "hello" {
		t.Fatalf("expected hit with 'hello', got found=%v val=%q err=%v", found, got, err)
	}
}

func TestDiskCache_Expiry(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := BuildKey("url", "json", "direct", nil)
	if err := c.Set(key, 1*time.Millisecond, "expire me"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	var got string
	found, err := c.Get(key, &got)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected expired entry to be a miss")
	}
}

func TestNew_EmptyDir(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestDiskCache_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Get("../../etc/passwd", nil)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestNew_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	sub := dir + "/sub/cache"
	c, err := New(sub)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if _, err := os.Stat(sub); os.IsNotExist(err) {
		t.Fatal("expected cache dir to be created")
	}
}
