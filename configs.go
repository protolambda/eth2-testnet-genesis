package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

func loadSpec(configPath string) (*common.Spec, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read eth2 config file: %v", err)
	}
	var p0 common.Phase0Config
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&p0); err != nil {
		return nil, fmt.Errorf("failed to decode phase0 portion of eth2 config file: %v", err)
	}
	var alt common.AltairConfig
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&p0); err != nil {
		return nil, fmt.Errorf("failed to decode altair portion of eth2 config file: %v", err)
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

func loadEth1GenesisConf(configPath string) (*core.Genesis, error) {
	eth1ConfData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read eth1 config file: %v", err)
	}
	var eth1Genesis core.Genesis
	if err := json.NewDecoder(bytes.NewReader(eth1ConfData)).Decode(&eth1Genesis); err != nil {
		return nil, fmt.Errorf("failed to decode eth1 config file: %v", err)
	}
	return &eth1Genesis, nil
}
