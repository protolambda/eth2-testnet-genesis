package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/tyler-smith/go-bip39"
	util "github.com/wealdtech/go-eth2-util"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

func loadValidatorKeys(spec *common.Spec, mnemonicsConfigPath string, tranchesDir string, ethWithdrawalAddress common.Eth1Address) ([]phase0.KickstartValidatorData, error) {
	mnemonics, err := loadMnemonics(mnemonicsConfigPath)
	if err != nil {
		return nil, err
	}

	//var validators []phase0.KickstartValidatorData
	var valCount uint64 = 0
	for _, mnemonicSrc := range mnemonics {
		valCount += mnemonicSrc.Count
	}
	validators := make([]phase0.KickstartValidatorData, valCount)

	offset := uint64(0)
	for m, mnemonicSrc := range mnemonics {
		var g errgroup.Group
		var prog int32
		fmt.Printf("processing mnemonic %d, for %d validators\n", m, mnemonicSrc.Count)
		seed, err := seedFromMnemonic(mnemonicSrc.Mnemonic)
		if err != nil {
			return nil, fmt.Errorf("mnemonic %d is bad", m)
		}
		pubs := make([]string, mnemonicSrc.Count)
		for i := uint64(0); i < mnemonicSrc.Count; i++ {
			valIndex := offset + i
			idx := i
			g.Go(func() error {
				signingKey, err := util.PrivateKeyFromSeedAndPath(seed, validatorKeyName(idx))
				if err != nil {
					return err
				}
				withdrawalKey, err := util.PrivateKeyFromSeedAndPath(seed, withdrawalKeyName(idx))
				if err != nil {
					return err
				}

				// BLS signing key
				var data phase0.KickstartValidatorData
				copy(data.Pubkey[:], signingKey.PublicKey().Marshal())
				pubs[idx] = data.Pubkey.String()

				if ethWithdrawalAddress == (common.Eth1Address{}) {
					// BLS withdrawal credentials
					h := sha256.New()
					h.Write(withdrawalKey.PublicKey().Marshal())
					copy(data.WithdrawalCredentials[:], h.Sum(nil))
					data.WithdrawalCredentials[0] = common.BLS_WITHDRAWAL_PREFIX
				} else {
					// spec:
					// The withdrawal_credentials field must be such that:
					//   withdrawal_credentials[:1] == ETH1_ADDRESS_WITHDRAWAL_PREFIX
					//   withdrawal_credentials[1:12] == b'\x00' * 11
					//   withdrawal_credentials[12:] == eth1_withdrawal_address
					data.WithdrawalCredentials[0] = common.ETH1_ADDRESS_WITHDRAWAL_PREFIX
					copy(data.WithdrawalCredentials[12:], ethWithdrawalAddress[:])
				}

				// Max effective balance by default for activation
				data.Balance = spec.MAX_EFFECTIVE_BALANCE
				validators[valIndex] = data
				atomic.AddInt32(&prog, 1)
				if prog%100 == 0 {
					fmt.Printf("...validator %d/%d\n", prog, mnemonicSrc.Count)
				}
				return nil
			})

		}
		offset += mnemonicSrc.Count
		if err := g.Wait(); err != nil {
			return nil, err
		}

		fmt.Println("Writing pubkeys list file...")
		if err := outputPubkeys(filepath.Join(tranchesDir, fmt.Sprintf("tranche_%04d.txt", m)), pubs); err != nil {
			return nil, err
		}
	}
	return validators, nil
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
