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

package tpes

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/cgrates/birpc/context"

	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
)

type TPDispatchers struct {
	dm *engine.DataManager
}

// newTPDispatchers is the constructor for TPDispatchers
func newTPDispatchers(dm *engine.DataManager) *TPDispatchers {
	return &TPDispatchers{
		dm: dm,
	}
}

// exportItems for TPDispatchers will implement the method for tpExporter interface
func (tpDsp TPDispatchers) exportItems(ctx *context.Context, wrtr io.Writer, tnt string, itmIDs []string) (err error) {
	csvWriter := csv.NewWriter(wrtr)
	csvWriter.Comma = utils.CSVSep
	// before writing the profiles, we must write the headers
	if err = csvWriter.Write([]string{"#Tenant", "ID", "FilterIDs", "Weight", "Strategy", "StrategyParameters", "ConnID", "ConnFilterIDs", "ConnWeight", "ConnBlocker", "ConnParameters"}); err != nil {
		return
	}
	for _, dspID := range itmIDs {
		var dspPrf *engine.DispatcherProfile
		dspPrf, err = tpDsp.dm.GetDispatcherProfile(ctx, tnt, dspID, true, true, utils.NonTransactional)
		if err != nil {
			if err.Error() == utils.ErrNotFound.Error() {
				return fmt.Errorf("<%s> cannot find DispatcherProfile with id: <%v>", err, dspID)
			}
			return err
		}
		dspMdls := engine.APItoModelTPDispatcherProfile(engine.DispatcherProfileToAPI(dspPrf))
		if len(dspMdls) == 0 {
			return
		}
		// for every profile, convert it into model to be compatible in csv format
		for _, tpItem := range dspMdls {
			// transform every record into a []string
			var record []string
			record, err = engine.CsvDump(tpItem)
			if err != nil {
				return err
			}
			// record is a line of a csv file
			if err = csvWriter.Write(record); err != nil {
				return err
			}
		}
	}
	csvWriter.Flush()
	return
}