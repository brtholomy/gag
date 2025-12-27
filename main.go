package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// The layout string must be a representation of:
	// Jan 2 15:04:05 2006 MST
	// 1   2  3  4  5    6  -7
	DATE_FORMAT = "2006.01.02"

	// ^: YYYY.DD.MM$
	DATE_REGEXP = `(?m)^\: ([\.0-9]+?)$`

	// ^+ tag$
	TAG_REGEXP = `(?m)^\+ (.+)$`
)

type Operator string

const (
	SINGLE Operator = ""
	OR     Operator = ","
	AND    Operator = "+"
	WILD   Operator = "*"
)

type Query struct {
	Op   Operator
	Tags []string
}

type Entry struct {
	filename string
	date     time.Time
	content  string
	tags     []string
}

// TODO: consider using this in tagmap and adjacencies. Currently only for printed results for the
// sake of slices.SortFunc
type Tag struct {
	name  string
	count int
}

// convenience shorthand for this awkward map type.
type Set map[string]bool

// add members to the "set"
func (s Set) Add(mems ...string) {
	for _, m := range mems {
		s[m] = true
	}
}

// get all members in a slice
func (s Set) Members() []string {
	return slices.Collect(maps.Keys(s))
}

// s ∪ t
func (s Set) Union(tt ...Set) {
	for _, t := range tt {
		s.Add(t.Members()...)
	}
}

// s ∩ t
func (s Set) Intersect(t Set) {
	for m, _ := range s {
		if !t[m] {
			delete(s, m)
		}
	}
}

func isStdinLoaded() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func GetStdin() ([]string, error) {
	if !isStdinLoaded() {
		return nil, errors.New("stdin not loaded")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	// TODO: there's got to be a better way:
	s, _ := strings.CutSuffix(string(data), "\n")
	return strings.Split(s, "\n"), nil
}

// reads files from stdin if present, otherwise from the glob pattern:
func Filelist(glob string) []string {
	filelist, err := GetStdin()
	// otherwise get from the glob:
	if err != nil {
		filelist, err = filepath.Glob(glob)
		if err != nil {
			log.Fatal(err)
		}
	}
	return filelist
}

// TODO: only accepts one kind of syntax at a time
func ParseQuery(query string) Query {
	// initialize for the single tag case:
	q := Query{
		Op:   SINGLE,
		Tags: []string{query},
	}
	// count a missing tag as WILD:
	if query == "" {
		// TODO: somewhat abusing this concept for the empty query case:
		q.Op = WILD
		return q
	}
	// NOTE: will match OR first
	ops := []Operator{OR, AND}
	for _, op := range ops {
		if s := strings.Split(query, string(op)); len(s) > 1 {
			q.Op = op
			q.Tags = s
			break
		}

	}
	return q
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

func Entries(filelist []string) []Entry {
	// NOTE: size 0, capacity specified:
	entries := make([]Entry, 0, len(filelist))
	for _, f := range filelist {
		dat, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
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

// produce a Set reduced to the files covered by combined queries
func ProcessQueries(tagmap map[string]Set, query Query) Set {
	set := Set{}
	// sanity check:
	if len(query.Tags) < 1 {
		return set
	}

	// initialize as first query
	q := query.Tags[0]

	// empty query
	// TODO: do I want to handle WILD and a tag? Actual regex?
	if query.Op == WILD && q == "" {
		// NOTE: this is all files with at least one tag and therefore of value:
		set.Union(slices.Collect(maps.Values(tagmap))...)
		return set
	}

	// NOTE: clone so that we don't accidentally overwrite the incoming tagmap
	set = maps.Clone(tagmap[q])
	// when queries < 2, this won't run
	for i := 1; i < len(query.Tags); i++ {
		q = query.Tags[i]
		switch query.Op {
		case OR:
			set.Union(tagmap[q])
		case AND:
			set.Intersect(tagmap[q])
		}
	}
	return set
}

// inverts the filelist using the full list from entries. works with intersected queries as long as
// ProcessQueries is called first.
func Invert(entries []Entry, files Set) Set {
	set := Set{}
	for _, e := range entries {
		if _, ok := files[e.filename]; !ok {
			set.Add(e.filename)
		}
	}
	return set
}

// adjacencies is a map from tag to a map of other tags occuring in the given files.
func Adjacencies(entries []Entry, files Set) map[string]map[string]Set {
	adjacencies := map[string]map[string]Set{}

	for _, e := range entries {
		// NOTE: this allows for a filelist shrunk after entries slice was made:
		if !files[e.filename] {
			continue
		}
		for i, tag := range e.tags {
			// make a slice copy but minus the current tag:
			others := make([]string, len(e.tags))
			copy(others, e.tags)
			others = slices.Delete(others, i, i+1)

			// allocate submap if necessary:
			if _, ok := adjacencies[tag]; !ok {
				adjacencies[tag] = map[string]Set{}
			}
			for _, other := range others {
				if _, ok := adjacencies[tag][other]; !ok {
					adjacencies[tag][other] = Set{}
				}
				adjacencies[tag][other].Add(e.filename)
			}
		}
	}
	return adjacencies
}

// reduces adjacencies to a single map[tag]Set not including the query tags
func ReduceAdjacencies(adjacencies map[string]map[string]Set, query Query, invert bool) map[string]Set {
	reduced := map[string]Set{}
	if invert {
		// TODO: something's wrong here...
		// adjacencies keys will already reflect all tags built from an inverted filelist
		for _, adjmap := range adjacencies {
			for adj, files := range adjmap {
				if _, ok := reduced[adj]; !ok {
					reduced[adj] = Set{}
				}
				reduced[adj].Union(files)
			}
		}

		return reduced
	}
	for _, qtag := range query.Tags {
		for adjtag, files := range adjacencies[qtag] {
			if !slices.Contains(query.Tags, adjtag) {
				if _, ok := reduced[adjtag]; !ok {
					reduced[adjtag] = Set{}
				}
				reduced[adjtag].Add(files.Members()...)
			}
		}
	}
	return reduced
}

// prints out the intersected tagmap
func SprintFiles(files Set) string {
	ordered_files := make([]string, 0, len(files))
	copy(ordered_files, slices.Collect(maps.Keys(files)))
	slices.Sort(ordered_files)
	return fmt.Sprintln(strings.Join(ordered_files, "\n"))
}

func OrderedTags(tagmap map[string]Set, query Query) []Tag {
	ordered_tags := []Tag{}
	// TODO: there's code smell about this whole approach.
	if query.Op == WILD {
		for q, s := range tagmap {
			ordered_tags = append(ordered_tags, Tag{q, len(s)})
		}
	} else {
		for _, q := range query.Tags {
			ordered_tags = append(ordered_tags, Tag{q, len(tagmap[q])})
		}
	}
	slices.SortFunc(ordered_tags, func(i, j Tag) int {
		return cmp.Compare(i.count, j.count)
	})
	return ordered_tags
}

// prints out the complete and ordered collection of files, adjacencies, sums,
// and original query tags.
//
// format is a TOML syntax possibly useful elsewhere.
func Print(entries []Entry, tagmap map[string]Set, files Set, adjacencies map[string]Set, query Query, verbose bool) {
	f := SprintFiles(files)
	if !verbose {
		fmt.Print(f)
		return
	}
	filesstr := fmt.Sprintln("[files]")
	filesstr += f

	tags := fmt.Sprintln("[tags]")
	otags := OrderedTags(tagmap, query)
	for _, t := range otags {
		tags += fmt.Sprintf("%-20s= %d\n", t.name, t.count)
	}

	adj := fmt.Sprintln("[adjacencies]")
	oadj := OrderedTags(adjacencies, Query{WILD, []string{}})
	for _, t := range oadj {
		// TODO: something's fucky about these len() with --invert :
		adj += fmt.Sprintf("%-20s= %-3d : %d\n", t.name, t.count, len(tagmap[t.name]))
	}

	sums := fmt.Sprintln("[sums]")
	sums += fmt.Sprintf("files               = %-3d : %d\n", len(files), len(entries))
	sums += fmt.Sprintf("adjacencies         = %-3d : %d\n", len(adjacencies), len(tagmap))

	fmt.Println(filesstr)
	fmt.Println(tags)
	fmt.Println(adj)
	fmt.Println(sums)
}

func main() {
	var glob = flag.String("glob", "./*md", "search for files with this glob pattern. stdin if present will override.")
	var query = flag.String("query", "", "search for files with the given tag(s). "+
		"This option may be passed implicitly as the first arg.")
	var date = flag.String("date", "", "search for files matching a date given in ISO 8601: "+
		"YYYY.MM.DD. May be a single date, or a range: YYYY.MM.DD-YYYY.MM.DD.")
	var invert = flag.Bool("invert", false, "whether to invert the tag matching.")
	var verbose = flag.Bool("verbose", false, "whether to print out a verbose summary")

	// take first positional arg as --query arg without the flag.
	// this solution allows trailing flags after the first positional arg.
	// NOTE: we ignore the case len(os.Args) <= 1 because we catch it in ParseQuery. Messy.
	// HACK: modifies os.Args before flag.Parse() :
	if len(os.Args) >= 2 && !strings.HasPrefix(os.Args[1], "-") {
		// horrible slice interpolation expression:
		os.Args = append(os.Args[:1], append([]string{"--query"}, os.Args[1:]...)...)
	}
	flag.Parse()

	queries := ParseQuery(*query)
	filelist := Filelist(*glob)
	entries := Entries(filelist)

	// we shrink the entries list immediately if we want a date range:
	if *date != "" {
		entries = Date(entries, *date)
	}
	tagmap := Tagmap(entries)

	// ProcessQueries must precede Invert because we want Invert to respect combined tags:
	files := ProcessQueries(tagmap, queries)
	if *invert {
		files = Invert(entries, files)
	}
	// NOTE: the full Adjacencies map may one day be useful on its own
	adjacencies := ReduceAdjacencies(Adjacencies(entries, files), queries, *invert)

	Print(entries, tagmap, files, adjacencies, queries, *verbose)
}
