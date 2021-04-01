package main

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type BridgeConfig struct {
	ETHInfuraProjectID string
	QKCRpc             string

	ETHContract       []common.Address
	QKCNativeContract []uint64

	ETHSpecialAddress common.Address

	ETHConfirmationBlock uint64
	FeeRate              *big.Rat

	QKCPrivateAddress string
}
