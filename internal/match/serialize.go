// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package match

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
)

const dfaMagic = "LREDFA01"

// MarshalMultiLRE serializes a MultiLRE into a binary format
// that can be loaded back with UnmarshalMultiLRE.
func MarshalMultiLRE(re *MultiLRE) []byte {
	words := re.dict.Words()

	// Collect start phrases into a sorted slice for deterministic output.
	var starts []phrase
	for p := range re.start {
		starts = append(starts, p)
	}
	sortPhrases(starts)

	// Calculate size.
	dictBytes := 0
	for _, w := range words {
		dictBytes += 2 + len(w) // uint16 length + UTF-8 bytes
	}
	size := 8 + 12 + dictBytes + len(re.dfa)*4 + len(starts)*8

	buf := make([]byte, 0, size)

	// Magic.
	buf = append(buf, dfaMagic...)

	// Header: dictLen, dfaLen, startLen.
	buf = appendUint32(buf, uint32(len(words)))
	buf = appendUint32(buf, uint32(len(re.dfa)))
	buf = appendUint32(buf, uint32(len(starts)))

	// Dict words in insertion order.
	for _, w := range words {
		buf = appendUint16(buf, uint16(len(w)))
		buf = append(buf, w...)
	}

	// DFA as raw int32 values.
	for _, v := range re.dfa {
		buf = appendInt32(buf, v)
	}

	// Start phrases.
	for _, p := range starts {
		buf = appendInt32(buf, int32(p[0]))
		buf = appendInt32(buf, int32(p[1]))
	}

	return buf
}

// UnmarshalMultiLRE deserializes a MultiLRE from the binary format
// produced by MarshalMultiLRE.
func UnmarshalMultiLRE(data []byte) (*MultiLRE, error) {
	if len(data) < 20 {
		return nil, errors.New("match: DFA data too short")
	}
	if string(data[:8]) != dfaMagic {
		return nil, fmt.Errorf("match: bad DFA magic %q", data[:8])
	}

	dictLen := binary.LittleEndian.Uint32(data[8:12])
	dfaLen := binary.LittleEndian.Uint32(data[12:16])
	startLen := binary.LittleEndian.Uint32(data[16:20])

	off := 20

	// Reconstruct Dict by inserting words in order.
	d := new(Dict)
	for i := uint32(0); i < dictLen; i++ {
		if off+2 > len(data) {
			return nil, errors.New("match: DFA data truncated in dict")
		}
		wlen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if off+wlen > len(data) {
			return nil, errors.New("match: DFA data truncated in dict word")
		}
		d.Insert(string(data[off : off+wlen]))
		off += wlen
	}

	// Read DFA.
	dfaBytes := int(dfaLen) * 4
	if off+dfaBytes > len(data) {
		return nil, errors.New("match: DFA data truncated in DFA")
	}
	dfa := make(reDFA, dfaLen)
	for i := range dfa {
		dfa[i] = int32(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
	}

	// Read start phrases.
	startBytes := int(startLen) * 8
	if off+startBytes > len(data) {
		return nil, errors.New("match: DFA data truncated in start phrases")
	}
	start := make(map[phrase]struct{}, startLen)
	for i := uint32(0); i < startLen; i++ {
		var p phrase
		p[0] = WordID(int32(binary.LittleEndian.Uint32(data[off : off+4])))
		p[1] = WordID(int32(binary.LittleEndian.Uint32(data[off+4 : off+8])))
		start[p] = struct{}{}
		off += 8
	}

	return &MultiLRE{dict: d, dfa: dfa, start: start}, nil
}

func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendInt32(buf []byte, v int32) []byte {
	return appendUint32(buf, uint32(v))
}

func sortPhrases(ps []phrase) {
	slices.SortFunc(ps, func(a, b phrase) int {
		if a[0] != b[0] {
			if a[0] < b[0] {
				return -1
			}
			return 1
		}
		if a[1] != b[1] {
			if a[1] < b[1] {
				return -1
			}
			return 1
		}
		return 0
	})
}
