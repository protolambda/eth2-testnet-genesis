package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2"
	"github.com/protolambda/zrnt/eth2/beacon/capella"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

type CapellaGenesisCmd struct {
	configs.SpecOptions `ask:"."`
	Eth1Config          string `ask:"--eth1-config" help:"Path to config JSON for eth1. No transition yet if empty."`

	Eth1BlockHash      common.Root      `ask:"--eth1-block" help:"If not transitioned: Eth1 block hash to put into state."`
	Eth1BlockTimestamp common.Timestamp `ask:"--eth1-timestamp" help:"If not transitioned: Eth1 block timestamp"`

	EthMatchGenesisTime bool `ask:"--eth1-match-genesis-time" help:"Use execution-layer genesis time as beacon genesis time. Overrides other genesis time settings."`

	MnemonicsSrcFilePath  string `ask:"--mnemonics" help:"File with YAML of key sources"`
	ValidatorsSrcFilePath string `ask:"--additional-validators" help:"File with list of additional validators"`
	StateOutputPath       string `ask:"--state-output" help:"Output path for state file"`
	TranchesDir           string `ask:"--tranches-dir" help:"Directory to dump lists of pubkeys of each tranche in"`

	EthWithdrawalAddress common.Eth1Address `ask:"--eth1-withdrawal-address" help:"Eth1 Withdrawal to set for the genesis validator set"`
	ShadowForkEth1RPC    string             `ask:"--shadow-fork-eth1-rpc" help:"Fetch the Eth1 block from the eth1 node for the shadow fork"`
}

func (g *CapellaGenesisCmd) Help() string {
	return "Create genesis state for Capella beacon chain, from execution-layer and consensus-layer configs"
}

func (g *CapellaGenesisCmd) Default() {
	g.SpecOptions.Default()
	g.Eth1Config = "engine_genesis.json"

	g.Eth1BlockHash = common.Root{}
	g.Eth1BlockTimestamp = common.Timestamp(time.Now().Unix())

	g.MnemonicsSrcFilePath = "mnemonics.yaml"
	g.ValidatorsSrcFilePath = ""
	g.StateOutputPath = "genesis.ssz"
	g.TranchesDir = "tranches"
	g.ShadowForkEth1RPC = ""
}

func (g *CapellaGenesisCmd) Run(ctx context.Context, args ...string) error {
	fmt.Printf("zrnt version: %s\n", eth2.VERSION)

	spec, err := g.SpecOptions.Spec()
	if err != nil {
		return err
	}

	var eth1BlockHash common.Root
	var beaconGenesisTimestamp common.Timestamp
	var execHeader *capella.ExecutionPayloadHeader
	var eth1Block *types.Block
	var prevRandaoMix [32]byte
	var txsRoot common.Root
	var eth1Genesis *core.Genesis

	// Load the Eth1 block from the Eth1 genesis config
	if g.Eth1Config != "" {
		eth1Genesis, err = loadEth1GenesisConf(g.Eth1Config)
		if err != nil {
			return err
		}
	}

	if g.EthMatchGenesisTime && eth1Genesis != nil {
		beaconGenesisTimestamp = common.Timestamp(eth1Genesis.Timestamp)
	} else if spec.MIN_GENESIS_TIME != 0 {
		// Load the genesis timestamp from the CL config, this is better in terms of compatibility for shadowforks
		fmt.Println("Using CL MIN_GENESIS_TIME for genesis timestamp")

		// Set beaconchain genesis timestamp based on config genesis timestamp
		beaconGenesisTimestamp = spec.MIN_GENESIS_TIME
	} else {
		beaconGenesisTimestamp = g.Eth1BlockTimestamp
	}

	if g.ShadowForkEth1RPC != "" {
		client, err := ethclient.Dial(g.ShadowForkEth1RPC)
		if err != nil {
			return fmt.Errorf("A fatal error occurred creating the ETH client %s", err)
		}

		// Get the latest block
		blockNumberUint64, err := client.BlockNumber(ctx)
		blockNumberBigint := new(big.Int).SetUint64(blockNumberUint64)
		resultBlock, err := client.BlockByNumber(ctx, blockNumberBigint)
		if err != nil {
			return fmt.Errorf("A fatal error occurred getting the ETH block %s", err)
		}

		// Set the eth1Block value for use later
		eth1Block = resultBlock

		// Convert and set the difficulty as the prevRandao field
		prevRandaoMix = bigIntToBytes32(eth1Block.Difficulty())

		// Compute the SSZ hash-tree-root of the transactions,
		// since that is what we put as transactions_root in the CL execution-payload.
		// Not to be confused with the legacy MPT root in the EL block header.
		clTransactions := make(common.PayloadTransactions, len(eth1Block.Transactions()))
		for i, tx := range eth1Block.Transactions() {
			opaqueTx, err := tx.MarshalBinary()
			if err != nil {
				return fmt.Errorf("failed to encode tx %d: %w", i, err)
			}
			clTransactions[i] = opaqueTx
		}
		txsRoot = clTransactions.HashTreeRoot(spec, tree.GetHashFn())
	} else if g.Eth1Config != "" {

		// Generate genesis block from the loaded config
		eth1Block = eth1Genesis.ToBlock()

		// Set as default values
		prevRandaoMix = common.Bytes32{}
		txsRoot = common.PayloadTransactionsType(spec).DefaultNode().MerkleRoot(tree.GetHashFn())

	} else {
		fmt.Println("no eth1 config found, using eth1 block hash and timestamp, with empty ExecutionPayloadHeader (no PoW->PoS transition yet in execution layer)")
		eth1BlockHash = g.Eth1BlockHash
		execHeader = &capella.ExecutionPayloadHeader{}
	}

	eth1BlockHash = common.Root(eth1Block.Hash())

	extra := eth1Block.Extra()
	if len(extra) > common.MAX_EXTRA_DATA_BYTES {
		return fmt.Errorf("extra data is %d bytes, max is %d", len(extra), common.MAX_EXTRA_DATA_BYTES)
	}

	baseFee, _ := uint256.FromBig(eth1Block.BaseFee())

	var withdrawalsRoot common.Root
	if eth1Block.Withdrawals() != nil {
		// Compute the SSZ hash-tree-root of the withdrawals,
		// since that is what we put as withdrawals_root in the CL execution-payload.
		// Not to be confused with the legacy MPT root in the EL block header.
		clWithdrawals := make(common.Withdrawals, len(eth1Block.Withdrawals()))
		for i, withdrawal := range eth1Block.Withdrawals() {
			clWithdrawals[i] = common.Withdrawal{
				Index:          common.WithdrawalIndex(withdrawal.Index),
				ValidatorIndex: common.ValidatorIndex(withdrawal.Validator),
				Address:        common.Eth1Address(withdrawal.Address),
				Amount:         common.Gwei(withdrawal.Amount),
			}
		}
		withdrawalsRoot = clWithdrawals.HashTreeRoot(spec, tree.GetHashFn())
	}

	execHeader = &capella.ExecutionPayloadHeader{
		ParentHash:       common.Root(eth1Block.ParentHash()),
		FeeRecipient:     common.Eth1Address(eth1Block.Coinbase()),
		StateRoot:        common.Bytes32(eth1Block.Root()),
		ReceiptsRoot:     common.Bytes32(eth1Block.ReceiptHash()),
		LogsBloom:        common.LogsBloom(eth1Block.Bloom()),
		PrevRandao:       prevRandaoMix,
		BlockNumber:      view.Uint64View(eth1Block.NumberU64()),
		GasLimit:         view.Uint64View(eth1Block.GasLimit()),
		GasUsed:          view.Uint64View(eth1Block.GasUsed()),
		Timestamp:        common.Timestamp(eth1Block.Time()),
		ExtraData:        extra,
		BaseFeePerGas:    view.Uint256View(*baseFee),
		BlockHash:        eth1BlockHash,
		WithdrawalsRoot:  withdrawalsRoot,
		TransactionsRoot: txsRoot,
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

	state := capella.NewBeaconStateView(spec)
	if err := setupState(spec, state, beaconGenesisTimestamp, eth1BlockHash, validators); err != nil {
		return err
	}

	if err := state.SetLatestExecutionPayloadHeader(execHeader); err != nil {
		return err
	}

	t, err := state.GenesisTime()
	if err != nil {
		return err
	}
	fmt.Printf("eth2 genesis at %d + %d = %d  (%s)\n", beaconGenesisTimestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

	historicalRoots, err := state.HistoricalRoots()
	if err != nil {
		return err
	}

	newHistoricalRootBytes, _ := hex.DecodeString("4242424242424242424242424242424242424242424242424242424242424242")
	newHistoricalRoot := common.Root{}
	copy(newHistoricalRoot[:], newHistoricalRootBytes)
	err = historicalRoots.Append(newHistoricalRoot)
	if err != nil {
		return err
	}

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
