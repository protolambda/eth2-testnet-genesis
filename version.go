package main

import (
	"context"
	"fmt"
	"github.com/protolambda/zrnt/eth2"
)

var GitCommit, GitBranch string

type VersionCmd struct {
}

func (g *VersionCmd) Help() string {
	return "Print version and exit"
}

func (g *VersionCmd) Default() {}

func (g *VersionCmd) Run(ctx context.Context, args ...string) error {
	var versionstr string
	versionstr = eth2.VERSION
	if len(GitBranch) > 0 {
		versionstr += "-" + GitBranch
	}
	if len(GitCommit) > 6 {
		versionstr += "-" + GitCommit[:6]
	}
	fmt.Printf("%s\n", versionstr)
	return nil
}
