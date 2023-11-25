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

package mempool

import (
	"context"
	"errors"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"pkg.berachain.dev/polaris/eth/core"

	sdk "github.com/cosmos/cosmos-sdk/types"

	evmtypes "pkg.berachain.dev/polaris/cosmos/x/evm/types"
	"pkg.berachain.dev/polaris/eth/common"
	coretypes "pkg.berachain.dev/polaris/eth/core/types"
	errorslib "pkg.berachain.dev/polaris/lib/errors"
)

func checkTxSigner(tx sdk.Tx) error {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return errors.New("invalid transaction type")
	}
	signers, err := sigTx.GetSigners()
	if err != nil {
		return errorslib.Wrap(err, "failed to get signers")
	}

	for _, signer := range signers {
		isReservedAccount := common.BytesToAddress(signer) == core.ReservedAddress
		if isReservedAccount {
			return errors.New("reserved account not allowed to send tx via mempool")
		}
	}

	return nil
}

// Insert is called when a transaction is added to the mempool.
func (etp *EthTxPool) Insert(ctx context.Context, tx sdk.Tx) error {
	etp.mu.Lock()
	defer etp.mu.Unlock()

	if err := checkTxSigner(tx); err != nil {
		return err
	}

	// Call the base mempool's Insert method
	if err := etp.PriorityNonceMempool.Insert(ctx, tx); err != nil {
		return err
	}

	// We want to cache the transaction for lookup.
	if ethTx := evmtypes.GetAsEthTx(tx); ethTx != nil {
		sender := coretypes.GetSender(ethTx)
		nonce := ethTx.Nonce()

		// Reject txs with a nonce lower than the nonce reported by the statedb.
		if sdbNonce := etp.nr.GetNonce(sender); sdbNonce > nonce {
			return errorslib.Wrap(etp.PriorityNonceMempool.Remove(tx), "nonce too low")
		}

		// Delete old hash if the sender has a tx with the same nonce.
		if senderNonceHash := etp.nonceToHash[sender]; senderNonceHash != nil {
			delete(etp.ethTxCache, senderNonceHash[nonce])
		}

		// Add new hash.
		newHash := ethTx.Hash()
		if etp.nonceToHash[sender] == nil {
			etp.nonceToHash[sender] = make(map[uint64]common.Hash)
		}
		etp.nonceToHash[sender][nonce] = newHash
		etp.ethTxCache[newHash] = ethTx
	}

	return nil
}

// Remove is called when a transaction is removed from the mempool.
func (etp *EthTxPool) Remove(tx sdk.Tx) error {
	etp.mu.Lock()
	defer etp.mu.Unlock()

	// Call the base mempool's Remove method
	if err := etp.PriorityNonceMempool.Remove(tx); err != nil {
		return err
	}

	// We want to remove any references to the tx from the cache.
	if ethTx := evmtypes.GetAsEthTx(tx); ethTx != nil {
		delete(etp.ethTxCache, ethTx.Hash())
		delete(etp.nonceToHash[coretypes.GetSender(ethTx)], ethTx.Nonce())
	}

	return nil
}
