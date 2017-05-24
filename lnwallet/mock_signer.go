package lnwallet

import 	(
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)


type mockSigner struct {
	key *btcec.PrivateKey
}

func (m *mockSigner) SignOutputRaw(tx *wire.MsgTx, signDesc *SignDescriptor) ([]byte, error) {
	shScript := signDesc.P2SHScript
	privKey := m.key

	sig, err := txscript.RawTxInSignature(tx,
		signDesc.InputIndex, shScript, txscript.SigHashAll, privKey)
	if err != nil {
		return nil, err
	}

	return sig[:len(sig)-1], nil
}
func (m *mockSigner) ComputeInputScript(tx *wire.MsgTx, signDesc *SignDescriptor) (*InputScript, error) {
	// It will generate simple p2pkh input script

	signature, err := txscript.RawTxInSignature(tx,
		signDesc.InputIndex, signDesc.Output.PkScript,
		txscript.SigHashAll, m.key)
	if err != nil {
		return nil, err
	}
	b := txscript.NewScriptBuilder()
	b.AddData(signature)
	b.AddData(m.key.PubKey().SerializeCompressed())
	unlockScript, err := b.Script()
	if err != nil {
		return nil, err
	}

	return &InputScript{
		ScriptSig: unlockScript,
	}, nil
}

type mockNotfier struct {
}

func (m *mockNotfier) RegisterConfirmationsNtfn(txid *chainhash.Hash, numConfs, heightHint uint32) (*chainntnfs.ConfirmationEvent, error) {
	return nil, nil
}
func (m *mockNotfier) RegisterBlockEpochNtfn() (*chainntnfs.BlockEpochEvent, error) {
	return nil, nil
}

func (m *mockNotfier) Start() error {
	return nil
}

func (m *mockNotfier) Stop() error {
	return nil
}
func (m *mockNotfier) RegisterSpendNtfn(outpoint *wire.OutPoint, heightHint uint32) (*chainntnfs.SpendEvent, error) {
	return &chainntnfs.SpendEvent{
		Spend: make(chan *chainntnfs.SpendDetail),
	}, nil
}
