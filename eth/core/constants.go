package core

import (
	"pkg.berachain.dev/polaris/eth/common"
	"pkg.berachain.dev/polaris/eth/crypto"
)

var (
	ReservedPrivateKey = crypto.ToECDSAUnsafe(common.FromHex("1234123412341234123412341234123412341234123412341234123412341234"))
	ReservedAddress    = common.HexToAddress("0xfF06ad5d076fa274B49C297f3fE9e29B5bA9AaDC")
)
