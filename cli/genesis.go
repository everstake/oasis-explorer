package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"oasisTracker/common/genesis"
	"oasisTracker/common/log"
	"oasisTracker/dao"
	"oasisTracker/dmodels"
	"oasisTracker/dmodels/oasis"

	"go.uber.org/zap"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/tendermint/tendermint/crypto"
)

const SetupGenesisJson = "setup-genesis"

type (
	ICli interface {
		Setup(args []string) error
		SetupGenesisJson(args []string) error
	}

	Cli struct {
		DAO dao.ParserDAO
	}
)

func NewCli(d dao.DAO) ICli {

	pDAO, _ := d.GetParserDAO()

	return &Cli{
		DAO: pDAO,
	}
}

func (cli *Cli) Setup(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("invalid arguments length %d", len(args))
	}

	switch args[0] {
	case SetupGenesisJson:
		return cli.SetupGenesisJson(args[1:])
	default:
		return fmt.Errorf("unsupported setup mode %s", args[0])
	}

}

func (cli *Cli) SetupGenesisJson(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("invalid arguments length %d, expected 1 arguments for setup-genesis", len(args))
	}

	gen, err := genesis.ReadGenesisFile(args[0])
	if err != nil {
		return err
	}

	genesisBlockHash := sha256.Sum256([]byte(gen.ChainID))

	balances := make([]dmodels.AccountBalance, len(gen.Staking.Ledger))

	i := 0
	//Genesis balances
	for accountAddress, balance := range gen.Staking.Ledger {

		delegations := gen.Staking.Delegations[accountAddress]
		var delegationsBalance uint64
		var selfDelegationBalance uint64
		for delegator, balance := range delegations {

			if delegator == accountAddress.String() {
				selfDelegationBalance += balance.Shares.ToBigInt().Uint64()
			}

			delegationsBalance += balance.Shares.ToBigInt().Uint64()
		}

		debondingDelegations := gen.Staking.DebondingDelegations[accountAddress]
		var debondingDelegationsBalance uint64
		for _, debondings := range debondingDelegations {
			for _, value := range debondings {
				debondingDelegationsBalance += value.Shares.ToBigInt().Uint64()
			}
		}

		balances[i] = dmodels.AccountBalance{
			Account:                     accountAddress.String(),
			Time:                        gen.GenesisTime,
			Height:                      int64(gen.GenesisHeight),
			Nonce:                       balance.General.Nonce,
			GeneralBalance:              balance.General.Balance.ToBigInt().Uint64(),
			EscrowBalanceActive:         balance.Escrow.Active.Balance.ToBigInt().Uint64(),
			EscrowBalanceShare:          balance.Escrow.Active.TotalShares.ToBigInt().Uint64(),
			EscrowDebondingActive:       balance.Escrow.Debonding.Balance.ToBigInt().Uint64(),
			EscrowDebondingShare:        balance.Escrow.Debonding.TotalShares.ToBigInt().Uint64(),
			DelegationsBalance:          delegationsBalance,
			DebondingDelegationsBalance: debondingDelegationsBalance,
			SelfDelegationBalance:       selfDelegationBalance,
			CommissionSchedule:          dmodels.CommissionSchedule{CommissionSchedule: balance.Escrow.CommissionSchedule},
		}

		i++
	}

	err = cli.DAO.CreateAccountBalances(balances)
	if err != nil {
		return err
	}

	txs := make([]dmodels.Transaction, 0, len(gen.Staking.Delegations)+len(gen.Staking.DebondingDelegations))

	//Genesis delegations
	for receiverAddress, senders := range gen.Staking.Delegations {

		for senderAddress, share := range senders {
			txHash := sha256.Sum256([]byte(fmt.Sprint(gen.ChainID, "delegation", receiverAddress, senderAddress, share.Shares.String())))

			escrowAmount, err := gen.Staking.Ledger[receiverAddress].Escrow.Active.StakeForShares(&share.Shares)
			if err != nil {
				log.Error("Delegations StakeForShares calc error", zap.Error(err))
				continue
			}

			txs = append(txs, dmodels.Transaction{
				BlockLevel:          gen.GenesisHeight,
				BlockHash:           hex.EncodeToString(genesisBlockHash[:]),
				Hash:                hex.EncodeToString(txHash[:]),
				Time:                gen.GenesisTime,
				Amount:              0,
				EscrowAmount:        escrowAmount.ToBigInt().Uint64(),
				EscrowReclaimAmount: 0,
				Type:                dmodels.TransactionTypeAddEscrow,
				Status:              true,
				Error:               "",
				Sender:              senderAddress,
				Receiver:            receiverAddress.String(),
				Nonce:               0,
				Fee:                 0,
				GasLimit:            0,
				GasPrice:            0,
			})
		}
	}

	//In this genesis not used
	//Genesis escrowreclaim
	for validator, senders := range gen.Staking.DebondingDelegations {

		for unstaker, shareArr := range senders {

			for i := range shareArr {

				txHash := sha256.Sum256([]byte(fmt.Sprint(gen.ChainID, "reclaim", validator, unstaker, shareArr[i].Shares.String())))

				escrowReclaimAmount, err := gen.Staking.Ledger[validator].Escrow.Debonding.StakeForShares(&shareArr[i].Shares)
				if err != nil {
					log.Error("DebondingDelegations StakeForShares calc error", zap.Error(err))
					continue
				}

				txs = append(txs, dmodels.Transaction{
					BlockLevel:          gen.GenesisHeight,
					BlockHash:           hex.EncodeToString(genesisBlockHash[:]),
					Hash:                hex.EncodeToString(txHash[:]),
					Time:                gen.GenesisTime,
					Amount:              0,
					EscrowAmount:        0,
					EscrowReclaimAmount: escrowReclaimAmount.ToBigInt().Uint64(),
					Receiver:            validator.String(),
					Type:                dmodels.TransactionTypeReclaimEscrow,
					Sender:              unstaker,
					Nonce:               0,
					Fee:                 0,
					GasLimit:            0,
					GasPrice:            0,
				})

			}
		}
	}

	err = cli.DAO.CreateTransfers(txs)
	if err != nil {
		return err
	}

	nodes := make([]dmodels.NodeRegistryTransaction, len(gen.Registry.Nodes))
	//Genesis nodes
	for i := range gen.Registry.Nodes {
		node := oasis.RegisterNode{}
		err = cbor.Unmarshal(gen.Registry.Nodes[i].Blob, &node)
		if err != nil {
			return err
		}

		consensusIDBytes, err := node.Consensus.ID.MarshalBinary()
		if err != nil {
			return err
		}

		var physicalAddress string
		if len(node.Consensus.Addresses) > 0 {
			physicalAddress = node.Consensus.Addresses[0].Address.String()
		}

		nodes[i] = dmodels.NodeRegistryTransaction{
			BlockLevel:       gen.GenesisHeight,
			Hash:             gen.Registry.Nodes[i].Hash().String(),
			Time:             gen.GenesisTime,
			ID:               node.ID.String(),
			Address:          api.NewAddress(node.ID).String(),
			EntityID:         node.EntityID.String(),
			EntityAddress:    api.NewAddress(node.EntityID).String(),
			Expiration:       node.Expiration,
			P2PID:            node.P2P.ID.String(),
			ConsensusID:      node.Consensus.ID.String(),
			ConsensusAddress: crypto.AddressHash(consensusIDBytes).String(),
			PhysicalAddress:  physicalAddress,
			Roles:            uint32(node.Roles),
		}
	}

	err = cli.DAO.CreateRegisterNodeTransactions(nodes)
	if err != nil {
		return err
	}

	entities := make([]dmodels.EntityRegistryTransaction, len(gen.Registry.Entities))
	//Genesis nodes
	for i := range gen.Registry.Entities {
		entity := oasis.RegisterEntity{}
		err = cbor.Unmarshal(gen.Registry.Entities[i].Blob, &entity)
		if err != nil {
			return err
		}

		nodes := make([]string, len(entity.Nodes))
		for i := range entity.Nodes {
			nodes[i] = entity.Nodes[i].String()
		}

		entities[i] = dmodels.EntityRegistryTransaction{
			BlockLevel: gen.GenesisHeight,
			Hash:       gen.Registry.Entities[i].Hash().String(),
			Time:       gen.GenesisTime,
			ID:         entity.ID.String(),
			Address:    api.NewAddress(entity.ID).String(),
			Nodes:      nodes,
		}
	}

	err = cli.DAO.CreateRegisterEntityTransactions(entities)
	if err != nil {
		return err
	}

	log.Info("Genesis json were successfully parsed")

	return nil
}
