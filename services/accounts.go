/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package services

import (
	"fmt"
	"sync"

	"github.com/cgrates/cgrates/apis"

	"github.com/cgrates/birpc"
	"github.com/cgrates/cgrates/accounts"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/cores"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/servmanager"
	"github.com/cgrates/cgrates/utils"
)

// NewAccountService returns the Account Service
func NewAccountService(cfg *config.CGRConfig, dm *DataDBService,
	cacheS *engine.CacheS, filterSChan chan *engine.FilterS,
	connMgr *engine.ConnManager, server *cores.Server,
	internalChan chan birpc.ClientConnector,
	anz *AnalyzerService, srvDep map[string]*sync.WaitGroup) servmanager.Service {
	return &AccountService{
		connChan:    internalChan,
		cfg:         cfg,
		dm:          dm,
		cacheS:      cacheS,
		filterSChan: filterSChan,
		connMgr:     connMgr,
		server:      server,
		anz:         anz,
		srvDep:      srvDep,
		rldChan:     make(chan struct{}, 1),
	}
}

// AccountService implements Service interface
type AccountService struct {
	sync.RWMutex
	cfg         *config.CGRConfig
	dm          *DataDBService
	cacheS      *engine.CacheS
	filterSChan chan *engine.FilterS
	connMgr     *engine.ConnManager
	server      *cores.Server

	rldChan  chan struct{}
	stopChan chan struct{}

	acts     *accounts.AccountS
	rpc      *apis.AccountSv1           // useful on restart
	connChan chan birpc.ClientConnector // publish the internal Subsystem when available
	anz      *AnalyzerService
	srvDep   map[string]*sync.WaitGroup
}

// Start should handle the service start
func (acts *AccountService) Start() (err error) {
	if acts.IsRunning() {
		return utils.ErrServiceAlreadyRunning
	}

	<-acts.cacheS.GetPrecacheChannel(utils.CacheAccounts)
	<-acts.cacheS.GetPrecacheChannel(utils.CacheAccountsFilterIndexes)

	filterS := <-acts.filterSChan
	acts.filterSChan <- filterS
	dbchan := acts.dm.GetDMChan()
	datadb := <-dbchan
	dbchan <- datadb

	acts.Lock()
	defer acts.Unlock()
	acts.acts = accounts.NewAccountS(acts.cfg, filterS, acts.connMgr, datadb)
	acts.stopChan = make(chan struct{})
	go acts.acts.ListenAndServe(acts.stopChan, acts.rldChan)

	utils.Logger.Info(fmt.Sprintf("<%s> starting <%s> subsystem", utils.CoreS, utils.AccountS))
	acts.rpc = apis.NewAccountSv1(acts.acts)
	srv, _ := birpc.NewService(acts.rpc, "", false)
	if !acts.cfg.DispatcherSCfg().Enabled {
		acts.server.RpcRegister(srv)
	}
	acts.connChan <- acts.anz.GetInternalCodec(srv, utils.AccountS)
	return
}

// Reload handles the change of config
func (acts *AccountService) Reload() (err error) {
	acts.rldChan <- struct{}{}
	return // for the moment nothing to reload
}

// Shutdown stops the service
func (acts *AccountService) Shutdown() (err error) {
	acts.Lock()
	close(acts.stopChan)
	acts.acts.Shutdown()
	acts.acts = nil
	acts.rpc = nil
	<-acts.connChan
	acts.Unlock()
	return
}

// IsRunning returns if the service is running
func (acts *AccountService) IsRunning() bool {
	acts.RLock()
	defer acts.RUnlock()
	return acts != nil && acts.acts != nil
}

// ServiceName returns the service name
func (acts *AccountService) ServiceName() string {
	return utils.AccountS
}

// ShouldRun returns if the service should be running
func (acts *AccountService) ShouldRun() bool {
	return acts.cfg.AccountSCfg().Enabled
}