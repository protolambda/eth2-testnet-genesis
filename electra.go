package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/protolambda/zrnt/eth2"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/deneb"
	"github.com/protolambda/zrnt/eth2/beacon/electra"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

type ElectraGenesisCmd struct {
	configs.SpecOptions `ask:"."`
	Eth1Config          string `ask:"--eth1-config" help:"Path to config JSON for eth1. No transition yet if empty."`

	Eth1BlockHash      common.Root      `ask:"--eth1-block" help:"If not transitioned: Eth1 block hash to put into state."`
	Eth1BlockTimestamp common.Timestamp `ask:"--timestamp" help:"Eth1 block timestamp"`

	EthMatchGenesisTime bool `ask:"--eth1-match-genesis-time" help:"Use execution-layer genesis time as beacon genesis time. Overrides other genesis time settings."`

	MnemonicsSrcFilePath  string `ask:"--mnemonics" help:"File with YAML of key sources"`
	ValidatorsSrcFilePath string `ask:"--additional-validators" help:"File with list of additional validators"`
	StateOutputPath       string `ask:"--state-output" help:"Output path for state file"`
	TranchesDir           string `ask:"--tranches-dir" help:"Directory to dump lists of pubkeys of each tranche in"`

	EthWithdrawalAddress common.Eth1Address `ask:"--eth1-withdrawal-address" help:"Eth1 Withdrawal to set for the genesis validator set"`
	ShadowForkEth1RPC    string             `ask:"--shadow-fork-eth1-rpc" help:"Fetch the Eth1 block from the eth1 node for the shadow fork"`
	ShadowForkBlockFile  string             `ask:"--shadow-fork-block-file" help:"Fetch the Eth1 block from a file for the shadow fork(overwrites RPC option)"`
}

func (g *ElectraGenesisCmd) Help() string {
	return "Create genesis state for Electra beacon chain, from execution-layer and consensus-layer configs"
}

func (g *ElectraGenesisCmd) Default() {
	g.SpecOptions.Default()
	g.Eth1Config = "engine_genesis.json"

	g.Eth1BlockHash = common.Root{}
	g.Eth1BlockTimestamp = common.Timestamp(time.Now().Unix())

	g.MnemonicsSrcFilePath = "mnemonics.yaml"
	g.ValidatorsSrcFilePath = ""
	g.StateOutputPath = "genesis.ssz"
	g.TranchesDir = "tranches"
	g.ShadowForkEth1RPC = ""
	g.ShadowForkBlockFile = ""
}

func (g *ElectraGenesisCmd) Run(ctx context.Context, args ...string) error {
	fmt.Printf("zrnt version: %s\n", eth2.VERSION)

	spec, err := g.SpecOptions.Spec()
	if err != nil {
		return err
	}

	var eth1BlockHash common.Root
	var beaconGenesisTimestamp common.Timestamp
	var execHeader *deneb.ExecutionPayloadHeader
	var eth1Block *types.Block
	var prevRandaoMix [32]byte
	var txsRoot common.Root = common.PayloadTransactionsType(spec).DefaultNode().MerkleRoot(tree.GetHashFn())
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

	if g.ShadowForkBlockFile != "" {
		// Read the JSON file from disk
		file, err := os.ReadFile(g.ShadowForkBlockFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Unmarshal the JSON into a types.Block object
		var resultData JSONData
		err = json.Unmarshal(file, &resultData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}

		// Set the eth1Block value for use later
		eth1Block, err = ParseEthBlock(resultData.Result)
		if err != nil {
			return fmt.Errorf("failed to parse eth1 block: %w", err)
		}

		// Convert and set the difficulty as the prevRandao field
		prevRandaoMix = bigIntToBytes32(eth1Block.Difficulty())

	} else if g.ShadowForkEth1RPC != "" {
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

	} else if g.Eth1Config != "" {

		// Generate genesis block from the loaded config
		eth1Block = eth1Genesis.ToBlock()

		// Set as default values
		prevRandaoMix = common.Bytes32{}

	} else {
		fmt.Println("no eth1 config found, using eth1 block hash and timestamp, with empty ExecutionPayloadHeader (no PoW->PoS transition yet in execution layer)")
		eth1BlockHash = g.Eth1BlockHash
		execHeader = &deneb.ExecutionPayloadHeader{}
	}

	// Sanity-check the new requests-hash
	if reqHash := eth1Block.RequestsHash(); reqHash == nil {
		return errors.New("pectra execution-layer block has missing requests-hash attribute")
	} else if *reqHash != types.EmptyRequestsHash {
		return fmt.Errorf("expected requests-hash of empty requests-list in genesis block (%s) but got %s instead", types.EmptyRequestsHash, *reqHash)
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

	if len(eth1Block.Transactions()) > 0 {
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
	}

	if eth1Block.BlobGasUsed() == nil {
		return errors.New("execution-layer Block has missing blob-gas-used field")
	}
	if eth1Block.ExcessBlobGas() == nil {
		return errors.New("execution-layer Block has missing excess-blob-gas field")
	}

	// Electra has the same deneb execution-payload-header format,
	// the new requests-root is not part of the beacon ExecutionPayloadHeader type
	execHeader = &deneb.ExecutionPayloadHeader{
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
		TransactionsRoot: txsRoot,
		WithdrawalsRoot:  withdrawalsRoot,
		BlobGasUsed:      view.Uint64View(*eth1Block.BlobGasUsed()),
		ExcessBlobGas:    view.Uint64View(*eth1Block.ExcessBlobGas()),
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

	state := electra.NewBeaconStateView(spec)
	if err := setupState(spec, state, beaconGenesisTimestamp, eth1BlockHash, validators); err != nil {
		return err
	}

	// To compute epochs: like the deneb-to-electra fork logic.
	currentEpoch := common.Epoch(0)
	earliestExitEpoch := spec.ComputeActivationExitEpoch(currentEpoch)
	// we assume no validators with exit epoch, so no earliestExitEpoch change
	earliestExitEpoch += 1 // in the fork upgrade spec we add 1, so we do that here too...
	fmt.Printf("earliest exit epoch: %d\n", earliestExitEpoch)

	earliestConsolidationEpoch := spec.ComputeActivationExitEpoch(currentEpoch)
	fmt.Printf("earliest consolidation epoch: %d\n", earliestConsolidationEpoch)

	// To compute the balances:
	//
	// EFFECTIVE_BALANCE_INCREMENT = 1_000_000_000 # in gwei, aka 1 eth
	// CHURN_LIMIT_QUOTIENT = 2**16
	// MIN_PER_EPOCH_CHURN_LIMIT_ELECTRA = 2**7 * 10**9
	// MAX_PER_EPOCH_ACTIVATION_EXIT_CHURN_LIMIT = 2**8 * 10**9
	//
	// def compute_balance_vals(validator_count: int):
	//     def get_total_active_balance() -> int:
	//         return validator_count * 32 * EFFECTIVE_BALANCE_INCREMENT
	//
	//     def get_balance_churn_limit() -> int:
	//        churn = max(
	//            MIN_PER_EPOCH_CHURN_LIMIT_ELECTRA,
	//            get_total_active_balance() // CHURN_LIMIT_QUOTIENT
	//        )
	//        return churn - churn % EFFECTIVE_BALANCE_INCREMENT
	//
	//     def get_activation_exit_churn_limit() -> int:
	//         return min(MAX_PER_EPOCH_ACTIVATION_EXIT_CHURN_LIMIT, get_balance_churn_limit())
	//
	//     def get_consolidation_churn_limit() -> int:
	//        return get_balance_churn_limit() - get_activation_exit_churn_limit()
	//     # In fork spec:
	//     exit_balance_to_consume = get_activation_exit_churn_limit()
	//     consolidation_balance_to_consume = get_consolidation_churn_limit()
	//     print("exit_balance_to_consume", exit_balance_to_consume)
	//     print("consolidation_balance_to_consume", consolidation_balance_to_consume)
	//
	// At <264k and less validators it doesn't matter.
	exitBalanceToConsume := common.Gwei(128000000000)
	consolidationBalanceToConsume := common.Gwei(0)
	if err := state.SetEarliestExitEpoch(earliestExitEpoch); err != nil {
		return err
	}
	if err := state.SetEarliestConsolidationEpoch(earliestConsolidationEpoch); err != nil {
		return err
	}
	if err := state.SetExitBalanceToConsume(exitBalanceToConsume); err != nil {
		return err
	}
	if err := state.SetConsolidationBalanceToConsume(consolidationBalanceToConsume); err != nil {
		return err
	}

	if err := state.SetLatestExecutionPayloadHeader(execHeader); err != nil {
		return err
	}

	t, err := state.GenesisTime()
	if err != nil {
		return err
	}
	fmt.Printf("genesis at %d + %d = %d  (%s)\n", beaconGenesisTimestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

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
