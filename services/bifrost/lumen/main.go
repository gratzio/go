package lumen

import (
	"database/sql"
	//"github.com/derekparker/delve/pkg/config"
	"github.com/stellar/go/services/bifrost/config"
	"github.com/stellar/go/support/log"
)

// var (
// 	eight = big.NewInt(8)
// 	ten   = big.NewInt(10)
// 	// satInBtc = 10^8
// 	satInBtc = new(big.Rat).SetInt(new(big.Int).Exp(ten, eight, nil))
// )

// Listener listens for transactions using bitcoin-core RPC. It calls TransactionHandler for each new
// transactions. It will reprocess the block if TransactionHandler returns error. It will
// start from the block number returned from Storage.GetBitcoinBlockToProcess or the latest block
// if it returned 0. Transactions can be processed more than once, it's TransactionHandler
// responsibility to ignore duplicates.
// Listener tracks only P2PKH payments.
// You can run multiple Listeners if Storage is implemented correctly.
type Listener struct {
	Enabled bool
	//Client             Client  `inject:""`
	Storage            Storage `inject:""`
	TransactionHandler TransactionHandler
	Testnet            bool
	Config             *config.Config `inject:""`

	//chainParams *chaincfg.Params
	log *log.Entry
}

type Client interface {
	//GetBlockCount() (int64, error)
	//GetBlockHash(blockHeight int64) (*chainhash.Hash, error)
	//GetBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error)
	//GetLastTransactionId() (uint64, error)
	//GetTransactions(uint64) ([]Transaction, error)
}

// Storage is an interface that must be implemented by an object using
// persistent storage.
type Storage interface {
	// GetLastProcessedStellarTransaction gets the latest processsed Stellar transactions. `0` means the
	// processing should start from the current.
	GetLastProcessedStellarTransaction() (uint64, error)
	// GetLastProcessedStellarTransaction should update id of the last processed Stellar
	// transaction. It should only update the id if id > current id in atomic transaction.
	SaveLastProcessedStellarTransaction(id uint64) error
}

type TransactionHandler func(transaction Transaction) error

type Transaction struct {
	ID   uint64 `db:"id" json:"id"`
	Hash string `db:"transaction_hash" json:"transaction_hash"`
	//TxOutIndex int //id int64 ??? ddd
	//TxEnvelope string
	// TxResult   string ???
	Details string         `db:"details" json:"details"`
	Memo    sql.NullString `db:"memo" json:"memo"`
}
