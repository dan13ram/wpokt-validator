package ethereum

import (
	"context"
	"time"

	"math/big"

	"github.com/dan13ram/wpokt-backend/app"
	"github.com/ethereum/go-ethereum/ethclient"

	log "github.com/sirupsen/logrus"
)

var Client *ethclient.Client

func ValidateNetwork() {
	var err error
	log.Debugln("Connecting to Ethereum node", "url", app.Config.Ethereum.RPCURL)
	Client, err = ethclient.Dial(app.Config.Ethereum.RPCURL)
	if err != nil {
		panic(err)
	}

	blockNumber, err := GetBlockNumber()
	if err != nil {
		panic(err)
	}
	log.Debugln("Connected to Ethereum node", "blockNumber", blockNumber)

	chainId, err := GetChainId()
	if err != nil {
		panic(err)
	}

	if chainId.Uint64() != app.Config.Ethereum.ChainId {
		log.Debugln("ethereum chainId mismatch", "config", app.Config.Ethereum.ChainId, "node", chainId.Int64())
		panic("ethereum chain id mismatch")
	}
	log.Debugln("Connected to Ethereum node", "chainId", chainId.String())
}

func GetBlockNumber() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.Config.Ethereum.RPCTimeOutSecs)*time.Second)
	defer cancel()

	blockNumber, err := Client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}

	return blockNumber, nil
}

func GetChainId() (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.Config.Ethereum.RPCTimeOutSecs)*time.Second)
	defer cancel()

	chainId, err := Client.ChainID(ctx)
	if err != nil {
		return nil, err
	}

	return chainId, nil
}
