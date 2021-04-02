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
	QKCConfirmationBlock uint64
	FeeRate              *big.Rat

	QKCPrivateAddress string
}

//getShuttleFee(token string, amount int) => fee int
//https://gist.github.com/ninjaahhh/e363dd881e34d2c8e698f4ccf656e8ee
