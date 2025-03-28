//go:build integration
// +build integration

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
package general_tests

import (
	"bytes"
	"testing"
	"time"

	"github.com/cgrates/birpc/context"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
)

func TestDmAsyncWrites(t *testing.T) {

	config := `{

"general": {
	"reply_timeout": "50s",
},
"logger": {
	"level": 7
},
"listen": {
	"rpc_json": ":2012",
	"rpc_gob": ":2013",
	"http": ":2080",
},
"data_db": {								
	"db_type": "redis",						
	"db_port": 6379, 						
	"db_name": "10", 
	"store_interval": "10s",						
},
"stor_db": {
	"db_password": "CGRateS.org"
},

"stats": {
	"enabled": true,
	"store_interval": "1s",
	"thresholds_conns": ["*internal"],
},
"thresholds": {
	"enabled": true,
	"store_interval": "1s",
},
"admins": {
	"enabled": true,
	"scheduler_conns": ["*internal"],
},
"rates": {
	"enabled": true
},
"actions": {
	"enabled": true,
	"accounts_conns": ["*localhost"]
},
"accounts": {
	"enabled": true
},

}
`
	ng := engine.TestEngine{
		ConfigJSON: config,
		Encoding:   *utils.Encoding,
		LogBuffer:  bytes.NewBuffer(nil),
	}
	client, _ := ng.Run(t)
	defer t.Log(ng.LogBuffer)

	t.Run("TestDmAsyncWrites", func(t *testing.T) {
		thp := &engine.ThresholdProfile{
			ID:      "THP1",
			Tenant:  "cgrates.org",
			MaxHits: 100,
			MinHits: 3,
		}
		var reply string
		if err := client.Call(context.Background(), utils.AdminSv1SetThresholdProfile, thp, &reply); err != nil {
			t.Error(err)
		} else if reply != utils.OK {
			t.Error("Expected OK, got ", reply)
		}
		var thp2 engine.ThresholdProfile
		if err := client.Call(context.Background(), utils.AdminSv1GetThresholdProfile, &utils.TenantID{Tenant: "cgrates.org", ID: "THP1"}, &thp2); err == nil || err.Error() != utils.ErrNotFound.Error() {
			t.Error(err)
		}
		time.Sleep(20 * time.Second)
		if err := client.Call(context.Background(), utils.AdminSv1GetThresholdProfile, &utils.TenantID{Tenant: "cgrates.org", ID: "THP1"}, &thp2); err != nil {
			t.Error(err)
		}
		if thp2.ID != "THP1" {
			t.Error("Expected THP1, got ", thp2.ID)
		}

	})
}
