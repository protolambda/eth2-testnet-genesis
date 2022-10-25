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
	fmt.Printf("version %s-%s-%s\n", eth2.VERSION, GitBranch, GitCommit[:6])
	return nil
}
