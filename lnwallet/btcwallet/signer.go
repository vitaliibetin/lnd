package btcwallet

import (
	"fmt"

	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/waddrmgr"
	base "github.com/btcsuite/btcwallet/wallet"
)

// FetchInputInfo queries for the WalletController's knowledge of the passed
// outpoint. If the base wallet determines this output is under its control,
// then the original txout should be returned. Otherwise, a non-nil error value
// of ErrNotMine should be returned instead.
//
// This is a part of the WalletController interface.
func (b *BtcWallet) FetchInputInfo(prevOut *wire.OutPoint) (*wire.TxOut, error) {
	var (
		err    error
		output *wire.TxOut
	)

	// First check to see if the output is already within the utxo cache.
	// If so we can return directly saving a disk access.
	b.cacheMtx.RLock()
	if output, ok := b.utxoCache[*prevOut]; ok {
		b.cacheMtx.RUnlock()
		return output, nil
	}
	b.cacheMtx.RUnlock()

	// Otherwse, we manually look up the output within the tx store.
	txid := &prevOut.Hash
	txDetail, err := base.UnstableAPI(b.wallet).TxDetails(txid)
	if err != nil {
		return nil, err
	} else if txDetail == nil {
		return nil, lnwallet.ErrNotMine
	}

	output = txDetail.TxRecord.MsgTx.TxOut[prevOut.Index]

	b.cacheMtx.Lock()
	b.utxoCache[*prevOut] = output
	b.cacheMtx.Unlock()

	return output, nil
}

// fetchOutputKey attempts to fetch the managed address corresponding to the
// passed output script. This function is used to look up the proper key which
// should be used to sign a specified input.
func (b *BtcWallet) fetchOutputAddr(script []byte) (waddrmgr.ManagedAddress, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, b.netParams)
	if err != nil {
		return nil, err
	}

	// If the case of a multi-sig output, several address may be extracted.
	// Therefore, we simply select the key for the first address we know
	// of.
	for _, addr := range addrs {
		addr, err := b.wallet.AddressInfo(addr)
		if err == nil {
			return addr, nil
		}
	}

	// TODO(roasbeef): use the errors.wrap package
	return nil, fmt.Errorf("address not found")
}

// fetchPrivKey attempts to retrieve the raw private key corresponding to the
// passed public key.
// TODO(roasbeef): alternatively can extract all the data pushes within the
// script, then attempt to match keys one by one
func (b *BtcWallet) fetchPrivKey(pub *btcec.PublicKey) (*btcec.PrivateKey, error) {
	hash160 := btcutil.Hash160(pub.SerializeCompressed())
	addr, err := btcutil.NewAddressPubKeyHash(hash160, b.netParams)
	if err != nil {
		return nil, err
	}

	return b.wallet.PrivKeyForAddress(addr)
}

// SignOutputRaw generates a signature for the passed transaction according to
// the data within the passed SignDescriptor.
//
// This is a part of the WalletController interface.
func (b *BtcWallet) SignOutputRaw(tx *wire.MsgTx, signDesc *lnwallet.SignDescriptor) ([]byte, error) {
	witnessScript := signDesc.P2SHScript

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := b.fetchPrivKey(signDesc.PubKey)
	if err != nil {
		return nil, err
	}

	// If a tweak is specified, then we'll need to use this tweak to derive
	// the final private key to be used for signing this output.
	if signDesc.PrivateTweak != nil {
		privKey = lnwallet.DeriveRevocationPrivKey(privKey,
			signDesc.PrivateTweak)
	}

	sig, err := txscript.RawTxInSignature(tx,
		signDesc.InputIndex, witnessScript, txscript.SigHashAll,
		privKey)
	if err != nil {
		return nil, err
	}

	// Chop off the sighash flag at the end of the signature.
	return sig[:len(sig)-1], nil
}

// ComputeInputScript generates a complete InputIndex for the passed
// transaction with the signature as defined within the passed SignDescriptor.
// This method is capable of generating the proper input script for p2pkh output
//
// This is a part of the WalletController interface.
func (b *BtcWallet) ComputeInputScript(tx *wire.MsgTx,
	signDesc *lnwallet.SignDescriptor) (*lnwallet.InputScript, error) {
	// TODO(mkl): actually check if this is correct for NOSEGWIT
	outputScript := signDesc.Output.PkScript
	walletAddr, err := b.fetchOutputAddr(outputScript)
	if err != nil {
		return nil, nil
	}

	pka := walletAddr.(waddrmgr.ManagedPubKeyAddress)
	privKey, err := pka.PrivKey()
	if err != nil {
		return nil, err
	}

	inputScript := &lnwallet.InputScript{}
	// TODO(mkl): check for other script types
	// Generate a valid witness stack for the input.
	// TODO(roasbeef): adhere to passed HashType
	unlockScript, err := txscript.SignatureScript(tx,
		signDesc.InputIndex, outputScript,
		txscript.SigHashAll, privKey, true)
	if err != nil {
		return nil, err
	}

	inputScript.ScriptSig = unlockScript
	return inputScript, nil
}

// A compile time check to ensure that BtcWallet implements the Signer
// interface.
var _ lnwallet.Signer = (*BtcWallet)(nil)

// SignMessage attempts to sign a target message with the private key that
// corresponds to the passed public key. If the target private key is unable to
// be found, then an error will be returned. The actual digest signed is the
// double SHA-256 of the passed message.
//
// NOTE: This is a part of the MessageSigner interface.
func (b *BtcWallet) SignMessage(pubKey *btcec.PublicKey,
	msg []byte) (*btcec.Signature, error) {

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := b.fetchPrivKey(pubKey)
	if err != nil {
		return nil, err
	}

	// Double hash and sign the data.
	msgDigest := chainhash.DoubleHashB(msg)
	sign, err := privKey.Sign(msgDigest)
	if err != nil {
		return nil, errors.Errorf("unable sign the message: %v", err)
	}

	return sign, nil
}

// A compile time check to ensure that BtcWallet implements the MessageSigner
// interface.
var _ lnwallet.MessageSigner = (*BtcWallet)(nil)
