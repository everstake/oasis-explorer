DROP TABLE account_balance_merge_mv;
CREATE MATERIALIZED VIEW IF NOT EXISTS account_balance_merge_mv
ENGINE = AggregatingMergeTree() PARTITION BY toYYYYMM(created_at) ORDER BY (acb_account)
POPULATE AS
SELECT
    acb_account,
    min(blk_time) created_at,
    maxState(acb_nonce) acb_nonce,
    argMaxState(acb_general_balance,blk_lvl) acb_general_balance,
    argMaxState(acb_escrow_balance_active,blk_lvl) acb_escrow_balance_active,
    argMaxState(acb_escrow_balance_share,blk_lvl) acb_escrow_balance_share,
    argMaxState(acb_escrow_debonding_active,blk_lvl) acb_escrow_debonding_active,
    argMaxState(acb_delegations_balance,blk_lvl) acb_delegations_balance,
    argMaxState(acb_debonding_delegations_balance,blk_lvl) acb_debonding_delegations_balance,
    argMaxState(acb_self_delegation_balance,blk_lvl) acb_self_delegation_balance,
    argMaxState(acb_commission_schedule, blk_lvl) acb_commission_schedule
FROM account_balance
GROUP BY acb_account;


DROP TABLE account_last_balance_view;
CREATE VIEW IF NOT EXISTS account_last_balance_view AS
SELECT
    acb_account,
    min(created_at) created_at,
    maxMerge(acb_nonce) acb_nonce,
    argMaxMerge(acb_general_balance) acb_general_balance,
    argMaxMerge(acb_escrow_balance_active) acb_escrow_balance_active,
    argMaxMerge(acb_escrow_balance_share) acb_escrow_balance_share,
    argMaxMerge(acb_escrow_debonding_active) acb_escrow_debonding_active,
    argMaxMerge(acb_delegations_balance) acb_delegations_balance,
    argMaxMerge(acb_debonding_delegations_balance) acb_debonding_delegations_balance,
    argMaxMerge(acb_self_delegation_balance) acb_self_delegation_balance,
    argMaxMerge(acb_commission_schedule) acb_commission_schedule
FROM account_balance_merge_mv
GROUP BY acb_account;