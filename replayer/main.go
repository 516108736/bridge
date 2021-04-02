package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
	"io/ioutil"
	"math/big"
	"time"

	"context"
	cc "github.com/QuarkChain/goqkcclient/client"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis/v8"
)

var (
	wei = new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000))
	ctx = context.Background()
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

type bridgeManager struct {
	config    *BridgeConfig
	ethClient *ethclient.Client
	qkcClient *cc.Client
	redis     *redis.Client

	qkcPrivate *ecdsa.PrivateKey

	KeyLastHeight   string
	KeyNextSequence string
	KeyQueueTx      string
}

func NewBridgeManager(config *BridgeConfig) *bridgeManager {
	r := redis.NewClient(&redis.Options{
		Addr:     "165.227.93.216:6379",
		Password: "",
		DB:       0,
	})

	ethClient, err := ethclient.Dial(config.ETHInfuraProjectID)
	checkError(err)

	qkcClient := cc.NewClient(config.QKCRpc)

	prvKey, err := crypto.ToECDSA(common.FromHex(config.QKCPrivateAddress))
	checkError(err)

	bm := &bridgeManager{
		config:          config,
		ethClient:       ethClient,
		qkcClient:       qkcClient,
		redis:           r,
		qkcPrivate:      prvKey,
		KeyLastHeight:   "eth_key_last_height",
		KeyNextSequence: "eth_next_sequence",
		KeyQueueTx:      "eth_key_queue_tx",
	}
	bm.InitAndVerify()

	return bm
}

func (b *bridgeManager) InitAndVerify() {
	_, err := b.qkcClient.NetworkID()
	checkError(err)

	fromAddr := crypto.PubkeyToAddress(b.qkcPrivate.PublicKey)
	nonce, err := b.qkcClient.GetNonce(&cc.QkcAddress{
		Recipient:    fromAddr,
		FullShardKey: 0,
	})
	checkError(err)
	b.redis.Set(ctx, b.KeyNextSequence, nonce, 0)
}

func (b *bridgeManager) EthMonitor() {
	for true {
		b.CheckEthMonitorTxQueue()
		lastHeight, err := b.redis.Get(ctx, b.KeyLastHeight).Uint64()
		if err != nil {
			lastHeight = 0
		}
		nonce, err := b.redis.Get(ctx, b.KeyNextSequence).Uint64()
		checkError(err)
		newLastHeight, monitorDatas := b.load(lastHeight)
		if len(monitorDatas) != 0 {
			txs, nonce := b.buildQKCTx(monitorDatas, nonce)
			jData, err := rlp.EncodeToBytes(txs)
			checkError(err)
			b.redis.RPush(ctx, b.KeyQueueTx, string(jData))
			b.redis.Set(ctx, b.KeyNextSequence, nonce, 0)

			//TODO notify to slack
			b.relayQKCMsg(txs)
		}
		fmt.Println("newLastHeight", newLastHeight, lastHeight)
		b.redis.Set(ctx, b.KeyLastHeight, newLastHeight, 0)
		if newLastHeight == lastHeight {
			time.Sleep(20 * time.Second)
		}
	}
}

func (b *bridgeManager) relayQKCMsg(txs []QKCTxDetail) {
	for _, tx := range txs {
		_, err := b.qkcClient.SendTransaction(tx.Tx)
		checkError(err)
		for true {
			time.Sleep(1 * time.Second)
			sb, err := cc.ByteToTransactionId(append(tx.TxHash.Bytes(), common.FromHex("0x00000000")...))

			checkError(err)
			rs, err := b.qkcClient.GetTransactionReceipt(sb)
			if err != nil || rs.Result == nil {
				continue
			}
			status, ok := rs.Result.(map[string]interface{})["status"]
			if ok && status.(string) == "0x1" {
				break
			}
		}
		fmt.Println("succ tx", tx.TxHash.String())
		checkError(err)
	}
	//TODO check status
	return
}

type QKCTxDetail struct {
	Tx       *cc.EvmTransaction
	TxHash   common.Hash
	CreateAt time.Time
}

func (b *bridgeManager) GetNativeTokenIDInQkc(addr common.Address) uint64 {
	for index, v := range b.config.ETHContract {
		if bytes.Equal(addr.Bytes(), v.Bytes()) {
			return b.config.QKCNativeContract[index]
		}
	}
	panic("sb")
}

func (b *bridgeManager) buildQKCTx(dates []MonitoringData, nonce uint64) ([]QKCTxDetail, uint64) {
	rs := make([]QKCTxDetail, 0)
	for _, data := range dates {
		nativeTokenOnQkc := b.GetNativeTokenIDInQkc(data.Contract)

		tx, err := b.qkcClient.CreateTransaction(nonce, 0, &cc.QkcAddress{
			Recipient:    common.HexToAddress("0x514b430000000000000000000000000000000002"),
			FullShardKey: 0,
		}, new(big.Int), 3000000, new(big.Int).SetUint64(1000000000), cc.TokenIDEncode("QKC"), MintMsg(nativeTokenOnQkc, data.Amount))
		checkError(err)
		tx, err = cc.SignTx(tx, b.qkcPrivate)
		checkError(err)
		nonce++
		rs = append(rs, QKCTxDetail{
			Tx:       tx,
			TxHash:   (&cc.Transaction{TxType: cc.EvmTx, EvmTx: tx}).Hash(),
			CreateAt: time.Now(),
		})

		tx, err = b.qkcClient.CreateTransaction(nonce, 0, &cc.QkcAddress{
			Recipient:    data.To,
			FullShardKey: 0,
		}, data.Amount, 30000, new(big.Int).SetUint64(1000000000), b.GetNativeTokenIDInQkc(data.Contract), nil)
		checkError(err)
		tx, err = cc.SignTx(tx, b.qkcPrivate)
		checkError(err)
		nonce++
		rs = append(rs, QKCTxDetail{
			Tx:       tx,
			TxHash:   (&cc.Transaction{TxType: cc.EvmTx, EvmTx: tx}).Hash(),
			CreateAt: time.Now(),
		})
	}
	return rs, nonce
}

//499500000000000000
//499500000000000000

func (b *bridgeManager) load(lastHeight uint64) (uint64, []MonitoringData) {
	bb, err := b.ethClient.BlockByNumber(ctx, nil)
	checkError(err)
	newHeight := bb.NumberU64()
	newHeight -= b.config.ETHConfirmationBlock
	// skip no new blocks generated
	if lastHeight >= newHeight {
		return newHeight, nil
	}

	fromBlock := newHeight
	if lastHeight != 0 {
		fromBlock = lastHeight + 1
	}

	toBlock := newHeight
	if fromBlock+100 < newHeight {
		toBlock = fromBlock + 100
	}

	rs := make([]MonitoringData, 0)
	for _, c := range b.config.ETHContract {
		rs = append(rs, b.getMonitoringData(c, fromBlock, toBlock)...)
	}
	return toBlock, rs
}

type MonitoringData struct {
	BlockNumber uint64
	TxHash      common.Hash
	Sender      common.Address
	To          common.Address
	Request     *big.Int
	Amount      *big.Int
	Fee         *big.Int
	Contract    common.Address
	nativeIndex int
}

func (b *bridgeManager) calFee(r *big.Int) *big.Int {
	rs := new(big.Int)
	rs = rs.Mul(r, b.config.FeeRate.Num())
	rs = rs.Div(rs, b.config.FeeRate.Denom())
	return rs
}

func (b *bridgeManager) getMonitoringData(contract common.Address, from, to uint64) []MonitoringData {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contract},
		Topics: [][]common.Hash{
			[]common.Hash{common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
		},
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
	}

	rs := make([]MonitoringData, 0)
	logs, err := b.ethClient.FilterLogs(ctx, query)
	checkError(err)
	for _, log := range logs {
		if !bytes.Equal(common.BytesToAddress(log.Topics[2].Bytes()).Bytes(), common.HexToAddress("0x9317D5F30ff07ff091b2cC6fA170Ca418ca14380").Bytes()) {
			continue
		}

		requestd := new(big.Int).SetBytes(log.Data)
		fee := b.calFee(requestd)
		amount := new(big.Int).Sub(requestd, fee)

		rs = append(rs, MonitoringData{
			BlockNumber: log.BlockNumber,
			TxHash:      log.TxHash,
			Sender:      common.BytesToAddress(log.Topics[1].Bytes()), //TODO
			To:          common.BytesToAddress(log.Topics[1].Bytes()), //TODO
			Request:     requestd,
			Amount:      amount,
			Fee:         fee,
			Contract:    log.Address,
		})
	}
	fmt.Println("getMonitoringData contract", from, to, "res", len(rs))
	return rs
}
func (b *bridgeManager) CheckEthMonitorTxQueue() {
	int64Min := func(a, b int64) int64 {
		if a < b {
			return a
		}
		return b
	}

	//ts := time.Now()
	sb := b.redis.LLen(ctx, b.KeyQueueTx).Val()
	if sb == 0 {
		return
	}

	relayDats := b.redis.LRange(ctx, b.KeyQueueTx, 0, int64Min(3, sb))

	targetGasPrice, err := b.ethClient.SuggestGasPrice(ctx)
	checkError(err)
	targetGasPrice = targetGasPrice.Add(targetGasPrice, targetGasPrice.Div(targetGasPrice, new(big.Int).SetUint64(5))) // x *1.2
	for index, v := range relayDats.Val() {
		mm := new([]QKCTxDetail)
		err = rlp.DecodeBytes([]byte(v), mm)
		checkError(err)
		b.relayQKCMsg(*mm)
		b.redis.LSet(ctx, b.KeyQueueTx, int64(index), "DELETE")
	}
	b.redis.LRem(ctx, b.KeyQueueTx, 3, "DELETE")

}

func readLocalConfig(path string) *BridgeConfig {
	data, err := ioutil.ReadFile(path)
	checkError(err)
	c := new(BridgeConfig)
	err = json.Unmarshal(data, c)
	checkError(err)
	return c
}
func main() {
	config := readLocalConfig("./config.json")
	manager := NewBridgeManager(config)
	manager.EthMonitor()

	time.Sleep(1000000 * time.Second)
}

type TerraManager struct {
	config    *BridgeConfig
	ethClient *ethclient.Client
	qkcClient *cc.Client
	redis     *redis.Client

	qkcPrivate *ecdsa.PrivateKey

	KeyLastHeight   string
	KeyNextSequence string
	KeyQueueTx      string
}

func NewTerr(config *BridgeConfig) *TerraManager {
	return &TerraManager{
		config:          nil,
		ethClient:       nil,
		qkcClient:       nil,
		redis:           nil,
		qkcPrivate:      nil,
		KeyLastHeight:   "terr_key_last_height",
		KeyNextSequence: "terr_next_sequence",
		KeyQueueTx:      "terr_key_queue_tx",
	}
}

func (t *TerraManager) CheckQKCMonitorTxQueue() {
	int64Min := func(a, b int64) int64 {
		if a < b {
			return a
		}
		return b
	}

	//ts := time.Now()
	sb := t.redis.LLen(ctx, t.KeyQueueTx).Val()
	if sb == 0 {
		return
	}

	relayDats := t.redis.LRange(ctx, t.KeyQueueTx, 0, int64Min(3, sb))
	targetGasPrice, err := t.ethClient.SuggestGasPrice(ctx)
	checkError(err)
	targetGasPrice = targetGasPrice.Add(targetGasPrice, targetGasPrice.Div(targetGasPrice, new(big.Int).SetUint64(5))) // x *1.2
	for index, v := range relayDats.Val() {
		mm := new(ETHTxDetail)
		err = rlp.DecodeBytes([]byte(v), mm)
		checkError(err)

		if mm.Time.Second()-time.Now().Second() > 1000*60 {
			mm.Tx = t.makeEthTx(mm.toAddr, *(mm.Tx.To()), increaseGasPrice(mm.Tx.GasPrice()), mm.value)
		}

		t.redis.LSet(ctx, t.KeyQueueTx, int64(index), "DELETE")
	}
	t.redis.LRem(ctx, t.KeyQueueTx, 3, "DELETE")
}

func increaseGasPrice(data *big.Int) *big.Int {
	rs := new(big.Int).Set(data)
	rs = new(big.Int).Add(rs, new(big.Int).Div(data, new(big.Int).SetUint64(2)))
	return rs
}

func (t *TerraManager) relayerETH(detail *ETHTxDetail) {
	err := t.ethClient.SendTransaction(ctx, detail.Tx)
	checkError(err)
	for true {
		time.Sleep(1 * time.Second)
		rs, err := t.ethClient.TransactionReceipt(ctx, detail.TxHash)
		checkError(err)
		if rs.Status == 1 {
			break
		}
		fmt.Println("succc eth", detail.TxHash.String())
	}
}

type ETHTxDetail struct {
	TxHash common.Hash
	Tx     *types.Transaction
	Time   time.Time
	toAddr common.Address
	value  *big.Int
}

func (t *TerraManager) load(lastHeight uint64) (uint64, []*MonitoringData) {
	bb, err := t.qkcClient.GetMinorBlockByHeight(0, nil)
	checkError(err)
	newHeight := new(big.Int).SetBytes(common.FromHex(bb.Result.(string))).Uint64()
	newHeight -= t.config.ETHConfirmationBlock
	// skip no new blocks generated
	if lastHeight >= newHeight {
		return newHeight, nil
	}

	fromBlock := newHeight
	if lastHeight != 0 {
		fromBlock = lastHeight + 1
	}

	toBlock := newHeight
	if fromBlock+100 < newHeight {
		toBlock = fromBlock + 100
	}

	rs := make([]*MonitoringData, 0)
	for index := fromBlock; index <= toBlock; index++ {

	}

	return toBlock, rs
}

func (t *TerraManager) makeEthTx(toAddress, tokenAddress common.Address, gasPrice *big.Int, amount *big.Int) *types.Transaction {
	fromAddress := crypto.PubkeyToAddress(t.qkcPrivate.PublicKey)
	nonce, err := t.ethClient.PendingNonceAt(context.Background(), fromAddress)
	checkError(err)
	value := big.NewInt(0) // in wei (0 eth)
	if gasPrice == nil {
		gasPrice, err = t.ethClient.SuggestGasPrice(context.Background())
		checkError(err)
	}

	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]
	fmt.Println(hexutil.Encode(methodID)) // 0xa9059cbb

	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	fmt.Println(hexutil.Encode(paddedAddress)) // 0x0000000000000000000000004592d8f8d7b001e72cb26a73e4fa1806a51ac79d

	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	fmt.Println(hexutil.Encode(paddedAmount)) // 0x00000000000000000000000000000000000000000000003635c9adc5dea00000

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)
	gasLimit, err := t.ethClient.EstimateGas(context.Background(), ethereum.CallMsg{
		To:   &toAddress,
		Data: data,
	})
	checkError(err)

	tx := types.NewTransaction(nonce, tokenAddress, value, gasLimit, gasPrice, data)

	chainID, err := t.ethClient.NetworkID(context.Background())
	checkError(err)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), t.qkcPrivate)
	checkError(err)
	return signedTx

}

func (t *TerraManager) buildEthTx(datas []*MonitoringData) []*ETHTxDetail {
	txs := make([]*ETHTxDetail, 0)
	for _, v := range datas {
		tx := t.makeEthTx(v.To, v.Contract, nil, v.Amount)
		detail := &ETHTxDetail{
			TxHash: tx.Hash(),
			Tx:     tx,
			Time:   time.Now(),
		}
		bs, err := rlp.EncodeToBytes(detail)
		checkError(err)
		t.redis.LPush(ctx, t.KeyQueueTx, string(bs))
		txs = append(txs, detail)
	}
	return txs
}
func (t *TerraManager) QKCMonitor() {
	for true {
		t.CheckQKCMonitorTxQueue()
		lastHeight, err := t.redis.Get(ctx, t.KeyLastHeight).Uint64()
		if err != nil {
			lastHeight = 0
		}
		newLastHeight, monitorDatas := t.load(lastHeight)
		if len(monitorDatas) != 0 {
			txs := t.buildEthTx(monitorDatas)

			for _, v := range txs {
				t.relayerETH(v)
			}
		}
		fmt.Println("newLastHeight", newLastHeight, lastHeight)
		t.redis.Set(ctx, t.KeyQueueTx, newLastHeight, 0)
		if newLastHeight == lastHeight {
			time.Sleep(20 * time.Second)
		}
	}
}

//
//type qkcNoncePool struct {
//	curr map[common.Address]uint64
//}
//
//func newQkcNoncePool() *qkcNoncePool {
//	return &qkcNoncePool{curr: make(map[common.Address]uint64, 0)}
//}
//
//func (q *qkcNoncePool) getNonce(from common.Address) uint64 {
//	defer func() {
//		q.curr[from]++
//	}()
//	if data, ok := q.curr[from]; ok {
//		return data
//	}
//
//	nonce, err := qkcClient.GetNonce(&cc.QkcAddress{
//		Recipient:    from,
//		FullShardKey: 0,
//	})
//	if err != nil {
//		panic(err)
//	}
//	q.curr[from] = nonce
//	return nonce
//}
//
//func MonitorQKC() {
//	index := uint64(6174834)
//
//	for true {
//		//time.Sleep(1 * time.Second)
//		b, err := qkcClient.GetMinorBlockByHeight(0, new(big.Int).SetUint64(index))
//		if err != nil {
//			time.Sleep(1 * time.Second)
//			continue
//		}
//
//		txs := b.Result.(map[string]interface{})["transactions"]
//		fmt.Println("newHead on QKC", index)
//
//		if len(txs.([]interface{})) != 0 {
//			for _, v := range txs.([]interface{}) {
//				txHashByte, err := hex.DecodeString(v.(string)[2:])
//				rr, err := cc.ByteToTransactionId(txHashByte)
//				if err != nil {
//					panic(err)
//				}
//				rs, err := qkcClient.GetTransactionReceipt(rr)
//				if err != nil {
//					panic(err)
//				}
//				logs := rs.Result.(map[string]interface{})["logs"]
//
//				if len(logs.([]interface{})) != 0 {
//					for _, v := range logs.([]interface{}) {
//						topics := v.(map[string]interface{})["topics"]
//
//						if len(topics.([]interface{})) != 0 {
//							fmt.Println("vvvv", reflect.TypeOf(topics), index)
//							if topics.([]interface{})[0].(string) == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
//								from := common.BytesToAddress(common.Hex2Bytes(topics.([]interface{})[1].(string)[2:]))
//								to := common.BytesToAddress(common.Hex2Bytes(topics.([]interface{})[2].(string)[2:]))
//								value := new(big.Int).SetBytes(common.Hex2Bytes(v.(map[string]interface{})["data"].(string)[2:]))
//
//								if bytes.Equal(to.Bytes(), common.HexToAddress(addr1).Bytes()) {
//									fmt.Println("from", from.String(), "to", to.String(), "value", value)
//									TransferOnEth(from, value)
//								}
//							}
//						}
//					}
//				}
//			}
//			fmt.Println()
//		}
//		index++
//	}
//
//}
//
//func MonitorETH() {
//
//}
//
//func makeMintMsg(addr common.Address, value *big.Int) []byte {
//	rs := make([]byte, 0)
//	rs = append(rs, common.FromHex("0x40c10f19000000000000000000000000")...)
//	rs = append(rs, addr.Bytes()...)
//	rs = append(rs, common.BigToHash(value).Bytes()...)
//	return rs
//}

func MintMsg(addr uint64, value *big.Int) []byte {
	rs := make([]byte, 0)
	rs = append(rs, common.FromHex("0x0f2dc31a")...)
	rs = append(rs, common.BigToHash(new(big.Int).SetUint64(addr)).Bytes()...)
	rs = append(rs, common.BigToHash(value).Bytes()...)
	return rs

}

//func makeTransferMsg(addr common.Address, value *big.Int) []byte {
//	rs := make([]byte, 0)
//	rs = append(rs, common.FromHex("0xa9059cbb000000000000000000000000")...)
//	rs = append(rs, addr.Bytes()...)
//	rs = append(rs, common.BigToHash(value).Bytes()...)
//	return rs
//}

//func MintOnQKC(addr common.Address, value *big.Int) {
//	//fmt.Println("MMMMMMMMMMMMMMMMMMM", addr.String())
//	prvkey, _ := crypto.ToECDSA(common.FromHex(pri1))
//	from := crypto.PubkeyToAddress(prvkey.PublicKey)
//
//	//合约代码："0x60806040523480156200001157600080fd5b506040518060400160405280600681526020017f53434633333300000000000000000000000000000000000000000000000000008152506040518060400160405280600681526020017f7363663333330000000000000000000000000000000000000000000000000000815250818181600390805190602001906200009892919062000191565b508060049080519060200190620000b192919062000191565b506012600560006101000a81548160ff021916908360ff16021790555050506000620000e26200018960201b60201c565b905080600560016101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508073ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a350505062000240565b600033905090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10620001d457805160ff191683800117855562000205565b8280016001018555821562000205579182015b8281111562000204578251825591602001919060010190620001e7565b5b50905062000214919062000218565b5090565b6200023d91905b80821115620002395760008160009055506001016200021f565b5090565b90565b611bac80620002506000396000f3fe608060405234801561001057600080fd5b50600436106101005760003560e01c8063715018a611610097578063a9059cbb11610066578063a9059cbb146104ff578063bcf64e0514610565578063dd62ed3e1461059d578063f2fde38b1461061557610100565b8063715018a6146103c25780638da5cb5b146103cc57806395d89b4114610416578063a457c2d71461049957610100565b8063313ce567116100d3578063313ce5671461029257806339509351146102b657806340c10f191461031c57806370a082311461036a57610100565b806306fdde0314610105578063095ea7b31461018857806318160ddd146101ee57806323b872dd1461020c575b600080fd5b61010d610659565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561014d578082015181840152602081019050610132565b50505050905090810190601f16801561017a5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6101d46004803603604081101561019e57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506106fb565b604051808215151515815260200191505060405180910390f35b6101f6610719565b6040518082815260200191505060405180910390f35b6102786004803603606081101561022257600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610723565b604051808215151515815260200191505060405180910390f35b61029a6107fc565b604051808260ff1660ff16815260200191505060405180910390f35b610302600480360360408110156102cc57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610813565b604051808215151515815260200191505060405180910390f35b6103686004803603604081101561033257600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506108c6565b005b6103ac6004803603602081101561038057600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061099e565b6040518082815260200191505060405180910390f35b6103ca6109e6565b005b6103d4610b71565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b61041e610b9b565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561045e578082015181840152602081019050610443565b50505050905090810190601f16801561048b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6104e5600480360360408110156104af57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610c3d565b604051808215151515815260200191505060405180910390f35b61054b6004803603604081101561051557600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610d0a565b604051808215151515815260200191505060405180910390f35b61059b6004803603604081101561057b57600080fd5b810190808035906020019092919080359060200190929190505050610d28565b005b6105ff600480360360408110156105b357600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610d93565b6040518082815260200191505060405180910390f35b6106576004803603602081101561062b57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610e1a565b005b606060038054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156106f15780601f106106c6576101008083540402835291602001916106f1565b820191906000526020600020905b8154815290600101906020018083116106d457829003601f168201915b5050505050905090565b600061070f61070861102a565b8484611032565b6001905092915050565b6000600254905090565b6000610730848484611229565b6107f18461073c61102a565b6107ec85604051806060016040528060288152602001611ac060289139600160008b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006107a261102a565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546114ea9092919063ffffffff16565b611032565b600190509392505050565b6000600560009054906101000a900460ff16905090565b60006108bc61082061102a565b846108b7856001600061083161102a565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546115aa90919063ffffffff16565b611032565b6001905092915050565b6108ce61102a565b73ffffffffffffffffffffffffffffffffffffffff16600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614610990576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e657281525060200191505060405180910390fd5b61099a8282611632565b5050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b6109ee61102a565b73ffffffffffffffffffffffffffffffffffffffff16600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614610ab0576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e657281525060200191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a36000600560016101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550565b6000600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905090565b606060048054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610c335780601f10610c0857610100808354040283529160200191610c33565b820191906000526020600020905b815481529060010190602001808311610c1657829003601f168201915b5050505050905090565b6000610d00610c4a61102a565b84610cfb85604051806060016040528060258152602001611b526025913960016000610c7461102a565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008a73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546114ea9092919063ffffffff16565b611032565b6001905092915050565b6000610d1e610d1761102a565b8484611229565b6001905092915050565b610d39610d3361102a565b836117f9565b80610d4261102a565b73ffffffffffffffffffffffffffffffffffffffff167fc3599666213715dfabdf658c56a97b9adfad2cd9689690c70c79b20bc61940c9846040518082815260200191505060405180910390a35050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b610e2261102a565b73ffffffffffffffffffffffffffffffffffffffff16600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614610ee4576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260208152602001807f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e657281525060200191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415610f6a576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526026815260200180611a526026913960400191505060405180910390fd5b8073ffffffffffffffffffffffffffffffffffffffff16600560019054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a380600560016101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b600033905090565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1614156110b8576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526024815260200180611b2e6024913960400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141561113e576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526022815260200180611a786022913960400191505060405180910390fd5b80600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925836040518082815260200191505060405180910390a3505050565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1614156112af576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526025815260200180611b096025913960400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff161415611335576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526023815260200180611a0d6023913960400191505060405180910390fd5b6113408383836119bd565b6113ab81604051806060016040528060268152602001611a9a602691396000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546114ea9092919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555061143e816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546115aa90919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3505050565b6000838311158290611597576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561155c578082015181840152602081019050611541565b50505050905090810190601f1680156115895780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b5060008385039050809150509392505050565b600080828401905083811015611628576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601b8152602001807f536166654d6174683a206164646974696f6e206f766572666c6f77000000000081525060200191505060405180910390fd5b8091505092915050565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1614156116d5576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f45524332303a206d696e7420746f20746865207a65726f20616464726573730081525060200191505060405180910390fd5b6116e1600083836119bd565b6116f6816002546115aa90919063ffffffff16565b60028190555061174d816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546115aa90919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a35050565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141561187f576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526021815260200180611ae86021913960400191505060405180910390fd5b61188b826000836119bd565b6118f681604051806060016040528060228152602001611a30602291396000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546114ea9092919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555061194d816002546119c290919063ffffffff16565b600281905550600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a35050565b505050565b6000611a0483836040518060400160405280601e81526020017f536166654d6174683a207375627472616374696f6e206f766572666c6f7700008152506114ea565b90509291505056fe45524332303a207472616e7366657220746f20746865207a65726f206164647265737345524332303a206275726e20616d6f756e7420657863656564732062616c616e63654f776e61626c653a206e6577206f776e657220697320746865207a65726f206164647265737345524332303a20617070726f766520746f20746865207a65726f206164647265737345524332303a207472616e7366657220616d6f756e7420657863656564732062616c616e636545524332303a207472616e7366657220616d6f756e74206578636565647320616c6c6f77616e636545524332303a206275726e2066726f6d20746865207a65726f206164647265737345524332303a207472616e736665722066726f6d20746865207a65726f206164647265737345524332303a20617070726f76652066726f6d20746865207a65726f206164647265737345524332303a2064656372656173656420616c6c6f77616e63652062656c6f77207a65726fa2646970667358221220338f38308607cc9178171f652759077d2d0259508b5df15cfb552ce26a7a5d3c64736f6c63430006020033"
//
//	tx, err := qkcClient.CreateTransaction(nonceManger.getNonce(from), &cc.QkcAddress{Recipient: from, FullShardKey: 0}, &cc.QkcAddress{Recipient: qkcContract, FullShardKey: 0}, new(big.Int), uint64(3000000), new(big.Int).SetUint64(1000000000), makeMintMsg(addr, value))
//
//	if err != nil {
//		fmt.Println(err.Error())
//	}
//	tx, err = cc.SignTx(tx, prvkey)
//	if err != nil {
//		fmt.Println(err.Error())
//	}
//
//	txid, err := qkcClient.SendTransaction(tx)
//	if err != nil {
//		fmt.Println("SendTransaction error: ", err.Error())
//	}
//
//	fmt.Println("QKC主网 Mint成功", "to", addr.String(), "value", value, "0x"+common.Bytes2Hex(txid))
//}

//func testTransferTokenOnQkc(addr common.Address, value *big.Int) {
//	prvkey, _ := crypto.ToECDSA(common.FromHex(pri2))
//	from := crypto.PubkeyToAddress(prvkey.PublicKey)
//
//	tx, err := qkcClient.CreateTransaction(nonceManger.getNonce(from), &cc.QkcAddress{Recipient: from, FullShardKey: 0}, &cc.QkcAddress{Recipient: qkcContract, FullShardKey: 0}, new(big.Int), uint64(3000000), new(big.Int).SetUint64(1000000000), makeTransferMsg(addr, value))
//
//	if err != nil {
//		fmt.Println(err.Error())
//	}
//	tx, err = cc.SignTx(tx, prvkey)
//	if err != nil {
//		fmt.Println(err.Error())
//	}
//
//	txid, err := qkcClient.SendTransaction(tx)
//	if err != nil {
//		fmt.Println("SendTransaction error: ", err.Error())
//	}
//
//	fmt.Println("QKC主网 transfer addr2->addr1", "from", from.String(), "to", addr.String(), "value", value, "0x"+common.Bytes2Hex(txid))
//}
//
//func main() {
//	config := readLocalConfig("./config.json")
//	b := NewBridgeManager(config)
//
//	instance, err := token.NewErcToken(b.config.ETHContract[0], b.ethClient)
//	checkError(err)
//
//	name, err := instance.Name(&bind.CallOpts{})
//	checkError(err)
//	fmt.Printf("币种名字: %s\n", name)
//
//	fmt.Println("准备测试", "ETH => QKC")
//
//	auth := bind.NewKeyedTransactor(b.qkcPrivate)
//
//	toMintAddr := common.HexToAddress("0xFf4f755E64fb5975f83Aa516adC6A3D97Ee19F12")
//	toMintValue := new(big.Int).Mul(new(big.Int).SetUint64(6), wei)
//
//	tx, err := instance.Mint(auth, toMintAddr, toMintValue)
//	fmt.Println("ropsten网络 mint addr2", "addr", toMintAddr.String(), "value", toMintValue.String(), "tx", tx.Hash().String())
//	if err != nil {
//		panic(err)
//	}
//	//
//	//Sleep(tx.Hash())
//	//fmt.Println("初始化完成")
//	//time.Sleep(100000000 * time.Second)
//	//
//	//pr, err = crypto.ToECDSA(common.FromHex(pri2))
//	//auth = bind.NewKeyedTransactor(pr)
//	//toTransferAddr := common.HexToAddress(addr1)
//	//toTransferValue := new(big.Int).Mul(new(big.Int).SetUint64(6), wei)
//	//tx, err = instance.Transfer(auth, toTransferAddr, toTransferValue)
//	//fmt.Println("ropsten网络 tranfer addr2->addr1", "from", crypto.PubkeyToAddress(pr.PublicKey).String(), "to", toTransferAddr.String(), "value", toMintValue, "tx", tx.Hash().String())
//	//if err != nil {
//	//	panic(err)
//	//}
//	//Sleep(tx.Hash())
//	//
//	//fmt.Println("准备测试 QKC => ETH")
//	//toTransferValue = new(big.Int).Mul(new(big.Int).SetUint64(1), wei)
//	//testTransferTokenOnQkc(toTransferAddr, toTransferValue)
//	//time.Sleep(1000 * time.Second)
//}
