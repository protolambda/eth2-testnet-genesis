package main

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type rpcBlock struct {
	Hash         common.Hash         `json:"hash"`
	Transactions []rpcTransaction    `json:"transactions"`
	UncleHashes  []common.Hash       `json:"uncles"`
	Withdrawals  []*types.Withdrawal `json:"withdrawals,omitempty"`
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

func (tx *rpcTransaction) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &tx.tx); err != nil {
		return err
	}
	return json.Unmarshal(msg, &tx.txExtraInfo)
}

func ParseEthBlock(blockData json.RawMessage) (*types.Block, error) {
	var resultHeader types.Header
	if err := json.Unmarshal(blockData, &resultHeader); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON header: %w", err)
	}

	var body rpcBlock
	if err := json.Unmarshal(blockData, &body); err != nil {
		return nil, err
	}

	// get transactions
	txs := make([]*types.Transaction, len(body.Transactions))
	for idx := range body.Transactions {
		txs[idx] = body.Transactions[idx].tx
	}

	return types.NewBlockWithHeader(&resultHeader).WithBody(types.Body{
		Transactions: txs,
		Uncles:       nil,
		Withdrawals:  body.Withdrawals,
	}), nil
}
