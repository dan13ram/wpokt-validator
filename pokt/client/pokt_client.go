package client

import (
	"io"

	log "github.com/sirupsen/logrus"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dan13ram/wpokt-validator/app"
	"github.com/pokt-network/pocket-core/app/cmd/rpc"
)

type PocketClient interface {
	GetBlock() (*BlockResponse, error)
	GetHeight() (*HeightResponse, error)
	SubmitRawTx(params rpc.SendRawTxParams) (*SubmitRawTxResponse, error)
	GetTx(hash string) (*TxResponse, error)
	GetAccountTxsByHeight(address string, height int64) ([]*TxResponse, error)
	ValidateNetwork()
}

type pocketClient struct{}

var (
	Client PocketClient = &pocketClient{}
)

var (
	sendRawTxPath,
	getNodePath,
	getACLPath,
	getUpgradePath,
	getDAOOwnerPath,
	getHeightPath,
	getAccountPath,
	getAppPath,
	getTxPath,
	getBlockPath,
	getSupportedChainsPath,
	getBalancePath,
	getAccountTxsPath,
	getNodeParamsPath,
	getNodesPath,
	getSigningInfoPath,
	getAppsPath,
	getAppParamsPath,
	getPocketParamsPath,
	getNodeClaimsPath,
	getNodeClaimPath,
	getBlockTxsPath,
	getSupplyPath,
	getAllParamsPath,
	getParamPath,
	getStopPath,
	getQueryChains,
	getAccountsPath string
)

func init() {
	routes := rpc.GetRoutes()
	for _, route := range routes {
		switch route.Name {
		case "SendRawTx":
			sendRawTxPath = route.Path
		case "QueryNode":
			getNodePath = route.Path
		case "QueryACL":
			getACLPath = route.Path
		case "QueryUpgrade":
			getUpgradePath = route.Path
		case "QueryDAOOwner":
			getDAOOwnerPath = route.Path
		case "QueryHeight":
			getHeightPath = route.Path
		case "QueryAccount":
			getAccountPath = route.Path
		case "QueryAccounts":
			getAccountsPath = route.Path
		case "QueryApp":
			getAppPath = route.Path
		case "QueryTX":
			getTxPath = route.Path
		case "QueryBlock":
			getBlockPath = route.Path
		case "QuerySupportedChains":
			getSupportedChainsPath = route.Path
		case "QueryBalance":
			getBalancePath = route.Path
		case "QueryAccountTxs":
			getAccountTxsPath = route.Path
		case "QueryNodeParams":
			getNodeParamsPath = route.Path
		case "QueryNodes":
			getNodesPath = route.Path
		case "QuerySigningInfo":
			getSigningInfoPath = route.Path
		case "QueryApps":
			getAppsPath = route.Path
		case "QueryAppParams":
			getAppParamsPath = route.Path
		case "QueryPocketParams":
			getPocketParamsPath = route.Path
		case "QueryBlockTxs":
			getBlockTxsPath = route.Path
		case "QuerySupply":
			getSupplyPath = route.Path
		case "QueryNodeClaim":
			getNodeClaimPath = route.Path
		case "QueryNodeClaims":
			getNodeClaimsPath = route.Path
		case "QueryAllParams":
			getAllParamsPath = route.Path
		case "QueryParam":
			getParamPath = route.Path
		case "Stop":
			getStopPath = route.Path
		case "QueryChains":
			getQueryChains = route.Path
		default:
			continue
		}
	}
}

func queryRPC(path string, jsonArgs []byte) (string, error) {
	cliURL := app.Config.Pocket.RPCURL + path
	log.Debugln("[POKT] Querying RPC", cliURL)

	req, err := http.NewRequest("POST", cliURL, bytes.NewBuffer(jsonArgs))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: time.Millisecond * time.Duration(app.Config.Pocket.RPCTimeoutMillis),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	res, err := strconv.Unquote(string(bz))
	if err == nil {
		bz = []byte(res)
	}

	// Check if the status code is 2XX
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, bz, "", "    ")
		if err == nil {
			res := prettyJSON.String()
			var obj RPCError
			innerErr := json.Unmarshal([]byte(res), &obj)
			if innerErr == nil && obj.Error.Code != 0 {
				return "", fmt.Errorf("the rpc response returned an error: %+v", obj.Error)
			}
			return res, nil
		}

		return string(bz), nil
	}
	return "", fmt.Errorf("the http status code was not okay: %d, with a response of %+v", resp.StatusCode, resp)
}

func (c *pocketClient) GetBlock() (*BlockResponse, error) {
	res, err := queryRPC(getBlockPath, []byte{})
	if err != nil {
		return nil, err
	}
	var obj BlockResponse
	err = json.Unmarshal([]byte(res), &obj)
	return &obj, err
}

func (c *pocketClient) GetHeight() (*HeightResponse, error) {
	res, err := queryRPC(getHeightPath, []byte{})
	if err != nil {
		return nil, err
	}
	var obj HeightResponse
	err = json.Unmarshal([]byte(res), &obj)
	return &obj, err
}

func (c *pocketClient) GetTx(hash string) (*TxResponse, error) {
	params := rpc.HashAndProveParams{Hash: hash, Prove: false}
	j, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	res, err := queryRPC(getTxPath, j)
	if err != nil {
		return nil, err
	}
	var obj TxResponse
	err = json.Unmarshal([]byte(res), &obj)
	return &obj, err
}

func (c *pocketClient) SubmitRawTx(params rpc.SendRawTxParams) (*SubmitRawTxResponse, error) {
	j, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	res, err := queryRPC(sendRawTxPath, j)
	if err != nil {
		return nil, err
	}
	var obj SubmitRawTxResponse
	err = json.Unmarshal([]byte(res), &obj)
	return &obj, err
}

func (c *pocketClient) getAccountTxsPerPage(address string, page uint32) (*AccountTxsResponse, error) {
	// filter by received transactions
	params := rpc.PaginateAddrParams{
		Address:  address,
		Page:     int(page),
		PerPage:  50,
		Received: true,
		Prove:    false,
		Sort:     "asc",
	}
	j, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	res, err := queryRPC(getAccountTxsPath, j)
	if err != nil {
		return nil, err
	}
	var obj AccountTxsResponse
	err = json.Unmarshal([]byte(res), &obj)
	return &obj, err
}

func (c *pocketClient) GetAccountTxsByHeight(address string, height int64) ([]*TxResponse, error) {
	var txs []*TxResponse
	var page uint32 = 1
	for {
		res, err := c.getAccountTxsPerPage(address, page)
		if err != nil {
			return nil, err
		}
		// filter only type pos/Send
		for _, tx := range res.Txs {
			if tx.StdTx.Msg.Type == "pos/Send" && tx.Height >= height {
				txs = append(txs, tx)
			}
		}
		if len(res.Txs) == 0 || len(txs) >= int(res.TotalTxs) || res.Txs[len(res.Txs)-1].Height < height {
			break
		}
		page++
	}

	return txs, nil
}

func (c *pocketClient) ValidateNetwork() {
	log.Debugln("[POKT] Validating network")
	log.Debugln("[POKT] uri", app.Config.Pocket.RPCURL)
	res, err := c.GetBlock()
	if err != nil {
		log.Fatalln("[POKT] Error getting block", err)
	}
	height, err := c.GetHeight()
	if err != nil {
		log.Fatalln("[POKT] Error getting height", err)
	}
	if res.Block.Header.ChainID != app.Config.Pocket.ChainId {
		log.Fatalln("[POKT] Chain ID mismatch", "expected", app.Config.Pocket.ChainId, "got", res.Block.Header.ChainID)
	}
	log.Debugln("[POKT]", "chainId", res.Block.Header.ChainID)

	blockHeight, err := strconv.Atoi(res.Block.Header.Height)
	if err != nil {
		log.Fatalln("[POKT] Error parsing height", err)
	}
	if height.Height-int64(blockHeight) > 3 {
		log.Fatalln("[POKT] Height mismatch", "expected", height.Height, "got", res.Block.Header.Height)
	}
	log.Debugln("[POKT]", "height", res.Block.Header.Height)
	log.Infoln("[POKT] Validated network")
}

func NewClient() PocketClient {
	return &pocketClient{}
}
