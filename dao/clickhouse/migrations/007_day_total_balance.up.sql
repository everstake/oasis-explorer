CREATE VIEW IF NOT EXISTS account_day_balance_view AS
select acb_account,
       toStartOfDay(blk_time) start_of_period,
       argMax(acb_general_balance, blk_lvl) acb_general_balance,
       argMax(acb_escrow_balance_active, blk_lvl)   escrow_balance_active,
       argMax(acb_escrow_debonding_active, blk_lvl) escrow_debonding_active,
       argMax(acb_delegations_balance, blk_lvl) acb_delegations_balance,
       argMax(acb_debonding_delegations_balance, blk_lvl) acb_debonding_delegations_balance,
       argMax(acb_self_delegation_balance, blk_lvl) acb_self_delegation_balance
from account_balance
group by acb_account, start_of_period;

CREATE VIEW IF NOT EXISTS day_total_balance_view AS
select start_of_period,
       sum(acb_general_balance)         general_balance,
       sum(escrow_balance_active)   escrow_balance_active,
       sum(escrow_debonding_active) escrow_debonding_active
from account_day_balance_view
GROUP BY start_of_period
ORDER BY start_of_period asc;

-- TODO check this view
CREATE VIEW IF NOT EXISTS day_total_balance_new_view AS
select day start_of_period, sum(acb_general_balance) general_balance, sum(escrow_balance_active) escrow_balance_active, sum(escrow_debonding_active) escrow_debonding_active
from (
       select arrayJoin(timeSlots(toStartOfDay(now()) - INTERVAL 1 MONTH,
              toUInt32(dateDiff('second', toStartOfDay(now()) - INTERVAL 1 MONTH, toStartOfDay(now()))),
                                  86400)) day,
              acb_account,
              max(start_of_period)        max_start_of_period
       from account_day_balance_view
       where start_of_period < day + INTERVAL 1 day
       group by day, acb_account) gr ANY
       left join (select acb_account,
                         start_of_period max_start_of_period,
                         acb_general_balance,
                         escrow_balance_active,
                         escrow_debonding_active
                  from account_day_balance_view) als USING acb_account, max_start_of_period
group by day
ORDER BY day asc;

CREATE VIEW IF NOT EXISTS day_accounts_view AS
select arrayJoin(timeSlots(toStartOfDay(now()) - INTERVAL 1 MONTH, toUInt32(
           dateDiff('second', toStartOfDay(now()) - INTERVAL 1 MONTH, toStartOfDay(now()))),
                                  86400)) start_of_period,
       count(distinct (acb_account)) accounts_number
from account_last_balance_view
where created_at < start_of_period + INTERVAL 1 day
group by start_of_period;