package main

import (
	"context"
	"fmt"
	hbls "github.com/herumi/bls-eth-go-binary/bls"
	"github.com/protolambda/ask"
	"os"
)

func init() {
	hbls.Init(hbls.BLS12_381)
	hbls.SetETHmode(hbls.EthModeLatest)
}

type GenesisCmd struct{}

func (c *GenesisCmd) Help() string {
	return "Create genesis state. See sub-commands for different fork versions."
}

func (c *GenesisCmd) Cmd(route string) (cmd interface{}, err error) {
	switch route {
	case "phase0":
		cmd = &Phase0GenesisCmd{}
	case "altair":
		cmd = &AltairGenesisCmd{}
	case "merge":
		cmd = &MergeGenesisCmd{}
	default:
		return nil, fmt.Errorf("unrecognized cmd route: %s", route)
	}
	return
}

func (c *GenesisCmd) Routes() []string {
	return []string{"phase0", "altair", "merge"}
}

func main() {
	cmd := &GenesisCmd{}
	descr, err := ask.Load(cmd)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load main command: %v", err.Error())
		os.Exit(1)
	}
	if cmd, isHelp, err := descr.Execute(context.Background(), os.Args[1:]...); err != nil && err != ask.UnrecognizedErr {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	} else if cmd == nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to load command")
		os.Exit(1)
	} else if isHelp || (err == ask.UnrecognizedErr) {
		_, _ = fmt.Fprintln(os.Stdout, cmd.Usage())
		os.Exit(0)
	}
}
