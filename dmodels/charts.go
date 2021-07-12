package dmodels

import (
	"time"
)

type ChartData struct {
	BeginOfPeriod     time.Time `db:"start_of_period"`
	TransactionVolume string    `db:"transaction_volume"`
	EscrowRatio       float64   `db:"escrow_ratio"`
	OperationsCount   uint64    `db:"operations"`
	Fees              uint64    `db:"tx_fee"`
	AvgBlockTime      float64   `db:"avg_delay"`
	AccountNumber     uint64    `db:"accounts_number"`
	ReclaimAmount     uint64    `db:"reclaim_amount"`
}

type BalanceChartData struct {
	BeginOfPeriod               time.Time `db:"start_of_period"`
	AccountID                   string    `db:"acb_account"`
	GeneralBalance              uint64    `db:"acb_general_balance"`
	EscrowBalance               uint64    `db:"escrow_balance_active"`
	DebondingBalance            uint64    `db:"escrow_debonding_active"`
	DelegationsBalance          uint64    `db:"acb_delegations_balance"`
	DebondingDelegationsBalance uint64    `db:"acb_debonding_delegations_balance"`
	SelfStakeBalance            uint64    `json:"self_stake_balance"`
}
