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
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"unicode/utf8"

	"github.com/cgrates/cgrates/utils"
)

func NewTPExporter(storDb LoadStorage, tpID, expPath, fileFormat, sep string, compress bool) (*TPExporter, error) {
	if len(tpID) == 0 {
		return nil, errors.New("Missing TPid")
	}
	if utils.CSV != fileFormat {
		return nil, errors.New("Unsupported file format")
	}
	tpExp := &TPExporter{
		storDb:     storDb,
		tpID:       tpID,
		exportPath: expPath,
		fileFormat: fileFormat,
		compress:   compress,
		cacheBuff:  new(bytes.Buffer),
	}
	runeSep, _ := utf8.DecodeRuneInString(sep)
	if runeSep == utf8.RuneError {
		return nil, fmt.Errorf("Invalid field separator: %s", sep)
	} else {
		tpExp.sep = runeSep
	}
	if compress {
		if len(tpExp.exportPath) == 0 {
			tpExp.zipWritter = zip.NewWriter(tpExp.cacheBuff)
		} else {
			if fileOut, err := os.Create(path.Join(tpExp.exportPath, "tpexport.zip")); err != nil {
				return nil, err
			} else {
				tpExp.zipWritter = zip.NewWriter(fileOut)
			}
		}
	}
	return tpExp, nil
}

// Export TariffPlan to a folder
type TPExporter struct {
	storDb        LoadStorage   // StorDb connection handle
	tpID          string        // Load data on this tpid
	exportPath    string        // Directory path to export to
	fileFormat    string        // The file format <csv>
	sep           rune          // Separator in the csv file
	compress      bool          // Use ZIP to compress the folder
	cacheBuff     *bytes.Buffer // Will be written in case of no output folder is specified
	zipWritter    *zip.Writer   // Populated in case of needing to write zipped content
	exportedFiles []string
}

func (tpExp *TPExporter) Run() error {
	tpExp.removeFiles() // Make sure we clean the folder before starting with new one
	var withError bool
	toExportMap := make(map[string][]any)

	storDataTimings, err := tpExp.storDb.GetTPTimings(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpTiming))
		withError = true

	}
	if len(storDataTimings) != 0 {
		storDataModelTimings := APItoModelTimings(storDataTimings)
		toExportMap[utils.TimingsCsv] = make([]any, 0, len(storDataModelTimings))
		for _, sd := range storDataModelTimings {
			toExportMap[utils.TimingsCsv] = append(toExportMap[utils.TimingsCsv], sd)
		}
	}
	storDataDestinations, err := tpExp.storDb.GetTPDestinations(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpDestinations))
		withError = true
	}
	if len(storDataDestinations) != 0 {
		toExportMap[utils.DestinationsCsv] = make([]any, 0, len(storDataDestinations))
		for _, sd := range storDataDestinations {
			sdModels := APItoModelDestination(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.DestinationsCsv] = append(toExportMap[utils.DestinationsCsv], sdModel)
			}
		}
	}
	storDataRates, err := tpExp.storDb.GetTPRates(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpRates))
		withError = true
	}
	if len(storDataRates) != 0 {
		toExportMap[utils.RatesCsv] = make([]any, 0, len(storDataRates))
		for _, sd := range storDataRates {
			sdModels := APItoModelRate(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.RatesCsv] = append(toExportMap[utils.RatesCsv], sdModel)
			}
		}
	}
	storDataDestinationRates, err := tpExp.storDb.GetTPDestinationRates(tpExp.tpID, "", nil)
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpDestinationRates))
		withError = true
	}

	if len(storDataDestinationRates) != 0 {
		toExportMap[utils.DestinationRatesCsv] = make([]any, 0, len(storDataDestinationRates))
		for _, sd := range storDataDestinationRates {
			sdModels := APItoModelDestinationRate(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.DestinationRatesCsv] = append(toExportMap[utils.DestinationRatesCsv], sdModel)
			}
		}
	}

	storDataRatingPlans, err := tpExp.storDb.GetTPRatingPlans(tpExp.tpID, "", nil)
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpRatingPlans))
		withError = true
	}
	if len(storDataRatingPlans) != 0 {
		toExportMap[utils.RatingPlansCsv] = make([]any, 0, len(storDataRatingPlans))
		for _, sd := range storDataRatingPlans {
			sdModels := APItoModelRatingPlan(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.RatingPlansCsv] = append(toExportMap[utils.RatingPlansCsv], sdModel)
			}
		}
	}

	storDataRatingProfiles, err := tpExp.storDb.GetTPRatingProfiles(&utils.TPRatingProfile{TPid: tpExp.tpID})
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpRatingProfiles))
		withError = true
	}

	if len(storDataRatingProfiles) != 0 {
		toExportMap[utils.RatingProfilesCsv] = make([]any, 0, len(storDataRatingProfiles))
		for _, sd := range storDataRatingProfiles {
			sdModels := APItoModelRatingProfile(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.RatingProfilesCsv] = append(toExportMap[utils.RatingProfilesCsv], sdModel)
			}
		}
	}

	storDataSharedGroups, err := tpExp.storDb.GetTPSharedGroups(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpSharedGroups))
		withError = true
	}

	if len(storDataSharedGroups) != 0 {
		toExportMap[utils.SharedGroupsCsv] = make([]any, 0, len(storDataSharedGroups))
		for _, sd := range storDataSharedGroups {
			sdModels := APItoModelSharedGroup(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.SharedGroupsCsv] = append(toExportMap[utils.SharedGroupsCsv], sdModel)
			}
		}
	}

	storDataActions, err := tpExp.storDb.GetTPActions(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpActions))
		withError = true
	}

	if len(storDataActions) != 0 {
		toExportMap[utils.ActionsCsv] = make([]any, 0, len(storDataActions))
		for _, sd := range storDataActions {
			sdModels := APItoModelAction(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ActionsCsv] = append(toExportMap[utils.ActionsCsv], sdModel)
			}
		}
	}

	storDataActionPlans, err := tpExp.storDb.GetTPActionPlans(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpActionPlans))
		withError = true
	}
	if len(storDataActionPlans) != 0 {
		toExportMap[utils.ActionPlansCsv] = make([]any, 0, len(storDataActionPlans))
		for _, sd := range storDataActionPlans {
			sdModels := APItoModelActionPlan(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ActionPlansCsv] = append(toExportMap[utils.ActionPlansCsv], sdModel)
			}
		}
	}

	storDataActionTriggers, err := tpExp.storDb.GetTPActionTriggers(tpExp.tpID, "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpActionTriggers))
		withError = true
	}

	if len(storDataActionTriggers) != 0 {
		toExportMap[utils.ActionTriggersCsv] = make([]any, 0, len(storDataActionTriggers))
		for _, sd := range storDataActionTriggers {
			sdModels := APItoModelActionTrigger(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ActionTriggersCsv] = append(toExportMap[utils.ActionTriggersCsv], sdModel)
			}
		}
	}

	storDataAccountActions, err := tpExp.storDb.GetTPAccountActions(&utils.TPAccountActions{TPid: tpExp.tpID})
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpAccountActionsV))
		withError = true
	}
	if len(storDataAccountActions) != 0 {
		toExportMap[utils.AccountActionsCsv] = make([]any, 0, len(storDataAccountActions))
		for _, sd := range storDataAccountActions {
			sdModel := APItoModelAccountAction(sd)
			toExportMap[utils.AccountActionsCsv] = append(toExportMap[utils.AccountActionsCsv], sdModel)
		}
	}

	storDataResources, err := tpExp.storDb.GetTPResources(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpResources))
		withError = true
	}
	if len(storDataResources) != 0 {
		toExportMap[utils.ResourcesCsv] = make([]any, 0, len(storDataResources))
		for _, sd := range storDataResources {
			sdModels := APItoModelResource(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ResourcesCsv] = append(toExportMap[utils.ResourcesCsv], sdModel)
			}
		}
	}

	storDataIPs, err := tpExp.storDb.GetTPIPs(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpIPs))
		withError = true
	}
	if len(storDataIPs) != 0 {
		toExportMap[utils.IPsCsv] = make([]any, 0, len(storDataIPs))
		for _, sd := range storDataIPs {
			sdModels := APItoModelIP(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.IPsCsv] = append(toExportMap[utils.IPsCsv], sdModel)
			}
		}
	}

	storDataStats, err := tpExp.storDb.GetTPStats(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpStats))
		withError = true
	}
	if len(storDataStats) != 0 {
		toExportMap[utils.StatsCsv] = make([]any, 0, len(storDataStats))
		for _, sd := range storDataStats {
			sdModels := APItoModelStats(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.StatsCsv] = append(toExportMap[utils.StatsCsv], sdModel)
			}
		}
	}

	storDataTrends, err := tpExp.storDb.GetTPTrends(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s,when getting %s from stordb for export", utils.ApierS, err, utils.TpTrends))
	}
	if len(storDataTrends) != 0 {
		toExportMap[utils.TrendsCsv] = make([]any, len(storDataTrends))
		for _, sd := range storDataTrends {
			sModels := APItoModelTrends(sd)
			for _, sdModel := range sModels {
				toExportMap[utils.TrendsCsv] = append(toExportMap[utils.TrendsCsv], sdModel)
			}
		}
	}

	storDataRankings, err := tpExp.storDb.GetTPRankings(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s,when getting %s from stordb for export", utils.ApierS, err, utils.TpRankings))
	}
	if len(storDataRankings) != 0 {
		toExportMap[utils.RankingsCsv] = make([]any, 0, len(storDataRankings))
		for _, sd := range storDataRankings {
			sdModels := APItoModelTPRanking(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.RankingsCsv] = append(toExportMap[utils.RankingsCsv], sdModel)
			}
		}
	}

	storDataThresholds, err := tpExp.storDb.GetTPThresholds(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpThresholds))
		withError = true
	}

	if len(storDataThresholds) != 0 {
		toExportMap[utils.ThresholdsCsv] = make([]any, 0, len(storDataThresholds))
		for _, sd := range storDataThresholds {
			sdModels := APItoModelTPThreshold(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ThresholdsCsv] = append(toExportMap[utils.ThresholdsCsv], sdModel)
			}
		}
	}

	storDataFilters, err := tpExp.storDb.GetTPFilters(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpFilters))
		withError = true
	}

	if len(storDataFilters) != 0 {
		toExportMap[utils.FiltersCsv] = make([]any, 0, len(storDataFilters))
		for _, sd := range storDataFilters {
			sdModels := APItoModelTPFilter(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.FiltersCsv] = append(toExportMap[utils.FiltersCsv], sdModel)
			}
		}
	}

	storDataRoutes, err := tpExp.storDb.GetTPRoutes(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpRoutes))
		withError = true
	}

	if len(storDataRoutes) != 0 {
		toExportMap[utils.RoutesCsv] = make([]any, 0, len(storDataRoutes))
		for _, sd := range storDataRoutes {
			sdModels := APItoModelTPRoutes(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.RoutesCsv] = append(toExportMap[utils.RoutesCsv], sdModel)
			}
		}
	}

	storeDataAttributes, err := tpExp.storDb.GetTPAttributes(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpAttributes))
		withError = true
	}

	if len(storeDataAttributes) != 0 {
		toExportMap[utils.AttributesCsv] = make([]any, 0, len(storeDataAttributes))
		for _, sd := range storeDataAttributes {
			sdModels := APItoModelTPAttribute(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.AttributesCsv] = append(toExportMap[utils.AttributesCsv], sdModel)
			}
		}
	}

	storDataChargers, err := tpExp.storDb.GetTPChargers(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpChargers))
		withError = true
	}

	if len(storDataChargers) != 0 {
		toExportMap[utils.ChargersCsv] = make([]any, 0, len(storDataChargers))
		for _, sd := range storDataChargers {
			sdModels := APItoModelTPCharger(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.ChargersCsv] = append(toExportMap[utils.ChargersCsv], sdModel)
			}
		}
	}

	storDataDispatcherProfiles, err := tpExp.storDb.GetTPDispatcherProfiles(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpDispatcherProfiles))
		withError = true
	}
	if len(storDataDispatcherProfiles) != 0 {
		toExportMap[utils.DispatcherProfilesCsv] = make([]any, 0, len(storDataDispatcherProfiles))
		for _, sd := range storDataDispatcherProfiles {
			sdModels := APItoModelTPDispatcherProfile(sd)
			for _, sdModel := range sdModels {
				toExportMap[utils.DispatcherProfilesCsv] = append(toExportMap[utils.DispatcherProfilesCsv], sdModel)
			}
		}
	}

	storDataDispatcherHosts, err := tpExp.storDb.GetTPDispatcherHosts(tpExp.tpID, "", "")
	if err != nil && err.Error() != utils.ErrNotFound.Error() {
		utils.Logger.Warning(fmt.Sprintf("<%s> error: %s, when getting %s from stordb for export", utils.ApierS, err, utils.TpDispatcherHosts))
		withError = true
	}

	if len(storDataDispatcherHosts) != 0 {
		toExportMap[utils.DispatcherHostsCsv] = make([]any, 0, len(storDataDispatcherHosts))
		for _, sd := range storDataDispatcherHosts {
			toExportMap[utils.DispatcherHostsCsv] = append(toExportMap[utils.DispatcherHostsCsv], APItoModelTPDispatcherHost(sd))
		}
	}

	if len(toExportMap) == 0 { // if we don't have anything to export we return not found error
		return utils.ErrNotFound
	}

	for fileName, storData := range toExportMap {
		if err := tpExp.writeOut(fileName, storData); err != nil {
			tpExp.removeFiles()
			return err
		}
		tpExp.exportedFiles = append(tpExp.exportedFiles, fileName)
	}

	if tpExp.compress {
		if err := tpExp.zipWritter.Close(); err != nil {
			return err
		}
	}
	if withError { // if we export something but have error we return partially executed
		return utils.ErrPartiallyExecuted
	}
	return nil
}

// Some export did not end up well, remove the files here
func (tpExp *TPExporter) removeFiles() error {
	if len(tpExp.exportPath) == 0 {
		return nil
	}
	for _, fileName := range tpExp.exportedFiles {
		os.Remove(path.Join(tpExp.exportPath, fileName))
	}
	return nil
}

// General method to write the content out to a file on path or zip archive
func (tpExp *TPExporter) writeOut(fileName string, tpData []any) error {
	if len(tpData) == 0 {
		return nil
	}
	var fWriter io.Writer
	var writerOut utils.CgrRecordWriter
	var err error

	if tpExp.compress {
		if fWriter, err = tpExp.zipWritter.Create(fileName); err != nil {
			return err
		}
	} else if len(tpExp.exportPath) != 0 {
		if f, err := os.Create(path.Join(tpExp.exportPath, fileName)); err != nil {
			return err
		} else {
			fWriter = f
			defer f.Close()
		}

	} else {
		fWriter = new(bytes.Buffer)
	}

	switch tpExp.fileFormat {
	case utils.CSV:
		csvWriter := csv.NewWriter(fWriter)
		csvWriter.Comma = tpExp.sep
		writerOut = csvWriter
	default:
		writerOut = utils.NewCgrIORecordWriter(fWriter)
	}
	for _, tpItem := range tpData {
		record, err := CsvDump(tpItem)
		if err != nil {
			return err
		}
		if err := writerOut.Write(record); err != nil {
			return err
		}
	}
	writerOut.Flush() // In case of .csv will dump data on hdd
	return nil
}

func (tpExp *TPExporter) ExportStats() *utils.ExportedTPStats {
	return &utils.ExportedTPStats{ExportPath: tpExp.exportPath, ExportedFiles: tpExp.exportedFiles, Compressed: tpExp.compress}
}

func (tpExp *TPExporter) GetCacheBuffer() *bytes.Buffer {
	return tpExp.cacheBuff
}
