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
package sessions

import (
	"path"
	"testing"
	"time"

	"github.com/cgrates/birpc"
	"github.com/cgrates/birpc/context"
	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
)

var (
	dataCfgPath string
	dataCfgDIR  string
	dataCfg     *config.CGRConfig
	sDataRPC    *birpc.Client

	SessionsDataTests = []func(t *testing.T){
		testSessionsDataInitCfg,
		testSessionsDataResetDataDb,
		testSessionsDataResetStorDb,
		testSessionsDataStartEngine,
		testSessionsDataApierRpcConn,
		testSessionsDataTPFromFolder,
		testSessionsDataLastUsedData,
		testSessionsDataLastUsedMultipleUpdates,
		testSessionsDataTTLExpired,
		testSessionsDataTTLExpMultiUpdates,
		testSessionsDataMultipleDataNoUsage,
		testSessionsDataTTLUsageProtection,
		testSessionsDataTTLLastUsage,
		testSessionsDataTTKillEngine,
	}
)

// Tests starts here
func TestSessionsData(t *testing.T) {
	switch *utils.DBType {
	case utils.MetaInternal:
		dataCfgDIR = "sessions_internal"
	case utils.MetaMySQL:
		dataCfgDIR = "sessions_mysql"
	case utils.MetaMongo:
		dataCfgDIR = "sessions_mongo"
	case utils.MetaPostgres:
		t.SkipNow()
	default:
		t.Fatal("Unknown Database type")
	}
	for _, stest := range SessionsDataTests {
		t.Run(dataCfgDIR, stest)
	}
}

// Init config first
func testSessionsDataInitCfg(t *testing.T) {
	dataCfgPath = path.Join(*utils.DataDir, "conf", "samples", dataCfgDIR)
	var err error
	dataCfg, err = config.NewCGRConfigFromPath(dataCfgPath)
	if err != nil {
		t.Error(err)
	}
}

// Remove data in both rating and accounting db
func testSessionsDataResetDataDb(t *testing.T) {
	if err := engine.InitDataDB(dataCfg); err != nil {
		t.Fatal(err)
	}
}

// Wipe out the cdr database
func testSessionsDataResetStorDb(t *testing.T) {
	if err := engine.InitStorDb(dataCfg); err != nil {
		t.Fatal(err)
	}
}

// Start CGR Engine
func testSessionsDataStartEngine(t *testing.T) {
	if _, err := engine.StopStartEngine(dataCfgPath, *utils.WaitRater); err != nil {
		t.Fatal(err)
	}
}

// Connect rpc client to rater
func testSessionsDataApierRpcConn(t *testing.T) {
	var err error
	sDataRPC, err = newRPCClient(dataCfg.ListenCfg()) // We connect over JSON so we can also troubleshoot if needed
	if err != nil {
		t.Fatal(err)
	}
}

// Load the tariff plan, creating accounts and their balances
func testSessionsDataTPFromFolder(t *testing.T) {
	attrs := &utils.AttrLoadTpFromFolder{FolderPath: path.Join(*utils.DataDir, "tariffplans", "oldtutorial")}
	var loadInst utils.LoadInstance
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2LoadTariffPlanFromFolder, attrs, &loadInst); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*utils.WaitRater) * time.Millisecond) // Give time for scheduler to execute topups
}

func testSessionsDataLastUsedData(t *testing.T) {
	var acnt *engine.Account
	attrs := &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "1001"}
	eAcntVal := 102400.0
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f",
			eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}
	tStart, _ := utils.ParseTimeDetectLayout("2016-01-05T18:31:05Z", "")
	cd := engine.CallDescriptor{
		Category:    "data",
		Tenant:      "cgrates.org",
		Subject:     "1001",
		Account:     "1001",
		Destination: utils.MetaData,
		TimeStart:   tStart,
		TimeEnd:     tStart.Add(1024),
	}
	var cc engine.CallCost
	// Make sure the cost is what we expect to be for 1MB of data
	if err := sDataRPC.Call(context.Background(), utils.ResponderGetCost, &engine.CallDescriptorWithAPIOpts{CallDescriptor: &cd}, &cc); err != nil {
		t.Error("Got error on Responder.GetCost: ", err.Error())
	} else if cc.Cost != 1024 {
		t.Errorf("Calling Responder.GetCost got callcost: %v", cc.Cost)
	}

	usage := int64(5120)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedData",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123491",
				utils.AccountField: "1001",
				utils.Subject:      "1001",
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "5120", // 5MB
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Error(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 97280.0 // 100 -5
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f",
			eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}

	updateArgs := &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedData",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123491",
				utils.AccountField: "1001",
				utils.Subject:      "1001",
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "5120",
				utils.LastUsed:     "4096",
			},
		},
	}

	var updateRpl *V1UpdateSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Error(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 93184.0 // 100-9
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}

	termArgs := &V1TerminateSessionArgs{
		TerminateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedData",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123491",
				utils.AccountField: "1001",
				utils.Subject:      "1001",
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 49, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.LastUsed:     "0",
			},
		},
	}

	var rpl string
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1TerminateSession, termArgs, &rpl); err != nil || rpl != utils.OK {
		t.Error(err)
	}
	eAcntVal = 98304.0 //100-4
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, attrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f",
			eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}
}

func testSessionsDataLastUsedMultipleUpdates(t *testing.T) {
	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{Tenant: "cgrates.org",
		Account: "TestSessionsDataLastUsedMultipleData"}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "TestSessionsDataLastUsedMultipleData",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(6144)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedMultipleUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123492",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       acntAttrs.Tenant,
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "6144", // 5MB
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Error(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 96256 // 100-6
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}
	aSessions := make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		aSessions[0].Usage != 6144 {
		t.Errorf("wrong active sessions: %f", aSessions[0].Usage.Seconds())
	}

	usage = int64(8192)
	updateArgs := &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedMultipleUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123492",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       acntAttrs.Tenant,
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "8192", // 8 MB
				utils.LastUsed:     "7168",
				"Extra1":           "other",
			},
		},
	}

	var updateRpl *V1UpdateSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Error(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 87040.000000 // 15MB used
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		aSessions[0].Usage != 15360 {
		t.Errorf("wrong active sessions: %v", aSessions[0].Usage)
	} else if aSessions[0].ExtraFields["Extra1"] != "other" {
		t.Errorf("Expected: \"other\", received: %v", aSessions[0].ExtraFields["Extra1"])
	}

	usage = int64(1024)
	updateArgs = &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedMultipleUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123492",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       acntAttrs.Tenant,
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "1024", // 8 MB
				utils.LastUsed:     "5120", // 5 MB
				"Extra1":           "other2",
				"Extra2":           "other",
			},
		},
	}

	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Error(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 87040.000000 // the amount is not modified and there will be 1024 extra left in SMG
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		aSessions[0].Usage != 13312 { // 14MB in used, 2MB extra reserved
		t.Errorf("wrong active sessions: %+v", aSessions[0].Usage)
	} else if aSessions[0].ExtraFields["Extra1"] != "other2" {
		t.Errorf("Expected: \"other2\", received: %v", aSessions[0].ExtraFields["Extra1"])
	} else if _, has := aSessions[0].ExtraFields["Extra"]; has {
		t.Errorf("Expected: no Extra2, received: %v", aSessions[0].ExtraFields["Extra2"])
	}

	usage = int64(1024)
	updateArgs = &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedMultipleUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123492",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       acntAttrs.Tenant,
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "1024", // 8 MB
			},
		},
	}

	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Error(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 87040.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		aSessions[0].Usage != 14336 { // 14MB in use
		t.Errorf("wrong active sessions: %v", aSessions[0].Usage)
	}

	termArgs := &V1TerminateSessionArgs{
		TerminateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataLastUsedMultipleUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123492",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       acntAttrs.Tenant,
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 49, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.LastUsed:     "0", // refund 1024 (extra used) + 1024 (extra reserved)
				"Extra1":           "done",
			},
		},
	}

	var rpl string
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1TerminateSession, termArgs, &rpl); err != nil || rpl != utils.OK {
		t.Error(err)
	}

	eAcntVal = 89088.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions,
		new(utils.SessionFilter), &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1ProcessCDR, termArgs.CGREvent, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received reply: %s", reply)
	}

	time.Sleep(20 * time.Millisecond)

	var cdrs []*engine.ExternalCDR
	req := utils.RPCCDRsFilter{RunIDs: []string{utils.MetaDefault},
		Accounts: []string{acntAttrs.Account}}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetCDRs, &req, &cdrs); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if len(cdrs) != 1 {
		t.Error("Unexpected number of CDRs returned: ", len(cdrs))
	} else {
		if cdrs[0].Usage != "13312" {
			t.Errorf("Unexpected CDR Usage received, cdr: %v %+v ", cdrs[0].Usage, cdrs[0])
		}
		if cdrs[0].ExtraFields["Extra1"] != "done" {
			t.Errorf("Expected: \"done\", received: %v", cdrs[0].ExtraFields["Extra1"])
		}
	}
}

func testSessionsDataTTLExpired(t *testing.T) {
	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{Tenant: "cgrates.org",
		Account: "TestSessionsDataTTLExpired"}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "TestSessionsDataTTLExpired",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(1024)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataTTLExpired",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "TestSessionsDataTTLExpired",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "1024",
			},
			APIOpts: map[string]any{
				utils.OptsSessionsTTLUsage: "2048", // will be charged on TTL
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Error(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 101376.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	time.Sleep(70 * time.Millisecond)

	eAcntVal = 100352.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}

	// verify the cdr usage SessionTTLUsage ( 2048)
	var cdrs []*engine.ExternalCDR
	req := utils.RPCCDRsFilter{Accounts: []string{acntAttrs.Account}}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetCDRs, &req, &cdrs); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if len(cdrs) != 1 {
		t.Error("Unexpected number of CDRs returned: ", len(cdrs))
	} else {
		if cdrs[0].Usage != "2048" {
			t.Errorf("Unexpected CDR Usage received, cdr: %v %+v ", cdrs[0].Usage, cdrs[0])
		}
	}
}

func testSessionsDataTTLExpMultiUpdates(t *testing.T) {

	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{
		Tenant:  "cgrates.org",
		Account: "TestSessionsDataTTLExpMultiUpdates",
	}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "TestSessionsDataTTLExpMultiUpdates",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(4096)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataTTLExpMultiUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "4096", // 4MB
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // give some time to allow the session to be created
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 98304.000000 //96MB
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	aSessions := make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		int64(aSessions[0].Usage) != 4096 {
		t.Errorf("wrong active sessions: %d", int64(aSessions[0].Usage))
	}

	usage = int64(4096)
	updateArgs := &V1UpdateSessionArgs{
		UpdateSession: true,

		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataTTLExpMultiUpdates",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.LastUsed:     "1024",
				utils.Usage:        "4096",
			},
			APIOpts: map[string]any{
				utils.OptsSessionsTTLUsage:    "2048", // will be charged on TTL
				utils.OptsSessionsTTLLastUsed: "1024",
			},
		},
	}

	var updateRpl *V1UpdateSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Error(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 97280.000000 // 20480
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}
	time.Sleep(60 * time.Millisecond) // TTL will kick in

	eAcntVal = 100352.000000 // initial balance ( 102400 ) - SessionTTLUsage from update ( 2048 )
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions,
		new(utils.SessionFilter), &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}

	// verify the cdr usage SessionTTLUsage ( 2048)
	var cdrs []*engine.ExternalCDR
	req := utils.RPCCDRsFilter{Accounts: []string{acntAttrs.Account}}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetCDRs, &req, &cdrs); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if len(cdrs) != 1 {
		t.Error("Unexpected number of CDRs returned: ", len(cdrs))
	} else {
		if cdrs[0].Usage != "2048" {
			t.Errorf("Unexpected CDR Usage received, cdr: %v %+v ", cdrs[0].Usage, cdrs[0])
		}
	}
}

func testSessionsDataMultipleDataNoUsage(t *testing.T) {
	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{Tenant: "cgrates.org",
		Account: "TestSessionsDataTTLExpMultiUpdates"}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "TestSessionsDataTTLExpMultiUpdates",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(2048)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataMultipleDataNoUsage",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "2048",
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Fatal(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 100352.000000 // 1054720
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	aSessions := make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		int64(aSessions[0].Usage) != 2048 {
		t.Errorf("wrong active sessions usage: %d", int64(aSessions[0].Usage))
	}

	usage = int64(1024)
	updateArgs := &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataMultipleDataNoUsage",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "1024",
				utils.LastUsed:     "1024",
			},
			APIOpts: map[string]any{
				utils.OptsSessionsTTL: "0", // cancel timeout since usage 0 will not update it
			},
		},
	}

	var updateRpl *V1UpdateSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Fatal(err)
	}
	if updateRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 100352.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	aSessions = make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		int64(aSessions[0].Usage) != 2048 {
		t.Errorf("wrong active sessions usage: %d", int64(aSessions[0].Usage))
	}

	usage = int64(0)
	updateArgs = &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataMultipleDataNoUsage",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "0",
				utils.LastUsed:     "0",
			},
			APIOpts: map[string]any{
				utils.OptsSessionsTTL: "1h", // cancel timeout since usage 0 will not update it
			},
		},
	}

	updateRpl = new(V1UpdateSessionReply) // because gob doesn't overwrite 0 value fields
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1UpdateSession, updateArgs, &updateRpl); err != nil {
		t.Fatal(err)
	}
	if *utils.Encoding != utils.MetaGOB {
		if updateRpl.MaxUsage.Nanoseconds() != usage {
			t.Errorf("Expected: %+v, received: %+v", usage, updateRpl.MaxUsage.Nanoseconds())
		}
	} else {
		if updateRpl.MaxUsage != nil { // gob returns 0 values as nil
			t.Errorf("Expected: nil, received: %+v", updateRpl.MaxUsage)
		}
	}

	eAcntVal = 100352.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	aSessions = make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		int64(aSessions[0].Usage) != 1024 {
		t.Errorf("wrong active sessions usage: %d", int64(aSessions[0].Usage))
	}

	termArgs := &V1TerminateSessionArgs{
		TerminateSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataMultipleDataNoUsage",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 49, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.LastUsed:     "0",
			},
		},
	}

	var rpl string
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1TerminateSession, termArgs, &rpl); err != nil || rpl != utils.OK {
		t.Error(err)
	}

	eAcntVal = 101376.000000 // refunded last 1MB reserved and unused
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MetaData].GetTotalValue() != eAcntVal {
		t.Errorf("Expected: %f, received: %f",
			eAcntVal, acnt.BalanceMap[utils.MetaData].GetTotalValue())
	}
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions,
		new(utils.SessionFilter), &aSessions); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
}

// TestSessionsDataTTLUsageProtection makes sure that original TTL (50ms)
// limits the additional debit without overloading memory
func testSessionsDataTTLUsageProtection(t *testing.T) {
	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{Tenant: "cgrates.org",
		Account: "TestSessionsDataTTLUsageProtection"}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "TestSessionsDataTTLUsageProtection",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(2048)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataTTLUsageProtection",
			Event: map[string]any{
				utils.EventName:    "TEST_EVENT",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "123495",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 53, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "2048",
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Fatal(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 100352.000000 // 1054720
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	aSessions := make([]*ExternalSession, 0)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions, new(utils.SessionFilter), &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 ||
		int64(aSessions[0].Usage) != 2048 {
		t.Errorf("wrong active sessions usage: %d", int64(aSessions[0].Usage))
	}
	time.Sleep(60 * time.Millisecond)
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1GetActiveSessions,
		new(utils.SessionFilter), &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, utils.ToJSON(aSessions))
	}
}

func testSessionsDataTTLLastUsage(t *testing.T) {
	var acnt *engine.Account
	acntAttrs := &utils.AttrGetAccount{Tenant: "cgrates.org",
		Account: "testSessionsDataTTLLastUsage"}
	eAcntVal := 102400.0
	attrSetBalance := utils.AttrSetBalance{
		Tenant: acntAttrs.Tenant, Account: acntAttrs.Account,
		BalanceType: utils.MetaData,
		Value:       eAcntVal,
		Balance: map[string]any{
			utils.ID: "testSessionsDataTTLLastUsage",
		},
	}
	var reply string
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2SetBalance, attrSetBalance, &reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Received: %s", reply)
	}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if totalVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); totalVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, totalVal)
	}

	usage := int64(1024)
	initArgs := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: &utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSessionsDataTTLLastUsage",
			Event: map[string]any{
				utils.EventName:    "testSessionsDataTTLLastUsage",
				utils.ToR:          utils.MetaData,
				utils.OriginID:     "testSessionsDataTTLLastUsage",
				utils.AccountField: acntAttrs.Account,
				utils.Subject:      acntAttrs.Account,
				utils.Destination:  utils.MetaData,
				utils.Category:     "data",
				utils.Tenant:       "cgrates.org",
				utils.RequestType:  utils.MetaPrepaid,
				utils.SetupTime:    time.Date(2016, time.January, 5, 18, 30, 59, 0, time.UTC),
				utils.AnswerTime:   time.Date(2016, time.January, 5, 18, 31, 05, 0, time.UTC),
				utils.Usage:        "1024",
			},
			APIOpts: map[string]any{
				utils.OptsSessionsTTLLastUsage: "2048",
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := sDataRPC.Call(context.Background(), utils.SessionSv1InitiateSession,
		initArgs, &initRpl); err != nil {
		t.Error(err)
	}
	if initRpl.MaxUsage.Nanoseconds() != usage {
		t.Errorf("Expecting : %+v, received: %+v", usage, initRpl.MaxUsage.Nanoseconds())
	}

	eAcntVal = 101376.000000
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}
	time.Sleep(70 * time.Millisecond)

	eAcntVal = 99328.000000 // 101376.000000 ( units remains after init session ) - SessionTTLLastUsage ( 2048 )
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetAccount, acntAttrs, &acnt); err != nil {
		t.Error(err)
	} else if dataVal := acnt.BalanceMap[utils.MetaData].GetTotalValue(); dataVal != eAcntVal {
		t.Errorf("Expected: %f, received: %f", eAcntVal, dataVal)
	}

	// verify the cdr usage to be 3072 (  init usage ( 1024 ) + SessionTTLLastUsage ( 2048 ) )
	var cdrs []*engine.ExternalCDR
	req := utils.RPCCDRsFilter{Accounts: []string{acntAttrs.Account}}
	if err := sDataRPC.Call(context.Background(), utils.APIerSv2GetCDRs, &req, &cdrs); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if len(cdrs) != 1 {
		t.Error("Unexpected number of CDRs returned: ", len(cdrs))
	} else {
		if cdrs[0].Usage != "3072" {
			t.Errorf("Unexpected CDR Usage received, cdr: %v %+v ", cdrs[0].Usage, cdrs[0])
		}
	}
}

func testSessionsDataTTKillEngine(t *testing.T) {
	if err := engine.KillEngine(100); err != nil {
		t.Error(err)
	}
}
