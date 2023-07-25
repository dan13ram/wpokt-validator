package ethereum

import (
	"math/big"
	"testing"
	"time"

	"github.com/dan13ram/wpokt-backend/app"
	"github.com/dan13ram/wpokt-backend/ethereum/autogen"
	"github.com/dan13ram/wpokt-backend/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestCreateBurn(t *testing.T) {
	app.Config.Ethereum.ChainId = "1"
	app.Config.Pocket.ChainId = "0001"
	TX_HASH := "0x0000000000000000000000000000000000000000000000001234567890abcdef"
	SENDER_ADDRESS := "0x0000000000000000000000000000000000abcDeF"
	RECIPIENT_ADDRESS := "0000000000000000000000000000001234567890"

	testCases := []struct {
		name            string
		event           *autogen.WrappedPocketBurnAndBridge
		expectedBurn    models.Burn
		expectedErr     bool
		expectedUpdated time.Duration
	}{
		{
			name: "Valid event",
			event: &autogen.WrappedPocketBurnAndBridge{
				Raw: types.Log{
					BlockNumber: 10,
					TxHash:      common.HexToHash(TX_HASH),
					Index:       0,
					Address:     common.HexToAddress(ZERO_ADDRESS),
				},
				From:        common.HexToAddress(SENDER_ADDRESS),
				PoktAddress: common.HexToAddress(RECIPIENT_ADDRESS),
				Amount:      big.NewInt(100),
			},
			expectedBurn: models.Burn{
				BlockNumber:      "10",
				Confirmations:    "0",
				TransactionHash:  TX_HASH,
				LogIndex:         "0",
				WPOKTAddress:     ZERO_ADDRESS,
				SenderAddress:    SENDER_ADDRESS,
				SenderChainId:    app.Config.Ethereum.ChainId,
				RecipientAddress: RECIPIENT_ADDRESS,
				RecipientChainId: app.Config.Pocket.ChainId,
				Amount:           "100",
				CreatedAt:        time.Now(), // We'll use assert.WithinDuration to check if within an acceptable range
				UpdatedAt:        time.Now(), // We'll use assert.WithinDuration to check if within an acceptable range
				Status:           models.StatusPending,
				Signers:          []string{},
			},
			expectedErr:     false,
			expectedUpdated: 2 * time.Second, // Update time should be within 2 seconds of the current time
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			result := createBurn(tc.event)
			assert.WithinDuration(t, time.Now(), result.CreatedAt, tc.expectedUpdated)
			assert.WithinDuration(t, time.Now(), result.UpdatedAt, tc.expectedUpdated)

			result.CreatedAt = tc.expectedBurn.CreatedAt
			result.UpdatedAt = tc.expectedBurn.UpdatedAt

			assert.Equal(t, tc.expectedBurn, result)

		})
	}
}
