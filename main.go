package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
	"strings"
	"time"
)

type erc20 struct {
	name    string
	address string
}

var (
	wei = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))

	tokenList = []erc20{
		{
			name:    "LUNA",
			address: "0xd2877702675e6cEb975b4A1dFf9fb7BAF4C91ea9",
		},
		{
			name:    "UST",
			address: "0xa47c8bf37f92aBed4A126BDA807A7b7498661acD",
		},
		{
			name:    "KRT",
			address: "0xcAAfF72A8CbBfc5Cf343BA4e26f65a257065bFF1",
		}, {
			name:    "SDT",
			address: "0x676Ad1b33ae6423c6618C1AEcf53BAa29cf39EE5",
		}, {
			name:    "MNT",
			address: "0x156B36ec68FdBF84a925230BA96cb1Ca4c4bdE45",
		},
		{
			name:    "MIR",
			address: "0x09a3EcAFa817268f77BE1283176B946C4ff2E608",
		}, {
			name:    "mAAPL",
			address: "0xd36932143F6eBDEDD872D5Fb0651f4B72Fd15a84",
		}, {
			name:    "mGOOGL",
			address: "0x59A921Db27Dd6d4d974745B7FfC5c33932653442",
		}, {
			name:    "mTSLA",
			address: "0x21cA39943E91d704678F5D00b6616650F066fD63",
		}, {
			name:    "mNFLX",
			address: "0xC8d674114bac90148d11D3C1d33C61835a0F9DCD",
		}, {
			name:    "mQQQ",
			address: "0x13B02c8dE71680e71F0820c996E4bE43c2F57d15",
		}, {
			name:    "mTWTR",
			address: "0xEdb0414627E6f1e3F082DE65cD4F9C693D78CCA9",
		}, {
			name:    "mMSFT",
			address: "0x41BbEDd7286dAab5910a1f15d12CBda839852BD7",
		}, {
			name:    "mAMZN",
			address: "0x0cae9e4d663793c2a2A0b211c1Cf4bBca2B9cAa7",
		}, {
			name:    "mBABA",
			address: "0x56aA298a19C93c6801FDde870fA63EF75Cc0aF72",
		}, {
			name:    "mIAU",
			address: "0x1d350417d9787E000cc1b95d70E9536DcD91F373",
		}, {
			name:    "mSLV",
			address: "0x9d1555d8cB3C846Bb4f7D5B1B1080872c3166676",
		}, {
			name:    "mUSO",
			address: "0x31c63146a635EB7465e5853020b39713AC356991",
		}, {
			name:    "mVIXY",
			address: "0xf72FCd9DCF0190923Fadd44811E240Ef4533fc86",
		},
		{
			name:    "mFB",
			address: "0x0e99cC0535BB6251F6679Fa6E65d6d3b430e840B",
		},
	}
)

func byteToBitInt(b []byte) *big.Int {
	data := new(big.Int).SetBytes(b)
	return data.Div(data, wei)
}

var (
	logTransferSig = []byte("Transfer(address,address,uint256)")
	//LogApprovalSig := []byte("Approval(address,address,uint256)")
	logBurnSig         = []byte("Burn(address,bytes32,uint256)")
	logTransferSigHash = crypto.Keccak256Hash(logTransferSig)
	//logApprovalSigHash := crypto.Keccak256Hash(LogApprovalSig)
	logBurnSigHash = crypto.Keccak256Hash(logBurnSig)

	logOwnerSig     = []byte("OwnershipTransferred(address,address)")
	logOwnerSigHash = crypto.Keccak256Hash(logOwnerSig)
)

func getLogs(token string, query ethereum.FilterQuery, cli *ethclient.Client) []types.Log {
	minnHeight := 11345215
	latest := 11949838
	interval := 10000

	logs := make([]types.Log, 0)

	for index := minnHeight; index < latest; index += interval {
		query.FromBlock = new(big.Int).SetUint64(uint64(index))
		query.ToBlock = new(big.Int).SetUint64(uint64(index+interval) - 1)
		tryCnt := 0
		for {
			ll, err := cli.FilterLogs(context.Background(), query)
			if err != nil {
				if tryCnt < 10 {
					tryCnt++
					continue
				} else {
					panic(fmt.Errorf("%s %d %s %d", token, index, err, tryCnt))
				}

			} else {
				logs = append(logs, ll...)
				break
			}

		}

	}
	return logs
}
func shuttle() {
	client, err := ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	if err != nil {
		log.Fatal(err)
	}

	topic := make([]common.Hash, 0)
	topic = append(topic, logBurnSigHash)
	topic = append(topic, logTransferSigHash)

	fmt.Println("LLLLLL", len(tokenList), logBurnSigHash.String(), logTransferSigHash.String())
	for _, v := range tokenList {
		query := ethereum.FilterQuery{
			Addresses: []common.Address{
				common.HexToAddress(v.address),
			},
			Topics: [][]common.Hash{topic},
		}
		logs := getLogs(v.name, query, client)
		DisplayLogs(v.name, v.address, logs)
		time.Sleep(10 * time.Second)
	}

}

func DisplayLogs(token string, contractAddress string, logs []types.Log) {
	burnCnt := 0
	burnAmount := new(big.Int)
	burnMaxx := new(big.Int)
	burnMaxxTxHash := ""

	transferCnt := 0
	transferAmount := new(big.Int)
	transferMaxx := new(big.Int)
	transferMaxxTxHash := ""
	mp := make(map[common.Hash]bool)

	//fmt.Println("-----", logBurnSigHash.String(), logTransferSigHash.String())
	for index := len(logs) - 1; index >= 0; index-- {
		log := logs[index]
		switch log.Topics[0].Hex() {
		case logBurnSigHash.Hex():
			mp[log.TxHash] = true
			//fmt.Println("BBB", log.BlockNumber, log.TxHash.String())
			burnCnt++
			value := byteToBitInt(log.Data)
			burnAmount.Add(burnAmount, value)
			if burnMaxx.Cmp(value) < 0 {
				burnMaxx = value
				burnMaxxTxHash = log.TxHash.String()
			}
			//fmt.Println("Burn", log.TxHash.Hex(), log.Topics[0].String(), log.Topics[1].String(), log.Topics[2].String(), byteToBitInt(log.Data).String())
		case logTransferSigHash.Hex():
			if mp[log.TxHash] || log.Topics[1].String() != "0x0000000000000000000000000000000000000000000000000000000000000000" {
				mp[log.TxHash] = true
				//failde++
				//fmt.Println("CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC", log.BlockNumber, log.TxHash.String())
				continue
			}
			mp[log.TxHash] = true
			transferCnt++
			//fmt.Println("TTT", log.BlockNumber, log.TxHash.String())
			value := byteToBitInt(log.Data)
			transferAmount.Add(transferAmount, value)
			if transferMaxx.Cmp(value) < 0 {
				transferMaxx = value
				transferMaxxTxHash = log.TxHash.String()
			}
			//fmt.Println("Transfer", log.TxHash.String(), log.Topics[0].String(), log.Topics[1].String(), log.Topics[2].String(), byteToBitInt(log.Data))

		default:
			panic("sb")
		}
	}
	fmt.Printf("%s,%s,%d,%d,%s,%s,%s,%d,%s,%s,%s\n", token, contractAddress, burnCnt+transferCnt, burnCnt, burnAmount, burnMaxx, burnMaxxTxHash, transferCnt, transferAmount, transferMaxx, transferMaxxTxHash)
}

var (
	mpTokenToAddress = map[string]string{
		"WETH":  "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		"WBTC":  "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
		"USDT":  "0xdac17f958d2ee523a2206206994597c13d831ec7",
		"LINK":  "0x514910771af9ca656af840dff83e8264ecf986ca",
		"DAI":   "0x6b175474e89094c44da98b954eedeac495271d0f",
		"SUSHI": "0x6b3595068778dd592e39a122f4f5a5cf09c90fe2",
		"AAVE":  "0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9",
		"UNI":   "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984",
		"YFI":   "0x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e",
		"GRT":   "0xc944e90c64b2c07662a292be6244bdf05cda44a7",
		"UMA":   "0x04Fa0d235C4abf4BcF4787aF4CF447DE572eF828",
		"QNT":   "0x4a220e6096b25eadb88358cb44068a3248254675",
		"POLS":  "0x83e6f1E41cdd28eAcEB20Cb649155049Fac3D5Aa",
		"BUSD":  "0x4fabb145d64652a948d72533023f6e7a623c7c53",
		"ZRX":   "0xE41d2489571d322189246DaFA5ebDe1F4699F498",
		"SNX":   "0xc011a73ee8576fb46f5e1c5751ca3b9fe0af2a6f",
		"1INCH": "0x111111111117dc0aa78b770fa6a738034120c302",
		"COMP":  "0xc00e94cb662c3520282e6f5717214004a7f26888",
		"RUNE":  "0x3155BA85D5F96b2d030a4966AF206230e46849cb",
		"BAT":   "0x0d8775f648430679a709e98d2b0cb6250d2887ef",
		"OXT":   "0x4575f41308EC1483f3d399aa9a2826d74Da13Deb",
		"AVAX":  "0x9debca6ea3af87bf422cea9ac955618ceb56efb4",
	}
	fakeMap = make(map[string]string)
)

func init() {
	for k, v := range mpTokenToAddress {
		vv := strings.ToLower(v)
		fakeMap[vv] = k
	}
}
func findTokenName(addr string) string {
	if data, ok := fakeMap["0x"+addr]; ok {
		return data
	} else {
		panic(fmt.Errorf("%s %s", addr, "0x"+addr))
	}
}

func calRealValue(token string, data *big.Int) *big.Int {
	dec6 := new(big.Int).SetUint64(1000000)
	dec8 := new(big.Int).SetUint64(100000000)
	dec12 := new(big.Int).Mul(dec6, dec6)
	dec18 := new(big.Int).Mul(dec6, dec12)
	mpDec := map[string]*big.Int{
		"WETH":  dec18,
		"WBTC":  dec8,
		"USDT":  dec6,
		"LINK":  dec18,
		"DAI":   dec18,
		"SUSHI": dec18,
		"AAVE":  dec18,
		"UNI":   dec18,
		"YFI":   dec18,
		"GRT":   dec18,
		"UMA":   dec18,
		"QNT":   dec18,
		"POLS":  dec18,
		"BUSD":  dec18,
		"ZRX":   dec18,
		"SNX":   dec18,
		"1INCH": dec18,
		"COMP":  dec18,
		"RUNE":  dec18,
		"BAT":   dec18,
		"OXT":   dec18,
		"AVAX":  dec18}

	return data.Div(data, mpDec[token])

}

func chainBradge() {
	client, err := ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	if err != nil {
		log.Fatal(err)
	}

	logDepositSig := []byte("Deposit(uint8,bytes32,uint64)")
	logDepositSigHash := crypto.Keccak256Hash(logDepositSig)
	topic := make([]common.Hash, 0)
	topic = append(topic, logDepositSigHash)
	topic = append(topic, common.HexToHash("0x803c5a12f6bde629cea32e63d4b92d1b560816a6fb72e939d3c89e1cab650417"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x96b845abe346b49135b865e5cedd735fc448c3ad"),
		},
		//FromBlock: new(big.Int).SetUint64(11956022 - 10000),
		//ToBlock:   new(big.Int).SetUint64(11956022),
		Topics: [][]common.Hash{topic},
	}
	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	type aboutTx struct {
		token  string
		txHash common.Hash
		txType string
	}

	allAboutTxList := make([]aboutTx, 0)
	for _, log := range logs {
		topic3 := new(big.Int).SetBytes(log.Topics[3].Bytes())
		switch log.Topics[0].Hex() {
		case "0xdbb69440df8433824a026ef190652f29929eb64b4d1d5d2a69be8afe3e6eaed8":
			token := findTokenName(log.Topics[2].String()[24:64])
			allAboutTxList = append(allAboutTxList, aboutTx{
				token:  token,
				txHash: log.TxHash,
				txType: "deposit",
			})

		case "0x803c5a12f6bde629cea32e63d4b92d1b560816a6fb72e939d3c89e1cab650417":
			if topic3.Uint64() != 3 {
				continue
			}
			token := findTokenName(hex.EncodeToString(log.Data)[22:62])
			allAboutTxList = append(allAboutTxList, aboutTx{
				token:  token,
				txHash: log.TxHash,
				txType: "execute",
			})
		default:
			panic("sb")
		}
	}

	type result struct {
		allCnt         int
		amount         *big.Int
		maxxAmount     *big.Int
		maxxAmountHash common.Hash
	}

	mppExecute := make(map[string]map[string]*result)
	mppExecute["deposit"] = make(map[string]*result)
	mppExecute["execute"] = make(map[string]*result)
	for k, _ := range mppExecute {
		for token, _ := range mpTokenToAddress {
			mppExecute[k][token] = &result{
				allCnt:         0,
				amount:         new(big.Int),
				maxxAmount:     new(big.Int),
				maxxAmountHash: common.Hash{},
			}
		}
	}

	cnt := 0
	for _, v := range allAboutTxList {
		rs, err := client.TransactionReceipt(context.Background(), v.txHash)
		if err != nil {
			panic(err)
		}
		for _, log := range rs.Logs {
			if log.Topics[0].Hex() == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
				data := mppExecute[v.txType][v.token]
				amount := calRealValue(v.token, new(big.Int).SetBytes(log.Data))
				data.amount.Add(data.amount, amount)
				data.allCnt++
				if data.maxxAmount.Cmp(amount) < 0 {
					data.maxxAmount = amount
					data.maxxAmountHash = v.txHash
				}
				cnt++
				break
			}
		}
	}
	fmt.Println("LLLL", len(allAboutTxList), cnt)

	for token, _ := range mpTokenToAddress {
		deposit := mppExecute["deposit"][token]
		execute := mppExecute["execute"][token]
		fmt.Printf("%s,%s,%d,%d,%s,%s,%s,%d,%s,%s,%s\n", token, mpTokenToAddress[token], deposit.allCnt+execute.allCnt, deposit.allCnt, deposit.amount, deposit.maxxAmount, deposit.maxxAmountHash, execute.allCnt, execute.amount, execute.maxxAmount, execute.maxxAmountHash)
	}
}

func main() {
	shuttle()
	//chainBradge()
}
