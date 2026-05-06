// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"os/user"
	"strconv"
	"syscall"
)

// HasAccess tests if a caller can access a file with permissions
// `perm` in mode `mask`
func HasAccess(callerUid, callerGid, fileUid, fileGid uint32, perm uint32, mask uint32) bool {
	mask = mask & 7
	if mask == 0 {
		return true
	}

	if callerUid == 0 {
		return rootHasAccess(perm, mask)
	}

	if callerUid == fileUid {
		allowed := (perm >> 6) & 7
		return allowed&mask == mask
	}
	if callerGid == fileGid {
		allowed := (perm >> 3) & 7
		return allowed&mask == mask
	}

	// Supplementary group membership selects the group bits too; if the lookup
	// fails, fall through to the other bits.
	u, err := user.LookupId(strconv.Itoa(int(callerUid)))
	if err == nil {
		gs, err := u.GroupIds()
		if err == nil {
			fileGidStr := strconv.Itoa(int(fileGid))
			for _, gidStr := range gs {
				if gidStr == fileGidStr {
					allowed := (perm >> 3) & 7
					return allowed&mask == mask
				}
			}
		}
	}

	allowed := perm & 7
	return allowed&mask == mask
}

// rootHasAccess follows Linux DAC override semantics: root can read/write
// regardless of mode bits, but executing a non-directory requires at least one
// execute bit. Directory search permission is still overridable.
func rootHasAccess(perm uint32, mask uint32) bool {
	if mask&1 == 0 {
		return true
	}
	return perm&syscall.S_IFMT == syscall.S_IFDIR || perm&0111 != 0
}
