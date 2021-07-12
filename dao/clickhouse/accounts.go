package clickhouse

import (
	"fmt"
	"oasisTracker/dmodels"
	"oasisTracker/smodels"

	sq "github.com/wedancedalot/squirrel"
)

func (cl Clickhouse) GetAccountTiming(accountID string) (resp dmodels.AccountTime, err error) {

	q := sq.Select("min(tx_time) created_at, max(tx_time) last_active").
		From(dmodels.TransactionsTable).
		Where("tx_receiver = ? OR tx_sender = ?", accountID, accountID)

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
		err := rows.Scan(&resp.CreatedAt, &resp.LastActive)
		if err != nil {
			return resp, err
		}
	}

	return resp, nil
}

func (cl Clickhouse) CreateAccountBalances(balances []dmodels.AccountBalance) (err error) {
	if len(balances) == 0 {
		return nil
	}

	tx, err := cl.db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		fmt.Sprintf("INSERT INTO %s (blk_lvl, blk_time, acb_account, acb_nonce, acb_general_balance, acb_escrow_balance_active, acb_escrow_balance_share, acb_escrow_debonding_active, acb_escrow_debonding_share, acb_delegations_balance, acb_debonding_delegations_balance, acb_self_delegation_balance , acb_commission_schedule)"+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", dmodels.AccountBalanceTable))
	if err != nil {
		return err
	}

	defer stmt.Close()

	for i := range balances {

		if balances[i].Time.IsZero() {
			return fmt.Errorf("timestamp can not be 0")
		}

		_, err = stmt.Exec(
			balances[i].Height,
			balances[i].Time,
			balances[i].Account,
			balances[i].Nonce,
			balances[i].GeneralBalance,
			balances[i].EscrowBalanceActive,
			balances[i].EscrowBalanceShare,
			balances[i].EscrowDebondingActive,
			balances[i].EscrowDebondingShare,
			balances[i].DelegationsBalance,
			balances[i].DebondingDelegationsBalance,
			balances[i].SelfDelegationBalance,
			balances[i].CommissionSchedule,
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

func (cl Clickhouse) GetTopEscrowAccounts(limit uint64) (resp []dmodels.AccountBalance, err error) {

	q := sq.Select("*").
		From(dmodels.AccountLastBalanceView).
		JoinClause("ANY LEFT JOIN (SELECT reg_entity_address acb_account, pvl_name from public_validators) s USING acb_account").
		OrderBy("acb_escrow_balance_active desc").
		Limit(limit)

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
		var row dmodels.AccountBalance

		err := rows.Scan(&row.Account, &row.Time, &row.Nonce, &row.GeneralBalance, &row.EscrowBalanceActive, &row.EscrowBalanceShare, &row.EscrowDebondingActive, &row.DelegationsBalance, &row.DebondingDelegationsBalance, &row.CommissionSchedule, &row.AccountName)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (cl Clickhouse) AccountsCount() (count uint64, err error) {
	q := sq.Select("count()").
		From(dmodels.AccountLastBalanceView)

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
		err := rows.Scan(&count)
		if err != nil {
			return count, err
		}
	}

	return count, nil
}

func (cl Clickhouse) GetAccountList(listParams smodels.AccountListParams) (resp []dmodels.AccountList, err error) {

	q := sq.Select("*").
		From(dmodels.AccountListTable).
		OrderBy(fmt.Sprintf("%s %s", listParams.SortColumn, listParams.SortSide)).
		Limit(listParams.Limit).
		Offset(listParams.Offset)

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
		row := dmodels.AccountList{}

		err := rows.Scan(&row.Account, &row.CreatedAt, &row.OperationsAmount, &row.OperationsNumber, &row.GeneralBalance, &row.EscrowBalanceActive, &row.EscrowBalanceShare, &row.DelegationsBalance, &row.DebondingDelegationsBalance, &row.Delegate, &row.EntityRegisterBlock, &row.NodeRegisterBlock)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}
