package clickhouse

import (
	"oasisTracker/dmodels"
	"oasisTracker/smodels"

	sq "github.com/wedancedalot/squirrel"
)

func (cl Clickhouse) GetLastDayTotalBalance() (bal dmodels.DayBalance, err error) {
	q := sq.Select("*").
		From(dmodels.DayTotalBalanceView).
		Limit(1)

	rawSql, args, err := q.ToSql()
	if err != nil {
		return bal, err
	}

	rows, err := cl.db.conn.Query(rawSql, args...)
	if err != nil {
		return bal, err
	}

	defer rows.Close()

	for rows.Next() {

		err = rows.Scan(&bal.StartOfPeriod, &bal.GeneralBalance, &bal.EscrowBalanceActive, &bal.EscrowDebondingActive)
		if err != nil {
			return bal, err
		}

	}

	return bal, nil
}

func (cl Clickhouse) GetBalanceChartData(accountID string, params smodels.ChartParams) (resp []dmodels.BalanceChartData, err error) {

	q := sq.Select("*").
		From(dmodels.AccountDayBalanceView).
		Where(sq.Eq{"acb_account": accountID}).
		Where(sq.GtOrEq{"start_of_period": params.From}).
		Where(sq.LtOrEq{"start_of_period": params.To}).
		OrderBy("start_of_period asc")

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
		row := dmodels.BalanceChartData{}

		err := rows.Scan(&row.AccountID, &row.BeginOfPeriod, &row.GeneralBalance, &row.EscrowBalance, &row.DebondingBalance, &row.DelegationsBalance, &row.DebondingDelegationsBalance, &row.SelfStakeBalance)
		if err != nil {
			return resp, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}
