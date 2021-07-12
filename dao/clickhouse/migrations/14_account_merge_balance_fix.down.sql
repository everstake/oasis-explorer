DROP TABLE account_balance_merge_mv;
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
    anyLastState(acb_debonding_delegations_balance) acb_debonding_delegations_balance
FROM account_balance
GROUP BY acb_account;


DROP TABLE account_last_balance_view;
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
    anyLastMerge(acb_debonding_delegations_balance) acb_debonding_delegations_balance
FROM account_balance_merge_mv
GROUP BY acb_account;



