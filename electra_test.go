package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/protolambda/zrnt/eth2/beacon/electra"
	"github.com/protolambda/ztyp/codec"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
)

func TestElectra(t *testing.T) {
	elGenesis := core.Genesis{
		Config: &params.ChainConfig{
			ChainID:                 big.NewInt(123),
			HomesteadBlock:          big.NewInt(0),
			DAOForkBlock:            nil,
			DAOForkSupport:          false,
			EIP150Block:             big.NewInt(0),
			EIP155Block:             big.NewInt(0),
			EIP158Block:             big.NewInt(0),
			ByzantiumBlock:          big.NewInt(0),
			ConstantinopleBlock:     big.NewInt(0),
			PetersburgBlock:         big.NewInt(0),
			IstanbulBlock:           big.NewInt(0),
			MuirGlacierBlock:        big.NewInt(0),
			BerlinBlock:             big.NewInt(0),
			LondonBlock:             big.NewInt(0),
			ArrowGlacierBlock:       big.NewInt(0),
			GrayGlacierBlock:        big.NewInt(0),
			MergeNetsplitBlock:      big.NewInt(0),
			ShanghaiTime:            new(uint64),
			CancunTime:              new(uint64),
			PragueTime:              new(uint64),
			OsakaTime:               nil,
			VerkleTime:              nil,
			TerminalTotalDifficulty: big.NewInt(0),
			DepositContractAddress:  gethcommon.Address{},
			EnableVerkleAtGenesis:   false,
			Ethash:                  nil,
			Clique:                  nil,
			BlobScheduleConfig: &params.BlobScheduleConfig{
				Cancun: params.DefaultCancunBlobConfig,
				Prague: params.DefaultPragueBlobConfig,
				Verkle: nil,
			},
		},
		Nonce:      0,
		Timestamp:  1740340649,
		ExtraData:  nil,
		GasLimit:   36_000_000,
		Difficulty: big.NewInt(0),
		Mixhash:    gethcommon.Hash{},
		Coinbase:   gethcommon.Address{},
		Alloc: types.GenesisAlloc{
			gethcommon.HexToAddress("0x0"): types.Account{
				Code:    nil,
				Storage: nil,
				Balance: new(big.Int).Mul(big.NewInt(1000e9), big.NewInt(1e9)),
				Nonce:   1,
			},
		},
		Number:        0,
		GasUsed:       0,
		ParentHash:    gethcommon.Hash{},
		BaseFee:       big.NewInt(7),
		ExcessBlobGas: new(uint64),
		BlobGasUsed:   new(uint64),
	}
	elGenesisData, err := json.Marshal(elGenesis)
	if err != nil {
		t.Fatal(err)
	}
	testResourceDir := t.TempDir()
	elGenesisPath := filepath.Join(testResourceDir, "genesis.json")
	if err := os.WriteFile(elGenesisPath, elGenesisData, 0755); err != nil {
		t.Fatal(err)
	}
	mnemonicsPath := filepath.Join(testResourceDir, "mnemonics.yaml")
	mnemonicsData := []byte(`
- mnemonic: "test test test test test test test test test test test junk"
  count: 1000
`)
	if err := os.WriteFile(mnemonicsPath, mnemonicsData, 0755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(testResourceDir, "out.ssz")
	tranchesPath := filepath.Join(testResourceDir, "tranches")
	c := &ElectraGenesisCmd{
		SpecOptions: configs.SpecOptions{
			Config:          "minimal",
			Phase0Preset:    "minimal",
			AltairPreset:    "minimal",
			BellatrixPreset: "minimal",
			CapellaPreset:   "minimal",
			DenebPreset:     "minimal",
			ElectraPreset:   "minimal",
		},
		Eth1Config:            elGenesisPath,
		Eth1BlockHash:         common.Root{},
		Eth1BlockTimestamp:    0,
		EthMatchGenesisTime:   true,
		MnemonicsSrcFilePath:  mnemonicsPath,
		ValidatorsSrcFilePath: "",
		StateOutputPath:       outPath,
		TranchesDir:           tranchesPath,
		EthWithdrawalAddress:  common.Eth1Address{},
		ShadowForkEth1RPC:     "",
		ShadowForkBlockFile:   "",
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	stateData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := c.SpecOptions.Spec()
	if err != nil {
		t.Fatal(err)
	}
	dec := codec.NewDecodingReader(bytes.NewReader(stateData), uint64(len(stateData)))
	var state electra.BeaconState
	if err := state.Deserialize(spec, dec); err != nil {
		t.Fatal(err)
	}
	payloadHash := gethcommon.Hash(state.LatestExecutionPayloadHeader.BlockHash)
	elGenesisBlock := elGenesis.ToBlock()
	elHash := elGenesisBlock.Hash()
	if elHash != payloadHash {
		t.Fatalf("el hash mismatch:\n%s <- from SSZ state\n%s <- from EL genesis", payloadHash, elHash)
	}
	t.Logf("successfully created genesis beacon state, with block hash %s", elHash)
}
