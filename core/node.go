package core

import (
	"math/big"
	"sort"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/tss"
)

type Node struct {
	PeerId  peer.ID
	PubKey  ctypes.PubKey
	PartyId *tss.PartyID
}

func NewNode(pubKey ctypes.PubKey) *Node {
	var p2pPubKey crypto.PubKey
	var err error
	switch pubKey.Type() {
	case "ed25519":
		p2pPubKey, err = crypto.UnmarshalEd25519PublicKey(pubKey.Bytes())
	case "secp256k1":
		p2pPubKey, err = crypto.UnmarshalSecp256k1PublicKey(pubKey.Bytes())
	default:
		utils.LogError("Unsupported pub key type", pubKey.Type())
		return nil
	}

	if err != nil {
		utils.LogError(err)
		return nil
	}

	peerId, err := peer.IDFromPublicKey(p2pPubKey)
	if err != nil {
		utils.LogError("Cannot convert pubkey to peerId")
		return nil
	}

	return &Node{
		peerId, pubKey, tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(pubKey.Bytes())),
	}
}

func NewNodes(tPubKeys []ctypes.PubKey) []*Node {
	nodes := make([]*Node, len(tPubKeys))
	pids := make([]*tss.PartyID, len(tPubKeys))

	for i, pubKey := range tPubKeys {
		node := NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}

	// Sort nodes by partyId
	tss.SortPartyIDs(pids)
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].PartyId.Index < nodes[j].PartyId.Index
	})

	return nodes
}
