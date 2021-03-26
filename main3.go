package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"time"

	"math/big"
)

func XHuanFilterLogs(client *ethclient.Client, query ethereum.FilterQuery) []types.Log {
	rs := make([]types.Log, 0)
	for index := 10590093; index <= 12086488; index += 200000 {
		query.FromBlock = new(big.Int).SetUint64(uint64(index + 1))
		query.ToBlock = new(big.Int).SetUint64(uint64(index + 200000))
		logs, err := client.FilterLogs(context.Background(), query)
		if err != nil {
			panic(err)
		}
		//fmt.Println("index", index, query.FromBlock, query.ToBlock)
		rs = append(rs, logs...)
	}
	return rs
}

type Info struct {
	token common.Address

	allAmount  *big.Int
	maxxAmount *big.Int
	maxxTxHash common.Hash
	cnt        int
}

func makeRequest(token string) string {
	return "https://api.ethplorer.io/getTokenInfo/" + token + "?apiKey=freekey"
}

type tokenInfo struct {
	symbol   string
	decimals int
	rate     *big.Float
	currency string
}

func getTokenInfo(tokens []string) map[string]*tokenInfo {
	rsList := make(map[string]*tokenInfo)
	for _, token := range tokens {
		response, err := http.Get(makeRequest(token))
		if err != nil {
			panic(err)
		}
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}

		var dat map[string]interface{}
		if err := json.Unmarshal(body, &dat); err == nil {
			//fmt.Println("???", token, makeRequest(token), string(body))
			decimals, _ := strconv.Atoi(dat["decimals"].(string))

			rat := new(big.Float)
			currency := ""
			if _, ok := dat["price"].(map[string]interface{}); ok {
				rat = new(big.Float).SetFloat64(dat["price"].(map[string]interface{})["rate"].(float64))
				currency = dat["price"].(map[string]interface{})["currency"].(string)
			} else {

			}

			rs := &tokenInfo{
				symbol:   dat["symbol"].(string),
				decimals: decimals,
				rate:     rat,
				currency: currency,
			}
			rsList[token] = rs
			time.Sleep(1500 * time.Millisecond)

		} else {
			panic("sb")
		}
	}
	return rsList
}

func main() {
	m1ain()
	return
	client, err := ethclient.Dial("wss://mainnet.infura.io/ws/v3/5f85acad140a4286858886f080177bc9")
	if err != nil {
		panic(err)
	}

	topic := make([]common.Hash, 0)
	topic = append(topic, common.HexToHash("0x59a9a8027b9c87b961e254899821c9a276b5efc35d1f7409ea4f291470f1629a"))
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x88ad09518695c6c3712AC10a214bE5109a655671"),
		},
		Topics: [][]common.Hash{topic},
	}
	logs := XHuanFilterLogs(client, query)

	mpEthToDai := make(map[common.Address]*Info)
	for _, log := range logs {
		token := common.BytesToAddress(log.Topics[1].Bytes())
		amount := new(big.Int).SetBytes(log.Data)
		if _, ok := mpEthToDai[token]; !ok {
			mpEthToDai[token] = &Info{
				allAmount:  new(big.Int),
				maxxAmount: new(big.Int),
				cnt:        0,
			}
		}
		mpEthToDai[token].cnt++
		mpEthToDai[token].allAmount = new(big.Int).Add(mpEthToDai[token].allAmount, amount)
		if mpEthToDai[token].maxxAmount.Cmp(amount) < 0 {
			mpEthToDai[token].maxxAmount = new(big.Int).Set(amount)
			mpEthToDai[token].maxxTxHash = log.TxHash
		}
	}
	fmt.Println("eth->dai", len(logs), len(mpEthToDai))

	listInfo := make(Persons, 0)
	for token, info := range mpEthToDai {
		info.token = token
		listInfo = append(listInfo, info)
	}
	sort.Sort(listInfo)
	listInfo = listInfo[0:10]

	tokens := make([]string, 0)
	for _, v := range listInfo {
		tokens = append(tokens, v.token.String())
	}
	tokenInfo := getTokenInfo(tokens)

	for _, v := range listInfo {
		allAmount := T(v.allAmount, tokenInfo[v.token.String()].decimals)
		maxxAmount := T(v.maxxAmount, tokenInfo[v.token.String()].decimals)
		allAmountPrice := new(big.Float).Mul(new(big.Float).SetInt(allAmount), tokenInfo[v.token.String()].rate)
		maxxAmoutPrice := new(big.Float).Mul(new(big.Float).SetInt(maxxAmount), tokenInfo[v.token.String()].rate)
		fmt.Printf("%s   %s,%d,%d,%.2f,%d,%.2f,%s\n", tokenInfo[v.token.String()].currency, tokenInfo[v.token.String()].symbol, v.cnt, allAmount, allAmountPrice, maxxAmount, maxxAmoutPrice, v.maxxTxHash.String())
	}

	// dai->eth

	topic = make([]common.Hash, 0)
	topic = append(topic, common.HexToHash("0x9afd47907e25028cdaca89d193518c302bbb128617d5a992c5abd45815526593"))
	query = ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress("0x88ad09518695c6c3712AC10a214bE5109a655671"),
		},
		Topics: [][]common.Hash{topic},
	}
	logs = XHuanFilterLogs(client, query)
	mpEthToDai = make(map[common.Address]*Info)
	for _, log := range logs {
		token := common.BytesToAddress(log.Topics[1].Bytes())
		amount := new(big.Int).SetBytes(log.Data)
		if _, ok := mpEthToDai[token]; !ok {
			mpEthToDai[token] = &Info{
				allAmount:  new(big.Int),
				maxxAmount: new(big.Int),
				cnt:        0,
			}
		}
		mpEthToDai[token].cnt++
		mpEthToDai[token].allAmount = new(big.Int).Add(mpEthToDai[token].allAmount, amount)
		if mpEthToDai[token].maxxAmount.Cmp(amount) < 0 {
			mpEthToDai[token].maxxAmount = new(big.Int).Set(amount)
			mpEthToDai[token].maxxTxHash = log.TxHash
		}
	}
	fmt.Println("dai->eth", len(logs), len(mpEthToDai))

	listInfo = make(Persons, 0)
	for token, info := range mpEthToDai {
		info.token = token
		listInfo = append(listInfo, info)
	}
	sort.Sort(listInfo)
	listInfo = listInfo[0:10]

	tokens = make([]string, 0)
	for _, v := range listInfo {
		tokens = append(tokens, v.token.String())
	}
	tokenInfo = getTokenInfo(tokens)

	for _, v := range listInfo {
		allAmount := T(v.allAmount, tokenInfo[v.token.String()].decimals)
		maxxAmount := T(v.maxxAmount, tokenInfo[v.token.String()].decimals)
		allAmountPrice := new(big.Float).Mul(new(big.Float).SetInt(allAmount), tokenInfo[v.token.String()].rate)
		maxxAmoutPrice := new(big.Float).Mul(new(big.Float).SetInt(maxxAmount), tokenInfo[v.token.String()].rate)
		fmt.Printf("%s %s,%d,%d,%.2f,%d,%.2f,%s\n", tokenInfo[v.token.String()].currency, tokenInfo[v.token.String()].symbol, v.cnt, allAmount, allAmountPrice, maxxAmount, maxxAmoutPrice, v.maxxTxHash.String())
	}

}

func T(data *big.Int, decimals int) *big.Int {
	ans := new(big.Int).Set(data)

	for index := 0; index < decimals; index++ {
		ans = ans.Div(ans, new(big.Int).SetInt64(10))
	}
	return ans
}

type Persons []*Info

func (p Persons) Len() int { return len(p) }

func (p Persons) Less(i, j int) bool {
	return p[i].cnt > p[j].cnt
}

func (p Persons) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
