package utils

import (
	"github.com/sisu-network/dheart/common"
)

func IsEcDSA(chain string) bool {
	return chain == common.CHAIN_ETH || chain == common.CHAIN_ETH_MAINNET
}
