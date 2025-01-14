// SPDX-License-Identifier: BUSL-1.1
//
// Copyright (C) 2023, Berachain Foundation. All rights reserved.
// Use of this software is govered by the Business Source License included
// in the LICENSE file of this repository and at www.mariadb.com/bsl11.
//
// ANY USE OF THE LICENSED WORK IN VIOLATION OF THIS LICENSE WILL AUTOMATICALLY
// TERMINATE YOUR RIGHTS UNDER THIS LICENSE FOR THE CURRENT AND ALL OTHER
// VERSIONS OF THE LICENSED WORK.
//
// THIS LICENSE DOES NOT GRANT YOU ANY RIGHT IN ANY TRADEMARK OR LOGO OF
// LICENSOR OR ITS AFFILIATES (PROVIDED THAT YOU MAY USE A TRADEMARK OR LOGO OF
// LICENSOR AS EXPRESSLY REQUIRED BY THIS LICENSE).
//
// TO THE EXTENT PERMITTED BY APPLICABLE LAW, THE LICENSED WORK IS PROVIDED ON
// AN “AS IS” BASIS. LICENSOR HEREBY DISCLAIMS ALL WARRANTIES AND CONDITIONS,
// EXPRESS OR IMPLIED, INCLUDING (WITHOUT LIMITATION) WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, NON-INFRINGEMENT, AND
// TITLE.

package state

import (
	"math/big"
	"pkg.berachain.dev/polaris/eth/core"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pkg.berachain.dev/polaris/eth/common"
)

// InitGenesis takes in a pointer to a genesis state object and populates the KV store.
func (p *plugin) InitGenesis(ctx sdk.Context, ethGen *core.Genesis) {
	p.Reset(ctx)

	p.CreateAccount(core.ReservedAddress)

	// Iterate over the genesis accounts and set the balances.
	for address, account := range ethGen.Alloc {
		// TODO: technically wrong since its overriding / hacking the auth keeper and
		// we are using the nonce from the account keeper as well.
		p.CreateAccount(address)

		// TODO(thai): we should rethink about this since we are using bank module for balances.
		// p.SetBalance(address, account.Balance)

		if account.Code != nil {
			p.SetCode(address, account.Code)
		}
		if account.Storage != nil {
			for k, v := range account.Storage {
				p.SetState(address, k, v)
			}
		}
		if account.Nonce != 0 {
			p.SetNonce(address, account.Nonce)
		}
	}
	p.Finalize()
}

// Export genesis modifies a pointer to a genesis state object and populates it.
func (p *plugin) ExportGenesis(ctx sdk.Context, ethGen *core.Genesis) {
	p.Reset(ctx)
	ethGen.Alloc = make(core.GenesisAlloc)

	// NOTE: we use bank module for balances, so we don't need to iterate balances to set the genesis accounts.
	//// Iterate Balances and set the genesis accounts.
	//p.IterateBalances(func(address common.Address, balance *big.Int) bool {
	//	account, ok := ethGen.Alloc[address]
	//	if !ok {
	//		account = core.GenesisAccount{}
	//	}
	//	account.Code = p.GetCode(address)
	//	if account.Code != nil {
	//		account.Storage = make(map[common.Hash]common.Hash)
	//	}
	//	account.Balance = p.GetBalance(address)
	//	account.Nonce = p.GetNonce(address)
	//	ethGen.Alloc[address] = account
	//	return false
	//})

	// Iterate Storage and set the genesis accounts.
	p.IterateState(func(address common.Address, key common.Hash, value common.Hash) bool {
		account, ok := ethGen.Alloc[address]
		if !ok {
			account = core.GenesisAccount{}
		}
		if account.Storage == nil {
			account.Storage = make(map[common.Hash]common.Hash)
		}
		account.Storage[key] = value

		account.Code = p.GetCode(address)
		account.Balance = p.GetBalance(address)
		account.Nonce = p.GetNonce(address)
		ethGen.Alloc[address] = account

		return false
	})

	// Iterate Code and set the genesis accounts.
	p.IterateCode(func(address common.Address, codeHash common.Hash) bool {
		account, ok := ethGen.Alloc[address]
		if !ok {
			account = core.GenesisAccount{}
		}
		account.Code = p.GetCode(address)
		account.Nonce = p.GetNonce(address)
		if account.Balance == nil {
			account.Balance = big.NewInt(0)
		}
		ethGen.Alloc[address] = account

		return false
	})
}
