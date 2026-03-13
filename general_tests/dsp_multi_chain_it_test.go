//go:build integration

/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>
*/

package general_tests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/cgrates/birpc/context"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
)

func TestHttpAgent(t *testing.T) {
	cfgHA2 := `{
"listen": {
	"rpc_json": ":2012",
	"rpc_gob": ":2013",
	"http": ":2080",
},
"attributes": {
	"enabled":true,
},
"thresholds":{
	"enabled": true
},
"data_db":{
	"db_type":"*internal"
},
"stor_db":{
	"db_type":"*internal"
},
"sessions": {
	"enabled": true,
	"attributes_conns": ["*localhost"],
	"thresholds_conns": ["*localhost"],
},
"apiers": {
"enabled": true,
},
"http_agent": [
	{
		"id": "HTTPAgent",
		"url": "/ussd/wasp/",
		"sessions_conns": ["*localhost"],
		"request_payload": "*url",
		"reply_payload": "*text_plain",
		"request_processors": [
			{
				"id": "message",
				"filters": ["*string:~*req.imsi:6550702345678"],
				"flags": ["*message", "*attributes","*thresholds"],
				"request_fields": [
					{"tag": "IMSI", "path": "*cgreq.IMSI", "type": "*variable", "value": "~*req.imsi"},
					{"tag": "VoucherID", "path": "*cgreq.VoucherID", "type": "*constant", "value": "101"},
					{"tag": "ServiceID", "path": "*cgreq.ServiceID", "type": "*constant", "value": "123456"},
				],
				"reply_fields": [
					{"tag": "Message", "path": "*rep.Message", "type": "*constant", "value": "OK"}
				]
			},
		]
	}
]
}`

	// Tariff plan files with dispatcher profiles for both DSP1 and DSP2
	tpFiles := map[string]string{
		utils.AttributesCsv: `#Tenant,ID,Context,FilterIDs,ActivationInterval,AttributeFilterIDs,Path,Type,Value,Blocker,Weight
cgrates.org,Attr_Recharge_Prf,,*string:~*req.IMSI:6550702345678,,,,,,,10
cgrates.org,Attr_Recharge_Prf,,,,,*req.RechargeProfile,*constant,MONETARY_10,,
cgrates.org,Attr_Recharge_Prf,,,,,*req.Account,*variable,~*req.IMSI,,`,
		utils.ActionsCsv: `#ActionsId[0],Action[1],ExtraParameters[2],Filter[3],BalanceId[4],BalanceType[5],Categories[6],DestinationIds[7],RatingSubject[8],SharedGroup[9],ExpiryTime[10],TimingIds[11],Units[12],BalanceWeight[13],BalanceBlocker[14],BalanceDisabled[15],Weight[16]
ACT_TOPUP,*topup,,,monetary1,*monetary,,,,,,,10,,,,`,
		utils.ThresholdsCsv: `#Tenant[0],Id[1],FilterIDs[2],ActivationInterval[3],MaxHits[4],MinHits[5],MinSleep[6],Blocker[7],Weight[8],ActionIDs[9],Async[10],EeIDs[11]
cgrates.org,THD_TOPUP,*string:~*req.Account:6550702345678,,-1,1,0,false,,ACT_TOPUP,false,`,
	}

	ngHA2 := engine.TestEngine{
		ConfigJSON: cfgHA2,
		TpFiles:    tpFiles,
		LogBuffer:  bytes.NewBuffer(nil),
	}
	t.Cleanup(func() { t.Log(ngHA2.LogBuffer) })
	client, _ := ngHA2.Run(t)
	var reply string
	attrs := &utils.AttrSetAccount{
		Tenant:          "cgrates.org",
		Account:         "6550702345678",
		ReloadScheduler: true,
	}
	if err := client.Call(context.Background(), utils.APIerSv1SetAccount, attrs, &reply); err != nil {
		t.Error("Got error on APIerSv1.SetAccount: ", err.Error())
	} else if reply != utils.OK {
		t.Errorf("Calling APIerSv1.SetAccount received: %s", reply)
	}

	httpC := &http.Client{}

	resp, err := httpC.Get(`http://127.0.0.1:2080/ussd/wasp?msisdn=27849991234&tid=16404714&reqtype=1&imsi=6550702345678&ussdstring=*101*1234567890123456%%23`)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP request returned %d: %s", resp.StatusCode, string(body))
	}
	t.Log(string(body))
	resp2, err := httpC.Get(`http://127.0.0.1:2080/ussd/wasp?msisdn=27849991234&tid=16404714&reqtype=1&imsi=6550702345678&ussdstring=*101*1234567890123456%%23`)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("HTTP request returned %d: %s", resp2.StatusCode, string(body2))
	}
	t.Log(string(body2))

	var acnt engine.Account
	if err := client.Call(context.Background(), utils.APIerSv2GetAccount,
		&utils.AttrGetAccount{
			Tenant:  "cgrates.org",
			Account: "6550702345678",
		}, &acnt); err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	fmt.Println(utils.ToJSON(acnt))
	// if bal := acnt.BalanceMap[utils.MetaMonetary][0]; bal == nil {
	// 	t.Errorf("balance not found for account %v", acnt)
	// } else if bal.Value != 10 {
	// 	t.Errorf("account %v balance = %v, want %v", acnt, bal.Value, 10)
	// }

}
