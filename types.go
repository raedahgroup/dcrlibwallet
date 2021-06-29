package dcrlibwallet

import (
	"context"
	"encoding/json"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v3"
	"github.com/decred/dcrd/wire"

	"decred.org/dcrwallet/wallet"
)

type WalletsIterator struct {
	currentIndex int
	wallets      []*Wallet
}

type BlockInfo struct {
	Height    int32
	Timestamp int64
}

type Amount struct {
	AtomValue int64
	DcrValue  float64
}

type TxFeeAndSize struct {
	Fee                 *Amount
	Change              *Amount
	EstimatedSignedSize int
}

type UnsignedTransaction struct {
	UnsignedTransaction       []byte
	EstimatedSignedSize       int
	ChangeIndex               int
	TotalOutputAmount         int64
	TotalPreviousOutputAmount int64
}

type Balance struct {
	Total                   int64
	Spendable               int64
	ImmatureReward          int64
	ImmatureStakeGeneration int64
	LockedByTickets         int64
	VotingAuthority         int64
	UnConfirmed             int64
}

type Account struct {
	WalletID         int
	Number           int32
	Name             string
	Balance          *Balance
	TotalBalance     int64
	ExternalKeyCount int32
	InternalKeyCount int32
	ImportedKeyCount int32
}

type AccountsIterator struct {
	currentIndex int
	accounts     []*Account
}

type Accounts struct {
	Count              int
	Acc                []*Account
	CurrentBlockHash   []byte
	CurrentBlockHeight int32
}

type PeerInfo struct {
	ID             int32  `json:"id"`
	Addr           string `json:"addr"`
	AddrLocal      string `json:"addr_local"`
	Services       string `json:"services"`
	Version        uint32 `json:"version"`
	SubVer         string `json:"sub_ver"`
	StartingHeight int64  `json:"starting_height"`
	BanScore       int32  `json:"ban_score"`
}

type AccountMixerNotificationListener interface {
	OnAccountMixerStarted(walletID int)
	OnAccountMixerEnded(walletID int)
}

/** begin sync-related types */

type SyncProgressListener interface {
	OnSyncStarted(wasRestarted bool)
	OnPeerConnectedOrDisconnected(numberOfConnectedPeers int32)
	OnCFiltersFetchProgress(cfiltersFetchProgress *CFiltersFetchProgressReport)
	OnHeadersFetchProgress(headersFetchProgress *HeadersFetchProgressReport)
	OnAddressDiscoveryProgress(addressDiscoveryProgress *AddressDiscoveryProgressReport)
	OnHeadersRescanProgress(headersRescanProgress *HeadersRescanProgressReport)
	OnSyncCompleted()
	OnSyncCanceled(willRestart bool)
	OnSyncEndedWithError(err error)
	Debug(debugInfo *DebugInfo)
}

type GeneralSyncProgress struct {
	TotalSyncProgress         int32 `json:"totalSyncProgress"`
	TotalTimeRemainingSeconds int64 `json:"totalTimeRemainingSeconds"`
}

type CFiltersFetchProgressReport struct {
	*GeneralSyncProgress
	beginFetchCFiltersTimeStamp int64
	startCFiltersHeight         int32
	cfiltersFetchTimeSpent      int64
	totalFetchedCFiltersCount   int32
	TotalCFiltersToFetch        int32 `json:"totalCFiltersToFetch"`
	CurrentCFilterHeight        int32 `json:"currentCFilterHeight"`
	CFiltersFetchProgress       int32 `json:"headersFetchProgress"`
}

type HeadersFetchProgressReport struct {
	*GeneralSyncProgress
	headersFetchTimeSpent    int64
	beginFetchTimeStamp      int64
	startHeaderHeight        int32
	totalFetchedHeadersCount int32
	TotalHeadersToFetch      int32 `json:"totalHeadersToFetch"`
	CurrentHeaderHeight      int32 `json:"currentHeaderHeight"`
	CurrentHeaderTimestamp   int64 `json:"currentHeaderTimestamp"`
	HeadersFetchProgress     int32 `json:"headersFetchProgress"`
}

type AddressDiscoveryProgressReport struct {
	*GeneralSyncProgress
	addressDiscoveryStartTime int64
	totalDiscoveryTimeSpent   int64
	AddressDiscoveryProgress  int32 `json:"addressDiscoveryProgress"`
	WalletID                  int   `json:"walletID"`
}

type HeadersRescanProgressReport struct {
	*GeneralSyncProgress
	TotalHeadersToScan  int32 `json:"totalHeadersToScan"`
	CurrentRescanHeight int32 `json:"currentRescanHeight"`
	RescanProgress      int32 `json:"rescanProgress"`
	RescanTimeRemaining int64 `json:"rescanTimeRemaining"`
	WalletID            int   `json:"walletID"`
}

type DebugInfo struct {
	TotalTimeElapsed          int64
	TotalTimeRemaining        int64
	CurrentStageTimeElapsed   int64
	CurrentStageTimeRemaining int64
}

/** end sync-related types */

/** begin tx-related types */

type TxAndBlockNotificationListener interface {
	OnTransaction(transaction string)
	OnBlockAttached(walletID int, blockHeight int32)
	OnTransactionConfirmed(walletID int, hash string, blockHeight int32)
}

type BlocksRescanProgressListener interface {
	OnBlocksRescanStarted(walletID int)
	OnBlocksRescanProgress(*HeadersRescanProgressReport)
	OnBlocksRescanEnded(walletID int, err error)
}

// Transaction is used with storm for tx indexing operations.
// For faster queries, the `Hash`, `Type` and `Direction` fields are indexed.
type Transaction struct {
	WalletID      int    `json:"walletID"`
	Hash          string `storm:"id,unique" json:"hash"`
	Type          string `storm:"index" json:"type"`
	Hex           string `json:"hex"`
	Timestamp     int64  `storm:"index" json:"timestamp"`
	BlockHeight   int32  `storm:"index" json:"block_height"`
	TicketSpender string `storm:"index" json:"ticket_spender"`

	MixDenomination int64 `json:"mix_denom"`
	MixCount        int32 `json:"mix_count"`

	Version  int32 `json:"version"`
	LockTime int32 `json:"lock_time"`
	Expiry   int32 `json:"expiry"`
	Fee      int64 `json:"fee"`
	FeeRate  int64 `json:"fee_rate"`
	Size     int   `json:"size"`

	Direction int32       `storm:"index" json:"direction"`
	Amount    int64       `json:"amount"`
	Inputs    []*TxInput  `json:"inputs"`
	Outputs   []*TxOutput `json:"outputs"`

	// Vote Info
	VoteVersion        int32  `json:"vote_version"`
	LastBlockValid     bool   `json:"last_block_valid"`
	VoteBits           string `json:"vote_bits"`
	VoteReward         int64  `json:"vote_reward"`
	TicketSpentHash    string `storm:"unique" json:"ticket_spent_hash"`
	DaysToVoteOrRevoke int32  `json:"days_to_vote_revoke"`
}

type TxInput struct {
	PreviousTransactionHash  string `json:"previous_transaction_hash"`
	PreviousTransactionIndex int32  `json:"previous_transaction_index"`
	PreviousOutpoint         string `json:"previous_outpoint"`
	Amount                   int64  `json:"amount"`
	AccountNumber            int32  `json:"account_number"`
}

type TxOutput struct {
	Index         int32  `json:"index"`
	Amount        int64  `json:"amount"`
	Version       int32  `json:"version"`
	ScriptType    string `json:"script_type"`
	Address       string `json:"address"`
	Internal      bool   `json:"internal"`
	AccountNumber int32  `json:"account_number"`
}

// TxInfoFromWallet contains tx data that relates to the querying wallet.
// This info is used with `DecodeTransaction` to compose the entire details of a transaction.
type TxInfoFromWallet struct {
	WalletID    int
	Hex         string
	Timestamp   int64
	BlockHeight int32
	Inputs      []*WalletInput
	Outputs     []*WalletOutput
}

type WalletInput struct {
	Index    int32 `json:"index"`
	AmountIn int64 `json:"amount_in"`
	*WalletAccount
}

type WalletOutput struct {
	Index     int32  `json:"index"`
	AmountOut int64  `json:"amount_out"`
	Internal  bool   `json:"internal"`
	Address   string `json:"address"`
	*WalletAccount
}

type WalletAccount struct {
	AccountNumber int32  `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type TransactionDestination struct {
	Address    string
	AtomAmount int64
	SendMax    bool
}

type TransactionOverview struct {
	All         int
	Sent        int
	Received    int
	Transferred int
	Mixed       int
	Staking     int
	Coinbase    int
}

/** end tx-related types */

/** begin ticket-related types */

type PurchaseTicketsRequest struct {
	Account               uint32
	RequiredConfirmations uint32
	NumTickets            uint32
	Passphrase            []byte
	Expiry                uint32
	TxFee                 int64
	TicketAddress         string
	PoolAddress           string
	PoolFees              float64
	TicketFee             int64

	// VSPFeeProcessFunc Process the fee price for the vsp to register a ticket
	// so we can reserve the amount.
	VSPFeeProcess func(context.Context) (float64, error)
	// VSPFeePaymentProcess processes the payment of the vsp fee and returns
	// the paid fee tx.
	VSPFeePaymentProcess func(context.Context, *chainhash.Hash, *wire.MsgTx) error
}

type GetTicketsRequest struct {
	StartingBlockHash   []byte
	StartingBlockHeight int32
	EndingBlockHash     []byte
	EndingBlockHeight   int32
	TargetTicketCount   int32
}

type TicketInfo struct {
	BlockHeight int32
	Status      string
	Ticket      *wallet.TransactionSummary
	Spender     *wallet.TransactionSummary
}

type TicketPriceResponse struct {
	TicketPrice int64
	Height      int32
}

type VSPTicketPurchaseInfo struct {
	PoolAddress   string
	PoolFees      float64
	Script        string
	TicketAddress string
}

type StakingOverview struct {
	All      int
	Immature int
	Live     int
	Voted    int
	Expired  int
	Revoked  int
}

/** end ticket-related types */

/** begin politeia types */
type Proposal struct {
	ID               int    `storm:"id,increment"`
	Token            string `json:"token" storm:"unique"`
	Category         int32  `json:"category" storm:"index"`
	Name             string `json:"name"`
	State            int32  `json:"state"`
	Status           int32  `json:"status"`
	Timestamp        int64  `json:"timestamp"`
	UserID           string `json:"userid"`
	Username         string `json:"username"`
	NumComments      int32  `json:"numcomments"`
	Version          string `json:"version"`
	PublishedAt      int64  `json:"publishedat"`
	IndexFile        string `json:"indexfile"`
	IndexFileVersion string `json:"fileversion"`
	VoteStatus       int32  `json:"votestatus"`
	VoteApproved     bool   `json:"voteapproved"`
	YesVotes         int32  `json:"yesvotes"`
	NoVotes          int32  `json:"novotes"`
	EligibleTickets  int32  `json:"eligibletickets"`
	QuorumPercentage int32  `json:"quorumpercentage"`
	PassPercentage   int32  `json:"passpercentage"`
}

type ProposalVoteDetails struct {
	EligibleTickets []*EligibleTicket
	Votes           []*ProposalVote
	YesVotes        int32
	NoVotes         int32
}

type EligibleTicket struct {
	Hash    string
	Address string
}

type ProposalVote struct {
	Ticket *EligibleTicket
	Bit    string
}

type ProposalNotificationListener interface {
	OnProposalsSynced()
	OnNewProposal(proposal *Proposal)
	OnProposalVoteStarted(proposal *Proposal)
	OnProposalVoteFinished(proposal *Proposal)
}

/** end politea proposal types */

type UnspentOutput struct {
	TransactionHash []byte
	OutputIndex     uint32
	OutputKey       string
	ReceiveTime     int64
	Amount          int64
	FromCoinbase    bool
	Tree            int32
	PkScript        []byte
	Addresses       string // separated by commas
	Confirmations   int32
}

/** end politea proposal types */

/** begin vspd-related types */
type VspInfoResponse struct {
	APIVersions   []int64 `json:"apiversions"`
	Timestamp     int64   `json:"timestamp"`
	PubKey        []byte  `json:"pubkey"`
	FeePercentage float64 `json:"feepercentage"`
	VspClosed     bool    `json:"vspclosed"`
	Network       string  `json:"network"`
	VspdVersion   string  `json:"vspdversion"`
	Voting        int64   `json:"voting"`
	Voted         int64   `json:"voted"`
	Revoked       int64   `json:"revoked"`
}

type PayFeeRequest struct {
	Timestamp   int64             `json:"timestamp"`
	TicketHash  string            `json:"tickethash"`
	FeeTx       json.Marshaler    `json:"feetx"`
	VotingKey   string            `json:"votingkey"`
	VoteChoices map[string]string `json:"votechoices"`
}

type PayFeeResponse struct {
	Timestamp int64  `json:"timestamp"`
	Request   []byte `json:"request"`
}

type VspdTicketInfo struct {
	Hash            string            `storm:"id,unique" json:"tickethash"`
	FeeAddress      string            `json:"feeaddress"`
	FeeAmount       int64             `json:"feeamount"`
	Expiration      int64             `json:"expiration"`
	Timestamp       int64             `json:"timestamp"`
	FeeTx           string            `json:"feetx"`
	FeeTxHash       string            `json:"feetxhash"`
	FeeTxStatus     string            `json:"feetxstatus"`
	VoteChoices     map[string]string `json:"votechoices"`
	TicketConfirmed bool              `json:"ticketconfirmed"`
}

type FeeAddressResponse struct {
	Timestamp  int64  `json:"timestamp"`
	FeeAddress string `json:"feeaddress"`
	FeeAmount  int64  `json:"feeamount"`
	Request    []byte `json:"request"`
}

type FeeAddressRequest struct {
	Timestamp  int64          `json:"timestamp"`
	TicketHash string         `json:"tickethash"`
	TicketHex  json.Marshaler `json:"tickethex"`
	ParentHex  json.Marshaler `json:"parenthex"`
}

type PendingFee struct {
	CommitmentAddress dcrutil.Address
	VotingAddress     dcrutil.Address
	FeeAddress        dcrutil.Address
	FeeAmount         dcrutil.Amount
	FeeTx             *wire.MsgTx
}

/** end vspd-related types */
