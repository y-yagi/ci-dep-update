package main

import (
	"encoding/hex"

	"github.com/BurntSushi/toml"
	"github.com/golang/dep"
	"github.com/golang/dep/gps"
	"github.com/pkg/errors"
)

type rawLock struct {
	SolveMeta solveMeta          `toml:"solve-meta"`
	Projects  []rawLockedProject `toml:"projects"`
}

type solveMeta struct {
	InputsDigest    string `toml:"inputs-digest"`
	AnalyzerName    string `toml:"analyzer-name"`
	AnalyzerVersion int    `toml:"analyzer-version"`
	SolverName      string `toml:"solver-name"`
	SolverVersion   int    `toml:"solver-version"`
}

type rawLockedProject struct {
	Name     string   `toml:"name"`
	Branch   string   `toml:"branch,omitempty"`
	Revision string   `toml:"revision"`
	Version  string   `toml:"version,omitempty"`
	Source   string   `toml:"source,omitempty"`
	Packages []string `toml:"packages"`
}

func readLock(filename string) (*dep.Lock, error) {
	var rLock rawLock
	_, err := toml.DecodeFile(filename, &rLock)
	if err != nil {
		return nil, err
	}

	lock, err := fromRawLock(rLock)
	if err != nil {
		panic(err)
	}
	return lock, nil
}

func fromRawLock(raw rawLock) (*dep.Lock, error) {
	var err error
	l := &dep.Lock{
		P: make([]gps.LockedProject, len(raw.Projects)),
	}

	l.SolveMeta.InputsDigest, err = hex.DecodeString(raw.SolveMeta.InputsDigest)
	if err != nil {
		return nil, errors.Errorf("invalid hash digest in lock's memo field")
	}

	l.SolveMeta.AnalyzerName = raw.SolveMeta.AnalyzerName
	l.SolveMeta.AnalyzerVersion = raw.SolveMeta.AnalyzerVersion
	l.SolveMeta.SolverName = raw.SolveMeta.SolverName
	l.SolveMeta.SolverVersion = raw.SolveMeta.SolverVersion

	for i, ld := range raw.Projects {
		r := gps.Revision(ld.Revision)

		var v gps.Version = r
		if ld.Version != "" {
			if ld.Branch != "" {
				return nil, errors.Errorf("lock file specified both a branch (%s) and version (%s) for %s", ld.Branch, ld.Version, ld.Name)
			}
			v = gps.NewVersion(ld.Version).Pair(r)
		} else if ld.Branch != "" {
			v = gps.NewBranch(ld.Branch).Pair(r)
		} else if r == "" {
			return nil, errors.Errorf("lock file has entry for %s, but specifies no branch or version", ld.Name)
		}

		id := gps.ProjectIdentifier{
			ProjectRoot: gps.ProjectRoot(ld.Name),
			Source:      ld.Source,
		}
		l.P[i] = gps.NewLockedProject(id, v, ld.Packages)
	}

	return l, nil
}
