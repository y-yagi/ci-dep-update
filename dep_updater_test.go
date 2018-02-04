package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/dep/gps"
)

func TestGeneratePullRequestBody(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(wd, "testdata", "changed", "before", "Gopkg.lock")
	before, err := readLock(file)
	if err != nil {
		t.Fatal(err)
	}

	file = filepath.Join(wd, "testdata", "changed", "after", "Gopkg.lock")
	after, err := readLock(file)
	if err != nil {
		t.Fatal(err)
	}
	diff := gps.DiffLocks(before, after)
	updater := NewDepUpdater(nil)

	got := updater.generatePullRequestBody(diff)
	want := `**Changed:**

* [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) [v1.4.0...v1.6.0](https://github.com/mattn/go-sqlite3/compare/v1.4.0...v1.6.0)
* [github.com/olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) [65fec0d...96aac99](https://github.com/olekukonko/tablewriter/compare/65fec0d...96aac99)
* [github.com/y-yagi/goext](https://github.com/y-yagi/goext) [0c56270...fd0b1e8](https://github.com/y-yagi/goext/compare/0c56270...fd0b1e8)
`

	if got != want {
		t.Fatalf("want\n%v\nbut got \n\n%v", want, got)
	}
}
