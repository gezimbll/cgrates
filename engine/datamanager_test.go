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
package engine

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cgrates/baningo"
	"github.com/cgrates/birpc"
	"github.com/cgrates/birpc/context"
	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/utils"
)

func TestDatamanagerCacheDataFromDBNoPrfxErr(t *testing.T) {
	dm := NewDataManager(nil, nil, nil)
	err := dm.CacheDataFromDB(context.Background(), "", []string{}, false)
	if err == nil || err.Error() != "unsupported cache prefix" {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", "unsupported cache prefix", err)
	}
}

func TestDatamanagerCacheDataFromDBNoDMErr(t *testing.T) {
	var dm *DataManager
	err := dm.CacheDataFromDB(context.Background(), "", []string{}, false)
	if err == nil || err != utils.ErrNoDatabaseConn {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", utils.ErrNoDatabaseConn, err)
	}
}

func TestDatamanagerCacheDataFromDBNoLimitZeroErr(t *testing.T) {
	cfg := config.NewDefaultCGRConfig()
	dm := NewDataManager(nil, cfg.CacheCfg(), nil)
	dm.cacheCfg.Partitions = map[string]*config.CacheParamCfg{
		utils.CachePrefixToInstance[utils.AttributeProfilePrefix]: {
			Limit: 0,
		},
	}
	err := dm.CacheDataFromDB(context.Background(), utils.AttributeProfilePrefix, []string{}, false)
	if err != nil {
		t.Error(err)
	}
}

func TestDatamanagerCacheDataFromDBMetaAPIBanErr(t *testing.T) {
	cfg := config.NewDefaultCGRConfig()
	dm := NewDataManager(nil, cfg.CacheCfg(), nil)
	dm.cacheCfg.Partitions = map[string]*config.CacheParamCfg{
		utils.CachePrefixToInstance[utils.MetaAPIBan]: {
			Limit: 1,
		},
	}
	err := dm.CacheDataFromDB(context.Background(), utils.MetaAPIBan, []string{}, false)
	if err != nil {
		t.Error(err)
	}
}

func TestDatamanagerCacheDataFromDBMustBeCached(t *testing.T) {
	cfg := config.NewDefaultCGRConfig()
	dm := NewDataManager(nil, cfg.CacheCfg(), nil)
	dm.cacheCfg.Partitions = map[string]*config.CacheParamCfg{
		utils.CachePrefixToInstance[utils.AttributeProfilePrefix]: {
			Limit: 1,
		},
	}
	err := dm.CacheDataFromDB(context.Background(), utils.AttributeProfilePrefix, []string{utils.MetaAny}, true)
	if err != nil {
		t.Error(err)
	}
}

func TestDataManagerDataDB(t *testing.T) {
	var dm *DataManager
	rcv := dm.DataDB()
	if rcv != nil {
		t.Errorf("Expected DataDB to be nil, Received <%+v>", rcv)
	}
}

func TestDataManagerSetFilterDMNil(t *testing.T) {
	expErr := utils.ErrNoDatabaseConn
	var dm *DataManager
	err := dm.SetFilter(context.Background(), nil, true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}
}

func TestDataManagerSetFilterErrConnID(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaFilters].Remote = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	dm := NewDataManager(data, cfg.CacheCfg(), nil)
	fltr := &Filter{
		Tenant: "cgrates.org",
		ID:     "fltr1",
		Rules: []*FilterRule{{
			Type:    utils.MetaString,
			Element: "~*req.Account",
			Values:  []string{"1001", "1002"},
		}},
	}

	expErr := "MANDATORY_IE_MISSING: [connIDs]"
	err := dm.SetFilter(context.Background(), fltr, true)
	if err.Error() != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}
}

func TestDataManagerSetFilterErrSetFilterDrv(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	dm := NewDataManager(data, cfg.CacheCfg(), nil)
	dm.dataDB = &DataDBMock{
		GetFilterDrvF: func(ctx *context.Context, str1 string, str2 string) (*Filter, error) {
			return nil, utils.ErrNotFound
		},
	}
	fltr := &Filter{
		Tenant: "cgrates.org",
		ID:     "fltr1",
		Rules: []*FilterRule{{
			Type:    utils.MetaString,
			Element: "~*req.Account",
			Values:  []string{"1001", "1002"},
		}},
	}

	err := dm.SetFilter(context.Background(), fltr, true)
	if err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, Received error <%v>", utils.ErrNotImplemented, err)
	}
}

func TestDataManagerSetFilterErrUpdateFilterIndex(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return nil, utils.ErrNotImplemented
		},
		SetFilterDrvF: func(ctx *context.Context, fltr *Filter) error { return nil },
	}
	fltr := &Filter{
		Tenant: "cgrates.org",
		ID:     "*stirng:~*req.Account:1001",
		Rules: []*FilterRule{{
			Type:    utils.MetaString,
			Element: "~*req.Account",
			Values:  []string{"1001", "1002"},
		}},
	}

	err := dm.SetFilter(context.Background(), fltr, true)
	if err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, Received error <%v>", utils.ErrNotImplemented, err)
	}
}

func TestDataManagerSetFilterErrItemReplicate(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaFilters].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return nil, nil
		},
		SetFilterDrvF: func(ctx *context.Context, fltr *Filter) error { return nil },
	}
	fltr := &Filter{
		Tenant: "cgrates.org",
		ID:     "*stirng:~*req.Account:1001",
		Rules: []*FilterRule{{
			Type:    utils.MetaString,
			Element: "~*req.Account",
			Values:  []string{"1001", "1002"},
		}},
	}

	expErr := "MANDATORY_IE_MISSING: [connIDs]"
	err := dm.SetFilter(context.Background(), fltr, true)
	if err.Error() != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}
}

func TestDataManagerRemoveFilterNildm(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	err := dm.RemoveFilter(context.Background(), utils.CGRateSorg, "fltr1", true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterErrGetFilter(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetFilterDrvF: func(ctx *context.Context, str1, str2 string) (*Filter, error) {
			return nil, utils.ErrNotImplemented
		},
	}

	expErr := utils.ErrNotImplemented
	err := dm.RemoveFilter(context.Background(), utils.CGRateSorg, "fltr1", true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterErrGetIndexes(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetFilterDrvF: func(ctx *context.Context, str1, str2 string) (*Filter, error) {
			return nil, utils.ErrNotFound
		},
	}

	expErr := utils.ErrNotImplemented
	err := dm.RemoveFilter(context.Background(), utils.CGRateSorg, "fltr1", true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterErrGetIndexesBrokenReference(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return nil, nil
		},
	}

	fltrId := "*stirng:~*req.Account:1001:4fields"
	expErr := "cannot remove filter <cgrates.org:*stirng:~*req.Account:1001:4fields> because will broken the reference to following items: null"
	err := dm.RemoveFilter(context.Background(), utils.CGRateSorg, fltrId, true)
	if err.Error() != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterErrRemoveFilterDrv(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetFilterDrvF: func(ctx *context.Context, str1 string, str2 string) (*Filter, error) {
			return &Filter{}, nil
		},
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return map[string]utils.StringSet{}, utils.ErrNotFound
		},
		RemoveFilterDrvF: func(ctx *context.Context, str1 string, str2 string) error {
			return utils.ErrNotImplemented
		},
	}

	fltrId := "fltr1"
	expErr := utils.ErrNotImplemented
	err := dm.RemoveFilter(context.Background(), utils.CGRateSorg, fltrId, true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterErrNilOldFltr(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	var fltrId string
	var tnt string
	expErr := utils.ErrNotFound
	err := dm.RemoveFilter(context.Background(), tnt, fltrId, false)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveFilterReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaFilters].Replicate = true
	cfg.DataDbCfg().RplConns = []string{}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetFilterDrvF: func(ctx *context.Context, str1 string, str2 string) (*Filter, error) {
			return &Filter{}, nil
		},

		RemoveFilterDrvF: func(ctx *context.Context, str1 string, str2 string) error {
			return nil
		},
	}

	tnt := utils.CGRateSorg
	fltrId := "*stirng:~*req.Account:1001"

	// tested replicate
	dm.RemoveFilter(context.Background(), tnt, fltrId, false)

}

func TestDataManagerRemoveAccountNilDM(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	err := dm.RemoveAccount(context.Background(), utils.CGRateSorg, "acc1", false)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountErrGetAccount(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return nil, utils.ErrNotImplemented
		},
	}

	expErr := utils.ErrNotImplemented
	err := dm.RemoveAccount(context.Background(), utils.CGRateSorg, "fltr1", false)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountErrRemoveAccountDrv(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{}, nil
		},
		RemoveAccountDrvF: func(ctx *context.Context, str1, str2 string) error {
			return utils.ErrNotImplemented
		},
	}

	expErr := utils.ErrNotImplemented
	err := dm.RemoveAccount(context.Background(), utils.CGRateSorg, "fltr1", false)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountErrNiloldRpp(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	var fltrId string
	var tnt string
	expErr := utils.ErrNotFound
	err := dm.RemoveAccount(context.Background(), tnt, fltrId, false)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountErrRemoveItemFromFilterIndex(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{}, nil
		},
		RemoveAccountDrvF: func(ctx *context.Context, str1, str2 string) error {
			return nil
		},
		SetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx string, indexes map[string]utils.StringSet, commit bool, transactionID string) (err error) {
			return utils.ErrNotImplemented
		},
	}

	tnt := utils.CGRateSorg
	fltrId := "*stirng:~*req.Account:1001"
	expErr := utils.ErrNotImplemented
	err := dm.RemoveAccount(context.Background(), tnt, fltrId, true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountErrRemoveIndexFiltersItem(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{
				FilterIDs: []string{"fltr1"},
			}, nil
		},
		RemoveAccountDrvF: func(ctx *context.Context, str1, str2 string) error {
			return nil
		},
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return nil, utils.ErrNotImplemented
		},
	}

	tnt := utils.CGRateSorg
	fltrId := "*stirng:~*req.Account:1001"
	expErr := utils.ErrNotImplemented
	err := dm.RemoveAccount(context.Background(), tnt, fltrId, true)
	if err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDataManagerRemoveAccountReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaFilters].Replicate = true
	cfg.DataDbCfg().RplConns = []string{}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{}, nil
		},
		RemoveAccountDrvF: func(ctx *context.Context, str1, str2 string) error {
			return nil
		},
	}

	tnt := utils.CGRateSorg
	fltrId := "*stirng:~*req.Account:1001"

	// tested replicate
	dm.RemoveAccount(context.Background(), tnt, fltrId, false)

}

func TestDataManagerRemoveDispatcherHostErrNilDM(t *testing.T) {

	var dm *DataManager
	if err := dm.RemoveDispatcherHost(context.Background(), utils.CGRateSorg, "*stirng:~*req.Account:1001"); err != utils.ErrNoDatabaseConn {
		t.Error(err)
	}

}

func TestDataManagerRemoveDispatcherHostErroldDppNil(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	if err := dm.RemoveDispatcherHost(context.Background(), utils.CGRateSorg, "*stirng:~*req.Account:1001"); err != utils.ErrNotFound {
		t.Error(err)
	}

}

func TestDataManagerRemoveDispatcherHostErrGetDisp(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		GetDispatcherHostDrvF: func(ctx *context.Context, s1, s2 string) (*DispatcherHost, error) {
			return nil, utils.ErrNotImplemented
		},
	}

	if err := dm.RemoveDispatcherHost(context.Background(), utils.CGRateSorg, "*stirng:~*req.Account:1001"); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDataManagerRemoveDispatcherHostErrRemoveDisp(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	Cache.Clear(nil)
	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		GetDispatcherHostDrvF: func(ctx *context.Context, s1, s2 string) (*DispatcherHost, error) {
			return &DispatcherHost{}, nil
		},
		RemoveDispatcherHostDrvF: func(ctx *context.Context, s1, s2 string) error {
			return utils.ErrNotImplemented
		},
	}

	if err := dm.RemoveDispatcherHost(context.Background(), utils.CGRateSorg, "*stirng:~*req.Account:1001"); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDataManagerRemoveDispatcherHostReplicateTrue(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaDispatcherHosts].Replicate = true
	cfg.DataDbCfg().RplConns = []string{}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		GetDispatcherHostDrvF: func(ctx *context.Context, s1, s2 string) (*DispatcherHost, error) {
			return &DispatcherHost{}, nil
		},
		RemoveDispatcherHostDrvF: func(ctx *context.Context, s1, s2 string) error {
			return nil
		},
	}

	// tested replicate
	dm.RemoveDispatcherHost(context.Background(), utils.CGRateSorg, "*stirng:~*req.Account:1001")

}

func TestDataManagerSetDispatcherHostErrNilDM(t *testing.T) {

	var dm *DataManager
	if err := dm.SetDispatcherHost(context.Background(), nil); err != utils.ErrNoDatabaseConn {
		t.Error(err)
	}

}

func TestDataManagerSetDispatcherHostErrDataDB(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		SetDispatcherHostDrvF: func(ctx *context.Context, dh *DispatcherHost) error {
			return utils.ErrNotImplemented
		},
	}
	defer data.Close()
	if err := dm.SetDispatcherHost(context.Background(), nil); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDataManagerSetDispatcherHostReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaDispatcherHosts].Replicate = true

	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	dm := NewDataManager(data, cfg.CacheCfg(), nil)

	dpp := &DispatcherHost{
		Tenant: utils.CGRateSorg,
		RemoteHost: &config.RemoteHost{
			ID:                   "ID",
			Address:              "127.0.0.1",
			Transport:            utils.MetaJSON,
			ConnectAttempts:      1,
			Reconnects:           1,
			MaxReconnectInterval: time.Minute,
			ConnectTimeout:       time.Nanosecond,
			ReplyTimeout:         time.Nanosecond,
			TLS:                  true,
			ClientKey:            "key",
			ClientCertificate:    "ce",
			CaCertificate:        "ca",
		},
	}
	// tested replicate
	dm.SetDispatcherHost(context.Background(), dpp)

}

func TestDMRemoveAccountReplicate(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaAccounts].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{}, nil
		},
		RemoveAccountDrvF: func(ctx *context.Context, str1, str2 string) error {
			return nil
		},
	}

	// tested replicate
	if err := dm.RemoveAccount(context.Background(), utils.CGRateSorg, "accId", false); err != nil {
		t.Error(err)
	}
}

func TestDMSetAccountNilDM(t *testing.T) {

	var dm *DataManager
	ap := &utils.Account{}

	expErr := utils.ErrNoDatabaseConn
	if err := dm.SetAccount(context.Background(), ap, false); err != utils.ErrNoDatabaseConn {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetAccountcheckFiltersErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "accId",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{":::"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}

	expErr := "broken reference to filter: <:::> for item with ID: cgrates.org:accId"
	if err := dm.SetAccount(context.Background(), ap, true); err == nil || err.Error() != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetAccountGetAccountErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return nil, utils.ErrNotImplemented
		},
	}

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "accId",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{"*stirng:~*req.Account:1001"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.SetAccount(context.Background(), ap, true); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetAccountSetAccountDrvErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{
				Tenant: "cgrates.org",
			}, nil
		},
		SetAccountDrvF: func(ctx *context.Context, profile *utils.Account) error {
			return utils.ErrNotImplemented
		},
	}

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "accId",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{"*stirng:~*req.Account:1001"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.SetAccount(context.Background(), ap, true); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetAccountupdatedIndexesErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{
				Tenant: "cgrates.org",
			}, nil
		},
		SetAccountDrvF: func(ctx *context.Context, profile *utils.Account) error {
			return nil
		},
	}

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "accId",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{"*stirng:~*req.Account:1001"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.SetAccount(context.Background(), ap, true); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetAccountReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaAccounts].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return &utils.Account{}, nil
		},
		SetAccountDrvF: func(ctx *context.Context, profile *utils.Account) error {
			return nil
		},
	}

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "accId",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{"*stirng:~*req.Account:1001"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}
	// tests replicete
	dm.SetAccount(context.Background(), ap, false)
}

func TestDMRemoveThresholdProfileNilDM(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "ThrPrf1", false); err != utils.ErrNoDatabaseConn {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileGetErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return nil, utils.ErrNotImplemented
		},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "ThrPrf1", false); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileRmvErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return nil, nil
		},
		RemThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (err error) { return utils.ErrNotImplemented },
	}

	expErr := utils.ErrNotImplemented
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "ThrPrf1", false); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileOldThrNil(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return nil, nil
		},
		RemThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (err error) { return nil },
	}

	expErr := utils.ErrNotFound
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "ThrPrf1", false); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileIndxTrueErr1(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return &ThresholdProfile{
				Tenant:           "cgrates.org",
				ID:               "THD_2",
				FilterIDs:        []string{"*string:~*req.Account:1001"},
				ActionProfileIDs: []string{"actPrfID"},
				MaxHits:          7,
				MinHits:          0,
				Weights: utils.DynamicWeights{
					{
						Weight: 20,
					},
				},
				Async: true,
			}, nil
		},
		RemThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (err error) { return nil },
	}

	expErr := utils.ErrNotImplemented
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "THD_2", true); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileIndxTrueErr2(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return &ThresholdProfile{
				Tenant:           "cgrates.org",
				ID:               "THD_2",
				FilterIDs:        []string{"*string:~*req.Account:1001", "noPrefix"},
				ActionProfileIDs: []string{"actPrfID"},
				MaxHits:          7,
				MinHits:          0,
				Weights: utils.DynamicWeights{
					{
						Weight: 20,
					},
				},
				Async: true,
			}, nil
		},
		RemThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (err error) { return nil },
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return nil, utils.ErrNotImplemented
		},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "THD_2", true); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMRemoveThresholdProfileReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaThresholdProfiles].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return &ThresholdProfile{
				Tenant:           "cgrates.org",
				ID:               "THD_2",
				FilterIDs:        []string{"*string:~*req.Account:1001"},
				ActionProfileIDs: []string{"actPrfID"},
				MaxHits:          7,
				MinHits:          0,
				Weights: utils.DynamicWeights{
					{
						Weight: 20,
					},
				},
				Async: true,
			}, nil
		},
		RemThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (err error) { return nil },
	}

	// tests replicate

	dm.RemoveThresholdProfile(context.Background(), utils.CGRateSorg, "THD_2", false)
}

func TestDMSetThresholdErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		SetThresholdDrvF: func(ctx *context.Context, t *Threshold) error { return utils.ErrNotImplemented },
	}

	th := &Threshold{
		Tenant: "cgrates.org",
		ID:     "TH_1",
		Hits:   0,
	}
	expErr := utils.ErrNotImplemented
	if err := dm.SetThreshold(context.Background(), th); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetThresholdReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaThresholds].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	th := &Threshold{
		Tenant: "cgrates.org",
		ID:     "TH_1",
		Hits:   0,
	}

	// tests replicate

	dm.SetThreshold(context.Background(), th)
}

func TestDMRemoveThresholdReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaThresholds].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	th := &Threshold{
		Tenant: "cgrates.org",
		ID:     "TH_1",
		Hits:   0,
	}

	if err := dm.DataDB().SetThresholdDrv(context.Background(), th); err != nil {
		t.Error(err)
	}

	rcv, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", true, false, utils.NonTransactional)
	if err != nil {
		t.Error(err)
	} else if th != rcv {
		t.Errorf("Expected <%+v> , Received <%+v>", th, rcv)
	}

	// tests replicate

	if err := dm.RemoveThreshold(context.Background(), "cgrates.org", "TH_1"); err != nil {
		t.Error(err)
	}
	if getRcv, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", true, false, utils.NonTransactional); err != utils.ErrNotFound {
		t.Error(err)
	} else if getRcv != nil {
		t.Errorf("Expected <%+v>, \nReceived <%+v>\n", nil, getRcv)
	}
}

func TestDMGetThresholdCacheGetErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	if err := Cache.Set(context.Background(), utils.CacheThresholds, utils.ConcatenatedKey(utils.CGRateSorg, "TH_1"), nil, []string{}, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	_, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", true, false, utils.NonTransactional)
	if err != utils.ErrNotFound {
		t.Error(err)
	}
}

// unfinished, getting **Threshold GetThresholdDrv outputing *, we need plain
// func TestDMGetThresholdSetThErr(t *testing.T) {
// 	tmp := Cache
// 	cfgtmp := config.CgrConfig()
// 	tmpCM := connMgr
// 	defer func() {
// 		Cache = tmp
// 		config.SetCgrConfig(cfgtmp)
// 		connMgr = tmpCM
// 	}()
// 	Cache.Clear(nil)

// 	cfg := config.NewDefaultCGRConfig()
// 	cfg.DataDbCfg().Items[utils.MetaThresholds].Remote = true
// 	cfg.DataDbCfg().RmtConns = []string{utils.ConcatenatedKey(utils.MetaInternal,
// 		utils.MetaThresholds)}
// 	config.SetCgrConfig(cfg)
// 	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

// 	th := &Threshold{
// 		Tenant: "cgrates.org",
// 		ID:     "TH_1",
// 		Hits:   0,
// 	}

// 	cc := &ccMock{
// 		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
// 			utils.ReplicatorSv1GetThreshold: func(ctx *context.Context, args, reply interface{}) error {
// 				rplCast, canCast := reply.(*Threshold)
// 				if !canCast {
// 					t.Errorf("Wrong argument type : %T", reply)
// 					return nil
// 				}
// 				*rplCast = *th
// 				return nil
// 			},
// 		},
// 	}

// 	rpcInternal := make(chan birpc.ClientConnector, 1)
// 	rpcInternal <- cc
// 	cM := NewConnManager(cfg)
// 	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaThresholds), utils.ReplicatorSv1, rpcInternal)
// 	dm := NewDataManager(data, cfg.CacheCfg(), cM)

// 	_, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", false, false, utils.NonTransactional)
// 	if  err != utils.ErrNotFound {
// 		t.Error(err)
// 	}
// }

func TestDMGetThresholdSetThErr(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.MetaThresholds].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	expErr := utils.ErrNotImplemented
	_, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", false, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

func TestDMSetStatQueueNewErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	experr := "marshal mock error"
	dm.ms = mockMarshal(experr)

	sq := &StatQueue{
		SQMetrics: map[string]StatMetric{
			"key": statMetricMock(""),
		},
	}

	if err := dm.SetStatQueue(context.Background(), sq); err == nil || err.Error() != experr {
		t.Errorf("Expected error <%v>, Received <%v>", experr, err)
	}
}

func TestDMSetStatQueueSetDrvErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		SetStatQueueDrvF: func(ctx *context.Context, ssq *StoredStatQueue, sq *StatQueue) error { return utils.ErrNotImplemented },
	}

	sq := &StatQueue{

		SQMetrics: map[string]StatMetric{
			utils.MetaASR: &StatASR{
				Metric: &Metric{
					Value: utils.NewDecimal(1, 0),
					Count: 1,
					Events: map[string]*DecimalWithCompress{
						"cgrates.org:TestStatRemExpired_1": {Stat: utils.NewDecimalFromFloat64(1), CompressFactor: 1},
					},
				},
			},
		},
		sqPrfl: &StatQueueProfile{
			Metrics: []*MetricWithFilters{
				{
					MetricID: utils.MetaASR,
				},
			},
		},
	}

	expErr := utils.ErrNotImplemented
	if err := dm.SetStatQueue(context.Background(), sq); err != expErr {
		t.Errorf("Expected error <%v>, Received <%v>", expErr, err)
	}
}

func TestDMSetStatQueueReplicateTrue(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaStatQueues].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	sq := &StatQueue{

		SQMetrics: map[string]StatMetric{
			utils.MetaASR: &StatASR{
				Metric: &Metric{
					Value: utils.NewDecimal(1, 0),
					Count: 1,
					Events: map[string]*DecimalWithCompress{
						"cgrates.org:TestStatRemExpired_1": {Stat: utils.NewDecimalFromFloat64(1), CompressFactor: 1},
					},
				},
			},
		},
		sqPrfl: &StatQueueProfile{
			Metrics: []*MetricWithFilters{
				{
					MetricID: utils.MetaASR,
				},
			},
		},
	}

	// tests replicate
	dm.SetStatQueue(context.Background(), sq)
}

func TestDMRemoveStatQueueNildb(t *testing.T) {
	var dm *DataManager

	if err := dm.RemoveStatQueue(context.Background(), utils.CGRateSorg, "SQ99"); err != utils.ErrNoDatabaseConn {
		t.Error(err)
	}

}
func TestDMRemoveStatQueueErrDrv(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		RemStatQueueDrvF: func(ctx *context.Context, tenant, id string) (err error) { return utils.ErrNotImplemented },
	}

	if err := dm.RemoveStatQueue(context.Background(), utils.CGRateSorg, "SQ99"); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDMRemoveStatQueueReplicate(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaStatQueues].Replicate = true
	cfg.DataDbCfg().RplConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaStatQueues)}
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.ReplicatorSv1SetStatQueue: func(ctx *context.Context, args, reply interface{}) error {

				return nil
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaStatQueues), utils.ReplicatorSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	sq := &StatQueue{
		Tenant: utils.CGRateSorg,
		ID:     "sqid99",
		SQMetrics: map[string]StatMetric{
			utils.MetaASR: &StatASR{
				Metric: &Metric{
					Value: utils.NewDecimal(1, 0),
					Count: 1,
					Events: map[string]*DecimalWithCompress{
						"cgrates.org:TestStatRemExpired_1": {Stat: utils.NewDecimalFromFloat64(1), CompressFactor: 1},
					},
				},
			},
		},
		sqPrfl: &StatQueueProfile{
			Metrics: []*MetricWithFilters{
				{
					MetricID: utils.MetaASR,
				},
			},
		},
	}

	if err := dm.SetStatQueue(context.Background(), sq); err != nil {
		t.Error(err)
	}

	if rcv, err := dm.GetStatQueue(context.Background(), utils.CGRateSorg, "sqid99", true, false, utils.NonTransactional); err != nil {
		t.Error(err)
	} else if rcv != sq {
		t.Errorf("\nexpected \n<%v> \nreceived \n<%v>\n", sq, rcv)
	}

	//tests replicate
	if err := dm.RemoveStatQueue(context.Background(), utils.CGRateSorg, "sqid99"); err != nil {
		t.Error(err)
	}

	if _, err := dm.GetStatQueue(context.Background(), utils.CGRateSorg, "sqid99", true, false, utils.NonTransactional); err != utils.ErrNotFound {
		t.Error(err)
	}

}

func TestDMGetStatQueueProfileErrNildm(t *testing.T) {
	var dm *DataManager
	if _, err := dm.GetStatQueueProfile(context.Background(), utils.CGRateSorg, "sqp99", false, false, utils.NonTransactional); err != utils.ErrNoDatabaseConn {
		t.Error(err)
	}
}
func TestDMGetStatQueueProfileErrNilCacheRead(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	tntID := utils.ConcatenatedKey(utils.CGRateSorg, "sqp99")

	var setVal interface{}
	if err := Cache.Set(context.Background(), utils.CacheStatQueueProfiles, tntID, setVal, []string{}, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, err := dm.GetStatQueueProfile(context.Background(), utils.CGRateSorg, "sqp99", true, false, utils.NonTransactional); err != utils.ErrNotFound {
		t.Error(err)
	}
}

func TestDMGetStatQueueProfileErrRemote(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaStatQueueProfiles].Remote = true
	cfg.DataDbCfg().RmtConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaStatQueueProfiles)}

	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.ReplicatorSv1GetStatQueueProfile: func(ctx *context.Context, args, reply interface{}) error {

				return nil
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaStatQueueProfiles), utils.ReplicatorSv1, cc)

	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	sqp := &StatQueueProfile{
		Tenant:      "cgrates.org",
		ID:          "sqp99",
		FilterIDs:   []string{"fltr_test"},
		QueueLength: 10,
		TTL:         10 * time.Second,
		Metrics: []*MetricWithFilters{
			{
				MetricID: "*sum#~*req.Usage",
			},
		},
		ThresholdIDs: []string{},
		Stored:       true,
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
		MinItems: 1,
	}

	dm.dataDB = &DataDBMock{
		GetStatQueueProfileDrvF: func(ctx *context.Context, tenant, id string) (sq *StatQueueProfile, err error) {
			return sqp, utils.ErrNotFound
		},
		SetStatQueueProfileDrvF: func(ctx *context.Context, sq *StatQueueProfile) (err error) { return utils.ErrNotImplemented },
	}

	if _, err := dm.GetStatQueueProfile(context.Background(), utils.CGRateSorg, "sqp99", false, false, utils.NonTransactional); err != utils.ErrNotImplemented {
		t.Error(err)
	}
}

func TestDMGetStatQueueProfileErrCacheWrite(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.CacheStatQueueProfiles].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error { return utils.ErrNotImplemented },
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)

	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	sqp := &StatQueueProfile{
		Tenant:      "cgrates.org",
		ID:          "sqp99",
		FilterIDs:   []string{"fltr_test"},
		QueueLength: 10,
		TTL:         10 * time.Second,
		Metrics: []*MetricWithFilters{
			{
				MetricID: "*sum#~*req.Usage",
			},
		},
		ThresholdIDs: []string{},
		Stored:       true,
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
		MinItems: 1,
	}

	dm.dataDB = &DataDBMock{
		GetStatQueueProfileDrvF: func(ctx *context.Context, tenant, id string) (sq *StatQueueProfile, err error) {
			return sqp, utils.ErrNotFound
		},
	}

	Cache = NewCacheS(cfg, dm, cM, nil)

	if _, err := dm.GetStatQueueProfile(context.Background(), utils.CGRateSorg, "sqp99", false, true, utils.NonTransactional); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDMGetStatQueueProfileErr2CacheWrite(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.CacheStatQueueProfiles].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error { return utils.ErrNotImplemented },
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)

	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	sqp := &StatQueueProfile{
		Tenant:      "cgrates.org",
		ID:          "sqp99",
		FilterIDs:   []string{"fltr_test"},
		QueueLength: 10,
		TTL:         10 * time.Second,
		Metrics: []*MetricWithFilters{
			{
				MetricID: "*sum#~*req.Usage",
			},
		},
		ThresholdIDs: []string{},
		Stored:       true,
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
		MinItems: 1,
	}

	if err := dm.DataDB().SetStatQueueProfileDrv(context.Background(), sqp); err != nil {
		t.Error(err)
	}

	Cache = NewCacheS(cfg, dm, cM, nil)

	if _, err := dm.GetStatQueueProfile(context.Background(), utils.CGRateSorg, "sqp99", false, true, utils.NonTransactional); err != utils.ErrNotImplemented {
		t.Error(err)
	}

}

func TestDMGetThresholdProfileSetThErr2(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.MetaThresholds].Replicate = true

	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdDrvF: func(ctx *context.Context, tenant, id string) (*Threshold, error) {
			return &Threshold{}, nil
		},
	}

	Cache = NewCacheS(cfg, dm, connMgr, nil)

	expErr := utils.ErrNotImplemented
	_, err := dm.GetThreshold(context.Background(), utils.CGRateSorg, "TH_1", false, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

func TestDMGetThresholdGetThProflErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	if err := Cache.Set(context.Background(), utils.CacheThresholdProfiles, utils.ConcatenatedKey(utils.CGRateSorg, "TH_1"), nil, []string{}, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	expErr := utils.ErrNotFound
	_, err := dm.GetThresholdProfile(context.Background(), utils.CGRateSorg, "TH_1", true, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

func TestDMGetThresholdProfileDMErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	_, err := dm.GetThresholdProfile(context.Background(), utils.CGRateSorg, "TH_1", true, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

// unfinished, open issue
// func TestGetThresholdProfileSetThErr(t *testing.T) {
// 	tmp := Cache
// 	cfgtmp := config.CgrConfig()
// 	tmpCM := connMgr
// 	defer func() {
// 		Cache = tmp
// 		config.SetCgrConfig(cfgtmp)
// 		connMgr = tmpCM
// 	}()
// 	Cache.Clear(nil)

// 	cfg := config.NewDefaultCGRConfig()
// 	cfg.DataDbCfg().Items[utils.MetaThresholdProfiles].Remote = true
// 	cfg.DataDbCfg().RmtConns = []string{utils.ConcatenatedKey(utils.MetaInternal,
// 		utils.MetaThresholds)}
// 	config.SetCgrConfig(cfg)
// 	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

// 	th := &Threshold{
// 		Tenant: "cgrates.org",
// 		ID:     "TH_1",
// 		Hits:   0,
// 	}

// 	cc := &ccMock{
// 		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
// 			utils.ReplicatorSv1GetThresholdProfile: func(ctx *context.Context, args, reply interface{}) error {
// 				rplCast, canCast := reply.(*Threshold)
// 				if !canCast {
// 					t.Errorf("Wrong argument type : %T", reply)
// 					return nil
// 				}
// 				*rplCast = *th
// 				return nil
// 			},
// 		},
// 	}

// 	rpcInternal := make(chan birpc.ClientConnector, 1)
// 	rpcInternal <- cc
// 	cM := NewConnManager(cfg)
// 	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaThresholds), utils.ReplicatorSv1, rpcInternal)
// 	dm := NewDataManager(data, cfg.CacheCfg(), cM)

// 	_, err := dm.GetThresholdProfile(context.Background(), utils.CGRateSorg, "TH_1", false, false, utils.NonTransactional)
// 	if  err != utils.ErrNotFound {
// 		t.Error(err)
// 	}
// }

func TestDMGetThresholdProfileSetThPrfErr(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.MetaThresholdProfiles].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	expErr := utils.ErrNotImplemented
	_, err := dm.GetThresholdProfile(context.Background(), utils.CGRateSorg, "TH_1", false, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

func TestDMGetThresholdProfileSetThPrfErr2(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	cfg.CacheCfg().Partitions[utils.MetaThresholdProfiles].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	dm.dataDB = &DataDBMock{
		GetThresholdProfileDrvF: func(ctx *context.Context, tenant, id string) (tp *ThresholdProfile, err error) {
			return &ThresholdProfile{}, nil
		},
	}
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	expErr := utils.ErrNotImplemented
	_, err := dm.GetThresholdProfile(context.Background(), utils.CGRateSorg, "TH_1", false, true, utils.NonTransactional)
	if err != expErr {
		t.Errorf("Expected <%v> , Received <%v>", expErr, err)
	}
}

func TestDMCacheDataFromDBResourceProfilesPrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	rp := &ResourceProfile{
		Tenant:    "cgrates.org",
		ID:        "RL2",
		FilterIDs: []string{"fltr_test"},
		Weights: utils.DynamicWeights{
			{
				Weight: 100,
			}},
		Limit:             2,
		ThresholdIDs:      []string{"TEST_ACTIONS"},
		Blocker:           false,
		UsageTTL:          time.Millisecond,
		AllocationMessage: "ALLOC",
	}

	if err := dm.SetResourceProfile(context.Background(), rp, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheResourceProfiles, "cgrates.org:RL2"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ResourceProfilesPrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheResourceProfiles, "cgrates.org:RL2"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, rp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", rp, rcv)
	}

}

func TestDMCacheDataFromDBResourcesPrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	rs := &Resource{
		Tenant: "cgrates.org",
		ID:     "ResGroup2",
		Usages: map[string]*ResourceUsage{
			"RU1": {
				Tenant: "cgrates.org",
				ID:     "RU1",
				Units:  9,
			},
		},
		tUsage: utils.Float64Pointer(9),
	}

	if err := dm.SetResource(context.Background(), rs); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheResources, "cgrates.org:ResGroup2"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ResourcesPrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheResources, "cgrates.org:ResGroup2"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, rs) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", rs, rcv)
	}

}

func TestDMCacheDataFromDBStatQueueProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	sQP := &StatQueueProfile{
		Tenant:      "cgrates.org",
		ID:          "StatQueueProfile3",
		FilterIDs:   []string{"fltr_test"},
		QueueLength: 10,
		TTL:         10 * time.Second,
		Metrics: []*MetricWithFilters{
			{
				MetricID: "*sum#~*req.Usage",
			},
		},
		ThresholdIDs: []string{},
		Stored:       true,
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
		MinItems: 1,
	}

	if err := dm.SetStatQueueProfile(context.Background(), sQP, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheStatQueueProfiles, "cgrates.org:StatQueueProfile3"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.StatQueueProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheStatQueueProfiles, "cgrates.org:StatQueueProfile3"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, sQP) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", sQP, rcv)
	}

}

func TestDMCacheDataFromDBStatQueuePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	sq := &StatQueue{
		Tenant: "cgrates.org",
		ID:     "SQ1",
		SQItems: []SQItem{
			{
				EventID: "SqProcessEvent",
			},
		},
		SQMetrics: make(map[string]StatMetric),
	}

	if err := dm.SetStatQueue(context.Background(), sq); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheStatQueues, "cgrates.org:SQ1"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.StatQueuePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheStatQueues, "cgrates.org:SQ1"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, sq) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", sq, rcv)
	}

}

func TestDMCacheDataFromDBThresholdProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	thP := &ThresholdProfile{
		Tenant:           "cgrates.org",
		ID:               "THD_2",
		FilterIDs:        []string{"*string:~*req.Account:1001"},
		ActionProfileIDs: []string{"actPrfID"},
		MaxHits:          7,
		MinHits:          0,
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
		Async: true,
	}

	if err := dm.SetThresholdProfile(context.Background(), thP, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheThresholdProfiles, "cgrates.org:THD_2"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ThresholdProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheThresholdProfiles, "cgrates.org:THD_2"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, thP) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", thP, rcv)
	}

}

func TestDMCacheDataFromDBThresholdPrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	th := &Threshold{
		Tenant: "cgrates.org",
		ID:     "TH_3",
		Hits:   0,
	}

	if err := dm.SetThreshold(context.Background(), th); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheThresholds, "cgrates.org:TH_3"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ThresholdPrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheThresholds, "cgrates.org:TH_3"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, th) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", th, rcv)
	}

}

func TestDMCacheDataFromDBFilterPrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	fltr := &Filter{
		Tenant: cfg.GeneralCfg().DefaultTenant,
		ID:     "FLTR_CP_2",
		Rules: []*FilterRule{
			{
				Type:    utils.MetaString,
				Element: "~*req.Charger",
				Values:  []string{"ChargerProfile2"},
			},
		},
	}

	if err := dm.SetFilter(context.Background(), fltr, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheFilters, "cgrates.org:FLTR_CP_2"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.FilterPrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheFilters, "cgrates.org:FLTR_CP_2"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, fltr) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", fltr, rcv)
	}

}

func TestDMCacheDataFromDBRouteProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	routeProf := &RouteProfile{

		Tenant:            "cgrates.org",
		ID:                "RP_1",
		FilterIDs:         []string{"fltr_test"},
		Weights:           utils.DynamicWeights{{}},
		Sorting:           utils.MetaQOS,
		SortingParameters: []string{"param"},
		Routes: []*Route{{
			ID:             "RT1",
			FilterIDs:      []string{"fltr1"},
			AccountIDs:     []string{"acc1"},
			RateProfileIDs: []string{"rp1"},
			ResourceIDs:    []string{"res1"},
			StatIDs:        []string{"stat1"},
			Weights:        utils.DynamicWeights{{}},
			Blockers: utils.DynamicBlockers{
				{
					Blocker: true,
				},
			},
			RouteParameters: "params",
		}},
	}

	if err := dm.SetRouteProfile(context.Background(), routeProf, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheRouteProfiles, "cgrates.org:RP_1"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.RouteProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheRouteProfiles, "cgrates.org:RP_1"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, routeProf) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", routeProf, rcv)
	}

}

func TestDMCacheDataFromDBChargerProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	cpp := &ChargerProfile{
		Tenant:       "cgrates.org",
		ID:           "CPP_1",
		FilterIDs:    []string{"FLTR_CP_1"},
		RunID:        "TestRunID",
		AttributeIDs: []string{"*none"},
		Weights: utils.DynamicWeights{
			{
				Weight: 20,
			},
		},
	}

	if err := dm.SetChargerProfile(context.Background(), cpp, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheChargerProfiles, "cgrates.org:CPP_1"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ChargerProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheChargerProfiles, "cgrates.org:CPP_1"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, cpp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", cpp, rcv)
	}

}

func TestDMCacheDataFromDBDispatcherProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dpp := &DispatcherProfile{

		Tenant:         "cgrates.org",
		ID:             "ID",
		FilterIDs:      []string{"fltr1"},
		Weight:         65,
		Strategy:       utils.MetaLoad,
		StrategyParams: map[string]interface{}{"k": "v"},
		Hosts: DispatcherHostProfiles{
			{
				ID:        "C3",
				FilterIDs: []string{"fltr2"},
				Weight:    20,
				Params:    map[string]interface{}{},
				Blocker:   true,
			},
		},
	}

	if err := dm.SetDispatcherProfile(context.Background(), dpp, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheDispatcherProfiles, "cgrates.org:ID"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.DispatcherProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheDispatcherProfiles, "cgrates.org:ID"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, dpp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", dpp, rcv)
	}

}

func TestDMCacheDataFromDBDispatcherHostPrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dph := &DispatcherHost{
		Tenant: "cgrates.org",
		RemoteHost: &config.RemoteHost{
			ID:                   "ID",
			Address:              "127.0.0.1",
			Transport:            utils.MetaJSON,
			ConnectAttempts:      1,
			Reconnects:           1,
			MaxReconnectInterval: 1,
			ConnectTimeout:       time.Nanosecond,
			ReplyTimeout:         time.Nanosecond,
			TLS:                  true,
			ClientKey:            "key",
			ClientCertificate:    "ce",
			CaCertificate:        "ca",
		},
	}

	if err := dm.SetDispatcherHost(context.Background(), dph); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheDispatcherHosts, "cgrates.org:ID"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.DispatcherHostPrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheDispatcherHosts, "cgrates.org:ID"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, dph) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", dph, rcv)
	}

}

func TestDMCacheDataFromDBRateProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	rpp := &utils.RateProfile{
		ID:        "RP1",
		Tenant:    "cgrates.org",
		FilterIDs: []string{"*string:~*req.Destination:1234"},
		Rates: map[string]*utils.Rate{
			"RT1": {
				ID: "RT1",
				IntervalRates: []*utils.IntervalRate{
					{
						IntervalStart: utils.NewDecimal(0, 0),
						RecurrentFee:  utils.NewDecimal(1, 2),
						Unit:          utils.NewDecimal(int64(time.Second), 0),
						Increment:     utils.NewDecimal(int64(time.Second), 0),
					},
				},
			},
		},
	}

	if err := dm.SetRateProfile(context.Background(), rpp, false, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheRateProfiles, "cgrates.org:RP1"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.RateProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheRateProfiles, "cgrates.org:RP1"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, rpp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", rpp, rcv)
	}

}

func TestDMCacheDataFromDBActionProfilePrefix(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	ap := &ActionProfile{

		Tenant:    "cgrates.org",
		ID:        "ID",
		FilterIDs: []string{"fltr1"},
		Weights: utils.DynamicWeights{
			{
				Weight: 65,
			},
		},
		Schedule: "* * * * *",
		Targets:  map[string]utils.StringSet{utils.MetaAccounts: {"1001": {}}},
		Actions:  []*APAction{{}},
	}

	if err := dm.SetActionProfile(context.Background(), ap, false); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheActionProfiles, "cgrates.org:ID"); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ActionProfilePrefix, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	if rcv, ok := Cache.Get(utils.CacheActionProfiles, "cgrates.org:ID"); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, ap) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", ap, rcv)
	}

}

func TestDMCacheDataFromDBAttributeFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheAttributeFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.AttributeFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheAttributeFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBResourceFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheResourceFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheResourceFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ResourceFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheResourceFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBStatFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheStatFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheStatFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.StatFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheStatFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBThresholdFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheThresholdFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheThresholdFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ThresholdFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheThresholdFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBRouteFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheRouteFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheRouteFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.RouteFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheRouteFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBChargerFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheChargerFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheChargerFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ChargerFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheChargerFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBDispatcherFilterIndexes(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheDispatcherFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheDispatcherFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.DispatcherFilterIndexes, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheDispatcherFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBRateProfilesFilterIndexPrfx(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheRateProfilesFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheRateProfilesFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.RateProfilesFilterIndexPrfx, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheRateProfilesFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBRateFilterIndexPrfx(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheRateFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheRateFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.RateFilterIndexPrfx, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheRateFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBActionProfilesFilterIndexPrfx(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheActionProfilesFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheActionProfilesFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.ActionProfilesFilterIndexPrfx, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheActionProfilesFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBFilterIndexPrfx(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheReverseFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, ok := Cache.Get(utils.CacheReverseFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); ok {
		t.Error("expected ok to be false")
	}

	if err := dm.CacheDataFromDB(context.Background(), utils.FilterIndexPrfx, []string{utils.MetaAny}, false); err != nil {
		t.Error(err)
	}

	exp := utils.StringSet{"ATTR1": {}, "ATTR2": {}}

	if rcv, ok := Cache.Get(utils.CacheReverseFilterIndexes, utils.ConcatenatedKey("cgrates.org", "*string:*req.Account:1002")); !ok {
		t.Error("expected ok to be true")
	} else if !reflect.DeepEqual(rcv, exp) {
		t.Errorf("\nExpected <%+v>, \nReceived <%+v>", exp, rcv)
	}

}

func TestDMCacheDataFromDBAttributeFilterIndexErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.AttributeFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBResourceFilterIndexErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.ResourceFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBStatFilterIndexErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.StatFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBThresholdFilterIndexesErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.ThresholdFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBRouteFilterIndexesErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.RouteFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBChargerFilterIndexesErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.ChargerFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBDispatcherFilterIndexesErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.DispatcherFilterIndexes, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBRateProfilesFilterIndexPrfxErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.RateProfilesFilterIndexPrfx, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBRateFilterIndexPrfxErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.RateFilterIndexPrfx, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMCacheDataFromDBActionProfilesFilterIndexPrfxErr(t *testing.T) {
	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	errExp := "WRONG_IDX_KEY_FORMAT<tntCtx:*prefix:~*accounts>"
	if err := dm.CacheDataFromDB(context.Background(), utils.ActionProfilesFilterIndexPrfx, []string{"tntCtx:*prefix:~*accounts"}, false); errExp != err.Error() {
		t.Errorf("Expected %v\n but received %v", errExp, err)
	}
}

func TestDMGetAccountNil(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	if _, err := dm.GetAccount(context.Background(), utils.CGRateSorg, "1002"); err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}
}

func TestDMGetAccountReplicate(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaAccounts].Remote = true
	cfg.DataDbCfg().RmtConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaAccounts)}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := &ccMock{
		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.ReplicatorSv1GetAccount: func(ctx *context.Context, args, reply interface{}) error {
				return nil
			},
		},
	}
	rpcInternal := make(chan birpc.ClientConnector, 1)
	rpcInternal <- cc

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaAccounts), utils.ReplicatorSv1, rpcInternal)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	ap := &utils.Account{
		Tenant: "cgrates.org",
		ID:     "1002",
		Weights: []*utils.DynamicWeight{
			{
				Weight: 0,
			},
		},
		FilterIDs: []string{"*stirng:~*req.Account:1001"},
		Balances: map[string]*utils.Balance{
			"AbstractBalance1": {
				ID: "AbstractBalance1",
				Weights: utils.DynamicWeights{
					{
						Weight: 25,
					},
				},
				Type:  utils.MetaAbstract,
				Units: utils.NewDecimal(int64(40*time.Second), 0),
				CostIncrements: []*utils.CostIncrement{
					{
						Increment:    utils.NewDecimal(int64(time.Second), 0),
						FixedFee:     utils.NewDecimal(0, 0),
						RecurrentFee: utils.NewDecimal(0, 0),
					},
				},
			},
		},
		Blockers: utils.DynamicBlockers{
			{
				FilterIDs: []string{"fltrID"},
				Blocker:   true,
			},
		},
		Opts:         make(map[string]interface{}),
		ThresholdIDs: []string{utils.MetaNone},
	}

	dm.dataDB = &DataDBMock{
		GetAccountDrvF: func(ctx *context.Context, str1, str2 string) (*utils.Account, error) {
			return ap, utils.ErrNotFound
		},
		SetAccountDrvF: func(ctx *context.Context, profile *utils.Account) error {
			return nil
		},
	}

	// tests replicate
	if rcv, err := dm.GetAccount(context.Background(), utils.CGRateSorg, "1002"); err != nil {
		t.Error(err, rcv)
	} else if rcv != ap {
		t.Errorf("Expected <%v>, received <%v>", ap, rcv)
	}
}

func TestDMGetRateProfileRatesNil(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	if _, _, err := dm.GetRateProfileRates(context.Background(), &utils.ArgsSubItemIDs{}, false); err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}
}

func TestDMGetRateProfileRatesOK(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	rps := &utils.RateProfile{
		ID:        "test_ID1",
		Tenant:    "cgrates.org",
		FilterIDs: []string{"*string:~*req.Destination:1234"},
		Rates: map[string]*utils.Rate{
			"RT1": {
				ID: "RT1",
				IntervalRates: []*utils.IntervalRate{
					{
						IntervalStart: utils.NewDecimal(0, 0),
						RecurrentFee:  utils.NewDecimal(1, 2),
						Unit:          utils.NewDecimal(int64(time.Second), 0),
						Increment:     utils.NewDecimal(int64(time.Second), 0),
					},
				},
			},
		},
	}

	dm.DataDB().SetRateProfileDrv(context.Background(), rps, true)

	args := &utils.ArgsSubItemIDs{
		Tenant:      "cgrates.org",
		ProfileID:   "test_ID1",
		ItemsPrefix: "RT1",
	}

	exp := []*utils.Rate{
		{
			ID: "RT1",
			IntervalRates: []*utils.IntervalRate{
				{
					IntervalStart: utils.NewDecimal(0, 0),
					RecurrentFee:  utils.NewDecimal(1, 2),
					Unit:          utils.NewDecimal(int64(time.Second), 0),
					Increment:     utils.NewDecimal(int64(time.Second), 0),
				},
			},
		},
	}

	if _, rcv, err := dm.GetRateProfileRates(context.Background(), args, false); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(utils.ToJSON(exp), utils.ToJSON(rcv)) {
		t.Errorf("Expected \n<%+v>,\nreceived \n<%+v>", exp, rcv)
	}
}

func TestDMSetLoadIDsNil(t *testing.T) {

	var dm *DataManager

	expErr := utils.ErrNoDatabaseConn
	if err := dm.SetLoadIDs(context.Background(), map[string]int64{}); err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDMSetLoadIDsDrvErr(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		SetLoadIDsDrvF: func(ctx *context.Context, loadIDs map[string]int64) error { return utils.ErrNotImplemented },
	}

	itmLIDs := map[string]int64{
		"ID_1": 21,
	}

	expErr := utils.ErrNotImplemented
	if err := dm.SetLoadIDs(context.Background(), itmLIDs); err != expErr {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDMSetLoadIDsReplicate(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaLoadIDs].Replicate = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	itmLIDs := map[string]int64{
		"ID_1": 21,
	}

	// tests Replicate
	dm.SetLoadIDs(context.Background(), itmLIDs)

}

func TestDMCheckFiltersErrBadReference(t *testing.T) {

	var dm *DataManager

	expErr := "broken reference to filter: <*string:~*req.Account>"
	if err := dm.checkFilters(context.Background(), utils.CGRateSorg, []string{"*string:~*req.Account"}); expErr != err.Error() {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDMCheckFiltersErrBadPath(t *testing.T) {

	var dm *DataManager

	expErr := `Path is missing  for filter <{"Tenant":"cgrates.org","ID":"*string:~missing path:chp1","Rules":[{"Type":"*string","Element":"~missing path","Values":["chp1"]}]}>`
	if err := dm.checkFilters(context.Background(), utils.CGRateSorg, []string{"*string:~missing path:chp1"}); expErr != err.Error() {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDMCheckFiltersErrBrokenReferenceCache(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	var val interface{}
	if err := Cache.Set(context.Background(), utils.CacheFilters, utils.ConcatenatedKey(utils.CGRateSorg, "fltr1"), val, []string{}, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	expErr := `broken reference to filter: <fltr1>`
	if err := dm.checkFilters(context.Background(), utils.CGRateSorg, []string{"fltr1"}); expErr != err.Error() {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestDMCheckFiltersErrCall(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.MetaFilters].Remote = true
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		HasDataDrvF: func(ctx *context.Context, category, subject, tenant string) (bool, error) {
			return false, utils.ErrNotFound
		},
	}

	expErr := `broken reference to filter: <fltr1>`
	if err := dm.checkFilters(context.Background(), utils.CGRateSorg, []string{"fltr1"}); expErr != err.Error() {
		t.Errorf("Expected error <%v>, Received error <%v>", expErr, err)
	}

}

func TestGetAPIBanErrSingleCacheWrite(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.APIBanCfg().Keys = []string{"testKey"}
	cfg.CacheCfg().Partitions[utils.MetaAPIBan].Replicate = true
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	config.SetCgrConfig(cfg)

	var counter int
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		responses := map[string]struct {
			code int
			body []byte
		}{
			"/testKey/check/1.2.3.251": {code: http.StatusOK, body: []byte(`{"ipaddress":["1.2.3.251"], "ID":"987654321"}`)},
		}
		if val, has := responses[r.URL.EscapedPath()]; has {
			w.WriteHeader(val.code)
			if val.body != nil {
				w.Write(val.body)
			}
			return
		}
		counter++
		w.WriteHeader(http.StatusOK)
		if counter < 2 {
			_, _ = w.Write([]byte(`{"ipaddress": ["1.2.3.251", "ID": "100"}`))
		} else {
			_, _ = w.Write([]byte(`{"ID": "none"}`))
			counter = 0
		}
	}))
	defer testServer.Close()
	baningo.RootURL = testServer.URL + "/"

	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	if _, err := GetAPIBan(context.Background(), "1.2.3.251", []string{"testKey"}, true, false, true); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, Received error <%v>", utils.ErrNotImplemented, err)
	}
}

func TestGetAPIBanErrMultipleCacheWrite(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.APIBanCfg().Keys = []string{"testKey"}
	cfg.CacheCfg().Partitions[utils.MetaAPIBan].Replicate = true
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	config.SetCgrConfig(cfg)

	var counter int
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		responses := map[string]struct {
			code int
			body []byte
		}{
			"/testKey/check/1.2.3.251": {code: http.StatusOK, body: []byte(`{"ipaddress":["1.2.3.251"], "ID":"987654321"}`)},
		}
		if val, has := responses[r.URL.EscapedPath()]; has {
			w.WriteHeader(val.code)
			if val.body != nil {
				w.Write(val.body)
			}
			return
		}
		counter++
		w.WriteHeader(http.StatusOK)
		if counter < 2 {
			_, _ = w.Write([]byte(`{"ipaddress": ["1.2.3.251", "1.2.3.252"], "ID": "100"}`))
		} else {
			_, _ = w.Write([]byte(`{"ID": "none"}`))
			counter = 0
		}
	}))
	defer testServer.Close()
	baningo.RootURL = testServer.URL + "/"

	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	if _, err := GetAPIBan(context.Background(), "1.2.3.251", []string{"testKey"}, false, false, true); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, Received error <%v>", utils.ErrNotImplemented, err)
	}
}

func TestGetAPIBanErrNoBanCacheSet(t *testing.T) {
	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.APIBanCfg().Keys = []string{"testKey"}
	cfg.CacheCfg().Partitions[utils.MetaAPIBan].Replicate = true
	cfg.CacheCfg().ReplicationConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	config.SetCgrConfig(cfg)

	var counter int
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		responses := map[string]struct {
			code int
			body []byte
		}{
			"/testKey/check/1.2.3.251": {code: http.StatusOK, body: []byte(`{"ipaddress":["1.2.3.251"], "ID":"987654321"}`)},
			"/testKey/check/1.2.3.254": {code: http.StatusBadRequest, body: []byte(`{"ipaddress":["not blocked"], "ID":"none"}`)},
		}
		if val, has := responses[r.URL.EscapedPath()]; has {
			w.WriteHeader(val.code)
			if val.body != nil {
				w.Write(val.body)
			}
			return
		}
		counter++
		w.WriteHeader(http.StatusOK)
		if counter < 2 {
			_, _ = w.Write([]byte(`{"ipaddress": ["1.2.3.251", "1.2.3.252"], "ID": "100"}`))
		} else {
			_, _ = w.Write([]byte(`{"ID": "none"}`))
			counter = 0
		}
	}))
	defer testServer.Close()
	baningo.RootURL = testServer.URL + "/"

	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	if _, err := GetAPIBan(context.Background(), "1.2.3.254", []string{}, false, false, true); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, Received error <%v>", utils.ErrNotImplemented, err)
	}
}

func TestDMRemoveIndexesErrNilDm(t *testing.T) {

	var dm *DataManager

	if err := dm.RemoveIndexes(context.Background(), "indxItmtype", "cgrates.org", "indxkey"); err != utils.ErrNoDatabaseConn {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNoDatabaseConn, err)
	}

}

func TestDMRemoveIndexesErrDrv(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		RemoveIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey string) error {
			return utils.ErrNotImplemented
		},
	}

	if err := dm.RemoveIndexes(context.Background(), "indxItmtype", "cgrates.org", "indxkey"); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotImplemented, err)
	}

}

func TestDMRemoveIndexesReplicate(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.CacheAttributeFilterIndexes].Replicate = true
	config.SetCgrConfig(cfg)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.dataDB.SetIndexesDrv(context.Background(), utils.CacheAttributeFilterIndexes, "cgrates.org", indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	_, err := dm.dataDB.GetIndexesDrv(context.Background(), utils.CacheAttributeFilterIndexes, "cgrates.org", utils.EmptyString, utils.NonTransactional)
	if err != nil {
		t.Error(err)
	}

	if err := dm.RemoveIndexes(context.Background(), utils.CacheAttributeFilterIndexes, "cgrates.org", utils.EmptyString); err != nil {
		t.Errorf("Expected error <%v>, received error <%v>", nil, err)
	}

	_, err = dm.dataDB.GetIndexesDrv(context.Background(), utils.CacheAttributeFilterIndexes, "cgrates.org", utils.EmptyString, utils.NonTransactional)
	if err != utils.ErrNotFound {
		t.Error(err)
	}

}

func TestDMSetIndexesErrNilDm(t *testing.T) {

	var dm *DataManager

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, indexes, true, utils.NonTransactional); err != utils.ErrNoDatabaseConn {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNoDatabaseConn, err)
	}

}

func TestDMSetIndexesReplicate(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.CacheAttributeFilterIndexes].Replicate = true
	cfg.DataDbCfg().RplConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	config.SetCgrConfig(cfg)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.ReplicatorSv1SetIndexes: func(ctx *context.Context, args, reply interface{}) error {
				return nil

			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.ReplicatorSv1, cc)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, indexes, true, utils.NonTransactional); err != nil {
		t.Errorf("Expected error <%v>, received error <%v>", nil, err)
	}
	if _, err := dm.GetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, utils.EmptyString, utils.NonTransactional, true, false); err != nil {
		t.Error(err)
	}

}

func TestDMGetIndexesErrSetIdxDrv(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.DataDbCfg().Items[utils.CacheAttributeFilterIndexes].Remote = true
	cfg.DataDbCfg().RmtConns = []string{utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator)}
	config.SetCgrConfig(cfg)

	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.ReplicatorSv1GetIndexes: func(ctx *context.Context, args, reply interface{}) error {
				return nil

			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.ReplicatorSv1, cc)

	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	indexes2 := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	dm.dataDB = &DataDBMock{
		GetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx, idxKey, transactionID string) (indexes map[string]utils.StringSet, err error) {
			return indexes2, utils.ErrNotFound
		},

		SetIndexesDrvF: func(ctx *context.Context, idxItmType, tntCtx string, indexes map[string]utils.StringSet, commit bool, transactionID string) (err error) {
			return utils.ErrNotFound
		},
	}

	if _, err := dm.GetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, "idxKey", utils.NonTransactional, false, true); err != utils.ErrNotFound {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotFound, err)
	}

}

func TestDMGetIndexesErrCacheSet(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().Partitions[utils.CacheAttributeFilterIndexes].Replicate = true
	cfg.CacheCfg().ReplicationConns = []string{utils.MetaInternal}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	if _, err := dm.GetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, "idxKey", utils.NonTransactional, false, true); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotImplemented, err)
	}

}

func TestDMGetIndexesErrCacheWriteSet(t *testing.T) {

	tmp := Cache
	cfgtmp := config.CgrConfig()
	defer func() {
		Cache = tmp
		config.SetCgrConfig(cfgtmp)
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	cfg.CacheCfg().Partitions[utils.CacheAttributeFilterIndexes].Replicate = true
	cfg.CacheCfg().ReplicationConns = []string{utils.MetaInternal}
	config.SetCgrConfig(cfg)
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cc := make(chan birpc.ClientConnector, 1)
	cc <- &ccMock{

		calls: map[string]func(ctx *context.Context, args interface{}, reply interface{}) error{
			utils.CacheSv1ReplicateSet: func(ctx *context.Context, args, reply interface{}) error {

				return utils.ErrNotImplemented
			},
		},
	}

	cM := NewConnManager(cfg)
	cM.AddInternalConn(utils.ConcatenatedKey(utils.MetaInternal, utils.MetaReplicator), utils.CacheSv1, cc)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)
	Cache = NewCacheS(cfg, dm, connMgr, nil)

	indexes := map[string]utils.StringSet{"*string:*req.Account:1002": {"ATTR1": {}, "ATTR2": {}}}

	if err := dm.SetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, indexes, true, utils.NonTransactional); err != nil {
		t.Error(err)
	}

	if _, err := dm.GetIndexes(context.Background(), utils.CacheAttributeFilterIndexes, utils.CGRateSorg, utils.EmptyString, utils.NonTransactional, false, true); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotImplemented, err)
	}

}

func TestDMRemoveActionProfileErrNilDM(t *testing.T) {

	var dm *DataManager

	if err := dm.RemoveActionProfile(context.Background(), "cgrates.org", "AP1", false); err != utils.ErrNoDatabaseConn {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNoDatabaseConn, err)
	}

}

func TestDMRemoveActionProfileErrGetActionProf(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	dm.dataDB = &DataDBMock{
		GetActionProfileDrvF: func(ctx *context.Context, tenant, ID string) (*ActionProfile, error) {
			return nil, utils.ErrNotImplemented
		},
	}

	if err := dm.RemoveActionProfile(context.Background(), "cgrates.org", "AP1", false); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotImplemented, err)
	}

}

func TestDMRemoveActionProfileErrRemvProfDrv(t *testing.T) {

	tmp := Cache
	defer func() {
		Cache = tmp
	}()
	Cache.Clear(nil)

	cfg := config.NewDefaultCGRConfig()
	data := NewInternalDB(nil, nil, cfg.DataDbCfg().Items)
	cM := NewConnManager(cfg)
	dm := NewDataManager(data, cfg.CacheCfg(), cM)

	ap := &ActionProfile{

		Tenant:    "cgrates.org",
		ID:        "AP1",
		FilterIDs: []string{"fltr1"},
		Weights: utils.DynamicWeights{
			{
				Weight: 65,
			},
		},
		Schedule: "* * * * *",
		Targets:  map[string]utils.StringSet{utils.MetaAccounts: {"1001": {}}},
		Actions:  []*APAction{{}},
	}

	dm.dataDB = &DataDBMock{
		GetActionProfileDrvF: func(ctx *context.Context, tenant, ID string) (*ActionProfile, error) {
			return ap, nil
		},
		RemoveActionProfileDrvF: func(ctx *context.Context, tenant, ID string) error {
			return utils.ErrNotImplemented
		},
	}

	if err := dm.RemoveActionProfile(context.Background(), "cgrates.org", "AP1", false); err != utils.ErrNotImplemented {
		t.Errorf("Expected error <%v>, received error <%v>", utils.ErrNotImplemented, err)
	}

}