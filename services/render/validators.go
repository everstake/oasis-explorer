package render

import (
	"encoding/json"
	"oasisTracker/dmodels"
	"oasisTracker/smodels"
)

func ValidatorsList(accs []dmodels.ValidatorView) []smodels.Validator {
	accounts := make([]smodels.Validator, len(accs))
	for i := range accs {
		accounts[i] = Validator(accs[i])
	}
	return accounts
}

func Validator(a dmodels.ValidatorView) smodels.Validator {

	return smodels.Validator{
		Account:                     a.EntityID,
		AccountName:                 a.Name,
		NodeID:                      a.NodeAddress,
		GeneralBalance:              a.GeneralBalance,
		EscrowBalance:               a.EscrowBalance,
		EscrowBalanceShare:          a.EscrowBalanceShare,
		DebondingBalance:            a.DebondingBalance,
		DelegationsBalance:          a.DelegationsBalance,
		DebondingDelegationsBalance: a.DebondingDelegationsBalance,
		DayUptime:                   a.DayUptime,
		TotalUptime:                 a.TotalUptime,
		CreatedAt:                   a.ValidateSince.Unix(),
		MediaInfo:                   ValidatorMediaInfo(a.Info),
		ValidatorInfo: smodels.ValidatorInfo{
			Status:          a.Status,
			DepositorsCount: a.DepositorsNum,
			BlocksCount:     a.ProposedBlocksCount,
			SignaturesCount: a.SignaturesCount,
		},
		CommissionSchedule: a.CommissionSchedule.CommissionSchedule,
	}
}

func ValidatorMediaInfo(validatorMediaInfoString string) *smodels.ValidatorMediaInfo {
	if validatorMediaInfoString == "" {
		return nil
	}

	var mediaInfo smodels.ValidatorMediaInfo
	if validatorMediaInfoString != "" {
		json.Unmarshal([]byte(validatorMediaInfoString), &mediaInfo)
	}

	return &mediaInfo
}

func ValidatorStatList(sts []dmodels.ValidatorStats) []smodels.ValidatorStats {
	stats := make([]smodels.ValidatorStats, len(sts))
	for i := range sts {
		stats[i] = ValidatorStat(sts[i])
	}
	return stats
}

func ValidatorStat(s dmodels.ValidatorStats) smodels.ValidatorStats {

	return smodels.ValidatorStats{
		Timestamp:         s.BeginOfPeriod.Unix(),
		AvailabilityScore: s.AvailabilityScore,
		Uptime:            s.Uptime,
		BlocksCount:       s.BlocksCount,
		SignaturesCount:   s.SignaturesCount,
	}
}

func DelegatorList(sts []dmodels.Delegator) []smodels.Delegator {
	stats := make([]smodels.Delegator, len(sts))
	for i := range sts {
		stats[i] = Delegator(sts[i])
	}
	return stats
}

func Delegator(s dmodels.Delegator) smodels.Delegator {

	return smodels.Delegator{
		Account:       s.Address,
		EscrowAmount:  s.EscrowAmount,
		DelegateSince: s.DelegateSince.Unix(),
	}
}

func PublicValidatorSearch(sts []dmodels.ValidatorView) []smodels.ValidatorEntity {
	stats := make([]smodels.ValidatorEntity, len(sts))
	for i := range sts {
		stats[i] = ValidatorEntity(sts[i])
	}
	return stats
}

func ValidatorEntity(s dmodels.ValidatorView) smodels.ValidatorEntity {

	return smodels.ValidatorEntity{
		Account:     s.EntityID,
		AccountName: s.Name,
	}
}
