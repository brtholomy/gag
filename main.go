package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"
)

const PATTERN string = "/home/bth/x/writing/journal/*.md"

type Entry struct {
	filename string
	date     time.Time
	content  string
	tags     []string
}

type Term struct {
	tag         string
	files       []string
	adjacencies []string
}

func GetEntries(pattern string) []Entry {
	res := []Entry{}
	files, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		dat, err := os.ReadFile(f)
		if err != nil {
			panic(err)
		}
		s := string(dat)
		e := ParseContent(f, &s)
		res = append(res, e)
	}
	return res
}

func ParseTags(content *string) []string {
	r, _ := regexp.Compile(`(?m)^\+ (.+)$`)
	res := r.FindAllStringSubmatch(*content, -1)
	tags := make([]string, len(res))
	for i := range res {
		// group submatch is indexed at 1:
		// this shouldn't ever fail if there's a result:
		tags[i] = res[i][1]
	}
	return tags
}

func ParseDate(content *string) (time.Time, error) {
	r, _ := regexp.Compile(`(?m)^\: (.+)\n`)
	res := r.FindStringSubmatch(*content)
	if len(res) < 2 {
		return time.Time{}, errors.New("failed to find date string")
	}
	// The layout string must be a representation of:
	// Jan 2 15:04:05 2006 MST
	// 1   2  3  4  5    6  -7
	return time.Parse("2006.01.02", res[1])
}

func ParseContent(filename string, content *string) Entry {
	base := filepath.Base(filename)
	date, _ := ParseDate(content)
	tags := ParseTags(content)
	e := Entry{
		base,
		date,
		*content,
		tags,
	}
	return e
}

// map tag to filenames
func MakeTagmap(entries []Entry) map[string][]string {
	tagmap := make(map[string][]string, len(entries))
	for _, e := range entries {
		for _, tag := range e.tags {
			tagmap[tag] = append(tagmap[tag], e.filename)
		}
	}
	return tagmap
}

// adjacencies is a map from tag to other tags occuring in all files.
//
// technically a map[tag]set : go's "set" being a map[T]bool.
func Adjacencies(entries []Entry, tagmap map[string][]string) map[string]map[string]bool {
	adjacencies := make(map[string]map[string]bool, len(entries))
	for tag, _ := range tagmap {
		adjacencies[tag] = map[string]bool{}
	}

	for _, e := range entries {
		for i, tag := range e.tags {
			others := make([]string, len(e.tags))
			copy(others, e.tags)
			others = slices.Delete(others, i, i+1)
			set, ok := adjacencies[tag]
			if ok {
				for _, t := range others {
					set[t] = true
				}
			}
			adjacencies[tag] = set
		}
	}
	return adjacencies
}

func main() {
	var query = flag.String("query", "science", "query")
	flag.Parse()

	entries := GetEntries(PATTERN)
	tagmap := MakeTagmap(entries)
	for _, f := range tagmap[*query] {
		fmt.Println(f)
	}
	adjacencies := Adjacencies(entries, tagmap)
	for t, _ := range adjacencies[*query] {
		fmt.Println(t)
	}
}
