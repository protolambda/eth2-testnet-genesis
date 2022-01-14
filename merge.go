package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/merge"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
	"os"
	"time"
)

type MergeGenesisCmd struct {
	configs.SpecOptions `ask:"."`
	Eth1Config          string `ask:"--eth1-config" help:"Path to config JSON for eth1. No transition yet if empty."`

	Eth1BlockHash      common.Root      `ask:"--eth1-block" help:"If not transitioned: Eth1 block hash to put into state."`
	Eth1BlockTimestamp common.Timestamp `ask:"--eth1-timestamp" help:"If not transitioned: Eth1 block timestamp"`

	MnemonicsSrcFilePath string `ask:"--mnemonics" help:"File with YAML of key sources"`
	StateOutputPath      string `ask:"--state-output" help:"Output path for state file"`
	TranchesDir          string `ask:"--tranches-dir" help:"Directory to dump lists of pubkeys of each tranche in"`
}

func (g *MergeGenesisCmd) Help() string {
	return "Create genesis state for Merge beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs"
}

func (g *MergeGenesisCmd) Default() {
	g.SpecOptions.Default()
	g.Eth1Config = "engine_genesis.json"

	g.Eth1BlockHash = common.Root{}
	g.Eth1BlockTimestamp = common.Timestamp(time.Now().Unix())

	g.MnemonicsSrcFilePath = "mnemonics.yaml"
	g.StateOutputPath = "genesis.ssz"
	g.TranchesDir = "tranches"
}

func (g *MergeGenesisCmd) Run(ctx context.Context, args ...string) error {
	fmt.Printf("zrnt version: %s\n", eth2.VERSION)

	spec, err := g.SpecOptions.Spec()
	if err != nil {
		return err
	}

	var eth1BlockHash common.Root
	var eth1Timestamp common.Timestamp
	var execHeader *common.ExecutionPayloadHeader
	if g.Eth1Config != "" {
		fmt.Println("using eth1 config to create and embed ExecutionPayloadHeader in genesis BeaconState")
		var eth1Genesis *core.Genesis
		eth1Genesis, err := loadEth1GenesisConf(g.Eth1Config)
		if err != nil {
			return err
		}

		eth1Db := rawdb.NewMemoryDatabase()
		eth1GenesisBlock := eth1Genesis.ToBlock(eth1Db)

		eth1BlockHash = common.Root(eth1GenesisBlock.Hash())
		eth1Timestamp = common.Timestamp(eth1Genesis.Timestamp)

		extra := eth1GenesisBlock.Extra()
		if len(extra) > common.MAX_EXTRA_DATA_BYTES {
			return fmt.Errorf("extra data is %d bytes, max is %d", len(extra), common.MAX_EXTRA_DATA_BYTES)
		}

		baseFee, _ := uint256.FromBig(eth1GenesisBlock.BaseFee())

		execHeader = &common.ExecutionPayloadHeader{
			ParentHash:    common.Root(eth1GenesisBlock.ParentHash()),
			FeeRecipient:  common.Eth1Address(eth1GenesisBlock.Coinbase()),
			StateRoot:     common.Bytes32(eth1GenesisBlock.Root()),
			ReceiptRoot:   common.Bytes32(eth1GenesisBlock.ReceiptHash()),
			LogsBloom:     common.LogsBloom(eth1GenesisBlock.Bloom()),
			Random:        common.Bytes32{},
			BlockNumber:   view.Uint64View(eth1GenesisBlock.NumberU64()),
			GasLimit:      view.Uint64View(eth1GenesisBlock.GasLimit()),
			GasUsed:       view.Uint64View(eth1GenesisBlock.GasUsed()),
			Timestamp:     common.Timestamp(eth1GenesisBlock.Time()),
			ExtraData:     extra,
			BaseFeePerGas: view.Uint256View(*baseFee),
			BlockHash:     eth1BlockHash,
			// empty transactions root
			TransactionsRoot: common.PayloadTransactionsType(spec).DefaultNode().MerkleRoot(tree.GetHashFn()),
		}
	} else {
		fmt.Println("no eth1 config found, using eth1 block hash and timestamp, with empty ExecutionPayloadHeader (no PoW->PoS transition yet in execution layer)")
		eth1BlockHash = g.Eth1BlockHash
		eth1Timestamp = g.Eth1BlockTimestamp
		execHeader = &common.ExecutionPayloadHeader{}
	}

	if err := os.MkdirAll(g.TranchesDir, 0777); err != nil {
		return err
	}

	validators, err := loadValidatorKeys(spec, g.MnemonicsSrcFilePath, g.TranchesDir)
	if err != nil {
		return err
	}

	if uint64(len(validators)) < spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT {
		fmt.Printf("WARNING: not enough validators for genesis. Key sources sum up to %d total. But need %d.\n", len(validators), spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT)
	}

	state := merge.NewBeaconStateView(spec)
	if err := setupState(spec, state, eth1Timestamp, eth1BlockHash, validators); err != nil {
		return err
	}

	if err := state.SetLatestExecutionPayloadHeader(execHeader); err != nil {
		return err
	}

	t, err := state.GenesisTime()
	if err != nil {
		return err
	}
	fmt.Printf("eth2 genesis at %d + %d = %d  (%s)\n", eth1Timestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

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
