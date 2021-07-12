CREATE TABLE IF NOT EXISTS account_balance (
    blk_lvl UInt64,
    blk_time DateTime,
    acb_account FixedString(46),
    acb_nonce UInt64,
    acb_general_balance UInt64,
    acb_escrow_balance_active UInt64,
    acb_escrow_balance_share UInt64,
    acb_escrow_debonding_active UInt64,
    acb_escrow_debonding_share UInt64,
    acb_delegations_balance UInt64,
    acb_debonding_delegations_balance UInt64,
    acb_self_delegation_balance UInt64,
    acb_commission_schedule Array(UInt8)
) ENGINE ReplacingMergeTree()
PARTITION BY toYYYYMMDD(blk_time)
ORDER BY (acb_account,blk_lvl);

CREATE MATERIALIZED VIEW IF NOT EXISTS account_balance_merge_mv
ENGINE = AggregatingMergeTree() PARTITION BY toYYYYMM(created_at) ORDER BY (acb_account)
POPULATE AS
SELECT
    acb_account,
    min(blk_time) created_at,
    max(acb_nonce) acb_nonce,
    anyLastState(acb_general_balance) acb_general_balance,
    anyLastState(acb_escrow_balance_active) acb_escrow_balance_active,
    anyLastState(acb_escrow_balance_share) acb_escrow_balance_share,
    anyLastState(acb_escrow_debonding_active) acb_escrow_debonding_active,
    anyLastState(acb_delegations_balance) acb_delegations_balance,
    anyLastState(acb_debonding_delegations_balance) acb_debonding_delegations_balance,
    anyLastState(acb_self_delegation_balance) acb_self_delegation_balance,
    anyLastState(acb_commission_schedule) acb_commission_schedule
FROM account_balance
GROUP BY acb_account;

CREATE VIEW IF NOT EXISTS account_last_balance_view AS
SELECT
    acb_account,
    min(created_at) created_at,
    max(acb_nonce) acb_nonce,
    anyLastMerge(acb_general_balance) acb_general_balance,
    anyLastMerge(acb_escrow_balance_active) acb_escrow_balance_active,
    anyLastMerge(acb_escrow_balance_share) acb_escrow_balance_share,
    anyLastMerge(acb_escrow_debonding_active) acb_escrow_debonding_active,
    anyLastMerge(acb_delegations_balance) acb_delegations_balance,
    anyLastMerge(acb_debonding_delegations_balance) acb_debonding_delegations_balance,
    anyLastMerge(acb_self_delegation_balance) acb_self_delegation_balance,
    anyLastMerge(acb_commission_schedule) acb_commission_schedule
FROM account_balance_merge_mv
GROUP BY acb_account;