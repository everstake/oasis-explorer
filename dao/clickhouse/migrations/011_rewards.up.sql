CREATE TABLE IF NOT EXISTS rewards (
    blk_lvl UInt64,
    blk_epoch UInt64,
    created_at DateTime,
    rwd_amount UInt64,
    reg_entity_address  FixedString(46)
) ENGINE ReplacingMergeTree()
PARTITION BY toYYYYMMDD(created_at)
ORDER BY (reg_entity_address, blk_lvl);

CREATE VIEW IF NOT EXISTS validator_rewards_stat_view AS
select *
from (
       select *
       from (select reg_entity_address, sum(rwd_amount) total_amount
             from rewards
             group by reg_entity_address) total
              ANY
              LEFT JOIN (select reg_entity_address, sum(rwd_amount) day_amount
                         from rewards
                         where created_at >= toStartOfDay(now())
                         group by reg_entity_address) dayreward USING reg_entity_address
       ) daystat
       ANY
       LEFT JOIN (
  select *
  from (
         select reg_entity_address, sum(rwd_amount) week_amount
         from rewards
         where created_at >= toStartOfWeek(now())
         group by reg_entity_address) week
         ANY
         LEFT JOIN (select reg_entity_address, sum(rwd_amount) month_amount
                    from rewards
                    where created_at >= toStartOfMonth(now())
                    group by reg_entity_address) weekreward USING reg_entity_address
  ) weekstat USING reg_entity_address;