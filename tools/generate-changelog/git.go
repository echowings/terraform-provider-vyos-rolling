package main

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/go-version"
	"github.com/leodido/go-conventionalcommits"
	"github.com/leodido/go-conventionalcommits/parser"
)

func GenerateGitChanges() (previousVersion *version.Version, commitsSinceLastVersion []conventionalcommits.Message) {
	repo, err := git.PlainOpen("../..")
	die(err)

	tags, err := repo.Tags()
	die(err)

	latestVersion, err := version.NewVersion("0.0.0")
	die(err)
	var latestVersionRef *plumbing.Reference
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		v, err := version.NewVersion(ref.Name().Short())
		if err != nil {
			return nil
		}
		if v.GreaterThan(latestVersion) {
			latestVersion = v
			latestVersionRef = ref
		}
		return nil
	})
	die(err)
	previousVersion = latestVersion

	var logOptions git.LogOptions
	logOptions.Order = git.LogOrderCommitterTime

	if latestVersionRef != nil {
		releaseCommit, err := repo.CommitObject(latestVersionRef.Hash())
		die(err)
		logOptions.Since = &releaseCommit.Author.When
	}

	commits, err := repo.Log(&logOptions)
	die(err)

	chgs := make([]conventionalcommits.Message, 0)
	ccm := parser.NewMachine(parser.WithTypes(conventionalcommits.TypesConventional), parser.WithBestEffort())

	err = commits.ForEach(func(c *object.Commit) error {
		if c == nil {
			return nil
		}
		if latestVersionRef != nil && c.Hash == latestVersionRef.Hash() {
			return nil
		}

		cc, err := ccm.Parse([]byte(c.Message))
		// WithBestEffort still returns errors for malformed commits.
		// It also can return a nil error and a nil message for non-conventional commits.
		if err != nil || cc == nil {
			return nil
		}

		// Skip commits that are not conventional, which WithBestEffort parses into an empty struct
		if cc.Type == "" && cc.Description == "" {
			return nil
		}

		chgs = append(chgs, cc)
		return nil
	})
	die(err)

	return previousVersion, chgs
}