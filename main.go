package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const PATTERN string = "/home/bth/x/writing/journal/*.md"

type Entry struct {
	filename string
	date     time.Time
	content  string
	tags     []string
}

// convenience shorthand for this awkward type:
type Set map[string]bool

func ParseQuery(query string) []string {
	return strings.Split(query, ",")
}

func Entries(pattern string) (entries []Entry) {
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
		entries = append(entries, e)
	}
	return entries
}

func ParseTags(content *string) (tags []string) {
	r, _ := regexp.Compile(`(?m)^\+ (.+)$`)
	res := r.FindAllStringSubmatch(*content, -1)
	for i := range res {
		// group submatch is indexed at 1:
		// this shouldn't ever fail if there's a result:
		tags = append(tags, res[i][1])
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
	return Entry{
		base,
		date,
		*content,
		tags,
	}
}

// maps tags to a set of filenames
func Tagmap(entries []Entry) (tagmap map[string]Set) {
	tagmap = map[string]Set{}
	for _, e := range entries {
		for _, tag := range e.tags {
			// allocate submap if necessary:
			if _, ok := tagmap[tag]; !ok {
				tagmap[tag] = Set{}
			}
			tagmap[tag][e.filename] = true
		}
	}
	return tagmap
}

// adjacencies is a map from tag to other tags occuring in all files.
//
// technically a map[tag]set : go's "set" being a map[T]bool.
func Adjacencies(entries []Entry, tagmap map[string]Set) (adjacencies map[string]Set) {
	adjacencies = map[string]Set{}
	for tag, _ := range tagmap {
		adjacencies[tag] = Set{}
	}

	for _, e := range entries {
		for i, tag := range e.tags {
			others := make([]string, len(e.tags))
			copy(others, e.tags)
			others = slices.Delete(others, i, i+1)
			set, ok := adjacencies[tag]
			// TODO: is this necessary?
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

// extends the tagmap to include files which contain the query string, like grepping.
func Grep(entries []Entry, tagmap map[string]Set, queries []string) map[string]Set {
	for _, e := range entries {
		for _, query := range queries {
			// TODO: in the presence of multiple query strings, this is an OR.
			// Should be an AND.
			if strings.Contains(strings.ToLower(e.content), query) {
				tagmap[query][e.filename] = true
			}
		}
	}
	return tagmap
}

// extends the tagmap to include filenames which contain the query string, like find.
func Find(entries []Entry, tagmap map[string]Set, queries []string) map[string]Set {
	for _, e := range entries {
		for _, query := range queries {
			if strings.Contains(e.filename, query) {
				tagmap[query][e.filename] = true
			}
		}
	}
	return tagmap
}

// shrinks the tagmap to exclude filenames which contain the query as a tag.
func Diff(entries []Entry, tagmap map[string]Set, queries []string) map[string]Set {
	for _, e := range entries {
		for _, query := range queries {
			if slices.Contains(e.tags, query) {
				tagmap[query][e.filename] = false
			}
		}
	}
	return tagmap
}

// collects our maps between all tags:files and all tags:tags, into one Set of
// files, and one Set of adjacent tags.
//
// NOTE: would be more efficient to only map the relevant queried tag to file,
// but Adjacencies() is easier knowing about all tags.
func Collect(
	tagmap map[string]Set,
	adjacencies map[string]Set,
	queries []string,
) (collection map[string]Set) {
	collection = map[string]Set{}
	collection["files"] = Set{}
	collection["adjacencies"] = Set{}

	for _, query := range queries {
		for file, val := range tagmap[query] {
			// because the --diff command may have altered the entry to false
			if val {
				collection["files"][file] = true
			}
		}
	}

	for _, query := range queries {
		for tag, val := range adjacencies[query] {
			if val {
				collection["adjacencies"][tag] = true
			}
		}
	}
	return collection
}

func PrintCollection(collection map[string]Set, queries []string, grep bool) {
	fmt.Println("[tag]")
	fmt.Println(queries)
	fmt.Println()

	s := "[files]"
	if grep {
		s = "[files:grep]"
	}
	fmt.Println(s)
	for f, _ := range collection["files"] {
		fmt.Println(f)
	}
	fmt.Println()

	fmt.Println("[tags]")
	for t, _ := range collection["adjacencies"] {
		fmt.Println(t)
	}
}

func main() {
	var query = flag.String("query", "", "search for files with the given tag(s).")
	var grep = flag.Bool("grep", false, "whether to show files containing the query as content.")
	var find = flag.Bool("find", false, "whether to show files containing the query as filename.")
	var diff = flag.Bool("diff", false, "whether to omit files containing the query as tag.")
	flag.Parse()

	// take first positional arg as query:
	// NOTE: all flags must precede: gag --grep arg
	if *query == "" && len(flag.Args()) > 0 {
		*query = flag.Args()[0]
	}

	queries := ParseQuery(*query)
	entries := Entries(PATTERN)
	tagmap := Tagmap(entries)
	adjacencies := Adjacencies(entries, tagmap)
	if *grep {
		tagmap = Grep(entries, tagmap, queries)
	}
	if *find {
		tagmap = Find(entries, tagmap, queries)
	}
	if *diff {
		tagmap = Diff(entries, tagmap, queries)
	}

	collection := Collect(tagmap, adjacencies, queries)
	PrintCollection(collection, queries, *grep)
}
