package lumen

import (
	"time"
	"database/sql"
	
	"github.com/stellar/go/services/bifrost/common"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

func (l *Listener) Start() error {
	l.log = common.CreateLogger("StellarListener")
	l.log.Info("StellarListener starting")

	lastTranId, err := l.Storage.GetLastProcessedStellarTransaction()
	if err != nil {
		err = errors.Wrap(err, "Error getting stellar transaction id to process from DB")
		l.log.Error(err)
		return err
	}
	l.log.Infof("Stellar: last processed transaction id=%d", lastTranId)

	if lastTranId == 0 {
		lastTranId, err = l.GetLastTransactionId()
		if err != nil {
			err = errors.Wrap(err, "Error getting last transaction id from stellar horizon db")
			l.log.Error(err)
			return err
		}
		l.log.Infof("Stellar: starting from last transaction in history id=%d", lastTranId)

		// Persist last transaction id
		err = l.Storage.SaveLastProcessedStellarTransaction(lastTranId)
		if err != nil {
			err = errors.Wrap(err, "Error saving last processed transaction id")
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

			err = l.TransactionHandler(tran)
			if err != nil {
				err = errors.Wrap(err, "Error processing transaction " + tran.Hash)
				l.log.Error(err)
			}
			
			// Persist processed transaction id
			err = l.Storage.SaveLastProcessedStellarTransaction(tran.ID)
			if err != nil {
				l.log.WithField("err", err).Error("Error saving last processed transaction id")
				time.Sleep(time.Second)
				// We continue to the next transaction.
				// The idea behind this is if there was a problem with this single query we want to
				// continue processing because it's safe to reprocess blocks and we don't want a downtime.
			}

			id = tran.ID;
		}
	}
}

func (l *Listener) GetLastTransactionId() (uint64, error) {
	session, err := db.Open("postgres", l.Config.Lumen.HorizonDatabaseDsn)
	if err != nil {
		return 0, errors.Wrap(err, "Stellar Horizon db open failed")
	}

	defer session.DB.Close()

	var id sql.NullInt64
	err = session.DB.Get(&id, `select max(id) from history_transactions`)

	if err != nil {
		if session.NoRows(err) {
			return 0, nil
		}
		return 0, err	
	}

	if id.Valid {
		return uint64(id.Int64), nil
	}

	return 0, nil;
}

func (l *Listener) GetTransactions(id uint64) ([]Transaction, error) {
	session, err := db.Open("postgres", l.Config.Lumen.HorizonDatabaseDsn)
	if err != nil {
		return nil, errors.Wrap(err, "Stellar Horizon db open failed")
	}

	defer session.DB.Close()

	transactions := []Transaction{}
	err = session.DB.Select(&transactions, `select t.id, t.transaction_hash, t.memo, ops.details from history_transactions t join
		(select transaction_id, details from history_operations ho join (
		  select history_operation_id from history_operation_participants 
		 where history_account_id = (select id from history_accounts where address = $1)) hop
		  on ho.id = hop.history_operation_id
		   where ho.source_account != $1
			 and ho.type = 1
			 and ho.transaction_id > $2) ops
		on t.id = ops.transaction_id order by transaction_id limit 1000;`, l.Config.Lumen.AccountPublicKey, id)

	if err != nil {
		if session.NoRows(err) {
			return []Transaction{}, nil
		}
		return nil, err	
	}

	return transactions, nil
}