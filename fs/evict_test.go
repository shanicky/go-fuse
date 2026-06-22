// Copyright 2026 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvictCachedInodes(t *testing.T) {
	tc := newTestCase(t, &testOptions{attrCache: true, entryCache: true})
	root := tc.loopback.EmbeddedInode()

	const files = 8
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("file-%02d", i)
		tc.writeOrig(name, "hello", 0644)
		if _, err := os.Lstat(filepath.Join(tc.mntDir, name)); err != nil {
			t.Fatalf("Lstat(%q): %v", name, err)
		}
	}

	cached := root.CachedInodeCount()
	if cached < files+1 {
		t.Fatalf("CachedInodeCount() = %d, want at least %d", cached, files+1)
	}

	evicted := root.EvictCachedInodes(files)
	if evicted == 0 {
		t.Fatal("EvictCachedInodes() = 0, want at least 1")
	}

	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-timeout.C:
			t.Fatalf("CachedInodeCount() did not drop after evicting %d entries; got %d, initial %d",
				evicted, root.CachedInodeCount(), cached)
		case <-tick.C:
			if got := root.CachedInodeCount(); got < cached {
				return
			}
		}
	}
}

func TestEvictCachedInodesSkipsPersistent(t *testing.T) {
	root := &Inode{}
	mntDir, _ := testMount(t, root, &Options{
		FirstAutomaticIno: 1,
		OnAdd: func(ctx context.Context) {
			root.AddChild("persistent", root.NewPersistentInode(ctx, &Inode{}, StableAttr{}), false)
		},
	})

	if _, err := os.Lstat(filepath.Join(mntDir, "persistent")); err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	cached := root.CachedInodeCount()
	if cached < 2 {
		t.Fatalf("CachedInodeCount() = %d, want at least 2", cached)
	}
	if evicted := root.EvictCachedInodes(1); evicted != 0 {
		t.Fatalf("EvictCachedInodes() = %d, want 0 for persistent inode", evicted)
	}
	if got := root.CachedInodeCount(); got != cached {
		t.Fatalf("CachedInodeCount() = %d after eviction attempt, want %d", got, cached)
	}
}
