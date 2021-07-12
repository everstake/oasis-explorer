--Last registered unique entity
CREATE VIEW IF NOT EXISTS entity_register_view AS
select *
from (
      select reg_entity_id, max(blk_lvl) blk_lvl
      from register_entity_transactions
      group by reg_entity_id) m
 ANY
       LEFT JOIN register_entity_transactions USING reg_entity_id, blk_lvl;

--Last registered unique node
CREATE VIEW IF NOT EXISTS node_registry_view AS
select *
from (
      select reg_id, max(blk_lvl) blk_lvl
      from register_node_transactions
      group by reg_id) m
 ANY
       LEFT JOIN register_node_transactions USING reg_id, blk_lvl;

CREATE MATERIALIZED VIEW IF NOT EXISTS account_operations_amount_mv
  ENGINE = AggregatingMergeTree()
    PARTITION BY month
    ORDER BY (acb_account) POPULATE AS
select tx_sender                                                              acb_account,
       toYYYYMM(tx_time)                                                      month,
       sum(tx_amount) + sum(tx_escrow_amount) + sum(tx_escrow_reclaim_amount) operations_amount
from transactions
group by acb_account, month;

CREATE VIEW IF NOT EXISTS account_operations_amount_view AS
SELECT acb_account,
       sum(operations_amount) operations_amount
FROM account_operations_amount_mv
GROUP BY acb_account;

CREATE VIEW IF NOT EXISTS account_list_view AS
select acb_account, created_at, operations_amount, acb_nonce operations_number, acb_general_balance general_balance, acb_escrow_balance_active escrow_balance, acb_escrow_balance_share escrow_balance_share, acb_delegations_balance  delegations_balance, acb_debonding_delegations_balance debonding_delegations_balance, acb_self_delegation_balance self_delegation_balance, tx_receiver delegate, entity.blk_lvl entity, prp.blk_lvl node from (
select *
from (select *
      --All accounts list with
      from ( select * from account_last_balance_view ANY LEFT JOIN account_operations_amount_view USING acb_account) with_operation_amount
             ANY --active delegator
             LEFT JOIN ( SELECT tx_sender acb_account, tx_receiver from entity_depositors_view where balance > 0) al USING acb_account
       ) s
       ANY --node reg info
       LEFT JOIN (select reg_address acb_account, CASE WHEN max(blk_lvl) = 0 THEN 1 ELSE max(blk_lvl) END as blk_lvl from register_node_transactions group by acb_account) node USING acb_account) prp
ANY --entity reg info
LEFT JOIN ( select reg_entity_address acb_account, CASE WHEN max(blk_lvl) = 0 THEN 1 ELSE max(blk_lvl) END as blk_lvl from register_entity_transactions group by acb_account) entity USING acb_account;