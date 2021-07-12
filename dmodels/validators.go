package dmodels

import (
	"time"
)

const (
	ValidatorsTable       = "validators_list_view"
	PublicValidatorsTable = "public_validators"
	ValidatorStatsView    = "validator_day_stats_view"
	DepositorsView        = "entity_depositors_view"
)

type PublicValidator struct {
	EntityID      string `db:"reg_entity_id"`
	EntityAddress string `db:"reg_entity_address"`
	Name          string `db:"pvl_name"`
	Info          string `db:"pvl_info"`
}

type ValidatorView struct {
	PublicValidator

	ConsensusAddress  string    `db:"reg_consensus_address"`
	NodeAddress       string    `db:"node_address"`
	ValidateSince     time.Time `db:"created_time"`
	StartBlockLevel   uint64    `db:"start_blk_lvl"`
	LastBlockTime     time.Time `db:"last_block_time"`
	LastSignatureTime uint64    `db:"last_signature_time"`

	ProposedBlocksCount uint64 `db:"blocks"`

	SignaturesCount uint64 `db:"signatures"`
	//Total signed blocks
	SignedBlocksCount uint64 `db:"signed_blocks"`
	LastBlockLevel    uint64 `db:"max_day_block"`
	//Day
	DaySignaturesCount uint64 `db:"day_signatures"`
	DaySignedBlocks    uint64 `db:"day_signed_blocks"`
	DayBlocksCount     uint64 `db:"day_blocks"`

	GeneralBalance     uint64 `db:"acb_general_balance"`
	EscrowBalance      uint64 `db:"acb_escrow_balance_active"`
	EscrowBalanceShare uint64 `db:"acb_escrow_balance_share"`
	DebondingBalance   uint64 `db:"acb_escrow_debonding_active"`

	DelegationsBalance          uint64 `db:"acb_delegations_balance"`
	DebondingDelegationsBalance uint64 `db:"acb_debonding_delegations_balance"`
	SelfDelegationBalance       uint64 `db:"acb_self_delegation_balance"`

	CommissionSchedule CommissionSchedule `db:"acb_commission_schedule"`

	DepositorsNum uint64  `db:"depositors_num"`
	IsActive      bool    `db:"is_active"`
	DayUptime     float64 `db:"-"`
	TotalUptime   float64 `db:"-"`
	Status        string  `db:"-"`
}

type ValidatorStats struct {
	BeginOfPeriod     time.Time
	LastBlock         uint64
	AvailabilityScore uint64
	Uptime            float64
	BlocksCount       uint64
	SignaturesCount   uint64
}

type Delegator struct {
	Address       string
	EscrowAmount  uint64
	DelegateSince time.Time
}
