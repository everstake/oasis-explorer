package scanners

import (
	"context"
	"encoding/hex"
	"fmt"
	"oasisTracker/common/log"
	"oasisTracker/dmodels"
	"oasisTracker/dmodels/oasis"
	"reflect"
	"runtime"

	oasisAddress "github.com/oasisprotocol/oasis-core/go/common/crypto/address"

	"github.com/oasisprotocol/oasis-core/go/common/entity"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	beaconAPI "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	consensusAPI "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothashAPI "github.com/oasisprotocol/oasis-core/go/roothash/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/tendermint/tendermint/crypto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

//Struct for direct work with connection from worker
type ParserTask struct {
	ctx             context.Context
	consensusAPI    consensusAPI.ClientBackend
	beaconAPI       beaconAPI.Backend
	stakingAPI      stakingAPI.Backend
	registryAPI     registryAPI.Backend
	parserContainer *ParseContainer

	baseEpoch beaconAPI.EpochTime
}

func NewParserTask(ctx context.Context, conn *grpc.ClientConn, baseEpoch beaconAPI.EpochTime, parserContainer *ParseContainer) (*ParserTask, error) {
	consensusAPI := consensusAPI.NewConsensusClient(conn)
	stakingAPI := stakingAPI.NewStakingClient(conn)
	registryAPI := registryAPI.NewRegistryClient(conn)
	beaconAPI := beaconAPI.NewBeaconClient(conn)

	return &ParserTask{ctx: ctx, consensusAPI: consensusAPI, stakingAPI: stakingAPI, registryAPI: registryAPI, beaconAPI: beaconAPI, parserContainer: parserContainer, baseEpoch: baseEpoch}, nil
}

func (p *ParserTask) ParseBase(blockID uint64) error {
	blockData, err := p.consensusAPI.GetBlock(p.ctx, int64(blockID))
	if err != nil {
		return fmt.Errorf("api.Block.Get: %s", err.Error())
	}

	err = p.parseOasisBase(blockData, baseFlag)
	if err != nil {
		return fmt.Errorf("p.parseOasisBase: %s", err.Error())
	}

	return nil
}

func (p *ParserTask) EpochBalanceSnapshot(epoch beaconAPI.EpochTime) error {

	blockHeight, err := p.beaconAPI.GetEpochBlock(p.ctx, epoch)
	if err != nil {
		return fmt.Errorf("api.GetEpochBlock.Get: %s", err.Error())
	}

	blockData, err := p.consensusAPI.GetBlock(p.ctx, blockHeight)
	if err != nil {
		return fmt.Errorf("api.Block.Get: %s", err.Error())
	}

	err = p.parseOasisBase(blockData, balanceSnapshotFlag)
	if err != nil {
		return fmt.Errorf("p.parseOasisBase: %s", err.Error())
	}

	return nil
}

func (p *ParserTask) parseOasisBase(blockData *consensusAPI.Block, parseFlag ParseFlag) (err error) {

	b := oasis.Block{}
	//Nil pointer err
	err = cbor.Unmarshal(blockData.Meta, &b)
	if err != nil {
		return err
	}

	b.Hash = blockData.Hash

	var pipes []func(data oasis.Block) error

	if (parseFlag & baseFlag) != 0 {
		pipes = append(pipes, []func(data oasis.Block) error{
			p.parseBlock,
			p.parseBlockSignatures,
			p.parseBlockTransactions,
		}...)
	}

	if (parseFlag & balanceSnapshotFlag) != 0 {
		pipes = append(pipes, []func(data oasis.Block) error{
			p.epochBalanceSnapshot,
		}...)
	}

	for _, pipe := range pipes {
		err = pipe(b)
		if err != nil {
			funcName := runtime.FuncForPC(reflect.ValueOf(pipe).Pointer()).Name()
			return fmt.Errorf("%s (block:%d): %s", funcName, blockData.Height, err.Error())
		}
	}
	return nil
}

func (p *ParserTask) parseBlock(block oasis.Block) error {
	epoch, err := p.beaconAPI.GetEpoch(p.ctx, block.Header.Height)
	if err != nil {
		return err
	}

	p.parserContainer.blocks.Add([]dmodels.Block{{
		Height:          uint64(block.Header.Height),
		Hash:            hex.EncodeToString(block.Hash),
		CreatedAt:       block.Header.Time,
		Epoch:           uint64(epoch),
		ProposerAddress: block.Header.ProposerAddress.String(),
		ValidatorHash:   block.Header.ValidatorsHash.String(),
	}})

	return nil
}

func (p *ParserTask) parseBlockSignatures(block oasis.Block) error {

	blockSignatures := make([]dmodels.BlockSignature, 0, len(block.LastCommit.Signatures))

	for key := range block.LastCommit.Signatures {
		timestamp := block.LastCommit.Signatures[key].Timestamp
		if timestamp.IsZero() {
			timestamp = block.Header.Time
		}

		//Use block timestamp if signature time is zero
		blockSignatures = append(blockSignatures, dmodels.BlockSignature{
			BlockHeight:      block.Header.Height,
			Timestamp:        timestamp,
			BlockIDFlag:      block.LastCommit.Signatures[key].BlockIDFlag,
			ValidatorAddress: block.LastCommit.Signatures[key].ValidatorAddress.String(),
			Signature:        hex.EncodeToString(block.LastCommit.Signatures[key].Signature),
		})

	}

	p.parserContainer.blockSignatures.Add(blockSignatures)

	return nil
}

func (p *ParserTask) parseBlockTransactions(block oasis.Block) (err error) {
	txsWithResults, err := p.consensusAPI.GetTransactionsWithResults(p.ctx, block.Header.Height)
	if err != nil {
		return err
	}

	dTxs := make([]dmodels.Transaction, len(txsWithResults.Transactions))
	var nodeRegisterTxs []dmodels.NodeRegistryTransaction
	var entityRegisterTxs []dmodels.EntityRegistryTransaction

	accountBalanceUpdates := make([]dmodels.AccountBalance, 0, len(txsWithResults.Transactions))
	var tx transaction.SignedTransaction
	var raw transaction.Transaction

	for i := range txsWithResults.Transactions {
		err = cbor.Unmarshal(txsWithResults.Transactions[i], &tx)
		if err != nil {
			return err
		}

		err = cbor.Unmarshal(tx.Blob, &raw)
		if err != nil {
			return err
		}

		txType, err := dmodels.NewTransactionType(raw.Method)
		if err != nil {
			return err
		}

		dTxs[i] = dmodels.Transaction{
			BlockLevel: uint64(block.Header.Height),
			BlockHash:  hex.EncodeToString(block.Hash),
			Hash:       tx.Hash().String(),
			Time:       block.Header.Time,

			//Todo probably merge amount to single field

			//Save tx status and error if presented
			Status: txsWithResults.Results[i].IsSuccess(),
			Error:  txsWithResults.Results[i].Error.Message,

			Type:   txType.Type(),
			Sender: stakingAPI.NewAddress(tx.Signature.PublicKey).String(),
			//Use system address as default receiver
			Receiver: stakingAPI.NewAddress(oasis.SystemPublicKey).String(),
			Nonce:    raw.Nonce,
			Fee:      raw.Fee.Amount.ToBigInt().Uint64(),
			GasLimit: uint64(raw.Fee.Gas),
			GasPrice: raw.Fee.GasPrice().ToBigInt().Uint64(),
		}

		//Todo Move to sep method
		switch raw.Method {
		case "staking.Transfer":
			var xfer stakingAPI.Transfer

			if err := cbor.Unmarshal(raw.Body, &xfer); err != nil {
				return err
			}
			dTxs[i].Amount = xfer.Amount.ToBigInt().Uint64()
			dTxs[i].Receiver = xfer.To.String()
		case "staking.Burn":
			var burn stakingAPI.Burn

			if err := cbor.Unmarshal(raw.Body, &burn); err != nil {
				return err
			}
			dTxs[i].Amount = burn.Amount.ToBigInt().Uint64()
		case "staking.AddEscrow":
			var escrow stakingAPI.Escrow

			if err := cbor.Unmarshal(raw.Body, &escrow); err != nil {
				return err
			}

			dTxs[i].Receiver = escrow.Account.String()
			dTxs[i].EscrowAmount = escrow.Amount.ToBigInt().Uint64()
		case "staking.ReclaimEscrow":
			var reclaim stakingAPI.ReclaimEscrow

			if err := cbor.Unmarshal(raw.Body, &reclaim); err != nil {
				return err
			}

			dTxs[i].Receiver = reclaim.Account.String()

			//Find DebondingStart event
			for j := range txsWithResults.Results[i].Events {

				if txsWithResults.Results[i].Events[j].Staking != nil {
					if txsWithResults.Results[i].Events[j].Staking.Escrow != nil {
						if txsWithResults.Results[i].Events[j].Staking.Escrow.DebondingStart != nil {
							dTxs[i].EscrowReclaimAmount = txsWithResults.Results[i].Events[j].Staking.Escrow.DebondingStart.Amount.ToBigInt().Uint64()
							break
						}
					}
				}
			}
		case "staking.AmendCommissionSchedule":
			//	Todo save as extra field
			var amend stakingAPI.AmendCommissionSchedule

			if err := cbor.Unmarshal(raw.Body, &amend); err != nil {
				return err
			}
		case "staking.Allow":
			var allow stakingAPI.Allow

			if err := cbor.Unmarshal(raw.Body, &allow); err != nil {
				return err
			}

			//	Todo add negative amount support
			dTxs[i].Receiver = allow.Beneficiary.String()
			dTxs[i].Amount = allow.AmountChange.ToBigInt().Uint64()
		case "staking.Withdraw":
			var withdraw stakingAPI.Withdraw

			if err := cbor.Unmarshal(raw.Body, &withdraw); err != nil {
				return err
			}

			dTxs[i].Receiver = withdraw.From.String()
			dTxs[i].Amount = withdraw.Amount.ToBigInt().Uint64()

		case "registry.RegisterNode":
			var node signature.MultiSigned

			if err := cbor.Unmarshal(raw.Body, &node); err != nil {
				return err
			}

			regTx, err := p.parseNodeRegistryTransaction(block, node)
			if err != nil {
				return err
			}

			regTx.Hash = tx.Hash().String()
			nodeRegisterTxs = append(nodeRegisterTxs, regTx)
		case "registry.RegisterEntity":
			var entity entity.SignedEntity

			if err := cbor.Unmarshal(raw.Body, &entity); err != nil {
				return err
			}

			entityRegisterTx, err := p.parseEntityRegistryTransaction(block, entity)
			if err != nil {
				return err
			}

			entityRegisterTx.Hash = tx.Hash().String()
			entityRegisterTxs = append(entityRegisterTxs, entityRegisterTx)
		case "registry.DeregisterEntity":
			//No body
		case "registry.UnfreezeNode":
			var node registryAPI.UnfreezeNode

			if err := cbor.Unmarshal(raw.Body, &node); err != nil {
				return err
			}
		case "registry.RegisterRuntime":
			var runtime registryAPI.Runtime

			if err := cbor.Unmarshal(raw.Body, &runtime); err != nil {
				return err
			}

		case "roothash.ExecutorCommit":
			var commit roothashAPI.ExecutorCommit

			if err := cbor.Unmarshal(raw.Body, &commit); err != nil {
				return err
			}
		case "roothash.ExecutorProposerTimeout":
			var timeoutReq roothashAPI.ExecutorProposerTimeoutRequest

			if err := cbor.Unmarshal(raw.Body, &timeoutReq); err != nil {
				return err
			}
		case "roothash.Evidence":
			var evidence roothashAPI.Evidence

			if err := cbor.Unmarshal(raw.Body, &evidence); err != nil {
				return err
			}
		case "governance.SubmitProposal":
			var proposalContent governance.ProposalContent
			if err := cbor.Unmarshal(raw.Body, &proposalContent); err != nil {
				return err
			}
		case "governance.CastVote":
			var proposalVote governance.ProposalVote
			if err := cbor.Unmarshal(raw.Body, &proposalVote); err != nil {
				return err
			}
		default:
			fmt.Errorf("Unknown tx type: %s", raw.Method)
		}

		//Update balance of sender account as default
		accountsUpdateMap := map[stakingAPI.Address]bool{
			stakingAPI.NewAddress(tx.Signature.PublicKey): true,
		}

		for _, event := range txsWithResults.Results[i].Events {
			if event.Staking != nil {

				switch {
				case event.Staking.Transfer != nil:
					//Update destination account
					accountsUpdateMap[event.Staking.Transfer.To] = true
				case event.Staking.Escrow != nil:

					switch {
					case event.Staking.Escrow.Add != nil:
						//Escrow destination
						accountsUpdateMap[event.Staking.Escrow.Add.Escrow] = true
					case event.Staking.Escrow.DebondingStart != nil:
						//Debonding start destination
						accountsUpdateMap[event.Staking.Escrow.DebondingStart.Escrow] = true
					case event.Staking.Escrow.Take != nil:
						//Stake is slashed
						accountsUpdateMap[event.Staking.Escrow.Take.Owner] = true
						//Todo handle reclaim
						//case event.Staking.Escrow.Reclaim != nil:
					}

				case event.Staking.AllowanceChange != nil:
				//	Todo add after handle account allowances
				case event.Staking.Burn != nil:
					// Owner account already updated as sender
					//	Todo check burn from allowance
				}
			}
		}

		//Save updated balances
		balanceUpdates, err := p.parseAccountBalances(block, accountsUpdateMap)
		if err != nil {
			return err
		}
		accountBalanceUpdates = append(accountBalanceUpdates, balanceUpdates...)
	}

	p.parserContainer.txs.Add(dTxs, nodeRegisterTxs, entityRegisterTxs)
	p.parserContainer.balances.Add(accountBalanceUpdates)

	return nil
}

func (p *ParserTask) epochBalanceSnapshot(block oasis.Block) error {

	epoch, err := p.beaconAPI.GetEpoch(p.ctx, block.Header.Height)
	if err != nil {
		return err
	}

	epochBlock, err := p.beaconAPI.GetEpochBlock(p.ctx, epoch)
	if err != nil {
		return err
	}

	//Make snapshot only for epoch start block
	if block.Header.Height != epochBlock {
		return nil
	}

	entities, err := p.registryAPI.GetEntities(p.ctx, block.Header.Height)
	if err != nil {
		return err
	}

	if len(entities) == 0 {
		return nil
	}

	updates := make([]dmodels.AccountBalance, 0, len(entities))
	rewards := make([]dmodels.Reward, 0, len(entities))
	var rewardsAmount uint64

	var entityAddress stakingAPI.Address
	for i := range entities {

		entityAddress = stakingAPI.NewAddress(entities[i].ID)

		balance, err := p.getAccountBalance(block.Header.Height, entityAddress)
		if err != nil {
			return err
		}

		balance.Time = block.Header.Time
		updates = append(updates, balance)

		prevBalance, err := p.getAccountBalance(block.Header.Height-1, entityAddress)
		if err != nil {
			return err
		}

		//Todo check txs
		rewardsAmount = (balance.EscrowBalanceActive - prevBalance.EscrowBalanceActive) + (balance.GeneralBalance - prevBalance.GeneralBalance)

		if rewardsAmount > 0 {
			rewards = append(rewards, dmodels.Reward{
				EntityAddress: entityAddress.String(),
				BlockLevel:    block.Header.Height,
				Epoch:         uint64(epoch),

				Amount:    rewardsAmount,
				CreatedAt: block.Header.Time,
			})
		}
	}

	debondingUpdates, err := p.processDebondingDelegations(block)
	if err != nil {
		return err
	}

	updates = append(updates, debondingUpdates...)

	p.parserContainer.balances.Add(updates)
	p.parserContainer.rewards.Add(rewards)

	return nil
}

func (p *ParserTask) processDebondingDelegations(block oasis.Block) (updates []dmodels.AccountBalance, err error) {
	//Save undelegations
	addresses, err := p.stakingAPI.Addresses(context.Background(), block.Header.Height)
	if err != nil {
		return updates, err
	}

	for _, address := range addresses {

		debondingDelegations, err := p.stakingAPI.DebondingDelegationsFor(p.ctx, &stakingAPI.OwnerQuery{
			Height: block.Header.Height,
			Owner:  address,
		})
		if err != nil {
			return updates, err
		}

		previousDebondingDelegations, err := p.stakingAPI.DebondingDelegationsFor(p.ctx, &stakingAPI.OwnerQuery{
			Height: block.Header.Height - 1,
			Owner:  address,
		})
		if err != nil {
			return updates, err
		}

		//Equal so skip
		if compareDebondingDelegations(debondingDelegations, previousDebondingDelegations) {
			continue
		}

		accountBalance, err := p.getAccountBalance(block.Header.Height, address)
		if err != nil {
			return updates, err
		}

		accountBalance.Time = block.Header.Time

		updates = append(updates, accountBalance)
	}

	return updates, nil
}

func compareDebondingDelegations(debondingDelegations, previousDebondingDelegations map[stakingAPI.Address][]*stakingAPI.DebondingDelegation) bool {

	if len(debondingDelegations) != len(previousDebondingDelegations) {
		return false
	}

	for address, debondings := range debondingDelegations {
		prevDebonding, ok := previousDebondingDelegations[address]
		if !ok {
			return false
		}

		if len(prevDebonding) != len(debondings) {
			return false
		}

		if prevDebonding[0].Shares.Cmp(&debondings[0].Shares) != 0 || prevDebonding[0].DebondEndTime != debondings[0].DebondEndTime {
			return false
		}
	}

	return true
}

func (p *ParserTask) parseAccountBalances(block oasis.Block, addresses map[stakingAPI.Address]bool) (updates []dmodels.AccountBalance, err error) {
	updates = make([]dmodels.AccountBalance, 0, len(addresses))

	for address := range addresses {
		//Skip system address
		if (oasisAddress.Address)(address).Equal(oasis.SystemAddress) {
			continue
		}

		balance, err := p.getAccountBalance(block.Header.Height, address)
		if err != nil {
			return updates, err
		}

		balance.Time = block.Header.Time
		updates = append(updates, balance)
	}

	return updates, nil
}

func (p *ParserTask) getAccountBalance(height int64, address stakingAPI.Address) (balance dmodels.AccountBalance, err error) {

	accInfo, err := p.stakingAPI.Account(p.ctx, &stakingAPI.OwnerQuery{
		Height: height,
		Owner:  address,
	})
	if err != nil {
		return balance, err
	}

	delegations, err := p.stakingAPI.DelegationInfosFor(p.ctx, &stakingAPI.OwnerQuery{
		Height: height,
		Owner:  address,
	})

	var delegationsBalance uint64
	var selfDelegationBalance uint64

	for delegator, delegation := range delegations {

		stakeBalance, err := delegation.Pool.StakeForShares(&delegation.Shares)
		if err != nil {
			log.Error("Somehow delegations rpc values is wrong", zap.Error(err))
			continue
		}

		if delegator.Equal(address) {
			selfDelegationBalance += stakeBalance.ToBigInt().Uint64()
		}

		delegationsBalance += stakeBalance.ToBigInt().Uint64()
	}

	debondingDelegations, err := p.stakingAPI.DebondingDelegationInfosFor(p.ctx, &stakingAPI.OwnerQuery{
		Height: height,
		Owner:  address,
	})

	var debondingDelegationsBalance uint64
	for _, debDelegationList := range debondingDelegations {
		for _, debDelegation := range debDelegationList {

			debondingBalance, err := debDelegation.Pool.StakeForShares(&debDelegation.Shares)
			if err != nil {
				log.Error("Somehow debonding rpc values is wrong", zap.Error(err))
				continue
			}

			debondingDelegationsBalance += debondingBalance.ToBigInt().Uint64()
		}
	}

	return dmodels.AccountBalance{
		Account:        address.String(),
		Height:         height,
		Nonce:          accInfo.General.Nonce,
		GeneralBalance: accInfo.General.Balance.ToBigInt().Uint64(),

		//Income delegations
		EscrowBalanceActive: accInfo.Escrow.Active.Balance.ToBigInt().Uint64(),
		EscrowBalanceShare:  accInfo.Escrow.Active.TotalShares.ToBigInt().Uint64(),

		EscrowDebondingActive: accInfo.Escrow.Debonding.Balance.ToBigInt().Uint64(),
		EscrowDebondingShare:  accInfo.Escrow.Debonding.TotalShares.ToBigInt().Uint64(),

		//Outcome delegations
		DelegationsBalance:          delegationsBalance,
		DebondingDelegationsBalance: debondingDelegationsBalance,

		SelfDelegationBalance: selfDelegationBalance,

		CommissionSchedule: dmodels.CommissionSchedule{CommissionSchedule: accInfo.Escrow.CommissionSchedule},
	}, nil
}

func (p *ParserTask) parseNodeRegistryTransaction(block oasis.Block, raw signature.MultiSigned) (registerTx dmodels.NodeRegistryTransaction, err error) {
	regNode := oasis.RegisterNode{}
	err = cbor.Unmarshal(raw.Blob, &regNode)
	if err != nil {
		return registerTx, err
	}

	var physicalAddress string
	if len(regNode.Consensus.Addresses) > 0 {
		physicalAddress = regNode.Consensus.Addresses[0].Address.String()
	}

	consensusIDBytes, err := regNode.Consensus.ID.MarshalBinary()
	if err != nil {
		return registerTx, err
	}

	return dmodels.NodeRegistryTransaction{
		BlockLevel:       uint64(block.Header.Height),
		Time:             block.Header.Time,
		ID:               regNode.ID.String(),
		Address:          stakingAPI.NewAddress(regNode.ID).String(),
		EntityID:         regNode.EntityID.String(),
		EntityAddress:    stakingAPI.NewAddress(regNode.EntityID).String(),
		Expiration:       regNode.Expiration,
		P2PID:            regNode.P2P.ID.String(),
		ConsensusID:      regNode.Consensus.ID.String(),
		ConsensusAddress: crypto.AddressHash(consensusIDBytes).String(),
		PhysicalAddress:  physicalAddress,
		Roles:            uint32(regNode.Roles),
	}, nil
}

func (p *ParserTask) parseEntityRegistryTransaction(block oasis.Block, raw entity.SignedEntity) (registerTx dmodels.EntityRegistryTransaction, err error) {

	regEntity := oasis.RegisterEntity{}
	err = cbor.Unmarshal(raw.Blob, &regEntity)
	if err != nil {
		return registerTx, err
	}

	nodes := make([]string, len(regEntity.Nodes))
	for i := range regEntity.Nodes {
		nodes[i] = regEntity.Nodes[i].String()
	}

	return dmodels.EntityRegistryTransaction{
		BlockLevel: uint64(block.Header.Height),
		Time:       block.Header.Time,
		ID:         regEntity.ID.String(),
		Address:    stakingAPI.NewAddress(regEntity.ID).String(),
		Nodes:      nodes,
	}, nil
}
