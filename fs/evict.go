// Copyright 2026 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "math"

// CachedInodeCount returns the number of inodes in the kernel node ID cache.
func (n *Inode) CachedInodeCount() int {
	bridge := n.bridge
	if bridge == nil {
		return 0
	}

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return len(bridge.kernelNodeIds)
}

func (n *Inode) evictableEntry() (string, *Inode, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	p := n.parents.get()
	if p == nil || n.persistent || n.children.len() > 0 {
		return "", nil, false
	}
	return p.name, p.parent, true
}

// evictScanFactor bounds sampled cache entries per eviction call.
const evictScanFactor = 8

// EvictCachedInodes asks the kernel to forget up to max cached leaf entries.
// It is best-effort and returns the number of invalidation requests accepted.
// The inode leaves the cache only after the kernel later sends FORGET.
func (n *Inode) EvictCachedInodes(max int) int {
	if max <= 0 || n.bridge == nil {
		return 0
	}

	bridge := n.bridge

	// Bound scan work and allocation.
	budget := max
	if max <= math.MaxInt/evictScanFactor {
		budget *= evictScanFactor
	} else {
		budget = math.MaxInt
	}

	bridge.mu.Lock()
	if budget > len(bridge.kernelNodeIds) {
		budget = len(bridge.kernelNodeIds)
	}
	bridge.mu.Unlock()
	if budget == 0 {
		return 0
	}
	candidates := make([]*Inode, 0, budget)

	// Leaf checks take inode locks, so only copy pointers here.
	bridge.mu.Lock()
	for _, candidate := range bridge.kernelNodeIds {
		if candidate == bridge.root {
			continue
		}
		candidates = append(candidates, candidate)
		if len(candidates) >= budget {
			break
		}
	}
	bridge.mu.Unlock()

	evicted := 0
	for _, candidate := range candidates {
		if evicted >= max {
			break
		}
		name, parent, ok := candidate.evictableEntry()
		if !ok {
			continue
		}
		// NotifyEntry invalidates parent/name; FORGET removes the inode.
		if parent.NotifyEntry(name) == 0 {
			evicted++
		}
	}
	return evicted
}
