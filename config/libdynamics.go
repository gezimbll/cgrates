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
package config

import (
	"slices"
	"strings"
	"time"

	"github.com/cgrates/cgrates/utils"
	"github.com/ericlagergren/decimal"
)

type DynamicStringSliceOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Values    []string
}

type DynamicStringOptJson struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     string
}

type DynamicStringOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	value     string
	dynVal    RSRParsers
}

type DynamicIntOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     int
}

type DynamicFloat64Opt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     float64
}

type DynamicBoolOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     bool
}

type DynamicDurationOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	value     time.Duration
	dynVal    RSRParsers
}

type DynamicDecimalOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	value     *decimal.Big
	dynVal    RSRParsers
}

type DynamicInterfaceOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     any
}

type DynamicIntPointerOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     *int
}

type DynamicDurationPointerOpt struct {
	FilterIDs []string `json:",omitempty"`
	Tenant    string
	Value     *time.Duration
}

func CloneDynamicStringSliceOpt(in []*DynamicStringSliceOpt) (cl []*DynamicStringSliceOpt) {
	cl = make([]*DynamicStringSliceOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicStringSliceOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Values:    slices.Clone(val.Values),
		}
	}
	return
}

func CloneDynamicStringOpt(in []*DynamicStringOptJson) (cl []*DynamicStringOptJson) {
	cl = make([]*DynamicStringOptJson, len(in))
	for i, val := range in {
		cl[i] = &DynamicStringOptJson{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     val.Value,
		}
	}
	return
}

func CloneDynamicStringOpt2(in []*DynamicStringOpt) (cl []*DynamicStringOpt) {
	cl = make([]*DynamicStringOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicStringOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			value:     val.value,
			dynVal:    val.dynVal.Clone(),
		}
	}
	return
}

func CloneDynamicInterfaceOpt(in []*DynamicInterfaceOpt) (cl []*DynamicInterfaceOpt) {
	cl = make([]*DynamicInterfaceOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicInterfaceOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     val.Value,
		}
	}
	return
}

func CloneDynamicBoolOpt(in []*DynamicBoolOpt) (cl []*DynamicBoolOpt) {
	cl = make([]*DynamicBoolOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicBoolOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     val.Value,
		}
	}
	return
}

func CloneDynamicIntOpt(in []*DynamicIntOpt) (cl []*DynamicIntOpt) {
	cl = make([]*DynamicIntOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicIntOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     val.Value,
		}
	}
	return
}

func CloneDynamicFloat64Opt(in []*DynamicFloat64Opt) (cl []*DynamicFloat64Opt) {
	cl = make([]*DynamicFloat64Opt, len(in))
	for i, val := range in {
		cl[i] = &DynamicFloat64Opt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     val.Value,
		}
	}
	return
}

func CloneDynamicDurationOpt(in []*DynamicDurationOpt) (cl []*DynamicDurationOpt) {
	cl = make([]*DynamicDurationOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicDurationOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			value:     val.value,
			dynVal:    val.dynVal.Clone(),
		}
	}
	return
}

func CloneDynamicDecimalOpt(in []*DynamicDecimalOpt) (cl []*DynamicDecimalOpt) {
	cl = make([]*DynamicDecimalOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicDecimalOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			value:     utils.CloneDecimalBig(val.value),
			dynVal:    val.dynVal.Clone(),
		}
	}
	return
}

func CloneDynamicIntPointerOpt(in []*DynamicIntPointerOpt) (cl []*DynamicIntPointerOpt) {
	cl = make([]*DynamicIntPointerOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicIntPointerOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     utils.IntPointer(*val.Value),
		}
	}
	return
}

func CloneDynamicDurationPointerOpt(in []*DynamicDurationPointerOpt) (cl []*DynamicDurationPointerOpt) {
	cl = make([]*DynamicDurationPointerOpt, len(in))
	for i, val := range in {
		cl[i] = &DynamicDurationPointerOpt{
			Tenant:    val.Tenant,
			FilterIDs: slices.Clone(val.FilterIDs),
			Value:     utils.DurationPointer(*val.Value),
		}
	}
	return
}

func DynamicStringSliceOptEqual(v1, v2 []*DynamicStringSliceOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if !slices.Equal(v1[i].Values, v2[i].Values) {
			return false
		}
	}
	return true
}

func DynamicStringOptEqual(v1, v2 []*DynamicStringOptJson) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].Value != v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicBoolOptEqual(v1, v2 []*DynamicBoolOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].Value != v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicIntOptEqual(v1, v2 []*DynamicIntOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].Value != v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicFloat64OptEqual(v1, v2 []*DynamicFloat64Opt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].Value != v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicDurationOptEqual(v1, v2 []*DynamicDurationOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].value != v2[i].value {
			return false
		}
		if v1[i].dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep) != v2[i].dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep) {
			return false
		}
	}
	return true
}

func DynamicDecimalOptEqual(v1, v2 []*DynamicDecimalOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) ||
			v1[i].value.Cmp(v2[i].value) != 0 {
			return false
		}
		if v1[i].dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep) != v2[i].dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep) {
			return false
		}
	}
	return true
}

func DynamicInterfaceOptEqual(v1, v2 []*DynamicInterfaceOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if v1[i].Value != v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicIntPointerOptEqual(v1, v2 []*DynamicIntPointerOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if *v1[i].Value != *v2[i].Value {
			return false
		}
	}
	return true
}

func DynamicDurationPointerOptEqual(v1, v2 []*DynamicDurationPointerOpt) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i := range v1 {
		if v1[i].Tenant != v2[i].Tenant {
			return false
		}
		if !slices.Equal(v1[i].FilterIDs, v2[i].FilterIDs) {
			return false
		}
		if *v1[i].Value != *v2[i].Value {
			return false
		}
	}
	return true
}

func StringToDecimalBigDynamicOpts(strOpts []*DynamicStringOptJson) (decOpts []*DynamicDecimalOpt, err error) {
	decOpts = make([]*DynamicDecimalOpt, len(strOpts))
	for index, opt := range strOpts {
		decOpts[index] = &DynamicDecimalOpt{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
		}
		if strings.HasPrefix(opt.Value, utils.DynamicDataPrefix) {
			decOpts[index].dynVal, err = NewRSRParsers(opt.Value, CgrConfig().GeneralCfg().RSRSep)
			if err != nil {
				return nil, err
			}
			continue
		}

		if decOpts[index].value, err = utils.StringAsBig(opt.Value); err != nil {
			return
		}
	}
	return
}

func DecimalToStringDynamicOpts(decOpts []*DynamicDecimalOpt) (strOpts []*DynamicStringOptJson) {
	strOpts = make([]*DynamicStringOptJson, len(decOpts))
	for index, opt := range decOpts {
		strOpts[index] = &DynamicStringOptJson{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
		}
		if opt.value == nil {
			strOpts[index].Value = opt.dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep)
			continue
		}
		strOpts[index].Value = opt.value.String()
	}
	return
}

func StringToDurationDynamicOpts(strOpts []*DynamicStringOptJson) (durOpts []*DynamicDurationOpt, err error) {
	durOpts = make([]*DynamicDurationOpt, len(strOpts))
	for index, opt := range strOpts {
		durOpts[index] = &DynamicDurationOpt{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
		}
		if strings.HasPrefix(opt.Value, utils.DynamicDataPrefix) {
			durOpts[index].dynVal, err = NewRSRParsers(opt.Value, CgrConfig().GeneralCfg().RSRSep)
			if err != nil {
				return nil, err
			}
			continue
		}
		if durOpts[index].value, err = utils.ParseDurationWithNanosecs(opt.Value); err != nil {
			return
		}
	}
	return
}

func DurationToStringDynamicOpts(durOpts []*DynamicDurationOpt) (strOpts []*DynamicStringOptJson) {
	strOpts = make([]*DynamicStringOptJson, len(durOpts))
	for index, opt := range durOpts {
		strOpts[index] = &DynamicStringOptJson{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
		}
		if opt.dynVal != nil {
			strOpts[index].Value = opt.dynVal.GetRule(CgrConfig().GeneralCfg().RSRSep)
			continue
		}
		strOpts[index].Value = opt.value.String()
	}
	return
}

func IntToIntPointerDynamicOpts(intOpts []*DynamicIntOpt) (intPtOpts []*DynamicIntPointerOpt) {
	intPtOpts = make([]*DynamicIntPointerOpt, len(intOpts))
	for index, opt := range intOpts {
		intPtOpts[index] = &DynamicIntPointerOpt{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
			Value:     utils.IntPointer(opt.Value),
		}
	}
	return
}

func IntPointerToIntDynamicOpts(intPtOpts []*DynamicIntPointerOpt) (intOpts []*DynamicIntOpt) {
	intOpts = make([]*DynamicIntOpt, len(intPtOpts))
	for index, opt := range intPtOpts {
		intOpts[index] = &DynamicIntOpt{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
			Value:     *opt.Value,
		}
	}
	return
}

func StringToDurationPointerDynamicOpts(strOpts []*DynamicStringOptJson) (durPtOpts []*DynamicDurationPointerOpt, err error) {
	durPtOpts = make([]*DynamicDurationPointerOpt, len(strOpts))
	for index, opt := range strOpts {
		var durOpt time.Duration
		if durOpt, err = utils.ParseDurationWithNanosecs(opt.Value); err != nil {
			return
		}
		durPtOpts[index] = &DynamicDurationPointerOpt{
			Tenant:    opt.Tenant,
			FilterIDs: opt.FilterIDs,
			Value:     utils.DurationPointer(durOpt),
		}
	}
	return
}

func DurationPointerToStringDynamicOpts(durPtOpts []*DynamicDurationPointerOpt) (strOpts []*DynamicStringOptJson) {
	strOpts = make([]*DynamicStringOptJson, len(durPtOpts))
	for index, opt := range durPtOpts {
		strOpts[index] = &DynamicStringOptJson{
			FilterIDs: opt.FilterIDs,
			Tenant:    opt.Tenant,
			Value:     (*opt.Value).String(),
		}
	}
	return
}

func NewDynamicDecimalOpt(filterIDs []string, tenant string, value *decimal.Big, dynValue RSRParsers) *DynamicDecimalOpt {
	return &DynamicDecimalOpt{
		FilterIDs: filterIDs,
		Tenant:    tenant,
		value:     value,
		dynVal:    dynValue,
	}
}

func (dynDec *DynamicDecimalOpt) Value(dP utils.DataProvider) (*decimal.Big, error) {
	if dynDec.value == nil {
		out, err := dynDec.dynVal.ParseDataProvider(dP)
		if err != nil {
			return nil, err
		}
		return utils.StringAsBig(out)
	}
	return dynDec.value, nil
}

func NewDynamicDurationOpt(filterIDs []string, tenant string, value time.Duration, dynValue RSRParsers) *DynamicDurationOpt {
	return &DynamicDurationOpt{
		FilterIDs: filterIDs,
		Tenant:    tenant,
		value:     value,
		dynVal:    dynValue,
	}
}

func (dynDur *DynamicDurationOpt) Value(dP utils.DataProvider) (time.Duration, error) {
	if dynDur.dynVal != nil {
		out, err := dynDur.dynVal.ParseDataProvider(dP)
		if err != nil {
			return 0, err
		}
		return utils.ParseDurationWithNanosecs(out)
	}
	return dynDur.value, nil
}

func JsonToDynamicStringOpts(in []*DynamicStringOptJson) (out []*DynamicStringOpt, err error) {
	out = make([]*DynamicStringOpt, len(in))
	for indx, opt := range in {
		out[indx] = &DynamicStringOpt{
			FilterIDs: opt.FilterIDs,
			Tenant:    opt.Tenant,
		}
		if strings.HasPrefix(opt.Value, utils.DynamicDataPrefix) {
			out[indx].dynVal, err = NewRSRParsers(CgrConfig().GeneralCfg().RSRSep, opt.Value)
			if err != nil {
				return
			}
			continue
		}
		out[indx].value = opt.Value
	}
	return
}

func (dynStr *DynamicStringOpt) Value(dP utils.DataProvider) (string, error) {
	if dynStr.dynVal != nil {
		out, err := dynStr.dynVal.ParseDataProvider(dP)
		if err != nil {
			return "", err
		}
		return out, nil
	}
	return dynStr.value, nil
}
