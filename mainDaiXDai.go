package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

func XunHuanFilterLogs(client *ethclient.Client, query ethereum.FilterQuery) []types.Log {
	rs := make([]types.Log, 0)
	for index := 6478422; index <= 12086025; index += 500000 {
		query.FromBlock = new(big.Int).SetUint64(uint64(index + 1))
		query.ToBlock = new(big.Int).SetUint64(uint64(index + 500000))
		logs, err := client.FilterLogs(context.Background(), query)
		if err != nil {
			panic(err)
		}
		//fmt.Println("index", index, query.FromBlock, query.ToBlock)
		rs = append(rs, logs...)
	}
	return rs
}

func Div18(data *big.Int) *big.Int {
	return new(big.Int).Div(data, wei)
}
func m1ain() {
	client, err := ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	if err != nil {
		panic(err)
	}

	topic := make([]common.Hash, 0)
	topic = append(topic, common.HexToHash("0x4ab7d581336d92edbea22636a613e8e76c99ac7f91137c1523db38dbfb3bf329"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016"),
		},
		Topics: [][]common.Hash{topic},
	}
	logs := XunHuanFilterLogs(client, query)

	allAmount := new(big.Int)
	maxAmount := new(big.Int)
	maxTxHash := common.Hash{}
	for _, log := range logs {
		amount := Div18(new(big.Int).SetBytes(log.Data[32:64]))

		allAmount = new(big.Int).Add(allAmount, amount)
		if maxAmount.Cmp(amount) < 0 {
			maxAmount = new(big.Int).Set(amount)
			maxTxHash = log.TxHash
		}
	}
	fmt.Println("dai->eth", len(logs), allAmount, maxTxHash, maxAmount)

	topic = make([]common.Hash, 0)
	topic = append(topic, common.HexToHash("0x1d491a427d1f8cc0d447496f300fac39f7306122481d8e663451eb268274146b"))
	query = ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x4aa42145Aa6Ebf72e164C9bBC74fbD3788045016"),
		},
		Topics: [][]common.Hash{topic},
	}
	logs = XunHuanFilterLogs(client, query)

	allAmount = new(big.Int)
	maxAmount = new(big.Int)
	maxTxHash = common.Hash{}
	for _, log := range logs {
		amount := Div18(new(big.Int).SetBytes(log.Data[32:64]))
		allAmount = new(big.Int).Add(allAmount, amount)
		if maxAmount.Cmp(amount) < 0 {
			maxAmount = new(big.Int).Set(amount)
			maxTxHash = log.TxHash
		}
	}
	fmt.Println("eth->xdai", len(logs), allAmount, maxTxHash, maxAmount)

}
