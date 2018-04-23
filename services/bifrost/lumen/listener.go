package lumen

import (
	"fmt"
	"log"
	"time"
	// "github.com/btcsuite/btcd/chaincfg"
	// "github.com/btcsuite/btcd/txscript"
	// "github.com/btcsuite/btcd/wire"
	//"github.com/stellar/go/services/horizon/internal/db2/history"

	"github.com/stellar/go/services/bifrost/common"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

func (l *Listener) Start() error {
	l.log = common.CreateLogger("StellarListener")
	l.log.Info("StellarListener starting")

	// if l.Testnet {
	// 	l.chainParams = &chaincfg.TestNet3Params
	// } else {
	// 	l.chainParams = &chaincfg.MainNetParams
	// }

	lastTranId, err := l.Storage.GetLastProcessedStellarTransaction()
	if err != nil {
		err = errors.Wrap(err, "Error getting stellar transaction id to process from DB")
		l.log.Error(err)
		return err
	}

	if lastTranId == 0 {
		lastTranId, err = l.GetLastTransactionId()
		if err != nil {
			err = errors.Wrap(err, "Error getting last transaction id from stellar horizon db")
			l.log.Error(err)
			return err
		}
	}

	go l.processTransactions(lastTranId)
	return nil
}

func (l *Listener) processTransactions(id uint64) {
	l.log.Infof("Stellar: starting from transaction id after %d", id)

	for {
		transactions, err := l.GetTransactions(id)
		if err != nil {
			err = errors.Wrap(err, "Error getting stellar transactions to process")
			l.log.Error(err)
		}

		if len(transactions) == 0 {
			time.Sleep(time.Second)
			continue
		}

		for _, tran := range transactions {
			l.log.Infof("!!!! processing transaction id=%s, hash=%s", tran.ID, tran.Hash)

			// Persist processed transaction id
			err = l.Storage.SaveLastProcessedStellarTransaction(tran.ID)
			if err != nil {
				l.log.WithField("err", err).Error("Error saving last processed transaction id")
				time.Sleep(time.Second)
				// We continue to the next transaction.
				// The idea behind this is if there was a problem with this single query we want to
				// continue processing because it's safe to reprocess blocks and we don't want a downtime.
			}
		}
	}
}

func (l *Listener) GetLastTransactionId() (uint64, error) {
	session, err := db.Open("postgres", l.Config.Lumen.HorizonDatabaseDsn)
	if err != nil {
		return 0, errors.Wrap(err, "Stellar Horizon db open failed")
	}

	//q := history.Q{Session: session}

	//people := []Person{}
	//db.Select(&people, "SELECT * FROM person ORDER BY first_name ASC")
	//jason, john := people[0], people[1]

	rows, err := session.QueryRaw(`select t.id, t.transaction_hash, t.memo, ops.details from history_transactions t join
		(select transaction_id, details from history_operations ho join (
		  select history_operation_id from history_operation_participants 
		 where history_account_id = (select id from history_accounts where address = ?)) hop
		  on ho.id = hop.history_operation_id
		   where ho.source_account != ?
			 and ho.type = 1
			 and ho.transaction_id > 0 /*last tran*/) ops
		on t.id = ops.transaction_id;`, "GAY6EHCKATOOV2DKUJFBGCJJ6DGLZHOQQ3DQ4HLTWVJTVRJ6EGXTF7UU", "GAY6EHCKATOOV2DKUJFBGCJJ6DGLZHOQQ3DQ4HLTWVJTVRJ6EGXTF7UU")

	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// Loop through rows using only one struct
	tran := Transaction{}
	///rows, err := db.Queryx("SELECT * FROM place")
	for rows.Next() {
		err := rows.StructScan(&tran)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%#v\n", tran)
	}

	// for rows.Next() {
	// 	// load data from the row
	// 	var id uint64
	// 	var hash, details string
	// 	var memo string `json:"memo,omitempty"`

	// 	err = rows.Scan(&id, &hash, &memo, &details)
	// 	if err != nil {
	// 		return 0, err
	// 	}

	// 	l.log.Infof("!!!! id=%s, hash=%s, memo=%s, details=%s", id, hash, memo, details)
	// }

	// res := q.SelectRaw(dest, `
	// 	SELECT count(*)
	// 	FROM history_ledgers`)

	l.log.Infof("!!!! GetLastTransactionId !!!, session %s, res=%s", session, rows)
	session.DB.Close()
	return 111, nil
}

func (l *Listener) GetTransactions(uint64) ([]Transaction, error) {
	return nil, nil
}

// func (l *Listener) processBlocks(blockNumber uint64) {
// 	l.log.Infof("Starting from block %d", blockNumber)

// 	// Time when last new block has been seen
// 	lastBlockSeen := time.Now()
// 	missingBlockWarningLogged := false

// 	for {
// 		block, err := l.getBlock(blockNumber)
// 		if err != nil {
// 			l.log.WithFields(log.F{"err": err, "blockNumber": blockNumber}).Error("Error getting block")
// 			time.Sleep(time.Second)
// 			continue
// 		}

// 		// Block doesn't exist yet
// 		if block == nil {
// 			if time.Since(lastBlockSeen) > 20*time.Minute && !missingBlockWarningLogged {
// 				l.log.Warn("No new block in more than 20 minutes")
// 				missingBlockWarningLogged = true
// 			}

// 			time.Sleep(time.Second)
// 			continue
// 		}

// 		// Reset counter when new block appears
// 		lastBlockSeen = time.Now()
// 		missingBlockWarningLogged = false

// 		err = l.processBlock(block)
// 		if err != nil {
// 			l.log.WithFields(log.F{"err": err, "blockHash": block.Header.BlockHash().String()}).Error("Error processing block")
// 			time.Sleep(time.Second)
// 			continue
// 		}

// 		// Persist block number
// 		err = l.Storage.SaveLastProcessedBitcoinBlock(blockNumber)
// 		if err != nil {
// 			l.log.WithField("err", err).Error("Error saving last processed block")
// 			time.Sleep(time.Second)
// 			// We continue to the next block.
// 			// The idea behind this is if there was a problem with this single query we want to
// 			// continue processing because it's safe to reprocess blocks and we don't want a downtime.
// 		}

// 		blockNumber++
// 	}
// }

// // getBlock returns (nil, nil) if block has not been found (not exists yet)
// func (l *Listener) getBlock(blockNumber uint64) (*wire.MsgBlock, error) {
// 	blockHeight := int64(blockNumber)
// 	blockHash, err := l.Client.GetBlockHash(blockHeight)
// 	if err != nil {
// 		if strings.Contains(err.Error(), "Block height out of range") {
// 			// Block does not exist yet
// 			return nil, nil
// 		}
// 		err = errors.Wrap(err, "Error getting block hash from bitcoin-core")
// 		l.log.WithField("blockHeight", blockHeight).Error(err)
// 		return nil, err
// 	}

// 	block, err := l.Client.GetBlock(blockHash)
// 	if err != nil {
// 		err = errors.Wrap(err, "Error getting block from bitcoin-core")
// 		l.log.WithField("blockHash", blockHash.String()).Error(err)
// 		return nil, err
// 	}

// 	return block, nil
// }

// func (l *Listener) processBlock(block *wire.MsgBlock) error {
// 	transactions := block.Transactions

// 	localLog := l.log.WithFields(log.F{
// 		"blockHash":    block.Header.BlockHash().String(),
// 		"blockTime":    block.Header.Timestamp,
// 		"transactions": len(transactions),
// 	})
// 	localLog.Info("Processing block")

// 	for _, transaction := range transactions {
// 		transactionLog := localLog.WithField("transactionHash", transaction.TxHash().String())

// 		for index, output := range transaction.TxOut {
// 			class, addresses, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, l.chainParams)
// 			if err != nil {
// 				// txscript.ExtractPkScriptAddrs returns error on non-standard scripts
// 				// so this can be Warn.
// 				transactionLog.WithField("err", err).Warn("Error extracting addresses")
// 				continue
// 			}

// 			// We only support P2PK and P2PKH addresses
// 			if class != txscript.PubKeyTy && class != txscript.PubKeyHashTy {
// 				transactionLog.WithField("class", class).Debug("Unsupported addresses class")
// 				continue
// 			}

// 			// Paranoid. We access address[0] later.
// 			if len(addresses) != 1 {
// 				transactionLog.WithField("addresses", addresses).Error("Invalid addresses length")
// 				continue
// 			}

// 			handlerTransaction := Transaction{
// 				Hash:       transaction.TxHash().String(),
// 				TxOutIndex: index,
// 				ValueSat:   output.Value,
// 				To:         addresses[0].EncodeAddress(),
// 			}

// 			err = l.TransactionHandler(handlerTransaction)
// 			if err != nil {
// 				return errors.Wrap(err, "Error processing transaction")
// 			}
// 		}
// 	}

// 	localLog.Info("Processed block")

// 	return nil
// }
