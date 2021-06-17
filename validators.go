package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/tyler-smith/go-bip39"
	util "github.com/wealdtech/go-eth2-util"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

func loadValidatorKeys(spec *common.Spec, mnemonicsConfigPath string, tranchesDir string) ([]phase0.KickstartValidatorData, error) {
	mnemonics, err := loadMnemonics(mnemonicsConfigPath)
	if err != nil {
		return nil, err
	}

	var validators []phase0.KickstartValidatorData
	for m, mnemonicSrc := range mnemonics {
		fmt.Printf("processing mnemonic %d, for %d validators\n", m, mnemonicSrc.Count)
		seed, err := seedFromMnemonic(mnemonicSrc.Mnemonic)
		if err != nil {
			return nil, fmt.Errorf("mnemonic %d is bad", m)
		}
		pubs := make([]string, 0, mnemonicSrc.Count)
		for i := uint64(0); i < mnemonicSrc.Count; i++ {
			if i%100 == 0 {
				fmt.Printf("...validator %d/%d\n", i, mnemonicSrc.Count)
			}
			signingKey, err := util.PrivateKeyFromSeedAndPath(seed, validatorKeyName(i))
			if err != nil {
				return nil, err
			}
			withdrawalKey, err := util.PrivateKeyFromSeedAndPath(seed, withdrawalKeyName(i))
			if err != nil {
				return nil, err
			}

			// BLS signing key
			var data phase0.KickstartValidatorData
			copy(data.Pubkey[:], signingKey.PublicKey().Marshal())
			pubs = append(pubs, data.Pubkey.String())

			// BLS withdrawal credentials
			h := sha256.New()
			h.Write(withdrawalKey.PublicKey().Marshal())
			copy(data.WithdrawalCredentials[:], h.Sum(nil))
			data.WithdrawalCredentials[0] = common.BLS_WITHDRAWAL_PREFIX

			// Max effective balance by default for activation
			data.Balance = spec.MAX_EFFECTIVE_BALANCE

			validators = append(validators, data)
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
