package pokt

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dan13ram/wpokt-validator/app"
	"github.com/dan13ram/wpokt-validator/eth/autogen"
	eth "github.com/dan13ram/wpokt-validator/eth/client"
	"github.com/dan13ram/wpokt-validator/models"
	pokt "github.com/dan13ram/wpokt-validator/pokt/client"
	"github.com/dan13ram/wpokt-validator/pokt/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pokt-network/pocket-core/crypto"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	BurnSignerName = "BURN SIGNER"
)

type BurnSignerRunner struct {
	privateKey     crypto.PrivateKey
	multisigPubKey crypto.PublicKeyMultiSig
	numSigners     int
	ethClient      eth.EthereumClient
	poktClient     pokt.PocketClient
	poktHeight     int64
	ethBlockNumber int64
	vaultAddress   string
	wpoktAddress   string
	wpoktContract  eth.WrappedPocketContract
	minimumAmount  *big.Int
}

func (x *BurnSignerRunner) Run() {
	x.UpdateBlocks()
	x.SyncTxs()
}
func (x *BurnSignerRunner) Status() models.RunnerStatus {
	return models.RunnerStatus{
		PoktHeight:     strconv.FormatInt(x.poktHeight, 10),
		EthBlockNumber: strconv.FormatInt(x.ethBlockNumber, 10),
	}
}

func (x *BurnSignerRunner) UpdateBlocks() {
	log.Debug("[BURN SIGNER] Updating blocks")

	poktHeight, err := x.poktClient.GetHeight()
	if err != nil {
		log.Error("[BURN SIGNER] Error fetching pokt block height: ", err)
		return
	}
	x.poktHeight = poktHeight.Height

	ethBlockNumber, err := x.ethClient.GetBlockNumber()
	if err != nil {
		log.Error("[BURN SIGNER] Error fetching eth block number: ", err)
		return
	}
	x.ethBlockNumber = int64(ethBlockNumber)

	log.Info("[BURN SIGNER] Updated blocks")
}

func (x *BurnSignerRunner) ValidateInvalidMint(doc *models.InvalidMint) (bool, error) {
	log.Debug("[BURN SIGNER] Validating invalid mint: ", doc.TransactionHash)

	tx, err := x.poktClient.GetTx(doc.TransactionHash)
	if err != nil {
		return false, errors.New("Error fetching transaction: " + err.Error())
	}

	if tx == nil || tx.Tx == "" {
		return false, errors.New("Transaction not found")
	}

	if tx.TxResult.Code != 0 {
		log.Debug("[BURN SIGNER] Transaction failed")
		return false, nil
	}

	if tx.TxResult.MessageType != "send" || tx.StdTx.Msg.Type != "pos/Send" {
		log.Debug("[BURN SIGNER] Transaction message type is not send")
		return false, nil
	}

	if strings.EqualFold(tx.StdTx.Msg.Value.ToAddress, "0000000000000000000000000000000000000000") {
		log.Debug("[BURN SIGNER] Transaction recipient is zero address")
		return false, nil
	}

	if !strings.EqualFold(tx.StdTx.Msg.Value.ToAddress, x.vaultAddress) {
		log.Debug("[BURN SIGNER] Transaction recipient is not vault address")
		return false, nil
	}

	if !strings.EqualFold(tx.StdTx.Msg.Value.FromAddress, doc.SenderAddress) {
		log.Debug("[BURN SIGNER] Transaction signer is not sender address")
		return false, nil
	}

	amount, ok := new(big.Int).SetString(tx.StdTx.Msg.Value.Amount, 10)

	if !ok || amount.Cmp(x.minimumAmount) != 1 {
		log.Debug("[BURN SIGNER] Transaction amount too low")
		return false, nil
	}

	if tx.StdTx.Msg.Value.Amount != doc.Amount {
		log.Debug("[BURN SIGNER] Transaction amount does not match invalid mint amount")
		return false, nil
	}

	if tx.StdTx.Memo != doc.Memo {
		log.Debug("[BURN SIGNER] Memo mismatch")
		return false, nil
	}

	_, valid := util.ValidateMemo(doc.Memo)
	if valid {
		log.Error("[BURN SIGNER] Memo is valid, should be invalid")
		return false, nil
	}

	log.Debug("[BURN SIGNER] Validated invalid mint")
	return true, nil
}

func (x *BurnSignerRunner) HandleInvalidMint(doc *models.InvalidMint) bool {
	if doc == nil {
		log.Error("[BURN SIGNER] Invalid mint is nil")
		return false
	}
	log.Debug("[BURN SIGNER] Handling invalid mint: ", doc.TransactionHash)

	doc, err := util.UpdateStatusAndConfirmationsForInvalidMint(doc, x.poktHeight)
	if err != nil {
		log.Error("[BURN SIGNER] Error getting invalid mint status: ", err)
		return false
	}

	var update bson.M

	valid, err := x.ValidateInvalidMint(doc)
	if err != nil {
		log.Error("[BURN SIGNER] Error validating invalid mint: ", err)
		return false
	}

	if !valid {
		log.Error("[BURN SIGNER] Invalid mint failed validation")
		update = bson.M{
			"$set": bson.M{
				"status":     models.StatusFailed,
				"updated_at": time.Now(),
			},
		}
		if doc.Confirmations == "0" {
			log.Debug("[BURN SIGNER] Invalid mint has no confirmations, skipping")
			return false
		}
	} else {

		if doc.Status == models.StatusConfirmed {
			log.Debug("[BURN SIGNER] Signing invalid mint")

			doc, err = util.SignInvalidMint(doc, x.privateKey, x.multisigPubKey, x.numSigners)
			if err != nil {
				log.Error("[BURN SIGNER] Error signing invalid mint: ", err)
				return false
			}

			update = bson.M{
				"$set": bson.M{
					"return_tx":     doc.ReturnTx,
					"signers":       doc.Signers,
					"status":        doc.Status,
					"confirmations": doc.Confirmations,
					"updated_at":    time.Now(),
				},
			}
		} else {
			log.Debug("[BURN SIGNER] Not signing invalid mint")
			update = bson.M{
				"$set": bson.M{
					"status":        doc.Status,
					"confirmations": doc.Confirmations,
					"updated_at":    time.Now(),
				},
			}
		}
	}

	filter := bson.M{
		"_id":    doc.Id,
		"status": bson.M{"$in": []string{models.StatusPending, models.StatusConfirmed}},
	}
	err = app.DB.UpdateOne(models.CollectionInvalidMints, filter, update)
	if err != nil {
		log.Error("[BURN SIGNER] Error updating invalid mint: ", err)
		return false
	}
	log.Info("[BURN SIGNER] Handled invalid mint: ", doc.TransactionHash)
	return true
}

func (x *BurnSignerRunner) ValidateBurn(doc *models.Burn) (bool, error) {
	log.Debug("[BURN SIGNER] Validating burn: ", doc.TransactionHash)

	txReceipt, err := x.ethClient.GetTransactionReceipt(doc.TransactionHash)

	if err != nil {
		return false, errors.New("Error fetching transaction receipt: " + err.Error())
	}

	logIndex, err := strconv.Atoi(doc.LogIndex)
	if err != nil {
		log.Debug("[BURN SIGNER] Error converting log index to int: ", err)
		return false, nil
	}

	var burnLog *types.Log

	for _, log := range txReceipt.Logs {
		if log.Index == uint(logIndex) {
			burnLog = log
			break
		}
	}

	if burnLog == nil {
		log.Debug("[BURN SIGNER] Burn log not found")
		return false, nil
	}

	burnEvent, err := x.wpoktContract.ParseBurnAndBridge(*burnLog)
	if err != nil {
		log.Error("[BURN SIGNER] Error parsing burn event: ", err)
		return false, nil
	}

	amount, ok := new(big.Int).SetString(doc.Amount, 10)
	if !ok || amount.Cmp(x.minimumAmount) != 1 {
		log.Debug("[BURN SIGNER] Burn amount too low")
		return false, nil
	}

	if burnEvent.Amount.Cmp(amount) != 0 {
		log.Error("[BURN SIGNER] Invalid burn amount")
		return false, nil
	}
	if !strings.EqualFold(burnEvent.From.Hex(), doc.SenderAddress) {
		log.Error("[BURN SIGNER] Invalid burn sender")
		return false, nil
	}
	receiver := common.HexToAddress(fmt.Sprintf("0x%s", doc.RecipientAddress))
	if !strings.EqualFold(burnEvent.PoktAddress.Hex(), receiver.Hex()) {
		log.Error("[BURN SIGNER] Invalid burn recipient")
		return false, nil
	}

	log.Debug("[BURN SIGNER] Validated burn")
	return true, nil
}

func (x *BurnSignerRunner) HandleBurn(doc *models.Burn) bool {
	if doc == nil {
		log.Error("[BURN SIGNER] Burn is nil")
		return false
	}
	log.Debug("[BURN SIGNER] Handling burn: ", doc.TransactionHash)

	doc, err := util.UpdateStatusAndConfirmationsForBurn(doc, x.ethBlockNumber)
	if err != nil {
		log.Error("[BURN SIGNER] Error getting burn status: ", err)
		return false
	}

	var update bson.M

	valid, err := x.ValidateBurn(doc)
	if err != nil {
		log.Error("[BURN SIGNER] Error validating burn: ", err)
		return false
	}
	if !valid {
		log.Error("[BURN SIGNER] Burn failed validation")
		update = bson.M{
			"$set": bson.M{
				"status":     models.StatusFailed,
				"updated_at": time.Now(),
			},
		}
		if doc.Confirmations == "0" {
			log.Debug("[BURN SIGNER] Burn has no confirmations, skipping")
			return false
		}
	} else {

		if doc.Status == models.StatusConfirmed {
			log.Debug("[BURN SIGNER] Signing burn")
			doc, err = util.SignBurn(doc, x.privateKey, x.multisigPubKey, x.numSigners)
			if err != nil {
				log.Error("[BURN SIGNER] Error signing burn: ", err)
				return false
			}

			update = bson.M{
				"$set": bson.M{
					"return_tx":     doc.ReturnTx,
					"signers":       doc.Signers,
					"status":        doc.Status,
					"confirmations": doc.Confirmations,
					"updated_at":    time.Now(),
				},
			}
		} else {
			log.Debug("[BURN SIGNER] Not signing burn")
			update = bson.M{
				"$set": bson.M{
					"status":        doc.Status,
					"confirmations": doc.Confirmations,
					"updated_at":    time.Now(),
				},
			}
		}
	}

	filter := bson.M{
		"_id":    doc.Id,
		"status": bson.M{"$in": []string{models.StatusPending, models.StatusConfirmed}},
	}
	err = app.DB.UpdateOne(models.CollectionBurns, filter, update)
	if err != nil {
		log.Error("[BURN SIGNER] Error updating burn: ", err)
		return false
	}
	log.Info("[BURN SIGNER] Handled burn: ", doc.TransactionHash)

	return true
}

func (x *BurnSignerRunner) SyncInvalidMints() bool {
	log.Debug("[BURN SIGNER] Syncing invalid mints")

	signersFilter := bson.M{"$nin": []string{strings.ToLower(x.privateKey.PublicKey().RawString())}}
	statusFilter := bson.M{"$in": []string{models.StatusPending, models.StatusConfirmed}}
	filter := bson.M{
		"vault_address": x.vaultAddress,
		"status":        statusFilter,
		"signers":       signersFilter,
	}

	invalidMints := []models.InvalidMint{}
	err := app.DB.FindMany(models.CollectionInvalidMints, filter, &invalidMints)
	if err != nil {
		log.Error("[BURN SIGNER] Error fetching invalid mints: ", err)
		return false
	}
	log.Info("[BURN SIGNER] Found invalid mints: ", len(invalidMints))

	var success bool = true
	for i := range invalidMints {
		doc := invalidMints[i]

		resourceId := fmt.Sprintf("%s/%s", models.CollectionInvalidMints, doc.Id.Hex())
		lockId, err := app.DB.XLock(resourceId)
		if err != nil {
			log.Error("[BURN SIGNER] Error locking invalid mint: ", err)
			success = false
			continue
		}
		log.Debug("[BURN SIGNER] Locked invalid mint: ", doc.TransactionHash)

		success = x.HandleInvalidMint(&doc) && success

		if err = app.DB.Unlock(lockId); err != nil {
			log.Error("[BURN SIGNER] Error unlocking invalid mint: ", err)
			success = false
		} else {
			log.Debug("[BURN SIGNER] Unlocked invalid mint: ", doc.TransactionHash)
		}

	}

	log.Info("[BURN SIGNER] Synced invalid mints")
	return success
}

func (x *BurnSignerRunner) SyncBurns() bool {
	log.Debug("[BURN SIGNER] Syncing burns")

	signersFilter := bson.M{"$nin": []string{strings.ToLower(x.privateKey.PublicKey().RawString())}}
	statusFilter := bson.M{"$in": []string{models.StatusPending, models.StatusConfirmed}}
	filter := bson.M{
		"wpokt_address": x.wpoktAddress,
		"status":        statusFilter,
		"signers":       signersFilter,
	}

	burns := []models.Burn{}
	err := app.DB.FindMany(models.CollectionBurns, filter, &burns)
	if err != nil {
		log.Error("[BURN SIGNER] Error fetching burns: ", err)
		return false
	}
	log.Info("[BURN SIGNER] Found burns: ", len(burns))

	var success bool = true

	for i := range burns {
		doc := burns[i]

		resourceId := fmt.Sprintf("%s/%s", models.CollectionBurns, doc.Id.Hex())
		lockId, err := app.DB.XLock(resourceId)
		if err != nil {
			log.Error("[BURN SIGNER] Error locking burn: ", err)
			success = false
			continue
		}
		log.Debug("[BURN SIGNER] Locked burn: ", doc.TransactionHash)

		success = x.HandleBurn(&doc) && success

		if err = app.DB.Unlock(lockId); err != nil {
			log.Error("[BURN SIGNER] Error unlocking burn: ", err)
			success = false
		} else {
			log.Debug("[BURN SIGNER] Unlocked burn: ", doc.TransactionHash)
		}

	}

	log.Info("[BURN SIGNER] Synced burns")
	return success
}

func (x *BurnSignerRunner) SyncTxs() bool {
	log.Debug("[BURN SIGNER] Syncing")

	success := x.SyncInvalidMints()
	success = x.SyncBurns() && success

	log.Info("[BURN SIGNER] Synced txs")
	return success
}

func NewBurnSigner(wg *sync.WaitGroup, health models.ServiceHealth) app.Service {
	if !app.Config.BurnSigner.Enabled {
		log.Debug("[BURN SIGNER] Disabled")
		return app.NewEmptyService(wg)
	}

	log.Debug("[BURN SIGNER] Initializing")

	pk, err := crypto.NewPrivateKey(app.Config.Pocket.PrivateKey)
	if err != nil {
		log.Fatal("[BURN SIGNER] Error initializing burn signer: ", err)
	}
	log.Info("[BURN SIGNER] public key: ", pk.PublicKey().RawString())
	log.Debug("[BURN SIGNER] address: ", pk.PublicKey().Address().String())

	var pks []crypto.PublicKey
	for _, pk := range app.Config.Pocket.MultisigPublicKeys {
		p, err := crypto.NewPublicKey(pk)
		if err != nil {
			log.Fatal("[BURN SIGNER] Error parsing multisig public key: ", err)
		}
		pks = append(pks, p)
	}

	multisigPk := crypto.PublicKeyMultiSignature{PublicKeys: pks}
	vaultAddress := multisigPk.Address().String()
	log.Debug("[BURN SIGNER] Vault address: ", vaultAddress)
	if strings.ToLower(vaultAddress) != strings.ToLower(app.Config.Pocket.VaultAddress) {
		log.Fatal("[BURN SIGNER] Multisig address does not match vault address")
	}

	poktClient := pokt.NewClient()
	ethClient, err := eth.NewClient()
	if err != nil {
		log.Fatal("[BURN SIGNER] Error initializing ethereum client: ", err)
	}

	log.Debug("[BURN SIGNER] Connecting to wpokt contract at: ", app.Config.Ethereum.WrappedPocketAddress)
	contract, err := autogen.NewWrappedPocket(common.HexToAddress(app.Config.Ethereum.WrappedPocketAddress), ethClient.GetClient())
	if err != nil {
		log.Fatal("[BURN SIGNER] Error initializing Wrapped Pocket contract", err)
	}
	log.Debug("[BURN SIGNER] Connected to wpokt contract")

	x := &BurnSignerRunner{
		privateKey:     pk,
		multisigPubKey: multisigPk,
		numSigners:     len(pks),
		ethClient:      ethClient,
		poktClient:     poktClient,
		vaultAddress:   strings.ToLower(vaultAddress),
		wpoktAddress:   strings.ToLower(app.Config.Ethereum.WrappedPocketAddress),
		wpoktContract:  eth.NewWrappedPocketContract(contract),
		minimumAmount:  big.NewInt(app.Config.Pocket.TxFee),
	}

	x.UpdateBlocks()

	log.Info("[BURN SIGNER] Initialized")

	return app.NewRunnerService(BurnSignerName, x, wg, time.Duration(app.Config.BurnSigner.IntervalMillis)*time.Millisecond)
}
