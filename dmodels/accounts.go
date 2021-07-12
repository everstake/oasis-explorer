package dmodels

import "time"

const (
	AccountBalanceTable          = "account_balance"
	AccountListTable             = "account_list_view"
	DayTotalBalanceView          = "day_total_balance_view"
	AccountDayBalanceView        = "account_day_balance_view"
	DayAccountsCountView         = "day_accounts_view"
	TopEscrowBalanceAccountsView = "top_escrow_balance_accounts_view"
	AccountLastBalanceView       = "account_last_balance_view"
)

type AccountTime struct {
	CreatedAt  time.Time `db:"created_at"`
	LastActive time.Time `db:"last_active"`
}

type AccountBalance struct {
	Account               string    `db:"acb_account"`
	AccountName           string    `db:"pvl_name"`
	Time                  time.Time `db:"blk_time"`
	Height                int64     `db:"blk_lvl"`
	Nonce                 uint64    `db:"acb_nonce"`
	GeneralBalance        uint64    `db:"acb_general_balance"`
	EscrowBalanceActive   uint64    `db:"acb_escrow_balance_active"`
	EscrowBalanceShare    uint64    `db:"acb_escrow_balance_share"`
	EscrowDebondingActive uint64    `db:"acb_escrow_debonding_active"`
	EscrowDebondingShare  uint64    `db:"acb_escrow_debonding_share"`

	DelegationsBalance          uint64 `db:"acb_delegations_balance"`
	DebondingDelegationsBalance uint64 `db:"acb_debonding_delegations_balance"`

	SelfDelegationBalance uint64 `db:"acb_self_delegation_balance"`

	CommissionSchedule CommissionSchedule `db:"acb_commission_schedule"`
}

type AccountList struct {
	Account                     string    `db:"acb_account"`
	CreatedAt                   time.Time `db:"created_at"`
	OperationsAmount            uint64    `db:"operations_amount"`
	OperationsNumber            uint64    `db:"operations_number"`
	GeneralBalance              uint64    `db:"general_balance"`
	EscrowBalanceActive         uint64    `db:"escrow_balance"`
	EscrowBalanceShare          uint64    `db:"escrow_balance_share"`
	DelegationsBalance          uint64    `db:"delegations_balance"`
	DebondingDelegationsBalance uint64    `db:"debonding_delegations_balance"`

	Delegate            string `db:"delegate"`
	EntityRegisterBlock uint64 `db:"entity"`
	NodeRegisterBlock   uint64 `db:"node"`
	Type                string
}
