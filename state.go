package main

import (
	"fmt"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/merge"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

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
