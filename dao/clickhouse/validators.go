package clickhouse

import (
	"fmt"
	"oasisTracker/dmodels"
	"oasisTracker/smodels"

	sq "github.com/wedancedalot/squirrel"
)

func (cl Clickhouse) GetValidatorsList(params smodels.ValidatorParams) (resp []dmodels.ValidatorView, err error) {

	q := sq.Select("reg_entity_address,reg_consensus_address,node_address,created_time,start_blk_lvl,blocks,signatures, signed_blocks, max_day_block, day_signatures, day_signed_blocks, day_blocks, acb_escrow_balance_active, acb_general_balance,acb_escrow_balance_share,acb_escrow_debonding_active, acb_delegations_balance , acb_debonding_delegations_balance, acb_self_delegation_balance, acb_commission_schedule, depositors_num, is_active, pvl_name, pvl_info").
		From(dmodels.ValidatorsTable).
		OrderBy("acb_escrow_balance_active desc").
		Limit(params.Limit).
		Offset(params.Offset)

	if params.ValidatorID != "" {
		q = q.Where(sq.Eq{"reg_entity_address": params.ValidatorID})
	}

	rawSql, args, err := q.ToSql()
	if err != nil {
		return resp, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	row := dmodels.ValidatorView{}
	for rows.Next() {

		err = rows.Scan(&row.EntityID, &row.ConsensusAddress, &row.NodeAddress, &row.ValidateSince, &row.StartBlockLevel, &row.ProposedBlocksCount, &row.SignaturesCount, &row.SignedBlocksCount, &row.LastBlockLevel, &row.DaySignaturesCount, &row.DaySignedBlocks, &row.DayBlocksCount, &row.EscrowBalance, &row.GeneralBalance, &row.EscrowBalanceShare, &row.DebondingBalance, &row.DelegationsBalance, &row.DebondingDelegationsBalance, &row.SelfDelegationBalance, &row.CommissionSchedule, &row.DepositorsNum, &row.IsActive, &row.Name, &row.Info)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) GetValidatorsCount(params smodels.ValidatorParams) (count uint64, err error) {
	q := sq.Select("count()").
		From("validator_entity_view")

	if params.ValidatorID != "" {
		q = q.Where(sq.Eq{"reg_entity_address": params.ValidatorID})
	}

	rawSql, args, err := q.ToSql()
	if err != nil {
		return count, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return count, err
	}
	defer rows.Close()

	for rows.Next() {

		err = rows.Scan(&count)
		if err != nil {
			return count, err
		}

	}

	return count, nil
}

func (cl Clickhouse) GetValidatorDayStats(consensusAddress string, params smodels.ChartParams) (resp []dmodels.ValidatorStats, err error) {

	q := sq.Select("day, day_signatures, blocks, blk_lvl, day_signed_blocks/blk_count uptime").
		From(dmodels.ValidatorStatsView).
		JoinClause("ANY LEFT JOIN day_max_block_lvl_view USING day").
		OrderBy("day asc").
		Where(sq.Eq{"reg_consensus_address": consensusAddress}).
		Where(sq.GtOrEq{"day": params.From}).
		Where(sq.LtOrEq{"day": params.To})

	rawSql, args, err := q.ToSql()
	if err != nil {
		return resp, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		row := dmodels.ValidatorStats{}

		err = rows.Scan(&row.BeginOfPeriod, &row.SignaturesCount, &row.BlocksCount, &row.LastBlock, &row.Uptime)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) GetValidatorDelegators(validatorID string, params smodels.CommonParams) (resp []dmodels.Delegator, err error) {

	q := sq.Select("tx_sender,escrow_since,balance").
		From(dmodels.DepositorsView).
		OrderBy("balance desc").
		Where(sq.Gt{"balance": 0}).
		Where(sq.Eq{"tx_receiver": validatorID}).
		Limit(params.Limit).
		Offset(params.Offset)

	rawSql, args, err := q.ToSql()
	if err != nil {
		return resp, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		row := dmodels.Delegator{}

		err = rows.Scan(&row.Address, &row.DelegateSince, &row.EscrowAmount)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) PublicValidatorsSearchList() (resp []dmodels.ValidatorView, err error) {
	q := sq.Select("reg_entity_address,pvl_name").
		From(dmodels.PublicValidatorsTable)

	rawSql, args, err := q.ToSql()
	if err != nil {
		return resp, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		row := dmodels.ValidatorView{}

		err = rows.Scan(&row.EntityID, &row.Name)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) PublicValidatorsList() (resp []dmodels.PublicValidator, err error) {
	q := sq.Select("reg_entity_id,reg_entity_address,pvl_name, pvl_info").
		From(dmodels.PublicValidatorsTable)

	rawSql, args, err := q.ToSql()
	if err != nil {
		return resp, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		row := dmodels.PublicValidator{}

		err = rows.Scan(&row.EntityID, &row.EntityAddress, &row.Name, &row.Info)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) UpdateValidators(list []dmodels.PublicValidator) (err error) {
	if len(list) == 0 {
		return nil
	}

	tx, err := cl.db.conn.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("TRUNCATE TABLE public_validators")
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		fmt.Sprintf("INSERT INTO %s (reg_entity_id, reg_entity_address, pvl_name, pvl_info) VALUES (?, ?, ?, ?)", dmodels.PublicValidatorsTable))
	if err != nil {
		return err
	}

	defer stmt.Close()

	for i := range list {

		_, err = stmt.Exec(
			list[i].EntityID,
			list[i].EntityAddress,
			list[i].Name,
			list[i].Info,
		)

		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
