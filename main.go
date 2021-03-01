package main

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	hbls "github.com/herumi/bls-eth-go-binary/bls"
	"github.com/protolambda/zrnt/eth2/beacon"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
	"github.com/tyler-smith/go-bip39"
	util "github.com/wealdtech/go-eth2-util"
	"gopkg.in/yaml.v3"
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
	depositDataTreeType := view.ListType(beacon.DepositDataType, 1<<32)
	emptyDepositTreeRoot := depositDataTreeType.DefaultNode().MerkleRoot(tree.GetHashFn())

	configPath := flag.String("config", "mainnet.yaml", "Path to config YAML for eth2")
	eth1BlockHashStr := flag.String("eth1-block", "0x0000000000000000000000000000000000000000000000000000000000000000", "Eth1 block hash to put into state")
	eth1BlockTimestamp := flag.Uint64("eth1-timestamp", uint64(time.Now().Unix()), "Eth1 block timestamp")
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
	spec, err := loadSpec(*configPath)
	check(err)

	check(os.MkdirAll(*tranchesDir, 0777))

	var eth1Hash beacon.Root
	check(eth1Hash.UnmarshalText([]byte(*eth1BlockHashStr)))

	mnemonics, err := loadMnemonics(*mnemonicsSrcFilePath)
	check(err)

	var validators []beacon.KickstartValidatorData
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
			var data beacon.KickstartValidatorData
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
		check(outputPubkeys(filepath.Join(*tranchesDir, fmt.Sprintf("tranche_%d", m)), pubs))
	}
	if uint64(len(validators)) < spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT {
		fmt.Printf("WARNING: not enough validators for genesis. Key sources sum up to %d total. But need %d.\n", len(validators), spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT)
	}

	state, _, err := spec.KickStartState(eth1Hash, 0, validators)
	check(err)

	// Set the genesis time as if it was exactly on the configured delay after the eth1 block hash
	t := beacon.Timestamp(*eth1BlockTimestamp) + spec.GENESIS_DELAY
	check(state.SetGenesisTime(t))
	fmt.Printf("genesis at %d + %d = %d  (%s)\n", *eth1BlockTimestamp, spec.GENESIS_DELAY, t, time.Unix(int64(t), 0).String())

	// Overwrite state eth1 data to have an empty deposit tree at the starting eth1 block
	eth1Dat := beacon.Eth1Data{
		DepositRoot:  emptyDepositTreeRoot,
		DepositCount: beacon.DepositIndex(0),
		BlockHash:    eth1Hash,
	}
	check(state.SetEth1Data(eth1Dat.View()))
	// Hack: Reset the deposit index to 0. No deposits happened. (The deposit_index field is 10, and in normal API not writeable, only increasing)
	check(state.Set(10, view.Uint64View(0)))

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

func loadSpec(configPath string) (*beacon.Spec, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var phase0 beacon.Phase0Config
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&phase0); err != nil {
		return nil, err
	}
	return &beacon.Spec{
		CONFIG_NAME:  "custom",
		Phase0Config: phase0,
		Phase1Config: configs.Mainnet.Phase1Config, // not used here
	}, nil
}
