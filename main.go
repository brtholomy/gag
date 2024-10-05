package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const PATTERN string = "/home/bth/x/writing/journal/139*.md"

type Entry struct {
	filename string
	date     string
	content  string
	tags     []string
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
		tags[i] = res[i][1]
	}
	return tags
}

func ParseDate(content *string) string {
	r, _ := regexp.Compile(`(?m)^\: (.+)\n`)
	res := r.FindStringSubmatch(*content)
	return res[1]
}

func ParseContent(filename string, content *string) Entry {
	tags := ParseTags(content)
	date := ParseDate(content)
	e := Entry{
		filepath.Base(filename),
		date,
		*content,
		tags,
	}
	return e
}

func main() {
	entries := GetEntries(PATTERN)
	fmt.Println(entries[9].tags)
	fmt.Println(entries[9].date)
	fmt.Println(entries[9].filename)
}
