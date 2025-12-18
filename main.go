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

// The layout string must be a representation of:
// Jan 2 15:04:05 2006 MST
// 1   2  3  4  5    6  -7
const DATE_FORMAT = "2006.01.02"

// ^: YYYY.DD.MM$
const DATE_REGEXP = `(?m)^\: ([\.0-9]+?)$`

// ^+ tag$
const TAG_REGEXP = `(?m)^\+ (.+)$`

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

// intersect two sets
func Intersect(p Set, q Set) Set {
	r := Set{}
	for m, _ := range p {
		if q[m] {
			r.Add(m)
		}
	}
	return r
}

// TODO: only accepts intersection syntax for now
func ParseQuery(query string) []string {
	return strings.Split(query, "+")
}

func ParseHeader(content *string) string {
	// returns complete string if not found:
	header, _, _ := strings.Cut(*content, "\n\n")
	return header
}

func ParseTags(content *string) (tags []string) {
	r, _ := regexp.Compile(TAG_REGEXP)
	res := r.FindAllStringSubmatch(*content, -1)
	for i := range res {
		// group submatch is indexed at 1:
		// this shouldn't ever fail if there's a result:
		tags = append(tags, res[i][1])
	}
	return tags
}

func ParseDate(content *string) (time.Time, error) {
	r, _ := regexp.Compile(DATE_REGEXP)
	res := r.FindStringSubmatch(*content)
	if len(res) < 2 {
		return time.Time{}, errors.New("failed to find date string")
	}
	return time.Parse(DATE_FORMAT, res[1])
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

// shrinks the entries to only include files within a date range.
func Date(entries []Entry, date string) []Entry {
	// deleting from the old slice would be less efficient than appending to a new one:
	ranged := make([]Entry, 0, len(entries))
	from, to := time.Time{}, time.Time{}

	// when there's no range, the first string here will be the input:
	f, t, ok := strings.Cut(date, "-")
	from, _ = time.Parse(DATE_FORMAT, f)
	if ok {
		to, _ = time.Parse(DATE_FORMAT, t)
	} else {
		// use the from date for the case of a single date given:
		to, _ = time.Parse(DATE_FORMAT, f)
	}
	for _, e := range entries {
		if from.Compare(e.date) <= 0 && 0 <= to.Compare(e.date) {
			ranged = append(ranged, e)
		}
	}
	return ranged
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

// produce a Set reduced to the files covered by combined queries
// TODO: handle comma separated groups as logical OR
func IntersectQueries(tagmap map[string]Set, queries []string) Set {
	set := Set{}
	// sanity check:
	if len(queries) < 1 {
		return set
	}
	// initialize as first query
	q := queries[0]
	set = tagmap[q]
	// when queries < 2, this won't run, and the Join will be identical to q
	for i := 1; i < len(queries); i++ {
		q = queries[i]
		set = Intersect(set, tagmap[q])
	}
	return set
}

// reduces adjacencies to relevant individual queries
// TODO: does not reduce to + intersected queries
func ReduceAdjacencies(adjacencies map[string]Set, queries []string) Set {
	reduced := Set{}
	for _, query := range queries {
		for tag, val := range adjacencies[query] {
			if val {
				reduced.Add(tag)
			}
		}
	}
	return reduced
}

// prints out the intersected tagmap
func SprintFiles(files Set) string {
	ordered_files := []string{}
	for f, _ := range files {
		ordered_files = append(ordered_files, f)
	}
	slices.Sort(ordered_files)
	return fmt.Sprintln(strings.Join(ordered_files, "\n"))
}

// prints out the complete and ordered collection of files, adjacencies, sums,
// and original query tags.
//
// format is a TOML syntax possibly useful elsewhere.
func Print(files Set, adjacencies Set, queries []string, verbose bool) {
	f := SprintFiles(files)
	if !verbose {
		fmt.Print(f)
		return
	}
	filesstr := fmt.Sprintln("[files]")
	filesstr += f

	tags := fmt.Sprintln("[tags]")
	for _, q := range queries {
		tags += fmt.Sprintln(q)
	}

	adj := fmt.Sprintln("[adjacencies]")
	for t, _ := range adjacencies {
		adj += fmt.Sprintln(t)
	}

	sums := fmt.Sprintln("[sums]")
	sums += fmt.Sprintln("files =", strings.Count(f, "\n"))
	sums += fmt.Sprintln("adjacencies =", len(adjacencies))

	fmt.Println(filesstr)
	fmt.Println(tags)
	fmt.Println(adj)
	fmt.Println(sums)
}

func main() {
	var glob = flag.String("glob", "./*md", "search for files with this glob pattern.")
	var query = flag.String("query", "", "search for files with the given tag(s). "+
		"This option may be passed implicitly as the first arg.")
	var date = flag.String("date", "", "search for files matching a date given in ISO 8601: "+
		"YYYY.MM.DD. May be a single date, or a range: YYYY.MM.DD-YYYY.MM.DD.")
	var grep = flag.Bool("grep", false, "whether to show files containing the query as content.")
	var find = flag.Bool("find", false, "whether to show files containing the query as filename.")
	var diff = flag.Bool("diff", false, "whether to omit files containing the query as tag.")
	var verbose = flag.Bool("verbose", false, "whether to print out a verbose summary")
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

	// we shrink the entries list immediately if we want a date range:
	if *date != "" {
		entries = Date(entries, *date)
	}
	tagmap := Tagmap(entries)

	// NOTE: these have to precede IntersectQueries because they expand the incoming tagmap:
	if *grep {
		tagmap = Grep(entries, tagmap, queries)
	}
	if *find {
		tagmap = Find(entries, tagmap, queries)
	}
	if *diff {
		tagmap = Diff(entries, tagmap, queries)
	}

	intersected := IntersectQueries(tagmap, queries)

	// TODO: should respect the Set as produced by IntersectQueries:
	adjacencies := map[string]Set{}
	if *verbose {
		adjacencies = Adjacencies(entries)
	}
	// TODO: making this unnecessary:
	reducedadj := ReduceAdjacencies(adjacencies, queries)

	Print(intersected, reducedadj, queries, *verbose)
}
