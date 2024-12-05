package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/protolambda/zrnt/eth2"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
)

type Phase0GenesisCmd struct {
	configs.SpecOptions   `ask:"."`
	Eth1BlockHash         common.Root      `ask:"--eth1-block" help:"Eth1 block hash to put into state"`
	Eth1BlockTimestamp    common.Timestamp `ask:"--timestamp" help:"Eth1 block timestamp"`
	EthMatchGenesisTime   bool             `ask:"--eth1-match-genesis-time" help:"Use execution-layer genesis time as beacon genesis time. Overrides other genesis time settings."`
	Eth1Config            string           `ask:"--eth1-config" help:"Path to config JSON for eth1. No transition yet if empty."`
	MnemonicsSrcFilePath  string           `ask:"--mnemonics" help:"File with YAML of key sources"`
	ValidatorsSrcFilePath string           `ask:"--additional-validators" help:"File with list of validators"`
	StateOutputPath       string           `ask:"--state-output" help:"Output path for state file"`
	TranchesDir           string           `ask:"--tranches-dir" help:"Directory to dump lists of pubkeys of each tranche in"`

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
	g.ValidatorsSrcFilePath = ""
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

	validators, err := loadValidatorKeys(spec, g.MnemonicsSrcFilePath, g.ValidatorsSrcFilePath, g.TranchesDir, g.EthWithdrawalAddress)
	if err != nil {
		return err
	}

	if uint64(len(validators)) < uint64(spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT) {
		fmt.Printf("WARNING: not enough validators for genesis. Key sources sum up to %d total. But need %d.\n", len(validators), spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT)
	}

	state := phase0.NewBeaconStateView(spec)

	var beaconGenesisTimestamp common.Timestamp
	var eth1Genesis *core.Genesis

	if g.Eth1Config != "" {
		eth1Genesis, err = loadEth1GenesisConf(g.Eth1Config)
		if err != nil {
			return err
		}
	}

	if g.EthMatchGenesisTime && eth1Genesis != nil {
		beaconGenesisTimestamp = common.Timestamp(eth1Genesis.Timestamp)
	} else if spec.MIN_GENESIS_TIME != 0 {
		fmt.Println("Using CL MIN_GENESIS_TIME for genesis timestamp")
		beaconGenesisTimestamp = spec.MIN_GENESIS_TIME
	} else {
		beaconGenesisTimestamp = g.Eth1BlockTimestamp
	}

	if err := setupState(spec, state, beaconGenesisTimestamp, g.Eth1BlockHash, validators); err != nil {
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
