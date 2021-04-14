package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	hbls "github.com/herumi/bls-eth-go-binary/bls"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/merge"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/tyler-smith/go-bip39"
	util "github.com/wealdtech/go-eth2-util"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	hbls.Init(hbls.BLS12_381)
	hbls.SetETHmode(hbls.EthModeLatest)
}

func main() {
	configPath := flag.String("config", "mainnet.yaml", "Path to config YAML for eth2")
	eth1BlockHashStr := flag.String("eth1-block", "0x0000000000000000000000000000000000000000000000000000000000000000", "Eth1 block hash to put into state")
	forkName := flag.String("fork-name", "phase0", "Name of the fork to generate a genesis state for. valid values: phase0, altair, merge")
	eth1BlockTimestampPtr := flag.Uint64("eth1-timestamp", uint64(time.Now().Unix()), "Eth1 block timestamp")
	mnemonicsSrcFilePath := flag.String("mnemonics", "mnemonics.yaml", "File with YAML of key sources")
	stateOutputPath := flag.String("state-output", "genesis.ssz", "Output path for state file")
	tranchesDir := flag.String("tranches-dir", "tranches", "")
	flag.Parse()

	check := func(err error) {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	if n := *forkName; n != "phase0" && n != "altair" && n != "merge" {
		check(fmt.Errorf("unknown fork name: %q", n))
	}
	spec, err := loadSpec(*configPath)
	check(err)

	check(os.MkdirAll(*tranchesDir, 0777))

	var eth1Hash common.Root
	check(eth1Hash.UnmarshalText([]byte(*eth1BlockHashStr)))

	mnemonics, err := loadMnemonics(*mnemonicsSrcFilePath)
	check(err)

	var validators []phase0.KickstartValidatorData
	for m, mnemonicSrc := range mnemonics {
		fmt.Printf("processing mnemonic %d, for %d validators\n", m, mnemonicSrc.Count)
		seed, err := seedFromMnemonic(mnemonicSrc.Mnemonic)
		if err != nil {
			check(fmt.Errorf("mnemonic %d is bad", m))
			return
		}
		pubs := make([]string, 0, mnemonicSrc.Count)
		for i := uint64(0); i < mnemonicSrc.Count; i++ {
			if i%100 == 0 {
				fmt.Printf("...validator %d/%d\n", i, mnemonicSrc.Count)
			}
			signingKey, err := util.PrivateKeyFromSeedAndPath(seed, validatorKeyName(i))
			check(err)
			withdrawalKey, err := util.PrivateKeyFromSeedAndPath(seed, withdrawalKeyName(i))
			check(err)

			// BLS signing key
			var data phase0.KickstartValidatorData
			copy(data.Pubkey[:], signingKey.PublicKey().Marshal())
			pubs = append(pubs, data.Pubkey.String())

			// BLS withdrawal credentials
			h := sha256.New()
			h.Write(withdrawalKey.PublicKey().Marshal())
			copy(data.WithdrawalCredentials[:], h.Sum(nil))
			data.WithdrawalCredentials[0] = spec.BLS_WITHDRAWAL_PREFIX[0]

			// Max effective balance by default for activation
			data.Balance = spec.MAX_EFFECTIVE_BALANCE

			validators = append(validators, data)
		}
		fmt.Println("Writing pubkeys list file...")
		check(outputPubkeys(filepath.Join(*tranchesDir, fmt.Sprintf("tranche_%04d.txt", m)), pubs))
	}
	if uint64(len(validators)) < spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT {
		fmt.Printf("WARNING: not enough validators for genesis. Key sources sum up to %d total. But need %d.\n", len(validators), spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT)
	}

	var state interface{
		common.BeaconState
		Serialize(w *codec.EncodingWriter) error
	}
	switch *forkName {
	case "phase0":
		state = phase0.NewBeaconStateView(spec)
	case "altair":
		state = altair.NewBeaconStateView(spec)
	case "merge":
		state = merge.NewBeaconStateView(spec)
	}
	eth1BlockTimestamp := common.Timestamp(*eth1BlockTimestampPtr)
	check(setupState(spec, state, eth1BlockTimestamp, eth1Hash, validators))

	if *forkName == "merge" {
		ms := state.(*merge.BeaconStateView)
		// TODO: load eth1 genesis block and use that to fill in header details
		check(ms.SetLatestExecutionPayloadHeader(&merge.ExecutionPayloadHeader{
			BlockHash:        eth1Hash,
			ParentHash:       merge.Hash32{},
			CoinBase:         common.Eth1Address{},
			StateRoot:        merge.Bytes32{},
			Number:           0,
			GasLimit:         0,
			Timestamp:        eth1BlockTimestamp,
			ReceiptRoot:      merge.Bytes32{},
			LogsBloom:        merge.LogsBloom{},
			// empty transactions root
			TransactionsRoot: merge.PayloadTransactionsType.DefaultNode().MerkleRoot(tree.GetHashFn()),
		}))
	}

	t, err := state.GenesisTime()
	check(err)
	fmt.Printf("genesis at %d + %d = %d  (%s)\n", eth1BlockTimestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

	fmt.Println("done preparing state, serializing SSZ now...")
	f, err := os.OpenFile(*stateOutputPath, os.O_CREATE|os.O_WRONLY, 0777)
	check(err)
	defer f.Close()
	buf := bufio.NewWriter(f)
	defer buf.Flush()
	w := codec.NewEncodingWriter(f)
	check(state.Serialize(w))
	fmt.Println("done!")
}

func setupState(spec *common.Spec, state common.BeaconState, eth1Time common.Timestamp,
	eth1BlockHash common.Root, validators []phase0.KickstartValidatorData) error {

	if err := state.SetGenesisTime(eth1Time + spec.GENESIS_DELAY); err != nil {
		return err
	}
	if err := state.SetFork(common.Fork{
		PreviousVersion: spec.GENESIS_FORK_VERSION,
		CurrentVersion:  spec.GENESIS_FORK_VERSION,
		Epoch:           common.GENESIS_EPOCH,
	}); err != nil {
		return err
	}
	// Empty deposit-tree
	eth1Dat := common.Eth1Data{
		DepositRoot:  phase0.NewDepositRootsView().HashTreeRoot(tree.GetHashFn()),
		DepositCount: 0,
		BlockHash:    eth1BlockHash,
	}
	if err := state.SetEth1Data(eth1Dat); err != nil {
		return err
	}
	// Leave the deposit index to 0. No deposits happened.
	if i, err := state.DepositIndex(); err != nil {
		return err
	} else if i != 0 {
		return fmt.Errorf("expected 0 deposit index in state, got %d", i)
	}
	var emptyBody tree.HTR
	switch state.(type) {
	case *merge.BeaconStateView:
		emptyBody = spec.Wrap(new(merge.BeaconBlockBody))
	case *altair.BeaconStateView:
		emptyBody = spec.Wrap(new(altair.BeaconBlockBody))
	default:
		emptyBody = spec.Wrap(new(phase0.BeaconBlockBody))
	}
	latestHeader := &common.BeaconBlockHeader{
		BodyRoot: emptyBody.HashTreeRoot(tree.GetHashFn()),
	}
	if err := state.SetLatestBlockHeader(latestHeader); err != nil {
		return err
	}
	// Seed RANDAO with Eth1 entropy
	err := state.SeedRandao(spec, eth1BlockHash)
	if err != nil {
		return err
	}

	for _, v := range validators {
		if err := state.AddValidator(spec, v.Pubkey, v.WithdrawalCredentials, v.Balance); err != nil {
			return err
		}
	}
	vals, err := state.Validators()
	if err != nil {
		return err
	}
	// Process activations
	for i := 0; i < len(validators); i++ {
		val, err := vals.Validator(common.ValidatorIndex(i))
		if err != nil {
			return err
		}
		vEff, err := val.EffectiveBalance()
		if err != nil {
			return err
		}
		if vEff == spec.MAX_EFFECTIVE_BALANCE {
			if err := val.SetActivationEligibilityEpoch(common.GENESIS_EPOCH); err != nil {
				return err
			}
			if err := val.SetActivationEpoch(common.GENESIS_EPOCH); err != nil {
				return err
			}
		}
	}
	if err := state.SetGenesisValidatorsRoot(vals.HashTreeRoot(tree.GetHashFn())); err != nil {
		return err
	}
	return nil
}

func validatorKeyName(i uint64) string {
	return fmt.Sprintf("m/12381/3600/%d/0/0", i)
}

func withdrawalKeyName(i uint64) string {
	return fmt.Sprintf("m/12381/3600/%d/0", i)
}

func seedFromMnemonic(mnemonic string) (seed []byte, err error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("mnemonic is not valid")
	}
	return bip39.NewSeed(mnemonic, ""), nil
}

func outputPubkeys(outPath string, data []string) error {
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range data {
		if _, err := f.WriteString(p + "\n"); err != nil {
			return err
		}
	}
	return nil
}

type MnemonicSrc struct {
	Mnemonic string `yaml:"mnemonic"`
	Count    uint64 `yaml:"count"`
}

func loadMnemonics(srcPath string) ([]MnemonicSrc, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data []MnemonicSrc
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func loadSpec(configPath string) (*common.Spec, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var p0 common.Phase0Config
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&p0); err != nil {
		return nil, err
	}
	var alt common.AltairConfig
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&p0); err != nil {
		return nil, err
	}
	return &common.Spec{
		CONFIG_NAME:  "custom",
		Phase0Config: p0,
		AltairConfig: alt,
		// TODO: no merge configurables currently, maybe later.
		// TODO: replace old phase1 config with sharding configs
		Phase1Config: configs.Mainnet.Phase1Config, // not used here
	}, nil
}
