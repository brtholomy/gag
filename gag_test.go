package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const TEST_PATTERN string = "./mock/*.md"

func TestParseHeader(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	header := ParseHeader(&entries[0].content)
	expected := "# 01.foo.md\n: 2024.09.25\n+ sot\n+ foo"
	assert.Equal(t, expected, header)
}

func TestEntriesLen(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	expected := 6
	if len(entries) != expected {
		t.Errorf("entries should be len == %v, got %v", expected, len(entries))
	}
}

func TestEntries(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	d, _ := time.Parse("2006.01.02", "2024.09.25")
	expected := Entry{filename: "01.foo.md", date: d, content: "# 01.foo.md\n: 2024.09.25\n+ sot\n+ foo\n\nFoo bar.\n", tags: []string{"sot", "foo"}}
	assert.Equal(t, expected, entries[0])
}

func TestTagmap(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	tagmap := Tagmap(entries)
	expected := Set{"01.foo.md": true, "02.foo.md": true, "03.bar.md": true}
	assert.Equal(t, expected, tagmap["sot"])
}

func TestAdjacencies(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	adjacencies := Adjacencies(entries)
	expected := Set{"science": true, "foo": true}
	assert.Equal(t, expected, adjacencies["sot"])
}

func TestGrep(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("foo")
	tagmap := Tagmap(entries)

	tagmap = Grep(entries, tagmap, queries)
	expected := Set{"01.foo.md": true, "02.foo.md": true, "04.baz.md": true}
	assert.Equal(t, expected, tagmap["foo"])
}

func TestBadTag(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("qaz")
	tagmap := Tagmap(entries)

	// test that these won't panic
	tagmap = Grep(entries, tagmap, queries)
	tagmap = Find(entries, tagmap, queries)
	tagmap = Diff(entries, tagmap, queries)
	_, ok := tagmap["qaz"]
	assert.False(t, ok)
}

func TestFind(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("baz")
	tagmap := Tagmap(entries)

	tagmap = Find(entries, tagmap, queries)
	expected := Set{"04.baz.md": true}
	assert.Equal(t, expected, tagmap["baz"])
}

func TestDiff(t *testing.T) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("diff")
	tagmap := Tagmap(entries)
	tagmap = Grep(entries, tagmap, queries)

	tagmap = Diff(entries, tagmap, queries)
	expected := Set{"06.quz.md": true}
	assert.Equal(t, expected, tagmap["diff"])
}

// Since Entries() involves filesystem reads, we test the underlying logic.
func BenchmarkParseContent(b *testing.B) {
	e := Entries(TEST_PATTERN)[0]
	for b.Loop() {
		ParseContent(e.filename, &e.content)
	}
}

func BenchmarkTagmap(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	for b.Loop() {
		Tagmap(entries)
	}
}

func BenchmarkAdjacencies(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	for b.Loop() {
		Adjacencies(entries)
	}
}

func BenchmarkGrep(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("foo")
	tagmap := Tagmap(entries)
	for b.Loop() {
		Grep(entries, tagmap, queries)
	}
}

func BenchmarkFind(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("foo")
	tagmap := Tagmap(entries)
	for b.Loop() {
		Find(entries, tagmap, queries)
	}
}

func BenchmarkDiff(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("foo")
	tagmap := Tagmap(entries)
	for b.Loop() {
		Diff(entries, tagmap, queries)
	}
}

func BenchmarkCollect(b *testing.B) {
	entries := Entries(TEST_PATTERN)
	queries := ParseQuery("foo")
	tagmap := Tagmap(entries)
	adjacencies := Adjacencies(entries)
	for b.Loop() {
		Collect(tagmap, adjacencies, queries)
	}
}
