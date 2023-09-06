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

package lib

import (
	"math/big"
	"time"

	"cosmossdk.io/core/address"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	libgenerated "pkg.berachain.dev/polaris/contracts/bindings/cosmos/lib"
	"pkg.berachain.dev/polaris/contracts/bindings/cosmos/precompile/governance"
	"pkg.berachain.dev/polaris/contracts/bindings/cosmos/precompile/staking"
	"pkg.berachain.dev/polaris/cosmos/precompile"
	"pkg.berachain.dev/polaris/lib/utils"
)

/**
 * This file contains conversions between native Cosmos SDK types and go-ethereum ABI types.
 */

// SdkCoinsToEvmCoins converts sdk.Coins into []libgenerated.CosmosCoin.
func SdkCoinsToEvmCoins(sdkCoins sdk.Coins) []libgenerated.CosmosCoin {
	evmCoins := make([]libgenerated.CosmosCoin, len(sdkCoins))
	for i, coin := range sdkCoins {
		evmCoins[i] = SdkCoinToEvmCoin(coin)
	}
	return evmCoins
}

// SdkCoinsToEvmCoin converts sdk.Coin into libgenerated.CosmosCoin.
func SdkCoinToEvmCoin(coin sdk.Coin) libgenerated.CosmosCoin {
	evmCoin := libgenerated.CosmosCoin{
		Amount: coin.Amount.BigInt(),
		Denom:  coin.Denom,
	}
	return evmCoin
}

func SdkPageResponseToEvmPageResponse(pageResponse *query.PageResponse) libgenerated.CosmosPageResponse {
	if pageResponse == nil {
		return libgenerated.CosmosPageResponse{}
	}
	return libgenerated.CosmosPageResponse{
		NextKey: string(pageResponse.GetNextKey()),
		Total:   pageResponse.GetTotal(),
	}
}

// ExtractCoinsFromInput converts coins from input (of type any) into sdk.Coins.
func ExtractCoinsFromInput(coins any) (sdk.Coins, error) {
	// note: we have to use unnamed struct here, otherwise the compiler cannot cast
	// the any type input into IBankModuleCoin.
	amounts, ok := utils.GetAs[[]struct {
		Amount *big.Int `json:"amount"`
		Denom  string   `json:"denom"`
	}](coins)
	if !ok {
		return nil, precompile.ErrInvalidCoin
	}

	sdkCoins := sdk.Coins{}
	for _, evmCoin := range amounts {
		sdkCoins = append(sdkCoins, sdk.Coin{
			Denom: evmCoin.Denom, Amount: sdkmath.NewIntFromBigInt(evmCoin.Amount),
		})
	}
	// sort the coins by denom, as Cosmos expects and remove any 0 amounts.
	sdkCoins = sdk.NewCoins(sdkCoins...)
	if len(sdkCoins) == 0 {
		return nil, precompile.ErrInvalidCoin
	}

	return sdkCoins, nil
}

func ExtractPageRequestFromInput(pageRequest any) *query.PageRequest {
	// note: we have to use unnamed struct here, otherwise the compiler cannot cast
	// the any type input into the contract's generated type.
	pageReq, ok := utils.GetAs[struct {
		Key        string `json:"key"`
		Offset     uint64 `json:"offset"`
		Limit      uint64 `json:"limit"`
		CountTotal bool   `json:"count_total"`
		Reverse    bool   `json:"reverse"`
	}](pageRequest)
	if !ok {
		return nil
	}

	return &query.PageRequest{
		Key:        []byte(pageReq.Key),
		Offset:     pageReq.Offset,
		Limit:      pageReq.Limit,
		CountTotal: pageReq.CountTotal,
		Reverse:    pageReq.Reverse,
	}
}

// ExtractCoinFromInputToCoin converts a coin from input (of type any) into sdk.Coins.
func ExtractCoinFromInputToCoin(coin any) (sdk.Coin, error) {
	// note: we have to use unnamed struct here, otherwise the compiler cannot cast
	// the any type input into IBankModuleCoin.
	amounts, ok := utils.GetAs[struct {
		Amount *big.Int `json:"amount"`
		Denom  string   `json:"denom"`
	}](coin)
	if !ok {
		return sdk.Coin{}, precompile.ErrInvalidCoin
	}

	sdkCoin := sdk.NewCoin(amounts.Denom, sdkmath.NewIntFromBigInt(amounts.Amount))
	return sdkCoin, nil
}

// GetGrantAsSendAuth maps a list of grants to a list of send authorizations.
func GetGrantAsSendAuth(
	grants []*authz.Grant, blocktime time.Time,
) ([]*banktypes.SendAuthorization, error) {
	var sendAuths []*banktypes.SendAuthorization
	for _, grant := range grants {
		// Check that the expiration is still valid.
		if grant.Expiration == nil || grant.Expiration.After(blocktime) {
			sendAuth, ok := utils.GetAs[*banktypes.SendAuthorization](grant.Authorization.GetCachedValue())
			if !ok {
				return nil, precompile.ErrInvalidGrantType
			}
			sendAuths = append(sendAuths, sendAuth)
		}
	}
	return sendAuths, nil
}

// SdkUDEToStakingUDE converts a Cosmos SDK Unbonding Delegation Entry list to a geth compatible
// list of Unbonding Delegation Entries.
func SdkUDEToStakingUDE(ude []stakingtypes.UnbondingDelegationEntry) []staking.IStakingModuleUnbondingDelegationEntry {
	entries := make([]staking.IStakingModuleUnbondingDelegationEntry, len(ude))
	for i, entry := range ude {
		entries[i] = staking.IStakingModuleUnbondingDelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.String(),
			InitialBalance: entry.InitialBalance.BigInt(),
			Balance:        entry.Balance.BigInt(),
		}
	}
	return entries
}

// SdkREToStakingRE converts a Cosmos SDK Redelegation Entry list to a geth compatible list of
// Redelegation Entries.
func SdkREToStakingRE(re []stakingtypes.RedelegationEntry) []staking.IStakingModuleRedelegationEntry {
	entries := make([]staking.IStakingModuleRedelegationEntry, len(re))
	for i, entry := range re {
		entries[i] = staking.IStakingModuleRedelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.String(),
			InitialBalance: entry.InitialBalance.BigInt(),
			SharesDst:      entry.SharesDst.BigInt(),
		}
	}
	return entries
}

// SdkValidatorsToStakingValidators converts a Cosmos SDK Validator list to a geth compatible list
// of Validators.
func SdkValidatorsToStakingValidators(valAddrCodec address.Codec, vals []stakingtypes.Validator) (
	[]staking.IStakingModuleValidator, error,
) {
	valsOut := make([]staking.IStakingModuleValidator, len(vals))
	for i, val := range vals {
		operEthAddr, err := EthAddressFromString(valAddrCodec, val.OperatorAddress)
		if err != nil {
			return nil, err
		}
		pubKey, err := val.ConsPubKey()
		if err != nil {
			return nil, err
		}
		valsOut[i] = staking.IStakingModuleValidator{
			OperatorAddr:    operEthAddr,
			ConsAddr:        pubKey.Address(),
			Jailed:          val.Jailed,
			Status:          val.Status.String(),
			Tokens:          val.Tokens.BigInt(),
			DelegatorShares: val.DelegatorShares.BigInt(),
			Description:     staking.IStakingModuleDescription(val.Description),
			UnbondingHeight: val.UnbondingHeight,
			UnbondingTime:   val.UnbondingTime.String(),
			Commission: staking.IStakingModuleCommission{
				CommissionRates: staking.IStakingModuleCommissionRates{
					Rate:          val.Commission.CommissionRates.Rate.BigInt(),
					MaxRate:       val.Commission.CommissionRates.MaxRate.BigInt(),
					MaxChangeRate: val.Commission.CommissionRates.MaxChangeRate.BigInt(),
				},
			},
			MinSelfDelegation:       val.MinSelfDelegation.BigInt(),
			UnbondingOnHoldRefCount: val.UnbondingOnHoldRefCount,
			UnbondingIds:            val.UnbondingIds,
		}
	}
	return valsOut, nil
}

// SdkProposalToGovProposal is a helper function to transform a `v1.Proposal` to an
// `IGovernanceModule.Proposal`.
func SdkProposalToGovProposal(proposal v1.Proposal) governance.IGovernanceModuleProposal {
	message := make([]byte, 0)
	for _, msg := range proposal.Messages {
		message = append(message, msg.Value...)
	}

	totalDeposit := make([]governance.CosmosCoin, 0)
	for _, coin := range proposal.TotalDeposit {
		totalDeposit = append(totalDeposit, governance.CosmosCoin{
			Denom:  coin.Denom,
			Amount: coin.Amount.BigInt(),
		})
	}

	return governance.IGovernanceModuleProposal{
		Id:      proposal.Id,
		Message: message,
		Status:  int32(proposal.Status), // Status is an alias for int32.
		FinalTallyResult: governance.IGovernanceModuleTallyResult{
			YesCount:        proposal.FinalTallyResult.YesCount,
			AbstainCount:    proposal.FinalTallyResult.AbstainCount,
			NoCount:         proposal.FinalTallyResult.NoCount,
			NoWithVetoCount: proposal.FinalTallyResult.NoWithVetoCount,
		},
		SubmitTime:      uint64(proposal.SubmitTime.Unix()),
		DepositEndTime:  uint64(proposal.DepositEndTime.Unix()),
		VotingStartTime: uint64(proposal.VotingStartTime.Unix()),
		VotingEndTime:   uint64(proposal.VotingEndTime.Unix()),
		TotalDeposit:    totalDeposit,
		Metadata:        proposal.Metadata,
		Title:           proposal.Title,
		Summary:         proposal.Summary,
		Proposer:        proposal.Proposer,
	}
}
