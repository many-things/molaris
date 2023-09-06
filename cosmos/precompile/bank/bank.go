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

package bank

import (
	"context"
	"math/big"
	"time"

	"cosmossdk.io/core/address"
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/baseapp"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"pkg.berachain.dev/polaris/contracts/bindings/cosmos/lib"
	bankgenerated "pkg.berachain.dev/polaris/contracts/bindings/cosmos/precompile/bank"
	cosmlib "pkg.berachain.dev/polaris/cosmos/lib"
	"pkg.berachain.dev/polaris/cosmos/precompile"
	"pkg.berachain.dev/polaris/eth/common"
	ethprecompile "pkg.berachain.dev/polaris/eth/core/precompile"
	"pkg.berachain.dev/polaris/eth/core/vm"
)

// Contract is the precompile contract for the bank module.
type Contract struct {
	ethprecompile.BaseContract

	addressCodec address.Codec
	msgRouter    baseapp.MessageRouter
	querier      banktypes.QueryServer
	authzQuerier authztypes.QueryServer
}

// NewPrecompileContract returns a new instance of the bank precompile contract.
func NewPrecompileContract(
	ak cosmlib.CodecProvider, mr baseapp.MessageRouter, qs banktypes.QueryServer, authzQs authztypes.QueryServer,
) *Contract {
	return &Contract{
		BaseContract: ethprecompile.NewBaseContract(
			bankgenerated.BankModuleMetaData.ABI,
			common.BytesToAddress(authtypes.NewModuleAddress(banktypes.ModuleName)),
		),
		addressCodec: ak.AddressCodec(),
		msgRouter:    mr,
		querier:      qs,
		authzQuerier: authzQs,
	}
}

func (c *Contract) CustomValueDecoders() ethprecompile.ValueDecoders {
	return ethprecompile.ValueDecoders{
		banktypes.AttributeKeySender:    c.ConvertAccAddressFromString,
		banktypes.AttributeKeyRecipient: c.ConvertAccAddressFromString,
		banktypes.AttributeKeySpender:   c.ConvertAccAddressFromString,
		banktypes.AttributeKeyReceiver:  c.ConvertAccAddressFromString,
		banktypes.AttributeKeyMinter:    c.ConvertAccAddressFromString,
		banktypes.AttributeKeyBurner:    c.ConvertAccAddressFromString,
	}
}

// GetBalance implements `getBalance(address,string)` method.
func (c *Contract) GetBalance(
	ctx context.Context,
	accountAddress common.Address,
	denom string,
) (*big.Int, error) {
	accAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, accountAddress)
	if err != nil {
		return nil, err
	}

	res, err := c.querier.Balance(
		ctx, &banktypes.QueryBalanceRequest{
			Address: accAddr,
			Denom:   denom,
		},
	)
	if err != nil {
		return nil, err
	}

	balance := res.GetBalance().Amount
	return balance.BigInt(), nil
}

// GetAllBalances implements `getAllBalances(address)` method.
func (c *Contract) GetAllBalances(
	ctx context.Context,
	accountAddress common.Address,
) ([]lib.CosmosCoin, error) {
	accAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, accountAddress)
	if err != nil {
		return nil, err
	}

	res, err := c.querier.AllBalances(
		ctx, &banktypes.QueryAllBalancesRequest{
			Address: accAddr,
		},
	)
	if err != nil {
		return nil, err
	}

	return cosmlib.SdkCoinsToEvmCoins(res.Balances), nil
}

// GetSpendableBalanceByDenom implements `getSpendableBalanceByDenom(address,string)` method.
func (c *Contract) GetSpendableBalance(
	ctx context.Context,
	accountAddress common.Address,
	denom string,
) (*big.Int, error) {
	accAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, accountAddress)
	if err != nil {
		return nil, err
	}

	res, err := c.querier.SpendableBalanceByDenom(
		ctx, &banktypes.QuerySpendableBalanceByDenomRequest{
			Address: accAddr,
			Denom:   denom,
		},
	)
	if err != nil {
		return nil, err
	}

	balance := res.GetBalance().Amount
	return balance.BigInt(), nil
}

// GetSpendableBalances implements `getAllSpendableBalances(address)` method.
func (c *Contract) GetAllSpendableBalances(
	ctx context.Context,
	accountAddress common.Address,
) ([]lib.CosmosCoin, error) {
	accAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, accountAddress)
	if err != nil {
		return nil, err
	}

	res, err := c.querier.SpendableBalances(
		ctx, &banktypes.QuerySpendableBalancesRequest{
			Address: accAddr,
		},
	)
	if err != nil {
		return nil, err
	}

	return cosmlib.SdkCoinsToEvmCoins(res.Balances), nil
}

// GetSupplyOf implements `getSupply(string)` method.
func (c *Contract) GetSupply(
	ctx context.Context,
	denom string,
) (*big.Int, error) {
	res, err := c.querier.SupplyOf(
		ctx, &banktypes.QuerySupplyOfRequest{
			Denom: denom,
		},
	)
	if err != nil {
		return nil, err
	}

	supply := res.GetAmount().Amount
	return supply.BigInt(), nil
}

// GetTotalSupply implements `getAllSupply()` method.
func (c *Contract) GetAllSupply(
	ctx context.Context,
) ([]lib.CosmosCoin, error) {
	// todo: add pagination here
	res, err := c.querier.TotalSupply(ctx, &banktypes.QueryTotalSupplyRequest{})
	if err != nil {
		return nil, err
	}

	return cosmlib.SdkCoinsToEvmCoins(res.Supply), nil
}

// GetDenomMetadata implements `getDenomMetadata(string)` method.
func (c *Contract) GetDenomMetadata(
	ctx context.Context,
	denom string,
) (bankgenerated.IBankModuleDenomMetadata, error) {
	res, err := c.querier.DenomMetadata(
		ctx, &banktypes.QueryDenomMetadataRequest{
			Denom: denom,
		},
	)
	if err != nil {
		return bankgenerated.IBankModuleDenomMetadata{}, err
	}

	denomUnits := make([]bankgenerated.IBankModuleDenomUnit, len(res.Metadata.DenomUnits))
	for i, d := range res.Metadata.DenomUnits {
		denomUnits[i] = bankgenerated.IBankModuleDenomUnit{
			Denom:    d.Denom,
			Aliases:  d.Aliases,
			Exponent: d.Exponent,
		}
	}

	result := bankgenerated.IBankModuleDenomMetadata{
		Description: res.Metadata.Description,
		DenomUnits:  denomUnits,
		Base:        res.Metadata.Base,
		Display:     res.Metadata.Display,
		Name:        res.Metadata.Name,
		Symbol:      res.Metadata.Symbol,
	}
	return result, nil
}

// GetSendEnabled implements `getSendEnabled(string)` method.
func (c *Contract) GetSendEnabled(
	ctx context.Context,
	denom string,
) (bool, error) {
	res, err := c.querier.SendEnabled(
		ctx, &banktypes.QuerySendEnabledRequest{
			Denoms: []string{denom},
		},
	)
	if err != nil {
		return false, err
	}
	if len(res.SendEnabled) == 0 {
		return false, precompile.ErrInvalidString
	}

	return res.SendEnabled[0].Enabled, nil
}

// Send implements `send(address,address,(uint256,string)[])` method.
func (c *Contract) Send(
	ctx context.Context,
	fromAddress common.Address,
	toAddress common.Address,
	coins any,
) (bool, error) {
	amount, err := cosmlib.ExtractCoinsFromInput(coins)
	if err != nil {
		return false, err
	}
	caller, err := cosmlib.StringFromEthAddress(
		c.addressCodec, vm.UnwrapPolarContext(ctx).MsgSender(),
	)
	if err != nil {
		return false, err
	}
	fromAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, fromAddress)
	if err != nil {
		return false, err
	}
	toAddr, err := cosmlib.StringFromEthAddress(c.addressCodec, toAddress)
	if err != nil {
		return false, err
	}

	var msg sdk.Msg = &banktypes.MsgSend{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      amount,
	}
	if caller != fromAddr {
		inner, err := cdctypes.NewAnyWithValue(msg)
		if err != nil {
			return false, err
		}

		msg = &authztypes.MsgExec{
			Grantee: caller,
			Msgs:    []*cdctypes.Any{inner},
		}
	}

	handler := c.msgRouter.Handler(msg)
	if handler == nil {
		return false, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized message route: %s", sdk.MsgTypeURL(msg))
	}

	if _, err := handler(sdk.UnwrapSDKContext(ctx), msg); err != nil {
		return false, errorsmod.Wrapf(err, "failed to execute message; message %v", msg)
	}

	return err == nil, err
}

// Allowance implements `allowance(address,string)` method.
func (c *Contract) Allowance(ctx context.Context, ownerAddress common.Address, spenderAddress common.Address, denom string) (*big.Int, error) {
	owner, err := cosmlib.StringFromEthAddress(c.addressCodec, ownerAddress)
	if err != nil {
		return nil, err
	}
	spender, err := cosmlib.StringFromEthAddress(c.addressCodec, spenderAddress)
	if err != nil {
		return nil, err
	}

	res, err := c.authzQuerier.Grants(
		ctx, &authztypes.QueryGrantsRequest{
			Granter:    owner,
			Grantee:    spender,
			MsgTypeUrl: banktypes.SendAuthorization{}.MsgTypeURL(),
			Pagination: nil,
		},
	)
	if err != nil {
		return nil, err
	}

	// Map the grants to send authorizations, should have the same type since we filtered by msg
	// type url.
	blocktime := time.Unix(int64(vm.UnwrapPolarContext(ctx).Block().Time), 0)
	sendAuths, err := cosmlib.GetGrantAsSendAuth(res.Grants, blocktime)
	if err != nil {
		return nil, err
	}

	// Get the highest allowance from the send authorizations.
	allowance := getHighestAllowance(sendAuths, denom)

	return allowance, nil
}

// getHighestAllowance returns the highest allowance for a given coin denom.
func getHighestAllowance(sendAuths []*banktypes.SendAuthorization, coinDenom string) *big.Int {
	// Init the max to 0.
	var max = big.NewInt(0)
	// Loop through the send authorizations and find the highest allowance.
	for _, sendAuth := range sendAuths {
		// Get the spendable limit for the coin denom that was specified.
		amount := sendAuth.SpendLimit.AmountOf(coinDenom)
		// If not set, the current is the max, if set, compare the current with the max.
		if max == nil || amount.BigInt().Cmp(max) > 0 {
			max = amount.BigInt()
		}
	}
	return max
}

// Approve implements `approve(address,(uint256,string)[])` method.
func (c *Contract) Approve(ctx context.Context, spenderAddress common.Address, coins any) (bool, error) {
	amount, err := cosmlib.ExtractCoinsFromInput(coins)
	if err != nil {
		return false, err
	}
	caller, err := cosmlib.StringFromEthAddress(
		c.addressCodec, vm.UnwrapPolarContext(ctx).MsgSender(),
	)
	if err != nil {
		return false, err
	}
	spender, err := cosmlib.StringFromEthAddress(c.addressCodec, spenderAddress)
	if err != nil {
		return false, err
	}

	msg := &authztypes.MsgGrant{
		Granter: caller,
		Grantee: spender,
		Grant:   authztypes.Grant{Expiration: nil},
	}

	if err = msg.SetAuthorization(
		&banktypes.SendAuthorization{
			SpendLimit: amount,
			AllowList:  []string{spender},
		},
	); err != nil {
		return false, err
	}

	handler := c.msgRouter.Handler(msg)
	if handler == nil {
		return false, sdkerrors.ErrUnknownRequest.Wrapf("unrecognized message route: %s", sdk.MsgTypeURL(msg))
	}

	if _, err = handler(sdk.UnwrapSDKContext(ctx), msg); err != nil {
		return false, errorsmod.Wrapf(err, "failed to execute message; message %v", msg)
	}

	return err == nil, err
}

// ConvertAccAddressFromString converts a Cosmos string representing a account address to a
// common.Address.
func (c *Contract) ConvertAccAddressFromString(attributeValue string) (any, error) {
	// extract the sdk.AccAddress from string value as common.Address
	return cosmlib.EthAddressFromString(c.addressCodec, attributeValue)
}
