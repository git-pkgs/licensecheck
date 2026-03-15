// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package match

import "testing"

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	d := new(Dict)
	d.Insert("copyright")
	d.Insert("http")

	lres := []struct {
		name string
		pat  string
	}{
		{"MIT", "permission is hereby granted free of charge"},
		{"BSD", "redistribution and use in source and binary forms"},
	}

	var list []*LRE
	for _, l := range lres {
		re, err := ParseLRE(d, l.name, l.pat)
		if err != nil {
			t.Fatalf("ParseLRE(%s): %v", l.name, err)
		}
		list = append(list, re)
	}

	orig, err := NewMultiLRE(list)
	if err != nil {
		t.Fatal("NewMultiLRE:", err)
	}

	data := MarshalMultiLRE(orig)
	if len(data) < 20 {
		t.Fatalf("MarshalMultiLRE returned %d bytes, want at least 20", len(data))
	}
	if string(data[:8]) != dfaMagic {
		t.Fatalf("bad magic: got %q, want %q", data[:8], dfaMagic)
	}

	restored, err := UnmarshalMultiLRE(data)
	if err != nil {
		t.Fatal("UnmarshalMultiLRE:", err)
	}

	// Verify dict words match.
	origWords := orig.Dict().Words()
	restoredWords := restored.Dict().Words()
	if len(origWords) != len(restoredWords) {
		t.Fatalf("dict length: got %d, want %d", len(restoredWords), len(origWords))
	}
	for i := range origWords {
		if origWords[i] != restoredWords[i] {
			t.Errorf("dict[%d]: got %q, want %q", i, restoredWords[i], origWords[i])
		}
	}

	// Verify match results are identical on test texts.
	texts := []string{
		"Permission is hereby granted, free of charge, to any person",
		"Redistribution and use in source and binary forms, with or without modification",
		"This is not a license at all",
		"",
	}
	for _, text := range texts {
		om := orig.Match(text)
		rm := restored.Match(text)

		if len(om.List) != len(rm.List) {
			t.Errorf("Match(%q): got %d matches, want %d", text, len(rm.List), len(om.List))
			continue
		}
		for i := range om.List {
			if om.List[i] != rm.List[i] {
				t.Errorf("Match(%q)[%d]: got %+v, want %+v", text, i, rm.List[i], om.List[i])
			}
		}
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte("LREDFA0")},
		{"bad magic", []byte("BADMAGIC\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")},
		{"truncated dict", append([]byte("LREDFA01"), []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalMultiLRE(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
