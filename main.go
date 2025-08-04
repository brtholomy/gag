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

type Entry struct {
	filename string
	date     time.Time
	content  string
	tags     []string
}

// convenience shorthand for this awkward map type.
type Set map[string]bool

// add a member to the "set"
func (s Set) Add(k string) {
	s[k] = true
}

func ParseQuery(query string) []string {
	return strings.Split(query, ",")
}

func ParseHeader(content *string) string {
	// returns complete string if not found:
	header, _, _ := strings.Cut(*content, "\n\n")
	return header
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
	r, _ := regexp.Compile(`(?m)^\: ([\.0-9]+?)$`)
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
	header := ParseHeader(content)
	date, _ := ParseDate(&header)
	tags := ParseTags(&header)
	return Entry{
		base,
		date,
		*content,
		tags,
	}
}

func Entries(pattern string) []Entry {
	files, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}
	// NOTE: size 0, capacity specified:
	entries := make([]Entry, 0, len(files))
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

// maps tags to a set of filenames
func Tagmap(entries []Entry) map[string]Set {
	tagmap := map[string]Set{}
	for _, e := range entries {
		for _, tag := range e.tags {
			// allocate submap if necessary:
			if _, ok := tagmap[tag]; !ok {
				tagmap[tag] = Set{}
			}
			tagmap[tag].Add(e.filename)
		}
	}
	return tagmap
}

// adjacencies is a map from tag to other tags occuring in all files.
//
// technically a map[tag]set : go's "set" being a map[T]bool.
func Adjacencies(entries []Entry) map[string]Set {
	adjacencies := map[string]Set{}

	for _, e := range entries {
		for i, tag := range e.tags {
			// make a slice copy but minus the current tag:
			others := make([]string, len(e.tags))
			copy(others, e.tags)
			others = slices.Delete(others, i, i+1)

			// allocate submap if necessary:
			if _, ok := adjacencies[tag]; !ok {
				adjacencies[tag] = Set{}
			}
			for _, other := range others {
				adjacencies[tag].Add(other)
			}
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
				// allocate submap if necessary:
				if _, ok := tagmap[query]; !ok {
					tagmap[query] = Set{}
				}
				tagmap[query].Add(e.filename)
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
				// allocate submap if necessary:
				if _, ok := tagmap[query]; !ok {
					tagmap[query] = Set{}
				}
				tagmap[query].Add(e.filename)
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
				if _, ok := tagmap[query]; ok {
					delete(tagmap[query], e.filename)
				}
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
func Collect(tagmap map[string]Set, adjacencies map[string]Set, queries []string) map[string]Set {
	collection := map[string]Set{}
	collection["files"] = Set{}
	collection["adjacencies"] = Set{}

	for _, query := range queries {
		for file, _ := range tagmap[query] {
			collection["files"].Add(file)
		}
	}

	for _, query := range queries {
		for tag, val := range adjacencies[query] {
			if val {
				collection["adjacencies"].Add(tag)
			}
		}
	}
	return collection
}

// prints out the complete and ordered collection of files, adjacencies, sums,
// and original query tags.
//
// default format is a TOML syntax possibly useful elsewhere. the pipe flag will
// spit out a simple list suitable for piping to cat.
func PrintCollection(collection map[string]Set, queries []string, pipe bool) {
	// sort the collection of files only by proxy at the last moment.
	ordered_files := []string{}
	for f, _ := range collection["files"] {
		ordered_files = append(ordered_files, f)
	}
	slices.Sort(ordered_files)

	// build up strings
	files := fmt.Sprintln("[files]")
	for _, f := range ordered_files {
		files += fmt.Sprintln(f)
	}

	tags := fmt.Sprintln("[tags]")
	for _, q := range queries {
		tags += fmt.Sprintln(q)
	}

	adj := fmt.Sprintln("[adjacencies]")
	for t, _ := range collection["adjacencies"] {
		adj += fmt.Sprintln(t)
	}

	sums := fmt.Sprintln("[sums]")
	sums += fmt.Sprintln("files =", len(collection["files"]))
	sums += fmt.Sprintln("adjacencies =", len(collection["adjacencies"]))

	if pipe {
		// slice off including the newline:
		files = files[8:]
		fmt.Println(files)
		return
	}
	fmt.Println(files)
	fmt.Println(tags)
	fmt.Println(adj)
	fmt.Println(sums)
}

func main() {
	var glob = flag.String("glob", "./*md", "search for files with this glob pattern.")
	var query = flag.String("query", "", "search for files with the given tag(s). "+
		"This option may be passed implicitly as the first arg.")
	var grep = flag.Bool("grep", false, "whether to show files containing the query as content.")
	var find = flag.Bool("find", false, "whether to show files containing the query as filename.")
	var diff = flag.Bool("diff", false, "whether to omit files containing the query as tag.")
	var pipe = flag.Bool("pipe", false, "whether to only print files for piping.")
	flag.Parse()

	// take first positional arg as query:
	// NOTE: all flags must precede: gag --grep arg
	if *query == "" {
		if len(flag.Args()) > 0 {
			*query = flag.Args()[0]
		} else {
			flag.Usage()
			return
		}
	}

	queries := ParseQuery(*query)
	entries := Entries(*glob)
	// a chance for concurrency:
	tmch := make(chan map[string]Set)
	adch := make(chan map[string]Set)
	go func() {
		tmch <- Tagmap(entries)
	}()
	go func() {
		adch <- Adjacencies(entries)
	}()
	tagmap := <-tmch
	adjacencies := <-adch
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
	PrintCollection(collection, queries, *pipe)
}
