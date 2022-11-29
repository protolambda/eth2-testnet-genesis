package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/protolambda/zrnt/eth2"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
)

type Phase0GenesisCmd struct {
	configs.SpecOptions  `ask:"."`
	Eth1BlockHash        common.Root      `ask:"--eth1-block" help:"Eth1 block hash to put into state"`
	Eth1BlockTimestamp   common.Timestamp `ask:"--timestamp" help:"Eth1 block timestamp"`
	MnemonicsSrcFilePath string           `ask:"--mnemonics" help:"File with YAML of key sources"`

	StateOutputPath string `ask:"--state-output" help:"Output path for state file"`
	TranchesDir     string `ask:"--tranches-dir" help:"Directory to dump lists of pubkeys of each tranche in"`

	EthWithdrawalAddress common.Eth1Address `ask:"--eth1-withdrawal-address" help:"Eth1 Withdrawal to set for the genesis validator set"`
}

func (g *Phase0GenesisCmd) Help() string {
	return "Create genesis state for phase0 beacon chain"
}

func (g *Phase0GenesisCmd) Default() {
	g.SpecOptions.Default()
	g.Eth1BlockHash = common.Root{}
	g.Eth1BlockTimestamp = common.Timestamp(time.Now().Unix())
	g.MnemonicsSrcFilePath = "mnemonics.yaml"
	g.StateOutputPath = "genesis.ssz"
	g.TranchesDir = "tranches"
}

func (g *Phase0GenesisCmd) Run(ctx context.Context, args ...string) error {
	fmt.Printf("zrnt version: %s\n", eth2.VERSION)
	spec, err := g.SpecOptions.Spec()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(g.TranchesDir, 0777); err != nil {
		return err
	}

	validators, err := loadValidatorKeys(spec, g.MnemonicsSrcFilePath, g.TranchesDir, g.EthWithdrawalAddress)
	if err != nil {
		return err
	}

	if uint64(len(validators)) < uint64(spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT) {
		fmt.Printf("WARNING: not enough validators for genesis. Key sources sum up to %d total. But need %d.\n", len(validators), spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT)
	}

	state := phase0.NewBeaconStateView(spec)
	if err := setupState(spec, state, g.Eth1BlockTimestamp, g.Eth1BlockHash, validators); err != nil {
		return err
	}

	t, err := state.GenesisTime()
	if err != nil {
		return err
	}
	fmt.Printf("genesis at %d + %d = %d  (%s)\n", g.Eth1BlockTimestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

	fmt.Println("done preparing state, serializing SSZ now...")
	f, err := os.OpenFile(g.StateOutputPath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	defer buf.Flush()
	w := codec.NewEncodingWriter(f)
	if err := state.Serialize(w); err != nil {
		return err
	}
	fmt.Println("done!")
	return nil
}
