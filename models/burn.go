package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CollectionBurns = "burns"
)

type Burn struct {
	Id               *primitive.ObjectID `bson:"_id,omitempty"`
	TransactionHash  string              `bson:"transaction_hash"`
	LogIndex         uint64              `bson:"log_index"`
	BlockNumber      uint64              `bson:"block_number"`
	SenderAddress    string              `bson:"sender_address"`
	SenderChainId    uint64              `bson:"sender_chain_id"`
	RecipientAddress string              `bson:"recipient_address"`
	RecipientChainId string              `bson:"recipient_chain_id"`
	Amount           string              `bson:"amount"`
	CreatedAt        time.Time           `bson:"created_at"`
	UpdatedAt        time.Time           `bson:"updated_at"`
	Status           string              `bson:"status"`
	Signers          []string            `bson:"signers"`
	Order            *Order              `bson:"order"`
}
