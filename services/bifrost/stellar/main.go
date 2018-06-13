package stellar

import (
	"sync"

	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/log"
)

// Status describes status of account processing
type Status string

const (
	StatusCreatingAccount    Status = "creating_account"
	StatusWaitingForSigner   Status = "waiting_for_signer"
	StatusConfiguringAccount Status = "configuring_account"
	StatusRemovingSigner     Status = "removing_signer"
)

// AccountConfigurator is responsible for configuring new Stellar accounts that
// participate in ICO.
type AccountConfigurator struct {
	Horizon               horizon.ClientInterface `inject:""`
	NetworkPassphrase     string
	IssuerSecretKey       string
	DistributionSecretKey string
	ChannelSecretKey      string
	SignerSecretKey       string
	LockUnixTimestamp     uint64
	WaitForSignerTimeout  int64
	NeedsAuthorize        bool
	TokenAssetCode        string
	TokenPriceBTC         string
	TokenPriceETH         string
	TokenPriceXLM         string
	StartingBalance       string
	OnAccountCreated      func(assetCode string, destination string)
	OnExchanged           func(assetCode string, estination string)
	OnExchangedTimelocked func(assetCode string, destination, transaction string)
	OnError               func(destination, assetCode, amount, accCreatedWithBalance, errorCode, errorMessage string)

	issuerPublicKey       string
	distributionPublicKey string
	signerPublicKey       string
	channelPublicKey      string
	channelSequence       uint64
	channelSequenceMutex  sync.Mutex
	accountStatus         map[string]Status
	accountStatusMutex    sync.Mutex
	log                   *log.Entry
}
