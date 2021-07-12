package metaregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"oasisTracker/dmodels"
	"oasisTracker/smodels"

	registry "github.com/oasisprotocol/metadata-registry-tools"
	epochAPI "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
)

type Repo interface {
	PublicBakerRepo
	BlocksRepo
}

type PublicBakerRepo interface {
	PublicValidatorsList() (resp []dmodels.PublicValidator, err error)
	UpdateValidators([]dmodels.PublicValidator) (err error)
}

type BlocksRepo interface {
	GetLastBlock() (block dmodels.Block, err error)
}

type AccountProvider interface {
	Account(ctx context.Context, query *stakingAPI.OwnerQuery) (*stakingAPI.Account, error)
}

func UpdatePublicValidators(unit Repo, provider AccountProvider) error {
	publicValidator, err := unit.PublicValidatorsList()
	if err != nil {
		return err
	}
	validatorsMap := map[string]dmodels.PublicValidator{}

	for i := range publicValidator {
		validatorsMap[publicValidator[i].EntityID] = publicValidator[i]
	}

	// Create a new instance of the registry client, pointing to the production instance.
	// Note that in order to refresh the data you currently need to create a new provider.
	gp, err := registry.NewGitProvider(registry.NewGitConfig())
	if err != nil {
		return err
	}

	// Get a list of all entities in the registy.
	entities, err := gp.GetEntities(context.Background())
	if err != nil {
		return err
	}

	updatedValidators := make([]dmodels.PublicValidator, 0, len(publicValidator))
	for pubKey, metadata := range entities {

		//Update validator info
		updatedValidator, err := validatorUpdate(pubKey, validatorsMap, metadata)
		if err != nil {
			return err
		}

		key, err := pubKey.MarshalText()
		if err != nil {
			return err
		}

		validatorsMap[string(key)] = updatedValidator
	}

	for key := range validatorsMap {
		updatedValidators = append(updatedValidators, validatorsMap[key])
	}

	err = unit.UpdateValidators(updatedValidators)
	if err != nil {
		return err
	}

	return nil
}

func buildTwitterUrl(acc string) string {
	if acc == "" {
		return acc
	}

	return fmt.Sprint("https://twitter.com/", acc)
}

func validatorUpdate(pubKey signature.PublicKey, validatorsMap map[string]dmodels.PublicValidator, metadata *registry.EntityMetadata) (validator dmodels.PublicValidator, err error) {

	validator, err = getAccount(pubKey, validatorsMap)
	if err != nil {
		return validator, err
	}

	validator.Name = metadata.Name

	var metaInfo smodels.ValidatorMediaInfo
	if validator.Info != "" {
		err = json.Unmarshal([]byte(validator.Info), &metaInfo)
		if err != nil {
			return validator, err
		}
	}

	metaInfo.WebsiteLink = metadata.URL
	metaInfo.EmailAddress = metadata.Email
	metaInfo.TwitterAcc = buildTwitterUrl(metadata.Twitter)

	bt, err := json.Marshal(metaInfo)
	if err != nil {
		return validator, err
	}

	validator.Info = string(bt)

	return validator, nil
}

func getAccount(pubKey signature.PublicKey, validatorsMap map[string]dmodels.PublicValidator) (validator dmodels.PublicValidator, err error) {
	key, err := pubKey.MarshalText()
	if err != nil {
		return validator, err
	}

	validator, ok := validatorsMap[string(key)]
	if !ok {
		validator = dmodels.PublicValidator{
			EntityID:      string(key),
			EntityAddress: stakingAPI.NewAddress(pubKey).String(),
		}
	}

	return validator, nil
}

//Return actual validator fee in percents
func getValidatorFee(pubKey signature.PublicKey, provider AccountProvider, unit BlocksRepo) (float64, error) {
	acc, err := provider.Account(context.Background(), &stakingAPI.OwnerQuery{
		Height: 0,
		Owner:  stakingAPI.NewAddress(pubKey),
	})
	if err != nil {
		return 0, err
	}

	block, err := unit.GetLastBlock()
	if err != nil {
		return 0, err
	}

	fee := acc.Escrow.CommissionSchedule.CurrentRate(epochAPI.EpochTime(block.Epoch))
	if fee == nil {
		return 0, nil
	}

	return float64(fee.ToBigInt().Uint64()) / 1000, nil
}
