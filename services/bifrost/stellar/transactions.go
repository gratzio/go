package stellar

import (
	"strconv"

	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
)

func (ac *AccountConfigurator) createAccountTransaction(destination string) error {
	transaction, err := ac.buildTransaction(
		ac.channelPublicKey,
		[]string{ac.ChannelSecretKey, ac.DistributionSecretKey},
		build.CreateAccount(
			build.SourceAccount{ac.distributionPublicKey},
			build.Destination{destination},
			build.NativeAmount{ac.StartingBalance},
		),
	)
	if err != nil {
		return errors.Wrap(err, "Error building transaction")
	}

	err = ac.submitTransaction(transaction)
	if err != nil {
		return errors.Wrap(err, "Error submitting a transaction")
	}

	return nil
}

// configureAccountTransaction is using a signer on an user accounts to configure the account.
func (ac *AccountConfigurator) configureAccountTransaction(destination, intermediateAssetCode, amount string, needsAuthorize bool) error {
	signers := []string{ac.SignerSecretKey, ac.DistributionSecretKey}

	mutators := []build.TransactionMutator{
		build.Trust(intermediateAssetCode, ac.issuerPublicKey),
		build.Trust(ac.TokenAssetCode, ac.issuerPublicKey),
	}

	if needsAuthorize {
		signers = append(signers, ac.IssuerSecretKey)
		mutators = append(
			mutators,
			// Chain token received (BTC/ETH/XLM)
			build.AllowTrust(
				build.SourceAccount{ac.issuerPublicKey},
				build.Trustor{destination},
				build.AllowTrustAsset{intermediateAssetCode},
				build.Authorize{true},
			),
			// Destination token
			build.AllowTrust(
				build.SourceAccount{ac.issuerPublicKey},
				build.Trustor{destination},
				build.AllowTrustAsset{ac.TokenAssetCode},
				build.Authorize{true},
			),
		)
	}

	var tokenPrice string
	switch intermediateAssetCode {
	case "BTC":
		tokenPrice = ac.TokenPriceBTC
	case "ETH":
		tokenPrice = ac.TokenPriceETH
	case "XLM":
		tokenPrice = ac.TokenPriceXLM
	default:
		return errors.Errorf("Invalid intermediateAssetCode: $%s", intermediateAssetCode)
	}

	mutators = append(
		mutators,
		build.Payment(
			build.SourceAccount{ac.distributionPublicKey},
			build.Destination{destination},
			build.CreditAmount{
				Code:   intermediateAssetCode,
				Issuer: ac.issuerPublicKey,
				Amount: amount,
			},
		),
		// Exchange BTC/ETH/XLM => token
		build.CreateOffer(
			build.Rate{
				Selling: build.CreditAsset(intermediateAssetCode, ac.issuerPublicKey),
				Buying:  build.CreditAsset(ac.TokenAssetCode, ac.issuerPublicKey),
				Price:   build.Price(tokenPrice),
			},
			build.Amount(amount),
		),
	)

	transaction, err := ac.buildTransaction(destination, signers, mutators...)
	if err != nil {
		return errors.Wrap(err, "Error building a transaction")
	}

	err = ac.submitTransaction(transaction)
	if err != nil {
		return errors.Wrap(err, "Error submitting a transaction")
	}

	return nil
}

// removeTemporarySigner is removing temporary signer from an account.
func (ac *AccountConfigurator) removeTemporarySigner(destination string) error {
	// Remove signer
	mutators := []build.TransactionMutator{
		build.SetOptions(
			build.MasterWeight(1),
			build.RemoveSigner(ac.signerPublicKey),
		),
	}

	transaction, err := ac.buildTransaction(destination, []string{ac.SignerSecretKey}, mutators...)
	if err != nil {
		return errors.Wrap(err, "Error building a transaction")
	}

	err = ac.submitTransaction(transaction)
	if err != nil {
		return errors.Wrap(err, "Error submitting a transaction")
	}

	return nil
}

// buildUnlockAccountTransaction creates and returns unlock account transaction.
func (ac *AccountConfigurator) buildUnlockAccountTransaction(source string) (string, error) {
	// Remove signer
	mutators := []build.TransactionMutator{
		build.Timebounds{
			MinTime: ac.LockUnixTimestamp,
		},
		build.SetOptions(
			build.MasterWeight(1),
			build.RemoveSigner(ac.signerPublicKey),
		),
	}

	return ac.buildTransaction(source, []string{ac.SignerSecretKey}, mutators...)
}

func (ac *AccountConfigurator) buildTransaction(source string, signers []string, mutators ...build.TransactionMutator) (string, error) {
	muts := []build.TransactionMutator{
		build.SourceAccount{source},
		build.Network{ac.NetworkPassphrase},
	}

	if source == ac.channelPublicKey {
		muts = append(muts, build.Sequence{ac.getchannelSequence()})
	} else {
		muts = append(muts, build.AutoSequence{ac.Horizon})
	}

	muts = append(muts, mutators...)
	tx, err := build.Transaction(muts...)
	if err != nil {
		return "", err
	}
	txe, err := tx.Sign(ac.uniqueSigners(signers)...)
	if err != nil {
		return "", err
	}
	return txe.Base64()
}

func (ac *AccountConfigurator) submitTransaction(transaction string) error {
	localLog := log.WithField("tx", transaction)
	localLog.Info("Submitting transaction")

	_, err := ac.Horizon.SubmitTransaction(transaction)
	if err != nil {
		fields := log.F{"err": err}
		if err, ok := err.(*horizon.Error); ok {
			fields["result"] = string(err.Problem.Extras["result_xdr"])
			ac.updateChannelSequence()
		}
		localLog.WithFields(fields).Error("Error submitting transaction")
		return errors.Wrap(err, "Error submitting transaction")
	}

	localLog.Info("Transaction successfully submitted")
	return nil
}

func (ac *AccountConfigurator) updateChannelSequence() error {
	ac.channelSequenceMutex.Lock()
	defer ac.channelSequenceMutex.Unlock()

	account, err := ac.Horizon.LoadAccount(ac.channelPublicKey)
	if err != nil {
		err = errors.Wrap(err, "Error loading channel account")
		ac.log.Error(err)
		return err
	}

	ac.channelSequence, err = strconv.ParseUint(account.Sequence, 10, 64)
	if err != nil {
		err = errors.Wrap(err, "Invalid ChannelPublicKey sequence")
		ac.log.Error(err)
		return err
	}

	return nil
}

func (ac *AccountConfigurator) getchannelSequence() uint64 {
	ac.channelSequenceMutex.Lock()
	defer ac.channelSequenceMutex.Unlock()
	ac.channelSequence++
	sequence := ac.channelSequence
	return sequence
}

func (ac *AccountConfigurator) uniqueSigners(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)
	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
}
