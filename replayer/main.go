package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"log"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	token "github.com/516108736/bridge/erc_token"
)

var (
	wei = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))
)

func main() {
	client, err := ethclient.Dial("wss://ropsten.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	//client, err := ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	if err != nil {
		log.Fatal(err)
	}

	tokenAddress := common.HexToAddress("0x299744722e0c80a23F1c0f48e712317cB89663f2")
	instance, err := token.NewErcToken(tokenAddress, client)
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
	tx, err := instance.Mint(auth, common.HexToAddress("0x5ea7b25C1Ffa0F1905078D06fD4875221bbc6863"), new(big.Int).Mul(new(big.Int).SetInt64(666), wei))
	//fmt.Println("a", a.Hash().String())
	fmt.Println("err", tx.Hash().String(), err)
}
