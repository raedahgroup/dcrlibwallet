package dcrlibwallet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"decred.org/dcrwallet/ticketbuyer"
	w "decred.org/dcrwallet/wallet"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v3"
	"github.com/planetdecred/dcrlibwallet/internal/certs"
)

const (
	smalletSplitPoint  = 000.00262144
	ShuffleServer      = "cspp.decred.org"
	MainnetShufflePort = "5760"
	TestnetShufflePort = "15760"
	MixedAccountBranch = 0
)

func (mw *MultiWallet) SetAccountMixerNotification(accountMixerNotificationListener AccountMixerNotificationListener) {
	mw.accountMixerNotificationListener = accountMixerNotificationListener
}

// CreateMixerAccounts creates the two accounts needed for the account mixer. This function
// is added to ease unlocking the wallet before creating accounts. This function should be
// used with auto cspp mixer setup.
func (wallet *Wallet) CreateMixerAccounts(mixedAccount, unmixedAccount, privPass string) error {
	accountMixerConfigSet := wallet.ReadBoolConfigValueForKey(AccountMixerConfigSet, false)
	if accountMixerConfigSet {
		return errors.New(ErrInvalid)
	}

	if wallet.HasAccount(mixedAccount) || wallet.HasAccount(unmixedAccount) {
		return errors.New(ErrExist)
	}

	err := wallet.UnlockWallet([]byte(privPass))
	if err != nil {
		return err
	}

	defer wallet.LockWallet()

	mixedAccountNumber, err := wallet.NextAccount(mixedAccount)
	if err != nil {
		return err
	}

	unmixedAccountNumber, err := wallet.NextAccount(unmixedAccount)
	if err != nil {
		return err
	}

	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, mixedAccountNumber)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, unmixedAccountNumber)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, true)

	return nil
}

// SetAccountMixerConfig sets the config for mixed and unmixed account. Private passphrase is verifed
// for security even if not used. This function should be used with manual cspp mixer setup.
func (wallet *Wallet) SetAccountMixerConfig(mixedAccount, unmixedAccount int32, privPass string) error {

	if mixedAccount == unmixedAccount {
		return errors.New(ErrInvalid)
	}

	// Verify that account numbers are correct
	_, err := wallet.GetAccount(mixedAccount)
	if err != nil {
		return errors.New(ErrNotExist)
	}

	_, err = wallet.GetAccount(unmixedAccount)
	if err != nil {
		return errors.New(ErrNotExist)
	}

	err = wallet.UnlockWallet([]byte(privPass))
	if err != nil {
		return err
	}
	wallet.LockWallet()

	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, mixedAccount)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, unmixedAccount)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, true)

	return nil
}

func (wallet *Wallet) ClearMixerConfig() {
	wallet.SetInt32ConfigValueForKey(AccountMixerMixedAccount, -1)
	wallet.SetInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)
	wallet.SetBoolConfigValueForKey(AccountMixerConfigSet, false)
}

func (mw *MultiWallet) ReadyToMix(walletID int) (bool, error) {
	wallet := mw.WalletWithID(walletID)
	if wallet == nil {
		return false, errors.New(ErrNotExist)
	}

	unmixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)

	hasMixableOutput, err := wallet.accountHasMixableOutput(unmixedAccount)
	if err != nil {
		return false, translateError(err)
	}

	return hasMixableOutput, nil
}

// StartAccountMixer starts the automatic account mixer
func (mw *MultiWallet) StartAccountMixer(walletID int, walletPassphrase string) error {

	if !mw.IsConnectedToDecredNetwork() {
		return errors.New(ErrNotConnected)
	}

	wallet := mw.WalletWithID(walletID)
	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	tb := ticketbuyer.New(wallet.internal)

	mixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerMixedAccount, -1)
	unmixedAccount := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)

	hasMixableOutput, err := wallet.accountHasMixableOutput(unmixedAccount)
	if err != nil {
		return translateError(err)
	} else if !hasMixableOutput {
		return errors.New(ErrNoMixableOutput)
	}

	var shufflePort = TestnetShufflePort
	var dialCSPPServer func(ctx context.Context, network, addr string) (net.Conn, error)
	if mw.chainParams.Net == chaincfg.MainNetParams().Net {
		shufflePort = MainnetShufflePort

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(certs.CSPP))

		csppTLSConfig := new(tls.Config)
		csppTLSConfig.ServerName = ShuffleServer
		csppTLSConfig.RootCAs = pool

		dailer := new(net.Dialer)
		dialCSPPServer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dailer.DialContext(context.Background(), network, addr)
			if err != nil {
				return nil, err
			}

			conn = tls.Client(conn, csppTLSConfig)
			return conn, nil
		}
	}

	tb.AccessConfig(func(c *ticketbuyer.Config) {
		c.MixedAccountBranch = MixedAccountBranch
		c.MixedAccount = uint32(mixedAccount)
		c.ChangeAccount = uint32(unmixedAccount)
		c.CSPPServer = ShuffleServer + ":" + shufflePort
		c.DialCSPPServer = dialCSPPServer
		c.BuyTickets = false
		c.MixChange = true
	})

	err = wallet.UnlockWallet([]byte(walletPassphrase))
	if err != nil {
		return translateError(err)
	}

	go func() {
		log.Info("Running account mixer")
		if mw.accountMixerNotificationListener != nil {
			mw.accountMixerNotificationListener.OnAccountMixerStarted(walletID)
		}

		ctx, cancel := mw.contextWithShutdownCancel()
		wallet.cancelAccountMixer = cancel
		err = tb.Run(ctx, []byte(walletPassphrase))
		if err != nil {
			log.Errorf("AccountMixer instance errored: %v", err)
		}

		wallet.cancelAccountMixer = nil
		if mw.accountMixerNotificationListener != nil {
			mw.accountMixerNotificationListener.OnAccountMixerEnded(walletID)
		}
	}()

	return nil
}

// StopAccountMixer stops the active account mixer
func (mw *MultiWallet) StopAccountMixer(walletID int) error {

	wallet := mw.WalletWithID(walletID)
	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	if wallet.cancelAccountMixer == nil {
		return errors.New(ErrInvalid)
	}

	wallet.cancelAccountMixer()
	wallet.cancelAccountMixer = nil
	return nil
}

func (wallet *Wallet) accountHasMixableOutput(accountNumber int32) (bool, error) {

	policy := w.OutputSelectionPolicy{
		Account:               uint32(accountNumber),
		RequiredConfirmations: wallet.RequiredConfirmations(),
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	inputDetail, err := wallet.internal.SelectInputs(wallet.shutdownContext(), dcrutil.Amount(0), policy)
	if err != nil {
		return false, nil
	}

	hasMixableOutput := false
	for _, input := range inputDetail.Inputs {
		if AmountCoin(input.ValueIn) > smalletSplitPoint {
			hasMixableOutput = true
			break
		}
	}

	if !hasMixableOutput {
		accountName, err := wallet.AccountName(accountNumber)
		if err != nil {
			return hasMixableOutput, nil
		}

		lockedOutpoints, err := wallet.internal.LockedOutpoints(wallet.shutdownContext(), accountName)
		if err != nil {
			return hasMixableOutput, nil
		}
		hasMixableOutput = len(lockedOutpoints) > 0
	}

	return hasMixableOutput, nil
}

// IsAccountMixerActive returns true if account mixer is active
func (wallet *Wallet) IsAccountMixerActive() bool {
	return wallet.cancelAccountMixer != nil
}
