package types

type KeysignRequest struct {
	OutChain       string
	OutHash        string
	OutBlockHeight int64
	OutBytes       []byte
}

type KeysignResult struct {
	Success   bool
	ErrMesage string

	OutChain       string
	OutHash        string
	OutBlockHeight int64

	PubKey    []byte // Public key of the private key that used for signing.
	OutBytes  []byte
	Signature []byte
}
