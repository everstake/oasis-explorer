package dmodels

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/oasisprotocol/oasis-core/go/staking/api"
)

type DayBalance struct {
	StartOfPeriod         time.Time `db:"start_of_period"`
	GeneralBalance        uint64    `db:"general_balance"`
	EscrowBalanceActive   uint64    `db:"escrow_balance_active"`
	EscrowDebondingActive uint64    `db:"escrow_debonding_active"`
}

//Wrapper to work with db
type CommissionSchedule struct {
	api.CommissionSchedule
}

func (c CommissionSchedule) Value() (driver.Value, error) {

	bt, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	return clickhouse.Array(bt), nil
}

func (c *CommissionSchedule) Scan(value interface{}) (err error) {
	if value == nil {
		return nil
	}

	data, ok := value.([]uint8)
	if !ok {
		return fmt.Errorf("invalid type")
	}

	if len(data) == 0 {
		return nil
	}

	err = json.Unmarshal(data, c)
	if err != nil {
		return fmt.Errorf("json.Unmarshal: %s", err.Error())
	}

	return nil
}
