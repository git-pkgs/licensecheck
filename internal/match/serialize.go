// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package match

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

const dfaMagic = "LREDFA02"

// MarshalMultiLRE serializes a MultiLRE into a compressed binary format
// that can be loaded back with UnmarshalMultiLRE.
func MarshalMultiLRE(re *MultiLRE) []byte {
	words := re.dict.Words()

	// Collect start phrases into a sorted slice for deterministic output.
	var starts []phrase
	for p := range re.start {
		starts = append(starts, p)
	}
	sortPhrases(starts)

	// Build the uncompressed payload (everything after the header).
	var payload bytes.Buffer

	// Dict words in insertion order.
	for _, w := range words {
		writeUint16(&payload, uint16(len(w)))
		payload.WriteString(w)
	}

	// DFA as raw int32 values.
	for _, v := range re.dfa {
		writeInt32(&payload, v)
	}

	// Start phrases.
	for _, p := range starts {
		writeInt32(&payload, int32(p[0]))
		writeInt32(&payload, int32(p[1]))
	}

	// Compress the payload.
	var out bytes.Buffer
	out.WriteString(dfaMagic)
	writeUint32Buf(&out, uint32(len(words)))
	writeUint32Buf(&out, uint32(len(re.dfa)))
	writeUint32Buf(&out, uint32(len(starts)))

	w, _ := flate.NewWriter(&out, flate.BestCompression)
	w.Write(payload.Bytes())
	w.Close()

	return out.Bytes()
}

// UnmarshalMultiLRE deserializes a MultiLRE from the compressed binary format
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

	// Empty DFA (bootstrap/triv file).
	if dictLen == 0 && dfaLen == 0 && startLen == 0 {
		return &MultiLRE{dict: new(Dict), start: make(map[phrase]struct{})}, nil
	}

	// Decompress the payload.
	r := flate.NewReader(bytes.NewReader(data[20:]))
	payload, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		return nil, fmt.Errorf("match: decompressing DFA: %v", err)
	}

	off := 0

	// Reconstruct Dict by inserting words in order.
	d := new(Dict)
	for i := uint32(0); i < dictLen; i++ {
		if off+2 > len(payload) {
			return nil, errors.New("match: DFA data truncated in dict")
		}
		wlen := int(binary.LittleEndian.Uint16(payload[off : off+2]))
		off += 2
		if off+wlen > len(payload) {
			return nil, errors.New("match: DFA data truncated in dict word")
		}
		d.Insert(string(payload[off : off+wlen]))
		off += wlen
	}

	// Read DFA.
	dfaBytes := int(dfaLen) * 4
	if off+dfaBytes > len(payload) {
		return nil, errors.New("match: DFA data truncated in DFA")
	}
	dfa := make(reDFA, dfaLen)
	for i := range dfa {
		dfa[i] = int32(binary.LittleEndian.Uint32(payload[off : off+4]))
		off += 4
	}

	// Read start phrases.
	startBytes := int(startLen) * 8
	if off+startBytes > len(payload) {
		return nil, errors.New("match: DFA data truncated in start phrases")
	}
	start := make(map[phrase]struct{}, startLen)
	for i := uint32(0); i < startLen; i++ {
		var p phrase
		p[0] = WordID(int32(binary.LittleEndian.Uint32(payload[off : off+4])))
		p[1] = WordID(int32(binary.LittleEndian.Uint32(payload[off+4 : off+8])))
		start[p] = struct{}{}
		off += 8
	}

	return &MultiLRE{dict: d, dfa: dfa, start: start}, nil
}

func writeUint16(buf *bytes.Buffer, v uint16) {
	buf.Write([]byte{byte(v), byte(v >> 8)})
}

func writeUint32Buf(buf *bytes.Buffer, v uint32) {
	buf.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func writeInt32(buf *bytes.Buffer, v int32) {
	u := uint32(v)
	buf.Write([]byte{byte(u), byte(u >> 8), byte(u >> 16), byte(u >> 24)})
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
