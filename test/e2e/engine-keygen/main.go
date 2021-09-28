package main

import (
	"flag"
	"math/big"
	"time"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/p2p"
	types2 "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/tss"
)

type EngineCallback struct {
	keygenDataCh  chan *types2.KeygenResult
	presignDataCh chan *types2.PresignResult
	signingDataCh chan *types2.KeysignResult
}

func NewEngineCallback(
	keygenDataCh chan *types2.KeygenResult,
	presignDataCh chan *types2.PresignResult,
	signingDataCh chan *types2.KeysignResult,
) *EngineCallback {
	return &EngineCallback{
		keygenDataCh, presignDataCh, signingDataCh,
	}
}

func (cb *EngineCallback) OnWorkKeygenFinished(result *types2.KeygenResult) {
	cb.keygenDataCh <- result
}

func (cb *EngineCallback) OnWorkPresignFinished(result *types2.PresignResult) {
	cb.presignDataCh <- result
}

func (cb *EngineCallback) OnWorkSigningFinished(result *types2.KeysignResult) {
	cb.signingDataCh <- result
}

func (cb *EngineCallback) OnWorkFailed(chain string, workType types.WorkType, culprit []*tss.PartyID) {

}

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllPrivateKeys(n)
	partyIds := make([]*tss.PartyID, n)

	// Creates list of party ids
	for i := 0; i < n; i++ {
		bz := keys[i].PubKey().Bytes()
		peerId := p2p.P2PIDFromKey(keys[i])
		party := tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(bz))
		partyIds[i] = party
	}

	return tss.SortPartyIDs(partyIds, 0)
}

func main() {
	var index, n int
	flag.IntVar(&index, "index", 0, "listening port")
	flag.Parse()

	n = 2

	config, privateKey := p2p.GetMockConnectionConfig(n, index)
	cm := p2p.NewConnectionManager(config)
	err := cm.Start(privateKey)
	if err != nil {
		panic(err)
	}

	pids := make([]*tss.PartyID, n)
	allKeys := p2p.GetAllPrivateKeys(n)
	nodes := make([]*core.Node, n)

	// Add nodes
	privKeys := p2p.GetAllPrivateKeys(n)
	for i := 0; i < n; i++ {
		pubKey := privKeys[i].PubKey()
		node := core.NewNode(pubKey)
		nodes[i] = node
		pids[i] = node.PartyId
	}
	pids = tss.SortPartyIDs(pids)

	// Create new engine
	outCh := make(chan *types2.KeygenResult)
	cb := NewEngineCallback(outCh, nil, nil)
	engine := core.NewEngine(nodes[index], cm, helper.NewMockDatabase(), cb, allKeys[index])
	cm.AddListener(p2p.TSSProtocolID, engine)

	// Add nodes
	for i := 0; i < n; i++ {
		engine.AddNodes(nodes)
	}

	time.Sleep(time.Second * 3)

	// Add request
	workId := "keygen0"
	request := types.NewKeygenRequest(workId, n, pids, *helper.LoadPreparams(index), n-1)
	err = engine.AddRequest(request)
	if err != nil {
		panic(err)
	}

	select {
	case result := <-outCh:
		utils.LogInfo("Result ", result)
	}
}
