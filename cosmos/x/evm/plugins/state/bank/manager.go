package bank

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math/big"
	evmtypes "pkg.berachain.dev/polaris/cosmos/x/evm/types"
	"pkg.berachain.dev/polaris/eth/common"
	"pkg.berachain.dev/polaris/lib/ds"
	"pkg.berachain.dev/polaris/lib/ds/stack"
)

const (
	initCapacity    = 16
	registryKey     = "bank"
	underlyingDenom = "umito"
)

type balanceChange struct {
	Addr  common.Address
	Delta *big.Int
}

type state struct {
	balanceChanges []balanceChange
	dirtyBalances  map[common.Address]*big.Int
}

type Manager struct {
	bankKeeper BankKeeper
	states     ds.Stack[*state]
	readOnly   bool
}

func NewManager(bankKeeper BankKeeper) *Manager {
	return &Manager{
		bankKeeper: bankKeeper,
		states:     stack.New[*state](initCapacity),
	}
}

func (m *Manager) getCurState() *state {
	if m.states.Size() == 0 {
		m.states.Push(&state{
			balanceChanges: []balanceChange{},
			dirtyBalances:  map[common.Address]*big.Int{},
		})
	}
	return m.states.Peek()
}

func (m *Manager) GetBalance(ctx sdk.Context, addr common.Address) *big.Int {
	curState := m.getCurState()
	balance := curState.dirtyBalances[addr]
	if balance != nil {
		return balance
	} else {
		return m.bankKeeper.GetBalance(ctx, addr.Bytes(), underlyingDenom).Amount.BigInt()
	}
}

func (m *Manager) SetBalance(ctx sdk.Context, addr common.Address, newBalance *big.Int) {
	oldBalance := m.GetBalance(ctx, addr)
	delta := new(big.Int).Sub(newBalance, oldBalance)
	if delta.Sign() == 0 {
		return
	}

	curState := m.getCurState()
	curState.balanceChanges = append(curState.balanceChanges, balanceChange{
		Addr:  addr,
		Delta: delta,
	})
	curState.dirtyBalances[addr] = newBalance
}

// RegistryKey implements `types.Registrable`.
func (m *Manager) RegistryKey() string {
	return registryKey
}

// Snapshot implements `types.Snapshottable`.
func (m *Manager) Snapshot() int {
	curState := m.getCurState()
	newState := state{
		balanceChanges: []balanceChange{},
		dirtyBalances:  map[common.Address]*big.Int{},
	}
	for addr, balance := range curState.dirtyBalances {
		newState.dirtyBalances[addr] = balance
	}

	return m.states.Push(&newState) - 1
}

// RevertToSnapshot implements `types.Snapshottable`.
func (m *Manager) RevertToSnapshot(id int) {
	m.states.PopToSize(id)
}

// Finalize implements `types.Finalizeable`.
func (m *Manager) Finalize() {}

// Commit commits pending changes to bank module.
func (m *Manager) Commit(ctx sdk.Context) error {
	// TODO(thai): must consider about error happening in the middle of this function.

	totalDirtyBalances := m.getCurState().dirtyBalances
	for addr := range totalDirtyBalances {
		bankBalance := m.bankKeeper.GetBalance(ctx, addr.Bytes(), underlyingDenom)
		ctx.Logger().Info(fmt.Sprintf("[evm->bank] BEFORE: %s: %s", addr.String(), bankBalance.String()))
	}

	count := 0
	for i := 0; i < m.states.Size(); i++ {
		s := m.states.PeekAt(i)

		for j, change := range s.balanceChanges {
			switch change.Delta.Sign() {
			case 1:
				amount := sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewIntFromBigInt(change.Delta)))
				if err := m.bankKeeper.MintCoins(ctx, evmtypes.ModuleName, amount); err != nil {
					return err
				}
				if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, change.Addr.Bytes(), amount); err != nil {
					return err
				}
				break

			case -1:
				amount := sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewIntFromBigInt(new(big.Int).Neg(change.Delta))))
				if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, change.Addr.Bytes(), evmtypes.ModuleName, amount); err != nil {
					return err
				}
				if err := m.bankKeeper.BurnCoins(ctx, evmtypes.ModuleName, amount); err != nil {
					return err
				}
				break

			default:
			}

			count++
			ctx.Logger().Info(fmt.Sprintf("[evm->bank] CHANGE(#%d)(%d,%d): %s: %s", count, i, j, change.Addr.String(), change.Delta.String()))
		}
	}

	for addr := range totalDirtyBalances {
		bankBalance := m.bankKeeper.GetBalance(ctx, addr.Bytes(), underlyingDenom)
		ctx.Logger().Info(fmt.Sprintf("[evm->bank] AFTER: %s: %s", addr.String(), bankBalance.String()))
	}

	return nil
}
