package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/wdwd5wd/test_client/client"
	"log"
	"math"
	"math/big"
	"time"

	"context"
	cc "github.com/QuarkChain/goqkcclient/client"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	token "github.com/516108736/bridge/erc_token"
)

var (
	ethClient, _ = ethclient.Dial("wss://ropsten.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	//client, err = ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")

	qkcClient = cc.NewClient("http://34.222.230.172:38291")

	wei = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))
)

func MonitorETH() {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress("0x1CEB4426BFb1503dee6544cDA2F78E7f892f8850")},
		Topics: [][]common.Hash{
			[]common.Hash{common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
		},
	}

	logs := make(chan types.Log)
	sub, err := ethClient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case vLog := <-logs:
			fmt.Println(vLog.TxHash.String()) // pointer to event log
			fmt.Println("from", vLog.Topics[1].String())
			fmt.Println("to", vLog.Topics[2].String())
			fmt.Println("value", new(big.Int).SetBytes(vLog.Data))
		}
	}
}

func SendToQkc() {
	prvkey, _ := crypto.ToECDSA(common.FromHex("82283e556e0d8cae13dc13a691c6a1cdc67ccd68216a14ae8e79ddd909d08a74"))
	from:=crypto.PubkeyToAddress(prvkey.PublicKey)

	contract:=common.HexToAddress("0x4179ac5f7dcf7Ef3D82b7fa6064Ea5616dde4948")

	tx, err := qkcClient.CreateTransaction(&cc.QkcAddress{Recipient: from, FullShardKey: 0}, &cc.QkcAddress{Recipient: contract, FullShardKey: 0}, new(big.Int), uint64(30000), new(big.Int).SetUint64(1000000000))

	if err != nil {
		fmt.Println(err.Error())
	}
	tx, err = cc.SignTx(tx, prvkey)
	if err != nil {
		fmt.Println(err.Error())
	}
	txid, err := client.SendTransaction(tx)
	if err != nil {
		fmt.Println("SendTransaction error: ", err.Error())
	}

	fmt.Println(common.Bytes2Hex(txid))
	return common.Bytes2Hex(txid)
}

func main() {
	go MonitorETH()
	tokenAddress := common.HexToAddress("0x1CEB4426BFb1503dee6544cDA2F78E7f892f8850")
	instance, err := token.NewErcToken(tokenAddress, ethClient)
	if err != nil {
		log.Fatal(err)
	}

	address := common.HexToAddress("0x9317d5f30ff07ff091b2cc6fa170ca418ca14380")
	bal, err := instance.BalanceOf(&bind.CallOpts{}, address)
	if err != nil {
		log.Fatal(err)
	}

	name, err := instance.Name(&bind.CallOpts{})
	if err != nil {
		log.Fatal(err)
	}

	symbol, err := instance.Symbol(&bind.CallOpts{})
	if err != nil {
		log.Fatal(err)
	}

	decimals, err := instance.Decimals(&bind.CallOpts{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("name: %s\n", name)         // "name: Golem Network"
	fmt.Printf("symbol: %s\n", symbol)     // "symbol: GNT"
	fmt.Printf("decimals: %v\n", decimals) // "decimals: 18"

	fmt.Printf("wei: %s\n", bal) // "wei: 74605500647408739782407023"

	fbal := new(big.Float)
	fbal.SetString(bal.String())
	value := new(big.Float).Quo(fbal, big.NewFloat(math.Pow10(int(decimals))))

	fmt.Printf("balance: %f", value) // "balance: 74605500.647409"

	pr, err := crypto.ToECDSA(common.FromHex("0x82283e556e0d8cae13dc13a691c6a1cdc67ccd68216a14ae8e79ddd909d08a74"))

	addr := crypto.PubkeyToAddress(pr.PublicKey)
	fmt.Println("addr", addr.String())

	auth := bind.NewKeyedTransactor(pr)
	tx, err := instance.Mint(auth, common.HexToAddress("0x5ea7b25C1Ffa0F1905078D06fD4875221bbc6863"), new(big.Int).Mul(new(big.Int).SetInt64(555), wei))
	//fmt.Println("a", a.Hash().String())
	fmt.Println("err", tx.Hash().String(), err)
	time.Sleep(1000 * time.Second)
}
