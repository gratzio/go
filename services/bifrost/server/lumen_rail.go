package server

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/stellar/go/services/bifrost/lumen"
	"github.com/stellar/go/services/bifrost/database"
	"github.com/stellar/go/services/bifrost/queue"
	"github.com/stellar/go/services/bifrost/sse"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
)

// onNewLumenTransaction checks if transaction is valid and adds it to
// the transactions queue for StellarAccountConfigurator to consume.
//
// Transaction added to transactions queue should be in a format described in
// queue.Transaction (especialy amounts). Pooling service should not have to deal with any
// conversions.
//
// This is very unlikely but it's possible that a single transaction will have more than
// one output going to bifrost account. Then only first output will be processed.
// Because it's very unlikely that this will happen and it's not a security issue this
// will be fixed in a future release.
func (s *Server) onNewLumenTransaction(transaction lumen.Transaction) error {
	localLog := s.log.WithFields(log.F{"transaction": transaction, "rail": "lumen"})
	localLog.Debug("Processing transaction")

	var details map[string]interface{}
	_ = json.Unmarshal(transaction.Details, &details)

	// Let's check if tx is valid first.

	if details["asset_type"] != "native" {
		return nil
	}

	amount := details["amount"].(string)
	MemoValue := strings.TrimSpace(transaction.Memo.String)

	if s.Config.Lumen.MemoPrefix != "" && !strings.HasPrefix(MemoValue, s.Config.Lumen.MemoPrefix) {
		return nil
	}
	
	MemoKey := strings.TrimSpace(MemoValue[len(s.Config.Lumen.MemoPrefix):])

	if MemoKey == "" {
		return nil
	}

	ValueFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return errors.Wrap(err, "Stellar: Invalid transaction amount: "+amount)
	}
	Value := int64(ValueFloat  * 10000000)
		
	// Check if value is above minimum required
	if Value < s.minimumValueXlmStroops {
		localLog.Debug("Value is below minimum required amount, skipping")
		return nil
	}
	
	addressAssociation, err := s.Database.GetAssociationByChainAddress(database.ChainLumen, MemoKey)
	if err != nil {
		return errors.Wrap(err, "Error getting association for " + MemoKey)
	}

	if addressAssociation == nil {
		localLog.Debug("Associated address not found, skipping")
		return nil
	}

	// Add transaction as processing.
	processed, err := s.Database.AddProcessedTransaction(database.ChainLumen, transaction.Hash, MemoKey)
	if err != nil {
		return err
	}

	if processed {
		localLog.Debug("Transaction already processed, skipping")
		return nil
	}

	// Add tx to the processing queue
	queueTx := queue.Transaction{
		TransactionID: transaction.Hash,
		AssetCode:     queue.AssetCodeXLM,
		// Amount in the base unit of currency.
		Amount:           amount,
		StellarPublicKey: addressAssociation.StellarPublicKey,
	}

	err = s.TransactionsQueue.QueueAdd(queueTx)
	if err != nil {
		return errors.Wrap(err, "Error adding transaction to the processing queue")
	}
	localLog.Info("Transaction added to transaction queue")

	// Broadcast event to address stream
	s.SSEServer.BroadcastEvent(MemoKey, sse.TransactionReceivedAddressEvent, nil)
	localLog.Info("Transaction processed successfully")
	return nil
}
