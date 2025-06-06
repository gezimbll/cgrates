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

package v1

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cgrates/birpc/context"
	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/ees"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/guardian"
	"github.com/cgrates/cgrates/scheduler"
	"github.com/cgrates/cgrates/utils"
)

// SchedulerGeter used to avoid ciclic dependency
type SchedulerGeter interface {
	GetScheduler() *scheduler.Scheduler
}

type APIerSv1 struct {
	StorDb           engine.LoadStorage // we should consider keeping only one of StorDB type
	CdrDb            engine.CdrStorage
	DataManager      *engine.DataManager
	Config           *config.CGRConfig
	Responder        *engine.Responder
	SchedulerService SchedulerGeter  // Need to have them capitalize so we can export in V2
	FilterS          *engine.FilterS //Used for CDR Exporter
	ConnMgr          *engine.ConnManager

	StorDBChan    chan engine.StorDB
	ResponderChan chan *engine.Responder
}

func (apierSv1 *APIerSv1) GetDestination(ctx *context.Context, dstId *string, reply *engine.Destination) error {
	if dst, err := apierSv1.DataManager.GetDestination(*dstId, true, true, utils.NonTransactional); err != nil {
		return utils.ErrNotFound
	} else {
		*reply = *dst
	}
	return nil
}

type AttrRemoveDestination struct {
	DestinationIDs []string
	Prefixes       []string
}

func (apierSv1 *APIerSv1) RemoveDestination(ctx *context.Context, attr *AttrRemoveDestination, reply *string) (err error) {
	for _, dstID := range attr.DestinationIDs {
		var oldDst *engine.Destination
		if oldDst, err = apierSv1.DataManager.GetDestination(dstID, true, true,
			utils.NonTransactional); err != nil && err != utils.ErrNotFound {
			return
		}
		if len(attr.Prefixes) != 0 {
			newDst := &engine.Destination{
				Id:       dstID,
				Prefixes: make([]string, 0, len(oldDst.Prefixes)),
			}
			toRemove := utils.NewStringSet(attr.Prefixes)
			for _, prfx := range oldDst.Prefixes {
				if !toRemove.Has(prfx) {
					newDst.Prefixes = append(newDst.Prefixes, prfx)
				}
			}
			if len(newDst.Prefixes) != 0 { // only update the current destination
				if err = apierSv1.DataManager.SetDestination(newDst, utils.NonTransactional); err != nil {
					return
				}
				if err = apierSv1.DataManager.UpdateReverseDestination(oldDst, newDst, utils.NonTransactional); err != nil {
					return
				}
				if err = apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
					utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
						ReverseDestinationIDs: oldDst.Prefixes,
						DestinationIDs:        []string{dstID},
					}, reply); err != nil {
					return
				}
				continue
			}
		}
		if err = apierSv1.DataManager.RemoveDestination(dstID, utils.NonTransactional); err != nil {
			return
		}
		if err = apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
			utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
				ReverseDestinationIDs: oldDst.Prefixes,
				DestinationIDs:        []string{dstID},
			}, reply); err != nil {
			return
		}
	}
	*reply = utils.OK
	return
}

// GetReverseDestination retrieves revese destination list for a prefix
func (apierSv1 *APIerSv1) GetReverseDestination(ctx *context.Context, prefix *string, reply *[]string) (err error) {
	if *prefix == utils.EmptyString {
		return utils.NewErrMandatoryIeMissing("prefix")
	}
	var revLst []string
	if revLst, err = apierSv1.DataManager.GetReverseDestination(*prefix, true, true, utils.NonTransactional); err != nil {
		return
	}
	*reply = revLst
	return
}

// ComputeReverseDestinations will rebuild complete reverse destinations data
func (apierSv1 *APIerSv1) ComputeReverseDestinations(ctx *context.Context, ignr *string, reply *string) (err error) {
	if err = apierSv1.DataManager.RebuildReverseForPrefix(utils.ReverseDestinationPrefix); err != nil {
		return
	}
	*reply = utils.OK
	return
}

// ComputeAccountActionPlans will rebuild complete reverse accountActions data
func (apierSv1 *APIerSv1) ComputeAccountActionPlans(ctx *context.Context, tnt *utils.TenantWithAPIOpts, reply *string) (err error) {
	if err = apierSv1.DataManager.RebuildReverseForPrefix(utils.AccountActionPlansPrefix); err != nil {
		return
	}
	return apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1Clear, &utils.AttrCacheIDsWithAPIOpts{
			Tenant:   tnt.Tenant,
			CacheIDs: []string{utils.CacheAccountActionPlans},
			APIOpts:  tnt.APIOpts,
		}, reply)
}

func (apierSv1 *APIerSv1) GetSharedGroup(ctx *context.Context, sgId *string, reply *engine.SharedGroup) error {
	if sg, err := apierSv1.DataManager.GetSharedGroup(*sgId, false, utils.NonTransactional); err != nil && err != utils.ErrNotFound { // Not found is not an error here
		return err
	} else {
		if sg != nil {
			*reply = *sg
		}
	}
	return nil
}

func (apierSv1 *APIerSv1) SetDestination(ctx *context.Context, attrs *utils.AttrSetDestination, reply *string) (err error) {
	if missing := utils.MissingStructFields(attrs, []string{"Id", "Prefixes"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	dest := &engine.Destination{Id: attrs.Id, Prefixes: attrs.Prefixes}
	var oldDest *engine.Destination
	if oldDest, err = apierSv1.DataManager.GetDestination(attrs.Id, true, true, utils.NonTransactional); err != nil {
		if err != utils.ErrNotFound {
			return utils.NewErrServerError(err)
		}
	} else if !attrs.Overwrite {
		return utils.ErrExists
	}
	if err := apierSv1.DataManager.SetDestination(dest, utils.NonTransactional); err != nil {
		return utils.NewErrServerError(err)
	}
	if err = apierSv1.DataManager.UpdateReverseDestination(oldDest, dest, utils.NonTransactional); err != nil {
		return
	}
	if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
			ReverseDestinationIDs: dest.Prefixes,
			DestinationIDs:        []string{attrs.Id},
		}, reply); err != nil {
		return err
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) GetRatingPlan(ctx *context.Context, rplnId *string, reply *engine.RatingPlan) error {
	rpln, err := apierSv1.DataManager.GetRatingPlan(*rplnId, false, utils.NonTransactional)
	if err != nil {
		if err.Error() == utils.ErrNotFound.Error() {
			return err
		}
		return utils.NewErrServerError(err)
	}
	*reply = *rpln
	return nil
}

func (apierSv1 *APIerSv1) RemoveRatingPlan(ctx *context.Context, ID *string, reply *string) error {
	if len(*ID) == 0 {
		return utils.NewErrMandatoryIeMissing("ID")
	}
	err := apierSv1.DataManager.RemoveRatingPlan(*ID, utils.NonTransactional)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
			RatingPlanIDs: []string{*ID},
		}, reply); err != nil {
		return err
	}
	//generate a loadID for CacheRatingPlans and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheRatingPlans: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) ExecuteAction(ctx *context.Context, attr *utils.AttrExecuteAction, reply *string) error {
	at := &engine.ActionTiming{
		ActionsID: attr.ActionsId,
	}
	tnt := attr.Tenant
	if tnt == utils.EmptyString {
		tnt = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	if attr.Account != "" {
		at.SetAccountIDs(utils.StringMap{utils.ConcatenatedKey(tnt, attr.Account): true})
	}
	if err := at.Execute(apierSv1.FilterS, utils.ApierS); err != nil {
		*reply = err.Error()
		return err
	}
	*reply = utils.OK
	return nil
}

type AttrLoadDestination struct {
	TPid string
	ID   string
}

// Load destinations from storDb into dataDb.
func (apierSv1 *APIerSv1) LoadDestination(ctx *context.Context, attrs *AttrLoadDestination, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone, apierSv1.Config.ApierCfg().CachesConns,
		apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if loaded, err := dbReader.LoadDestinationsFiltered(attrs.ID); err != nil {
		return utils.NewErrServerError(err)
	} else if !loaded {
		return utils.ErrNotFound
	}
	if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
			DestinationIDs: []string{attrs.ID},
		}, reply); err != nil {
		return err
	}
	*reply = utils.OK
	return nil
}

type AttrLoadRatingPlan struct {
	TPid         string
	RatingPlanId string
}

// Process dependencies and load a specific rating plan from storDb into dataDb.
func (apierSv1 *APIerSv1) LoadRatingPlan(ctx *context.Context, attrs *AttrLoadRatingPlan, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if loaded, err := dbReader.LoadRatingPlansFiltered(attrs.RatingPlanId); err != nil {
		return utils.NewErrServerError(err)
	} else if !loaded {
		return utils.ErrNotFound
	}
	*reply = utils.OK
	return nil
}

// Process dependencies and load a specific rating profile from storDb into dataDb.
func (apierSv1 *APIerSv1) LoadRatingProfile(ctx *context.Context, attrs *utils.TPRatingProfile, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	if attrs.Tenant == utils.EmptyString {
		attrs.Tenant = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := dbReader.LoadRatingProfilesFiltered(attrs); err != nil {
		return utils.NewErrServerError(err)
	}
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheRatingProfiles: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	// delay if needed before cache reload
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<LoadRatingProfile> Delaying cache reload for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	if err = dbReader.ReloadCache(config.CgrConfig().GeneralCfg().DefaultCaching, true, make(map[string]any), attrs.Tenant); err != nil {
		return utils.NewErrServerError(err)
	}
	*reply = utils.OK
	return nil
}

type AttrLoadSharedGroup struct {
	TPid          string
	SharedGroupId string
}

// Load destinations from storDb into dataDb.
func (apierSv1 *APIerSv1) LoadSharedGroup(ctx *context.Context, attrs *AttrLoadSharedGroup, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := dbReader.LoadSharedGroupsFiltered(attrs.SharedGroupId, true); err != nil {
		return utils.NewErrServerError(err)
	}
	*reply = utils.OK
	return nil
}

type AttrLoadTpFromStorDb struct {
	TPid     string
	DryRun   bool // Only simulate, no write
	Validate bool // Run structural checks
	APIOpts  map[string]any
	Caching  *string // Caching strategy
}

// Loads complete data in a TP from storDb
func (apierSv1 *APIerSv1) LoadTariffPlanFromStorDb(ctx *context.Context, attrs *AttrLoadTpFromStorDb, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := dbReader.LoadAll(); err != nil {
		return utils.NewErrServerError(err)
	}
	if attrs.Validate {
		if !dbReader.IsValid() {
			*reply = utils.OK
			return errors.New("invalid data")
		}
	}
	if attrs.DryRun {
		*reply = utils.OK
		return nil // Mission complete, no errors
	}
	if err := dbReader.WriteToDatabase(false, false); err != nil {
		return utils.NewErrServerError(err)
	}
	//verify If Caching is present in arguments
	caching := config.CgrConfig().GeneralCfg().DefaultCaching
	if attrs.Caching != nil {
		caching = *attrs.Caching
	}
	// delay if needed before cache reload
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<LoadTariffPlanFromStorDb> Delaying cache reload for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	// reload cache
	utils.Logger.Info("APIerSv1.LoadTariffPlanFromStorDb, reloading cache.")
	if err := dbReader.ReloadCache(caching, true, attrs.APIOpts, apierSv1.Config.GeneralCfg().DefaultTenant); err != nil {
		return utils.NewErrServerError(err)
	}
	if len(apierSv1.Config.ApierCfg().SchedulerConns) != 0 {
		utils.Logger.Info("APIerSv1.LoadTariffPlanFromStorDb, reloading scheduler.")
		if err := dbReader.ReloadScheduler(true); err != nil {
			return utils.NewErrServerError(err)
		}
	}
	// release the reader with it's structures
	dbReader.Init()
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) ImportTariffPlanFromFolder(ctx *context.Context, attrs *utils.AttrImportTPFromFolder, reply *string) error {
	if missing := utils.MissingStructFields(attrs, []string{"TPid", "FolderPath"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	if len(attrs.CsvSeparator) == 0 {
		attrs.CsvSeparator = ","
	}
	if fi, err := os.Stat(attrs.FolderPath); err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			return utils.ErrInvalidPath
		}
		return utils.NewErrServerError(err)
	} else if !fi.IsDir() {
		return utils.ErrInvalidPath
	}
	csvImporter := engine.TPCSVImporter{
		TPid:     attrs.TPid,
		StorDb:   apierSv1.StorDb,
		DirPath:  attrs.FolderPath,
		Sep:      rune(attrs.CsvSeparator[0]),
		Verbose:  false,
		ImportId: attrs.RunId,
	}
	if err := csvImporter.Run(); err != nil {
		return utils.NewErrServerError(err)
	}
	*reply = utils.OK
	return nil
}

// SetRatingProfile sets a specific rating profile working with data directly in the DataDB without involving storDb
func (apierSv1 *APIerSv1) SetRatingProfile(ctx *context.Context, attrs *utils.AttrSetRatingProfile, reply *string) (err error) {
	if missing := utils.MissingStructFields(attrs, []string{"Subject", "RatingPlanActivations"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	for _, rpa := range attrs.RatingPlanActivations {
		if missing := utils.MissingStructFields(rpa, []string{"ActivationTime", "RatingPlanId"}); len(missing) != 0 {
			return fmt.Errorf("%s:RatingPlanActivation:%v", utils.ErrMandatoryIeMissing.Error(), missing)
		}
	}
	tnt := attrs.Tenant
	if tnt == utils.EmptyString {
		tnt = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	keyID := utils.ConcatenatedKey(utils.MetaOut,
		tnt, attrs.Category, attrs.Subject)
	var rpfl *engine.RatingProfile
	if !attrs.Overwrite {
		if rpfl, err = apierSv1.DataManager.GetRatingProfile(keyID, false, utils.NonTransactional); err != nil && err != utils.ErrNotFound {
			return utils.NewErrServerError(err)
		}
	}
	if rpfl == nil {
		rpfl = &engine.RatingProfile{Id: keyID, RatingPlanActivations: make(engine.RatingPlanActivations, 0)}
	}
	for _, ra := range attrs.RatingPlanActivations {
		at, err := utils.ParseTimeDetectLayout(ra.ActivationTime,
			apierSv1.Config.GeneralCfg().DefaultTimezone)
		if err != nil {
			return fmt.Errorf("%s:Cannot parse activation time from %v", utils.ErrServerError.Error(), ra.ActivationTime)
		}
		if exists, err := apierSv1.DataManager.HasData(utils.RatingPlanPrefix,
			ra.RatingPlanId, ""); err != nil {
			return utils.NewErrServerError(err)
		} else if !exists {
			return fmt.Errorf("%s:RatingPlanId:%s", utils.ErrNotFound.Error(), ra.RatingPlanId)
		}
		rpfl.RatingPlanActivations = append(rpfl.RatingPlanActivations,
			&engine.RatingPlanActivation{
				ActivationTime: at,
				RatingPlanId:   ra.RatingPlanId,
				FallbackKeys: utils.FallbackSubjKeys(tnt,
					attrs.Category, ra.FallbackSubjects)})
	}
	if err := apierSv1.DataManager.SetRatingProfile(rpfl); err != nil {
		return utils.NewErrServerError(err)
	}

	//generate a loadID for CacheRatingProfiles and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheRatingProfiles: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	// delay if needed before cache call
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<SetRatingProfile> Delaying cache call for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	if err := apierSv1.CallCache(utils.IfaceAsString(attrs.APIOpts[utils.CacheOpt]), attrs.Tenant, utils.CacheRatingProfiles, keyID, utils.EmptyString, nil, nil, attrs.APIOpts); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

// GetRatingProfileIDs returns list of resourceProfile IDs registered for a tenant
func (apierSv1 *APIerSv1) GetRatingProfileIDs(ctx *context.Context, args *utils.PaginatorWithTenant, rsPrfIDs *[]string) error {
	tnt := args.Tenant
	if tnt == utils.EmptyString {
		tnt = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	prfx := utils.RatingProfilePrefix + "*out:" + tnt + utils.ConcatenatedKeySep
	keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return utils.ErrNotFound
	}
	retIDs := make([]string, len(keys))
	for i, key := range keys {
		retIDs[i] = key[len(prfx):]
	}
	*rsPrfIDs = args.PaginateStringSlice(retIDs)
	return nil
}

func (apierSv1 *APIerSv1) GetRatingProfile(ctx *context.Context, attrs *utils.AttrGetRatingProfile, reply *engine.RatingProfile) (err error) {
	if missing := utils.MissingStructFields(attrs, []string{utils.Category, utils.Subject}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	if attrs.Tenant == utils.EmptyString {
		attrs.Tenant = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	if rpPrf, err := apierSv1.DataManager.GetRatingProfile(attrs.GetID(),
		false, utils.NonTransactional); err != nil {
		return utils.APIErrorHandler(err)
	} else {
		*reply = *rpPrf
	}
	return
}

// Deprecated attrs
type V1AttrSetActions struct {
	ActionsId string        // Actions id
	Overwrite bool          // If previously defined, will be overwritten
	Actions   []*V1TPAction // Set of actions this Actions profile will perform
}
type V1TPActions struct {
	TPid      string        // Tariff plan id
	ActionsId string        // Actions id
	Actions   []*V1TPAction // Set of actions this Actions profile will perform
}

type V1TPAction struct {
	Identifier      string   // Identifier mapped in the code
	BalanceId       string   // Balance identification string (account scope)
	BalanceUuid     string   // Balance identification string (global scope)
	BalanceType     string   // Type of balance the action will operate on
	Units           float64  // Number of units to add/deduct
	ExpiryTime      string   // Time when the units will expire
	Filters         []string // The condition on balances that is checked before the action
	TimingTags      string   // Timing when balance is active
	DestinationIds  string   // Destination profile id
	RatingSubject   string   // Reference a rate subject defined in RatingProfiles
	Categories      string   // category filter for balances
	SharedGroups    string   // Reference to a shared group
	BalanceWeight   *float64 // Balance weight
	ExtraParameters string
	BalanceBlocker  string
	BalanceDisabled string
	Weight          float64 // Action's weight
}

func (apierSv1 *APIerSv1) SetActions(ctx *context.Context, attrs *V1AttrSetActions, reply *string) (err error) {
	if missing := utils.MissingStructFields(attrs, []string{"ActionsId", "Actions"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	for _, action := range attrs.Actions {
		requiredFields := []string{"Identifier", "Weight"}
		if action.BalanceType != utils.EmptyString { // Add some inter-dependent parameters - if balanceType then we are not talking about simply calling actions
			requiredFields = append(requiredFields, "Units")
		}
		if missing := utils.MissingStructFields(action, requiredFields); len(missing) != 0 {
			return fmt.Errorf("%s:Action:%s:%v", utils.ErrMandatoryIeMissing.Error(), action.Identifier, missing)
		}
	}
	if !attrs.Overwrite {
		if exists, err := apierSv1.DataManager.HasData(utils.ActionPrefix, attrs.ActionsId, ""); err != nil {
			return utils.NewErrServerError(err)
		} else if exists {
			return utils.ErrExists
		}
	}
	storeActions := make(engine.Actions, len(attrs.Actions))
	for idx, apiAct := range attrs.Actions {
		var blocker *bool
		if apiAct.BalanceBlocker != utils.EmptyString {
			if x, err := strconv.ParseBool(apiAct.BalanceBlocker); err == nil {
				blocker = &x
			} else {
				return err
			}
		}

		var disabled *bool
		if apiAct.BalanceDisabled != utils.EmptyString {
			if x, err := strconv.ParseBool(apiAct.BalanceDisabled); err == nil {
				disabled = &x
			} else {
				return err
			}
		}
		a := &engine.Action{
			Id:               attrs.ActionsId,
			ActionType:       apiAct.Identifier,
			Weight:           apiAct.Weight,
			ExpirationString: apiAct.ExpiryTime,
			ExtraParameters:  apiAct.ExtraParameters,
			Filters:          apiAct.Filters,
			Balance: &engine.BalanceFilter{ // TODO: update this part
				Uuid:           utils.StringPointer(apiAct.BalanceUuid),
				ID:             utils.StringPointer(apiAct.BalanceId),
				Type:           utils.StringPointer(apiAct.BalanceType),
				Value:          &utils.ValueFormula{Static: apiAct.Units},
				Weight:         apiAct.BalanceWeight,
				DestinationIDs: utils.StringMapPointer(utils.ParseStringMap(apiAct.DestinationIds)),
				RatingSubject:  utils.StringPointer(apiAct.RatingSubject),
				SharedGroups:   utils.StringMapPointer(utils.ParseStringMap(apiAct.SharedGroups)),
				Categories:     utils.StringMapPointer(utils.ParseStringMap(apiAct.Categories)),
				TimingIDs:      utils.StringMapPointer(utils.ParseStringMap(apiAct.TimingTags)),
				Blocker:        blocker,
				Disabled:       disabled,
			},
		}
		// load action timings from tags
		if apiAct.TimingTags != "" {
			timingIds := strings.Split(apiAct.TimingTags, utils.InfieldSep)
			for _, timingID := range timingIds {
				timing, err := apierSv1.DataManager.GetTiming(timingID, false,
					utils.NonTransactional)
				if err != nil {
					return fmt.Errorf("error: %v querying timing with id: %q",
						err.Error(), timingID)
				}
				a.Balance.Timings = append(a.Balance.Timings, &engine.RITiming{
					ID:        timingID,
					Years:     timing.Years,
					Months:    timing.Months,
					MonthDays: timing.MonthDays,
					WeekDays:  timing.WeekDays,
					StartTime: timing.StartTime,
					EndTime:   timing.EndTime,
				})
			}
		}
		storeActions[idx] = a
	}
	if err := apierSv1.DataManager.SetActions(attrs.ActionsId, storeActions); err != nil {
		return utils.NewErrServerError(err)
	}
	//CacheReload
	if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
			ActionIDs: []string{attrs.ActionsId},
		}, reply); err != nil {
		return err
	}
	//generate a loadID for CacheActions and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheActions: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

// Retrieves actions attached to specific ActionsId within cache
func (apierSv1 *APIerSv1) GetActions(ctx *context.Context, actsId *string, reply *[]*utils.TPAction) error {
	if len(*actsId) == 0 {
		return fmt.Errorf("%s ActionsId: %s", utils.ErrMandatoryIeMissing.Error(), *actsId)
	}
	acts := make([]*utils.TPAction, 0)
	engActs, err := apierSv1.DataManager.GetActions(*actsId, false, utils.NonTransactional)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	for _, engAct := range engActs {
		act := &utils.TPAction{
			Identifier:      engAct.ActionType,
			ExpiryTime:      engAct.ExpirationString,
			ExtraParameters: engAct.ExtraParameters,
			Filters:         strings.Join(engAct.Filters, utils.InfieldSep),
			Weight:          engAct.Weight,
		}
		bf := engAct.Balance
		if bf != nil {
			act.BalanceType = bf.GetType()
			act.Units = strconv.FormatFloat(bf.GetValue(), 'f', -1, 64)
			act.DestinationIds = bf.GetDestinationIDs().String()
			act.RatingSubject = bf.GetRatingSubject()
			act.SharedGroups = bf.GetSharedGroups().String()
			act.BalanceWeight = strconv.FormatFloat(bf.GetWeight(), 'f', -1, 64)
			act.TimingTags = bf.GetTimingIDs().String()
			act.BalanceId = bf.GetID()
			act.Categories = bf.GetCategories().String()
			act.BalanceBlocker = strconv.FormatBool(bf.GetBlocker())
			act.BalanceDisabled = strconv.FormatBool(bf.GetDisabled())
		}
		acts = append(acts, act)
	}
	*reply = acts
	return nil
}

func (apierSv1 *APIerSv1) SetActionPlan(ctx *context.Context, attrs *engine.AttrSetActionPlan, reply *string) (err error) {
	if missing := utils.MissingStructFields(attrs, []string{"Id", "ActionPlan"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	for _, at := range attrs.ActionPlan {
		requiredFields := []string{"ActionsId", "Time", "Weight"}
		if missing := utils.MissingStructFields(at, requiredFields); len(missing) != 0 {
			return fmt.Errorf("%s:Action:%s:%v", utils.ErrMandatoryIeMissing.Error(), at.ActionsId, missing)
		}
	}
	err = guardian.Guardian.Guard(func() error {
		var prevAccountIDs utils.StringMap
		if prevAP, err := apierSv1.DataManager.GetActionPlan(attrs.Id, true, true, utils.NonTransactional); err != nil && err != utils.ErrNotFound {
			return utils.NewErrServerError(err)
		} else if err == nil && !attrs.Overwrite {
			return utils.ErrExists
		} else if prevAP != nil {
			prevAccountIDs = prevAP.AccountIDs
		}
		ap := &engine.ActionPlan{
			Id: attrs.Id,
		}
		for _, apiAtm := range attrs.ActionPlan {
			if exists, err := apierSv1.DataManager.HasData(utils.ActionPrefix, apiAtm.ActionsId, ""); err != nil {
				return utils.NewErrServerError(err)
			} else if !exists {
				return fmt.Errorf("%s:%s", utils.ErrBrokenReference.Error(), apiAtm.ActionsId)
			}
			timing, err := apiAtm.GetRITiming(apierSv1.DataManager)
			if err != nil {
				return err
			}
			ap.ActionTimings = append(ap.ActionTimings, &engine.ActionTiming{
				Uuid:      utils.GenUUID(),
				Weight:    apiAtm.Weight,
				Timing:    &engine.RateInterval{Timing: timing},
				ActionsID: apiAtm.ActionsId,
			})
		}
		if err := apierSv1.DataManager.SetActionPlan(ap.Id, ap, true, utils.NonTransactional); err != nil {
			return utils.NewErrServerError(err)
		}
		if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
			utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
				ActionPlanIDs: []string{ap.Id},
			}, reply); err != nil {
			return err
		}
		for acntID := range prevAccountIDs {
			if err := apierSv1.DataManager.RemAccountActionPlans(acntID, []string{attrs.Id}); err != nil {
				return utils.NewErrServerError(err)
			}
		}
		if len(prevAccountIDs) != 0 {
			if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
				utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
					AccountActionPlanIDs: prevAccountIDs.Slice(),
				}, reply); err != nil {
				return err
			}
		}
		return nil
	}, config.CgrConfig().GeneralCfg().LockingTimeout, utils.ActionPlanPrefix)
	if err != nil {
		return err
	}
	if attrs.ReloadScheduler {
		sched := apierSv1.SchedulerService.GetScheduler()
		if sched == nil {
			return errors.New(utils.SchedulerNotRunningCaps)
		}
		sched.Reload()
	}
	//generate a loadID for CacheActionPlans and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheActionPlans: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

type AttrGetActionPlan struct {
	ID string
}

func (apierSv1 *APIerSv1) GetActionPlan(ctx *context.Context, attr *AttrGetActionPlan, reply *[]*engine.ActionPlan) error {
	var result []*engine.ActionPlan
	if attr.ID == "" || attr.ID == "*" {
		result = make([]*engine.ActionPlan, 0)
		aplsMap, err := apierSv1.DataManager.GetAllActionPlans()
		if err != nil {
			return err
		}
		for _, apls := range aplsMap {
			result = append(result, apls)
		}
	} else {
		apls, err := apierSv1.DataManager.GetActionPlan(attr.ID, true, true, utils.NonTransactional)
		if err != nil {
			return err
		}
		result = append(result, apls)
	}
	*reply = result
	return nil
}

func (apierSv1 *APIerSv1) RemoveActionPlan(ctx *context.Context, attr *AttrGetActionPlan, reply *string) (err error) {
	if missing := utils.MissingStructFields(attr, []string{"ID"}); len(missing) != 0 {
		return utils.NewErrMandatoryIeMissing(missing...)
	}
	if err = guardian.Guardian.Guard(func() error {
		var prevAccountIDs utils.StringMap
		if prevAP, err := apierSv1.DataManager.GetActionPlan(attr.ID, true, true, utils.NonTransactional); err != nil && err != utils.ErrNotFound {
			return err
		} else if prevAP != nil {
			prevAccountIDs = prevAP.AccountIDs
		}
		if err := apierSv1.DataManager.RemoveActionPlan(attr.ID, utils.NonTransactional); err != nil {
			return err
		}
		for acntID := range prevAccountIDs {
			if err := apierSv1.DataManager.RemAccountActionPlans(acntID, []string{attr.ID}); err != nil {
				return utils.NewErrServerError(err)
			}
		}
		if len(prevAccountIDs) != 0 {
			if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
				utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
					AccountActionPlanIDs: prevAccountIDs.Slice(),
				}, reply); err != nil {
				return err
			}
		}
		return nil
	}, config.CgrConfig().GeneralCfg().LockingTimeout, utils.ActionPlanPrefix); err != nil {
		return err
	}
	*reply = utils.OK
	return nil
}

// Process dependencies and load a specific AccountActions profile from storDb into dataDb.
func (apierSv1 *APIerSv1) LoadAccountActions(ctx *context.Context, attrs *utils.TPAccountActions, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := guardian.Guardian.Guard(func() error {
		return dbReader.LoadAccountActionsFiltered(attrs)
	}, config.CgrConfig().GeneralCfg().LockingTimeout, attrs.LoadId); err != nil {
		return utils.NewErrServerError(err)
	}
	// ToDo: Get the action keys loaded by dbReader so we reload only these in cache
	// Need to do it before scheduler otherwise actions to run will be unknown
	sched := apierSv1.SchedulerService.GetScheduler()
	if sched != nil {
		sched.Reload()
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) LoadTariffPlanFromFolder(ctx *context.Context, attrs *utils.AttrLoadTpFromFolder, reply *string) error {
	// verify if FolderPath is present
	if len(attrs.FolderPath) == 0 {
		return fmt.Errorf("%s:%s", utils.ErrMandatoryIeMissing.Error(), "FolderPath")
	}
	// check if exists or is valid
	if fi, err := os.Stat(attrs.FolderPath); err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			return utils.ErrInvalidPath
		}
		return utils.NewErrServerError(err)
	} else if !fi.IsDir() {
		return utils.ErrInvalidPath
	}

	// initialize CSV storage
	csvStorage, err := engine.NewFileCSVStorage(utils.CSVSep, attrs.FolderPath)
	if err != nil {
		return utils.NewErrServerError(err)
	}

	// create the TpReader
	loader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(),
		csvStorage, "", apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	//Load the data
	if err := loader.LoadAll(); err != nil {
		return utils.NewErrServerError(err)
	}
	if attrs.DryRun {
		*reply = utils.OK
		return nil // Mission complete, no errors
	}

	if attrs.Validate {
		if !loader.IsValid() {
			return errors.New("invalid data")
		}
	}

	// write data intro Database
	if err := loader.WriteToDatabase(false, false); err != nil {
		return utils.NewErrServerError(err)
	}
	//verify If Caching is present in arguments
	caching := config.CgrConfig().GeneralCfg().DefaultCaching
	if attrs.Caching != nil {
		caching = *attrs.Caching
	}
	// delay if needed before cache reload
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<V1LoadTariffPlanFromFolder> Delaying cache reload for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	// reload cache
	utils.Logger.Info("APIerSv1.LoadTariffPlanFromFolder, reloading cache.")
	if err := loader.ReloadCache(caching, true, attrs.APIOpts, apierSv1.Config.GeneralCfg().DefaultTenant); err != nil {
		return utils.NewErrServerError(err)
	}
	if len(apierSv1.Config.ApierCfg().SchedulerConns) != 0 {
		utils.Logger.Info("APIerSv1.LoadTariffPlanFromFolder, reloading scheduler.")
		if err := loader.ReloadScheduler(true); err != nil {
			return utils.NewErrServerError(err)
		}
	}
	// release the reader with it's structures
	loader.Init()
	*reply = utils.OK
	return nil
}

// RemoveTPFromFolder will load the tarrifplan from folder into TpReader object
// and will delete if from database
func (apierSv1 *APIerSv1) RemoveTPFromFolder(ctx *context.Context, attrs *utils.AttrLoadTpFromFolder, reply *string) error {
	// verify if FolderPath is present
	if len(attrs.FolderPath) == 0 {
		return fmt.Errorf("%s:%s", utils.ErrMandatoryIeMissing.Error(), "FolderPath")
	}
	// check if exists or is valid
	if fi, err := os.Stat(attrs.FolderPath); err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			return utils.ErrInvalidPath
		}
		return utils.NewErrServerError(err)
	} else if !fi.IsDir() {
		return utils.ErrInvalidPath
	}

	// initialize CSV storage
	csvStorage, err := engine.NewFileCSVStorage(utils.CSVSep, attrs.FolderPath)
	if err != nil {
		return utils.NewErrServerError(err)
	}

	// create the TpReader
	loader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(),
		csvStorage, "", apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	//Load the data
	if err := loader.LoadAll(); err != nil {
		return utils.NewErrServerError(err)
	}
	if attrs.DryRun {
		*reply = utils.OK
		return nil // Mission complete, no errors
	}

	if attrs.Validate {
		if !loader.IsValid() {
			return errors.New("invalid data")
		}
	}

	// remove data from Database
	if err := loader.RemoveFromDatabase(false, false); err != nil {
		return utils.NewErrServerError(err)
	}
	//verify If Caching is present in arguments
	caching := config.CgrConfig().GeneralCfg().DefaultCaching
	if attrs.Caching != nil {
		caching = *attrs.Caching
	}
	// delay if needed before cache reload
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<RemoveTPFromFolder> Delaying cache reload for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	// reload cache
	utils.Logger.Info("APIerSv1.RemoveTPFromFolder, reloading cache.")
	if err := loader.ReloadCache(caching, true, attrs.APIOpts, apierSv1.Config.GeneralCfg().DefaultTenant); err != nil {
		return utils.NewErrServerError(err)
	}
	if len(apierSv1.Config.ApierCfg().SchedulerConns) != 0 {
		utils.Logger.Info("APIerSv1.RemoveTPFromFolder, reloading scheduler.")
		if err := loader.ReloadScheduler(true); err != nil {
			return utils.NewErrServerError(err)
		}
	}
	// release the reader with it's structures
	loader.Init()
	*reply = utils.OK
	return nil
}

// RemoveTPFromStorDB will load the tarrifplan from StorDB into TpReader object
// and will delete if from database
func (apierSv1 *APIerSv1) RemoveTPFromStorDB(ctx *context.Context, attrs *AttrLoadTpFromStorDb, reply *string) error {
	if len(attrs.TPid) == 0 {
		return utils.NewErrMandatoryIeMissing("TPid")
	}
	dbReader, err := engine.NewTpReader(apierSv1.DataManager.DataDB(), apierSv1.StorDb,
		attrs.TPid, apierSv1.Config.GeneralCfg().DefaultTimezone,
		apierSv1.Config.ApierCfg().CachesConns, apierSv1.Config.ApierCfg().SchedulerConns,
		apierSv1.Config.DataDbCfg().Type == utils.MetaInternal)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if err := dbReader.LoadAll(); err != nil {
		return utils.NewErrServerError(err)
	}
	if attrs.Validate {
		if !dbReader.IsValid() {
			*reply = utils.OK
			return errors.New("invalid data")
		}
	}
	if attrs.DryRun {
		*reply = utils.OK
		return nil // Mission complete, no errors
	}
	// remove data from Database
	if err := dbReader.RemoveFromDatabase(false, false); err != nil {
		return utils.NewErrServerError(err)
	}
	//verify If Caching is present in arguments
	caching := config.CgrConfig().GeneralCfg().DefaultCaching
	if attrs.Caching != nil {
		caching = *attrs.Caching
	}
	// delay if needed before cache reload
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<RemoveTPFromStorDB> Delaying cache reload for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	// reload cache
	utils.Logger.Info("APIerSv1.RemoveTPFromStorDB, reloading cache.")
	if err := dbReader.ReloadCache(caching, true, attrs.APIOpts, apierSv1.Config.GeneralCfg().DefaultTenant); err != nil {
		return utils.NewErrServerError(err)
	}
	if len(apierSv1.Config.ApierCfg().SchedulerConns) != 0 {
		utils.Logger.Info("APIerSv1.RemoveTPFromStorDB, reloading scheduler.")
		if err := dbReader.ReloadScheduler(true); err != nil {
			return utils.NewErrServerError(err)
		}
	}
	// release the reader with it's structures
	dbReader.Init()
	*reply = utils.OK
	return nil
}

type AttrRemoveRatingProfile struct {
	Tenant   string
	Category string
	Subject  string
	APIOpts  map[string]any
}

func (arrp *AttrRemoveRatingProfile) GetId() (result string) {
	result = utils.MetaOut + utils.ConcatenatedKeySep
	if arrp.Tenant != utils.EmptyString && arrp.Tenant != utils.MetaAny {
		result += arrp.Tenant + utils.ConcatenatedKeySep
	} else {
		return
	}

	if arrp.Category != utils.EmptyString && arrp.Category != utils.MetaAny {
		result += arrp.Category + utils.ConcatenatedKeySep
	} else {
		return
	}
	if arrp.Subject != utils.EmptyString {
		result += arrp.Subject
	}
	return
}

func (apierSv1 *APIerSv1) RemoveRatingProfile(ctx *context.Context, attr *AttrRemoveRatingProfile, reply *string) error {
	if attr.Tenant == utils.EmptyString {
		attr.Tenant = apierSv1.Config.GeneralCfg().DefaultTenant
	}
	if (attr.Subject != utils.EmptyString && slices.Contains([]string{attr.Tenant, attr.Category}, utils.EmptyString)) ||
		(attr.Category != utils.EmptyString && attr.Tenant == utils.EmptyString) {
		return utils.ErrMandatoryIeMissing
	}
	keyID := attr.GetId()
	err := guardian.Guardian.Guard(func() error {
		return apierSv1.DataManager.RemoveRatingProfile(keyID)
	}, config.CgrConfig().GeneralCfg().LockingTimeout, "RemoveRatingProfile")
	if err != nil {
		*reply = err.Error()
		return utils.NewErrServerError(err)
	}
	//generate a loadID for CacheActionPlans and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheRatingProfiles: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	// delay if needed before cache call
	if apierSv1.Config.GeneralCfg().CachingDelay != 0 {
		utils.Logger.Info(fmt.Sprintf("<RemoveRatingProfile> Delaying cache call for %v", apierSv1.Config.GeneralCfg().CachingDelay))
		time.Sleep(apierSv1.Config.GeneralCfg().CachingDelay)
	}
	if err := apierSv1.CallCache(utils.IfaceAsString(attr.APIOpts[utils.CacheOpt]), attr.Tenant, utils.CacheRatingProfiles, keyID, utils.EmptyString, nil, nil, attr.APIOpts); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) GetLoadHistory(ctx *context.Context, attrs *utils.Paginator, reply *[]*utils.LoadInstance) error {
	nrItems := -1
	offset := 0
	if attrs.Offset != nil { // For offset we need full data
		offset = *attrs.Offset
	} else if attrs.Limit != nil {
		nrItems = *attrs.Limit
	}
	loadHist, err := apierSv1.DataManager.DataDB().GetLoadHistory(nrItems, true, utils.NonTransactional)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	if attrs.Offset != nil && attrs.Limit != nil { // Limit back to original
		nrItems = *attrs.Limit
	}
	if len(loadHist) == 0 || len(loadHist) <= offset || nrItems == 0 {
		return utils.ErrNotFound
	}
	if offset != 0 {
		nrItems = offset + nrItems
	}
	if nrItems == -1 || nrItems > len(loadHist) { // So we can use it in indexing bellow
		nrItems = len(loadHist)
	}
	*reply = loadHist[offset:nrItems]
	return nil
}

type AttrRemoveActions struct {
	ActionIDs []string
}

func (apierSv1 *APIerSv1) RemoveActions(ctx *context.Context, attr *AttrRemoveActions, reply *string) error {
	if attr.ActionIDs == nil {
		err := utils.ErrNotFound
		*reply = err.Error()
		return err
	}
	// The check could lead to very long execution time. So we decided to leave it at the user's risck.'
	/*
		stringMap := utils.NewStringMap(attr.ActionIDs...)
		keys, err := apiv1.DataManager.DataDB().GetKeysForPrefix(utils.ActionTriggerPrefix, true)
		if err != nil {
			*reply = err.Error()
			return err
		}
		for _, key := range keys {
			getAttrs, err := apiv1.DataManager.DataDB().GetActionTriggers(key[len(utils.ActionTriggerPrefix):])
			if err != nil {
				*reply = err.Error()
				return err
			}
			for _, atr := range getAttrs {
				if _, found := stringMap[atr.ActionsID]; found {
					// found action trigger referencing action; abort
					err := fmt.Errorf("action %s refenced by action trigger %s", atr.ActionsID, atr.ID)
					*reply = err.Error()
					return err
				}
			}
		}
		allAplsMap, err := apiv1.DataManager.GetAllActionPlans()
		if err != nil && err != utils.ErrNotFound {
			*reply = err.Error()
			return err
		}
		for _, apl := range allAplsMap {
			for _, atm := range apl.ActionTimings {
				if _, found := stringMap[atm.ActionsID]; found {
					err := fmt.Errorf("action %s refenced by action plan %s", atm.ActionsID, apl.Id)
					*reply = err.Error()
					return err
				}
			}
		}
	*/
	for _, aID := range attr.ActionIDs {
		if err := apierSv1.DataManager.RemoveActions(aID); err != nil {
			*reply = err.Error()
			return err
		}
	}
	//CacheReload
	if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().CachesConns,
		utils.CacheSv1ReloadCache, &utils.AttrReloadCacheWithAPIOpts{
			ActionIDs: attr.ActionIDs,
		}, reply); err != nil {
		return err
	}
	//generate a loadID for CacheActions and store it in database
	if err := apierSv1.DataManager.SetLoadIDs(map[string]int64{utils.CacheActions: time.Now().UnixNano()}); err != nil {
		return utils.APIErrorHandler(err)
	}
	*reply = utils.OK
	return nil
}

// ReplayFailedPostsParams contains parameters for replaying failed posts.
type ReplayFailedPostsParams struct {
	SourcePath string   // path for events to be replayed
	FailedPath string   // path for events that failed to replay, *none to discard, defaults to SourceDir if empty
	Modules    []string // list of modules to replay requests for, nil for all
}

// ReplayFailedPosts will repost failed requests found in the FailedRequestsInDir
func (apierSv1 *APIerSv1) ReplayFailedPosts(ctx *context.Context, args ReplayFailedPostsParams, reply *string) error {

	// Set default directories if not provided.
	if args.SourcePath == "" {
		args.SourcePath = apierSv1.Config.GeneralCfg().FailedPostsDir
	}
	if args.FailedPath == "" {
		args.FailedPath = args.SourcePath
	}

	if err := filepath.WalkDir(args.SourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			utils.Logger.Warning(fmt.Sprintf("<ReplayFailedPosts> failed to access path %s: %v", path, err))
			return nil // skip paths that cause an error
		}
		if d.IsDir() {
			return nil // skip directories
		}

		// Skip files not belonging to the specified modules.
		if len(args.Modules) != 0 && !slices.ContainsFunc(args.Modules, func(mod string) bool {
			return strings.HasPrefix(d.Name(), mod)
		}) {
			utils.Logger.Info(fmt.Sprintf("<ReplayFailedPosts> skipping file %s: not found within specified modules", d.Name()))
			return nil
		}

		expEv, err := ees.NewExportEventsFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to init ExportEvents from %s: %v", path, err)
		}

		// Determine the failover path.
		failoverPath := utils.MetaNone
		if args.FailedPath != utils.MetaNone {
			failoverPath = filepath.Join(args.FailedPath, d.Name())
		}

		failedPosts, err := expEv.ReplayFailedPosts(apierSv1.Config.GeneralCfg().PosterAttempts)
		if err != nil && failoverPath != utils.MetaNone {
			// Write the events that failed to be replayed to the failover directory
			if err = failedPosts.WriteToFile(failoverPath); err != nil {
				return fmt.Errorf("failed to write the events that failed to be replayed to %s: %v", path, err)
			}
		}
		return nil
	}); err != nil {
		return utils.NewErrServerError(err)
	}
	*reply = utils.OK
	return nil
}

// ReplayFailedReplicationsArgs contains args for replaying failed replications.
type ReplayFailedReplicationsArgs struct {
	SourcePath string // path for events to be replayed
	FailedPath string // path for events that failed to replay, *none to discard, defaults to SourcePath if empty
}

// ReplayFailedReplications will repost failed requests found in the SourcePath.
func (a *APIerSv1) ReplayFailedReplications(ctx *context.Context, args ReplayFailedReplicationsArgs, reply *string) error {

	// Set default directories if not provided.
	if args.SourcePath == "" {
		args.SourcePath = a.Config.DataDbCfg().RplFailedDir
	}
	if args.SourcePath == "" {
		return utils.NewErrServerError(
			errors.New("no source directory specified: both SourcePath and replication_failed_dir configuration are empty"),
		)
	}
	if args.FailedPath == "" {
		args.FailedPath = args.SourcePath
	}

	if err := filepath.WalkDir(args.SourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			utils.Logger.Warning(fmt.Sprintf("<ReplayFailedReplications> failed to access path %s: %v", path, err))
			return nil // skip paths that cause an error
		}
		if d.IsDir() {
			return nil // skip directories
		}

		task, err := engine.NewReplicationTaskFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to init ExportEvents from %s: %v", path, err)
		}

		// Determine the failover path.
		failoverPath := utils.MetaNone
		if args.FailedPath != utils.MetaNone {
			failoverPath = filepath.Join(args.FailedPath, d.Name())
		}

		if err := task.Execute(a.ConnMgr); err != nil && failoverPath != utils.MetaNone {
			// Write the events that failed to be replayed to the failover directory
			if err = task.WriteToFile(failoverPath); err != nil {
				return fmt.Errorf("failed to write the events that failed to be replayed to %s: %v", path, err)
			}
		}
		return nil
	}); err != nil {
		return utils.NewErrServerError(err)
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) GetLoadIDs(ctx *context.Context, args *string, reply *map[string]int64) (err error) {
	var loadIDs map[string]int64
	if loadIDs, err = apierSv1.DataManager.GetItemLoadIDs(*args, false); err != nil {
		return
	}
	*reply = loadIDs
	return
}

type LoadTimeArgs struct {
	Timezone string
	Item     string
}

func (apierSv1 *APIerSv1) GetLoadTimes(ctx *context.Context, args *LoadTimeArgs, reply *map[string]string) (err error) {
	if loadIDs, err := apierSv1.DataManager.GetItemLoadIDs(args.Item, false); err != nil {
		return err
	} else {
		provMp := make(map[string]string)
		for key, val := range loadIDs {
			timeVal, err := utils.ParseTimeDetectLayout(strconv.FormatInt(val, 10), args.Timezone)
			if err != nil {
				return err
			}
			provMp[key] = timeVal.String()
		}
		*reply = provMp
	}
	return
}

func (apierSv1 *APIerSv1) ComputeActionPlanIndexes(ctx *context.Context, _ string, reply *string) (err error) {
	if err = apierSv1.DataManager.RebuildReverseForPrefix(utils.AccountActionPlansPrefix); err != nil {
		return err
	}
	*reply = utils.OK
	return nil
}

// GetActionPlanIDs returns list of ActionPlan IDs registered for a tenant
func (apierSv1 *APIerSv1) GetActionPlanIDs(ctx *context.Context, args *utils.PaginatorWithTenant, attrPrfIDs *[]string) error {
	prfx := utils.ActionPlanPrefix
	keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(utils.ActionPlanPrefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return utils.ErrNotFound
	}
	retIDs := make([]string, len(keys))
	for i, key := range keys {
		retIDs[i] = key[len(prfx):]
	}
	*attrPrfIDs = args.PaginateStringSlice(retIDs)
	return nil
}

// GetRatingPlanIDs returns list of RatingPlan IDs registered for a tenant
func (apierSv1 *APIerSv1) GetRatingPlanIDs(ctx *context.Context, args *utils.PaginatorWithTenant, attrPrfIDs *[]string) error {
	prfx := utils.RatingPlanPrefix
	keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(utils.RatingPlanPrefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return utils.ErrNotFound
	}
	retIDs := make([]string, len(keys))
	for i, key := range keys {
		retIDs[i] = key[len(prfx):]
	}
	*attrPrfIDs = args.PaginateStringSlice(retIDs)
	return nil
}

// ListenAndServe listen for storbd reload
func (apierSv1 *APIerSv1) ListenAndServe(stopChan chan struct{}) {
	utils.Logger.Info(fmt.Sprintf("<%s> starting <%s> subsystem", utils.CoreS, utils.ApierS))
	for {
		select {
		case <-stopChan:
			return
		case stordb, ok := <-apierSv1.StorDBChan:
			if !ok { // the chanel was closed by the shutdown of stordbService
				return
			}
			apierSv1.CdrDb = stordb
			apierSv1.StorDb = stordb
		case resp := <-apierSv1.ResponderChan:
			apierSv1.Responder = resp
		}
	}
}

// ExportToFolder export specific items (or all items if items is empty) from DataDB back to CSV
func (apierSv1 *APIerSv1) ExportToFolder(ctx *context.Context, arg *utils.ArgExportToFolder, reply *string) error {
	// if items is empty we need to export all items
	if len(arg.Items) == 0 {
		arg.Items = []string{utils.MetaAttributes, utils.MetaChargers, utils.MetaDispatchers,
			utils.MetaDispatcherHosts, utils.MetaFilters, utils.MetaResources, utils.MetaStats,
			utils.MetaRoutes, utils.MetaThresholds, utils.MetaRankings, utils.MetaTrends}
	}
	if _, err := os.Stat(arg.Path); os.IsNotExist(err) {
		os.Mkdir(arg.Path, os.ModeDir)
	}
	for _, item := range arg.Items {
		switch item {
		case utils.MetaAttributes:
			prfx := utils.AttributeProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.AttributesCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.AttributeMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				attPrf, err := apierSv1.DataManager.GetAttributeProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPAttribute(
					engine.AttributeProfileToAPI(attPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaChargers:
			prfx := utils.ChargerProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.ChargersCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.ChargerMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				chrPrf, err := apierSv1.DataManager.GetChargerProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPCharger(
					engine.ChargerProfileToAPI(chrPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaDispatchers:
			prfx := utils.DispatcherProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.DispatcherProfilesCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.DispatcherProfileMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				dpsPrf, err := apierSv1.DataManager.GetDispatcherProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPDispatcherProfile(
					engine.DispatcherProfileToAPI(dpsPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaDispatcherHosts:
			prfx := utils.DispatcherHostPrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.DispatcherHostsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.DispatcherHostMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				dpsPrf, err := apierSv1.DataManager.GetDispatcherHost(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				if record, err := engine.CsvDump(engine.APItoModelTPDispatcherHost(
					engine.DispatcherHostToAPI(dpsPrf))); err != nil {
					return err
				} else if err := csvWriter.Write(record); err != nil {
					return err
				}
			}
			csvWriter.Flush()
		case utils.MetaFilters:
			prfx := utils.FilterPrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.FiltersCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.FilterMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				fltr, err := apierSv1.DataManager.GetFilter(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPFilter(
					engine.FilterToTPFilter(fltr)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaResources:
			prfx := utils.ResourceProfilesPrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.ResourcesCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.ResourceMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				resPrf, err := apierSv1.DataManager.GetResourceProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelResource(
					engine.ResourceProfileToAPI(resPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaIPs:
			prfx := utils.IPProfilesPrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.IPsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.IPMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				ipPrf, err := apierSv1.DataManager.GetIPProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelIP(
					engine.IPProfileToAPI(ipPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaStats:
			prfx := utils.StatQueueProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.StatsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.StatMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				stsPrf, err := apierSv1.DataManager.GetStatQueueProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelStats(
					engine.StatQueueProfileToAPI(stsPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaRankings:
			prfx := utils.RankingsProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.RankingsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			if err := csvWriter.Write(engine.RankingsMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				rnkPrf, err := apierSv1.DataManager.GetRankingProfile(tntID[0], tntID[1], true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				mdl := engine.APItoModelTPRanking(engine.RankingProfileToAPI(rnkPrf))
				record, err := engine.CsvDump(mdl)
				if err != nil {
					return err
				}
				csvWriter.Write(record)
			}
			csvWriter.Flush()
		case utils.MetaTrends:
			prfx := utils.TrendsProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.TrendsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.TrendsMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				trPrf, err := apierSv1.DataManager.GetTrendProfile(tntID[0], tntID[1], true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				mdl := engine.APItoModelTrends(engine.TrendProfileToAPI(trPrf))
				record, err := engine.CsvDump(mdl)
				if err != nil {
					return err
				}
				csvWriter.Write(record)
			}
			csvWriter.Flush()
		case utils.MetaRoutes:
			prfx := utils.RouteProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.RoutesCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.RouteMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				spp, err := apierSv1.DataManager.GetRouteProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPRoutes(
					engine.RouteProfileToAPI(spp)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		case utils.MetaThresholds:
			prfx := utils.ThresholdProfilePrefix
			keys, err := apierSv1.DataManager.DataDB().GetKeysForPrefix(prfx)
			if err != nil {
				return err
			}
			if len(keys) == 0 { // if we don't find items we skip
				continue
			}
			f, err := os.Create(path.Join(arg.Path, utils.ThresholdsCsv))
			if err != nil {
				return err
			}
			defer f.Close()

			csvWriter := csv.NewWriter(f)
			csvWriter.Comma = utils.CSVSep
			//write the header of the file
			if err := csvWriter.Write(engine.ThresholdMdls{}.CSVHeader()); err != nil {
				return err
			}
			for _, key := range keys {
				tntID := strings.SplitN(key[len(prfx):], utils.InInFieldSep, 2)
				thPrf, err := apierSv1.DataManager.GetThresholdProfile(tntID[0], tntID[1],
					true, false, utils.NonTransactional)
				if err != nil {
					return err
				}
				for _, model := range engine.APItoModelTPThreshold(
					engine.ThresholdProfileToAPI(thPrf)) {
					if record, err := engine.CsvDump(model); err != nil {
						return err
					} else if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
			csvWriter.Flush()
		}
	}
	*reply = utils.OK
	return nil
}

func (apierSv1 *APIerSv1) ExportCDRs(ctx *context.Context, args *utils.ArgExportCDRs, reply *map[string]any) (err error) {
	if len(apierSv1.Config.ApierCfg().EEsConns) == 0 {
		return utils.NewErrNotConnected(utils.EEs)
	}
	cdrsFltr, err := args.RPCCDRsFilter.AsCDRsFilter(apierSv1.Config.GeneralCfg().DefaultTimezone)
	if err != nil {
		return utils.NewErrServerError(err)
	}
	cdrs, _, err := apierSv1.CdrDb.GetCDRs(cdrsFltr, false)
	if err != nil {
		return err
	} else if len(cdrs) == 0 {
		return utils.ErrNotFound
	}
	withErros := false
	var rplyCdr map[string]map[string]any
	for _, cdr := range cdrs {
		argCdr := &engine.CGREventWithEeIDs{
			EeIDs:    args.ExporterIDs,
			CGREvent: cdr.AsCGREvent(),
		}
		if args.Verbose {
			argCdr.CGREvent.APIOpts[utils.OptsEEsVerbose] = struct{}{}
		}
		if err := apierSv1.ConnMgr.Call(context.TODO(), apierSv1.Config.ApierCfg().EEsConns,
			utils.EeSv1ProcessEvent,
			argCdr, &rplyCdr); err != nil {
			utils.Logger.Warning(fmt.Sprintf("<%s> error: <%s> processing event: <%s> with <%s>",
				utils.ApierS, err.Error(), utils.ToJSON(cdr.AsCGREvent()), utils.EEs))
			withErros = true
		}
	}
	if withErros {
		return utils.ErrPartiallyExecuted
	}
	// we consider only the last reply because it should have the metrics updated
	for exporterID, metrics := range rplyCdr {
		(*reply)[exporterID] = metrics
	}
	return
}

type TimeParams struct {
	TimingID string // Holds the TimingID to get from the DataDB
	Time     string // Time to compare with TimingID
}

// Replies true if the TimingID is active at the specified time, false if not
func (apierSv1 *APIerSv1) TimingIsActiveAt(ctx *context.Context, params TimeParams, reply *bool) (err error) {
	timing, err := apierSv1.DataManager.GetTiming(params.TimingID, false, utils.NonTransactional)
	if err != nil {
		return err
	}
	if tm, err := utils.ParseTimeDetectLayout(params.Time, apierSv1.Config.GeneralCfg().DefaultTimezone); err != nil {
		return err
	} else {
		*reply = timing.IsActiveAt(tm)
	}
	return
}

// DumpDataDB will dump all of datadb from memory to a file
func (apierSv1 *APIerSv1) DumpDataDB(ctx *context.Context, ignr *string, reply *string) (err error) {
	if err = apierSv1.DataManager.DataDB().DumpDataDB(); err != nil {
		return
	}
	*reply = utils.OK
	return
}

// Will rewrite every dump file of DataDB
func (apierSv1 *APIerSv1) RewriteDataDB(ctx *context.Context, ignr *string, reply *string) (err error) {
	if err = apierSv1.DataManager.DataDB().RewriteDataDB(); err != nil {
		return
	}
	*reply = utils.OK
	return
}

// DumpStorDB will dump all of stordb from memory to a file
func (apierSv1 *APIerSv1) DumpStorDB(ctx *context.Context, ignr *string, reply *string) (err error) {
	if err = apierSv1.StorDb.DumpStorDB(); err != nil {
		return
	}
	*reply = utils.OK
	return
}

// Will rewrite every dump file of StorDB
func (apierSv1 *APIerSv1) RewriteStorDB(ctx *context.Context, ignr *string, reply *string) (err error) {
	if err = apierSv1.StorDb.RewriteStorDB(); err != nil {
		return
	}
	*reply = utils.OK
	return
}

type DumpBackupParams struct {
	BackupFolderPath string // The path to the folder where the backup will be created
	Zip              bool   // creates a zip compressing the backup
}

// BackupDataDB will momentarely stop any dumping and rewriting in dataDB, until dump folder is backed up in folder path backupFolderPath. Making zip true will create a zip file in the path instead
func (apierSv1 *APIerSv1) BackupDataDB(ctx *context.Context, params DumpBackupParams, reply *string) (err error) {
	if err = apierSv1.DataManager.DataDB().BackupDataDB(params.BackupFolderPath, params.Zip); err != nil {
		return
	}
	*reply = utils.OK
	return
}

// BackupStorDB will momentarely stop any dumping and rewriting in storDB, until dump folder is backed up in folder path backupFolderPath. Making zip true will create a zip file in the path instead
func (apierSv1 *APIerSv1) BackupStorDB(ctx *context.Context, params DumpBackupParams, reply *string) (err error) {
	if err = apierSv1.StorDb.BackupStorDB(params.BackupFolderPath, params.Zip); err != nil {
		return
	}
	*reply = utils.OK
	return
}
