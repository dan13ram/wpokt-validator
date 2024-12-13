package eth

import (
	"context"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/dan13ram/wpokt-validator/app"
	"github.com/dan13ram/wpokt-validator/eth/autogen"
	eth "github.com/dan13ram/wpokt-validator/eth/client"
	"github.com/dan13ram/wpokt-validator/eth/util"
	"github.com/dan13ram/wpokt-validator/models"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	BurnMonitorName = "BURN MONITOR"
)

type BurnMonitorRunner struct {
	startBlockNumber   int64
	currentBlockNumber int64
	wpoktContract      eth.WrappedPocketContract
	client             eth.EthereumClient
	minimumAmount      *big.Int
}

func (x *BurnMonitorRunner) Run() {
	x.UpdateCurrentBlockNumber()
	x.SyncTxs()
}

func (x *BurnMonitorRunner) Status() models.RunnerStatus {
	return models.RunnerStatus{
		EthBlockNumber: strconv.FormatInt(x.startBlockNumber, 10),
	}
}

func (x *BurnMonitorRunner) UpdateCurrentBlockNumber() {
	res, err := x.client.GetBlockNumber()
	if err != nil {
		log.Error("[BURN MONITOR] Error while getting current block number: ", err)
		return
	}
	x.currentBlockNumber = int64(res)
	log.Info("[BURN MONITOR] Current block number: ", x.currentBlockNumber)
}

func (x *BurnMonitorRunner) HandleBurnEvent(event *autogen.WrappedPocketBurnAndBridge) bool {
	if event == nil {
		log.Error("[BURN MONITOR] Error while handling burn event: event is nil")
		return false
	}

	doc := util.CreateBurn(event)

	// each event is a combination of transaction hash and log index
	log.Debug("[BURN MONITOR] Handling burn event: ", event.Raw.TxHash, " ", event.Raw.Index)

	err := app.DB.InsertOne(models.CollectionBurns, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			log.Info("[BURN MONITOR] Found duplicate burn event: ", event.Raw.TxHash, " ", event.Raw.Index)
			return true
		}
		log.Error("[BURN MONITOR] Error while storing burn event in db: ", err)
		return false
	}

	log.Info("[BURN MONITOR] Stored burn event: ", event.Raw.TxHash, " ", event.Raw.Index)
	return true
}

func (x *BurnMonitorRunner) SyncBlocks(startBlockNumber uint64, endBlockNumber uint64) bool {
	filter, err := x.wpoktContract.FilterBurnAndBridge(&bind.FilterOpts{
		Start:   startBlockNumber,
		End:     &endBlockNumber,
		Context: context.Background(),
	}, []*big.Int{}, []common.Address{}, []common.Address{})

	if filter != nil {
		defer filter.Close()
	}

	if err != nil {
		log.Error("[BURN MONITOR] Error while syncing burn events: ", err)
		return false
	}

	var success bool = true
	for filter.Next() {
		if err := filter.Error(); err != nil {
			success = false
			break
		}

		event := filter.Event()

		if event == nil {
			success = false
			continue
		}

		if event.Raw.Removed || event.Amount.Cmp(x.minimumAmount) != 1 {
			continue
		}

		success = x.HandleBurnEvent(event) && success
	}

	if err := filter.Error(); err != nil {
		log.Error("[BURN MONITOR] Error while syncing burn events: ", err)
		return false
	}

	return success
}

func (x *BurnMonitorRunner) SyncTxs() bool {
	if x.currentBlockNumber <= x.startBlockNumber {
		log.Info("[BURN MONITOR] No new blocks to sync")
		return true
	}

	var success bool = true
	if (x.currentBlockNumber - x.startBlockNumber) > eth.MAX_QUERY_BLOCKS {
		log.Debug("[BURN MONITOR] Syncing burn txs in chunks")
		for i := x.startBlockNumber; i < x.currentBlockNumber; i += eth.MAX_QUERY_BLOCKS {
			endBlockNumber := i + eth.MAX_QUERY_BLOCKS
			if endBlockNumber > x.currentBlockNumber {
				endBlockNumber = x.currentBlockNumber
			}
			log.Info("[BURN MONITOR] Syncing burn txs from blockNumber: ", i, " to blockNumber: ", endBlockNumber)
			success = success && x.SyncBlocks(uint64(i), uint64(endBlockNumber))
		}
	} else {
		log.Info("[BURN MONITOR] Syncing burn txs from blockNumber: ", x.startBlockNumber, " to blockNumber: ", x.currentBlockNumber)
		success = success && x.SyncBlocks(uint64(x.startBlockNumber), uint64(x.currentBlockNumber))
	}

	if success {
		x.startBlockNumber = x.currentBlockNumber
	}

	return success
}

func (x *BurnMonitorRunner) InitStartBlockNumber(lastHealth models.ServiceHealth) {
	startBlockNumber := int64(app.Config.Ethereum.StartBlockNumber)

	if lastBlockNumber, err := strconv.ParseInt(lastHealth.EthBlockNumber, 10, 64); err == nil {
		startBlockNumber = lastBlockNumber
	}

	if startBlockNumber > 0 {
		x.startBlockNumber = startBlockNumber
	} else {
		log.Warn("Found invalid start block number, updating to current block number")
		x.startBlockNumber = x.currentBlockNumber
	}

	log.Info("[BURN MONITOR] Start block number: ", x.startBlockNumber)
}

func NewBurnMonitor(wg *sync.WaitGroup, lastHealth models.ServiceHealth) app.Service {
	if !app.Config.BurnMonitor.Enabled {
		log.Debug("[BURM MONITOR] Disabled")
		return app.NewEmptyService(wg)
	}

	log.Debug("[BURN MONITOR] Initializing burn monitor")
	client, err := eth.NewClient()
	if err != nil {
		log.Fatal("[BURN MONITOR] Error initializing ethereum client: ", err)
	}
	log.Debug("[BURN MONITOR] Connecting to wpokt contract at: ", app.Config.Ethereum.WrappedPocketAddress)
	contract, err := autogen.NewWrappedPocket(common.HexToAddress(app.Config.Ethereum.WrappedPocketAddress), client.GetClient())
	if err != nil {
		log.Fatal("[BURN MONITOR] Error connecting to wpokt contract: ", err)
	}

	log.Debug("[BURN MONITOR] Connected to wpokt contract")

	x := &BurnMonitorRunner{
		startBlockNumber:   0,
		currentBlockNumber: 0,
		wpoktContract:      eth.NewWrappedPocketContract(contract),
		client:             client,
		minimumAmount:      big.NewInt(app.Config.Pocket.TxFee),
	}

	x.UpdateCurrentBlockNumber()

	x.InitStartBlockNumber(lastHealth)

	log.Info("[BURN MONITOR] Initialized burn monitor")

	return app.NewRunnerService(
		BurnMonitorName,
		x,
		wg,
		time.Duration(app.Config.BurnMonitor.IntervalMillis)*time.Millisecond,
	)
}
