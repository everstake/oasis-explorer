package scanners

import (
	"context"
	"fmt"
	"oasisTracker/common/genesis"
	"oasisTracker/common/log"
	"oasisTracker/conf"
	"oasisTracker/dao"
	"oasisTracker/dmodels"

	"go.uber.org/zap"
)

type Watcher struct {
	dao        dao.DAO
	cfg        conf.Config
	parser     *Parser
	ctx        context.Context
	cancelFunc context.CancelFunc

	BlocksReSyncInit bool
	EpochReSyncInit  bool
}

func NewWatcher(cfg conf.Config, d dao.DAO) (*Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dao, err := d.GetParserDAO()
	if err != nil {
		return nil, fmt.Errorf("GetParserDAO: %s", err.Error())
	}

	parser, err := NewParser(ctx, cfg.Scanner, dao)
	if err != nil {
		return nil, fmt.Errorf("NewParser: %s", err.Error())
	}

	return &Watcher{
		cfg:        cfg,
		dao:        d,
		parser:     parser,
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

func (m *Watcher) Title() string {
	return "Watcher module"
}

func (m *Watcher) Stop() error {
	m.cancelFunc()
	return nil
}

func (m *Watcher) Run() error {
	ch, cPub, err := m.parser.api.WatchBlocks(m.ctx)
	if err != nil {
		return fmt.Errorf("WatchBlocks error: %s", err)
	}

	epochCh, epochCPub, err := m.parser.bAPI.WatchEpochs(m.ctx)
	if err != nil {
		return fmt.Errorf("WatchBlocks error: %s", err)
	}

	for {
		select {
		case <-m.ctx.Done():
			cPub.Close()
			epochCPub.Close()
			return nil
		case block := <-ch:
			if !m.BlocksReSyncInit {
				//Interval right border not included, so process current Height in watch
				err = m.addBlocksReSyncTask(block.Height)
				if err != nil {
					log.Error("AddReSyncTask error", zap.Error(err))
					continue
				}

				m.BlocksReSyncInit = true
			}

			err = m.parser.ParseWatchBlock(block)
			if err != nil {
				log.Error("ParseBlock error", zap.Error(err))
				continue
			}

			err = m.parser.Save()
			if err != nil {
				log.Error("Save error", zap.Error(err))
				continue
			}
		case epoch := <-epochCh:
			if !m.EpochReSyncInit {
				err = m.addEpochsReSyncTask(uint64(epoch))
				if err != nil {
					log.Error("AddReSyncTask error", zap.Error(err))
					continue
				}
				m.EpochReSyncInit = true
				//Current epoch will be processed by resync task
				continue
			}

			err = m.parser.ParseEpochSnap(epoch)
			if err != nil {
				log.Error("ParseEpochSnap error", zap.Error(err))
				continue
			}

			err = m.parser.Save()
			if err != nil {
				log.Error("Save error", zap.Error(err))
				continue
			}

			//Save all processed epochs
			err = m.dao.CreateTask(dmodels.Task{
				IsActive:      false,
				Title:         parserBalancesSnapshotTask,
				StartHeight:   uint64(epoch),
				CurrentHeight: uint64(epoch),
				EndHeight:     uint64(epoch + 1),
				Batch:         1,
			})
			if err != nil {
				return fmt.Errorf("CreateTask error: %s", err)
			}
		}
	}
}

func (m *Watcher) addBlocksReSyncTask(currentHeight int64) error {
	//Setup init startHeight from config
	startHeight := m.cfg.Scanner.StartHeight

	//Get last task
	task, isFound, err := m.dao.GetLastTask(parserBaseTask)
	if err != nil {
		return fmt.Errorf("GetLastTask error: %s", err)
	}
	if isFound {
		startHeight = task.EndHeight
	}

	//Get last block
	lastBlock, err := m.parser.dao.GetLastBlock()
	if err != nil {
		return fmt.Errorf("GetLastBlock error: %s", err)
	}

	//Last block already processed so increase by 1
	if lastBlock.Height > startHeight {
		startHeight = lastBlock.Height + 1
	}

	if startHeight >= uint64(currentHeight) {
		return nil
	}

	//Previous tasks not found
	if startHeight == 0 {
		gen, err := genesis.ReadGenesisFile(genesis.DefaultGenesisFileName)
		if err != nil {
			return fmt.Errorf("ReadGenesisFile error: %s", err)
		}

		startHeight = gen.GenesisHeight
	}

	//Blocks sync
	err = m.dao.CreateTask(dmodels.Task{
		IsActive:      true,
		Title:         parserBaseTask,
		StartHeight:   startHeight,
		CurrentHeight: startHeight,
		EndHeight:     uint64(currentHeight),
		Batch:         m.cfg.Scanner.BatchSize,
	})
	if err != nil {
		return fmt.Errorf("CreateTask error: %s", err)
	}

	return nil
}

func (m *Watcher) addEpochsReSyncTask(currentEpoch uint64) error {
	//Setup init startEpoch from config
	startEpoch := m.cfg.Scanner.StartHeight

	//Get last task
	task, isFound, err := m.dao.GetLastTask(parserBalancesSnapshotTask)
	if err != nil {
		return fmt.Errorf("GetLastTask error: %s", err)
	}
	if isFound {
		startEpoch = task.EndHeight
	}

	//Previous tasks not found
	if startEpoch == 0 {
		gen, err := genesis.ReadGenesisFile(genesis.DefaultGenesisFileName)
		if err != nil {
			return fmt.Errorf("ReadGenesisFile error: %s", err)
		}

		//Start from epoch +1
		startEpoch = gen.Beacon.Base + 1
	}

	//Already processed
	if startEpoch > currentEpoch {
		return nil
	}

	//Snaps sync
	err = m.dao.CreateTask(dmodels.Task{
		IsActive:      true,
		Title:         parserBalancesSnapshotTask,
		StartHeight:   startEpoch,
		CurrentHeight: startEpoch,
		//Current epoch presented and should be processed in
		EndHeight: currentEpoch + 1,
		//1 Epoch ~ 600 blocks
		Batch: 10,
	})
	if err != nil {
		return fmt.Errorf("CreateTask error: %s", err)
	}

	return nil
}
