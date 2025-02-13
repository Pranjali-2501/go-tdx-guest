// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package validate provides the library functions to validate a TDX quote
package validate

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/google/go-tdx-guest/abi"
	cpb "github.com/google/go-tdx-guest/proto/check"
	pb "github.com/google/go-tdx-guest/proto/tdx"
	vr "github.com/google/go-tdx-guest/verify"
	"go.uber.org/multierr"
)

const (
	// If bit X is 1 in xfamFixed1, it must be 1 in any xfam.
	xfamFixed1 = 0x00000003
	// If bit X is 0 in xfamFixed0, it must be 0 in any xfam.
	xfamFixed0 = 0x0006DBE7
	// If bit X is 1 in tdAttributesFixed1, it must be 1 in any tdAttributes.
	tdAttributesFixed1            = 0x0
	tdxAttributesSeptVeDisSupport = 1 << 28
	tdxAttributesPksSupport       = 1 << 30
	tdxAttributesPerfmonSupport   = 1 << 63
	// Supported ATTRIBUTES bits depend on the supported features - bits 0 (DEBUG), 30 (PKS), 63 (PERFMON)
	// and 28 (SEPT VE DISABLE)
	// If bit X is 0 in tdAttributesFixed0, it must be 0 in any tdAttributes.
	tdAttributesFixed0 = 0x1 | tdxAttributesSeptVeDisSupport | tdxAttributesPksSupport | tdxAttributesPerfmonSupport
	rtmrsCount         = 4
)

// Options represents validation options for a TDX attestation Quote.
type Options struct {
	HeaderOptions      HeaderOptions
	TdQuoteBodyOptions TdQuoteBodyOptions
}

// HeaderOptions represents validation options for a TDX attestation Quote Header.
type HeaderOptions struct {
	// MinimumQeSvn is the minimum QE security version number. Not checked if nil.
	MinimumQeSvn uint16
	// MinimumPceSvn is the minimum PCE security version number. Not checked if nil.
	MinimumPceSvn uint16
	// QeVendorID is the expected QE_VENDOR_ID field. Must be nil or 16 bytes long. Not checked if nil.
	QeVendorID []byte
}

// TdQuoteBodyOptions represents validation options for a TDX attestation Quote's TD Quote body.
type TdQuoteBodyOptions struct {
	// MinimumTeeTcbSvn is the component-wise minimum TEE_TCB security version number. Must be nil or 16 bytes long. Not checked if nil.
	MinimumTeeTcbSvn []byte
	// MrSeam is the expected MR_SEAM field. Must be nil or 48 bytes long. Not checked if nil.
	MrSeam []byte
	// TdAttributes is the expected TD_ATTRIBUTES field. Must be nil or 8 bytes long. Not checked if nil.
	TdAttributes []byte
	// Xfam is the expected XFAM field. Must be nil or 8 bytes long. Not checked if nil.
	Xfam []byte
	// MrTd is the expected MR_TD field. Must be nil or 48 bytes long. Not checked if nil.
	MrTd []byte
	// MrConfigID is the expected MR_CONFIG_ID field. Must be nil or 48 bytes long. Not checked if nil.
	MrConfigID []byte
	// MrOwner is the expected MR_OWNER field. Must be nil or 48 bytes long. Not checked if nil.
	MrOwner []byte
	// MrOwnerConfig is the expected MR_OWNER_CONFIG field. Must be nil or 48 bytes long. Not checked if nil.
	MrOwnerConfig []byte
	// Rtmrs is the expected RTMRS field. Must be nil or 48*rtmrsCount. Not checked if nil.
	Rtmrs [][]byte
	// ReportData is the expected REPORT_DATA field. Must be nil or 64 bytes long. Not checked if nil.
	ReportData []byte
}

func lengthCheck(name string, length int, value []byte) error {
	if value != nil && len(value) != length {
		return fmt.Errorf("option %q length is %d. Want %d", name, len(value), length)
	}
	return nil
}

func checkOptionsLengths(opts *Options) error {
	return multierr.Combine(
		lengthCheck("mr_seam", abi.MrSeamSize, opts.TdQuoteBodyOptions.MrSeam),
		lengthCheck("td_attributes", abi.TdAttributesSize, opts.TdQuoteBodyOptions.TdAttributes),
		lengthCheck("xfam", abi.XfamSize, opts.TdQuoteBodyOptions.Xfam),
		lengthCheck("mr_td", abi.MrTdSize, opts.TdQuoteBodyOptions.MrTd),
		lengthCheck("mr_config_id", abi.MrConfigIDSize, opts.TdQuoteBodyOptions.MrConfigID),
		lengthCheck("mr_owner", abi.MrOwnerSize, opts.TdQuoteBodyOptions.MrOwner),
		lengthCheck("mr_owner_config", abi.MrOwnerConfigSize, opts.TdQuoteBodyOptions.MrOwnerConfig),
		lengthCheck("rtmr", abi.RtmrSize, opts.TdQuoteBodyOptions.Rtmrs[0]),
		lengthCheck("rtmr", abi.RtmrSize, opts.TdQuoteBodyOptions.Rtmrs[1]),
		lengthCheck("rtmr", abi.RtmrSize, opts.TdQuoteBodyOptions.Rtmrs[2]),
		lengthCheck("rtmr", abi.RtmrSize, opts.TdQuoteBodyOptions.Rtmrs[3]),
		lengthCheck("report_data", abi.ReportDataSize, opts.TdQuoteBodyOptions.ReportData),
		lengthCheck("qe_vendor_id", abi.QeVendorIDSize, opts.HeaderOptions.QeVendorID),
	)
}

// PolicyToOptions returns an Options object that is represented by a Policy message.
func PolicyToOptions(policy *cpb.Policy) (*Options, error) {
	opts := &Options{
		HeaderOptions: HeaderOptions{
			MinimumQeSvn:  uint16(policy.GetHeaderPolicy().GetMinmumQeSvn()),
			MinimumPceSvn: uint16(policy.GetHeaderPolicy().GetMinimumPceSvn()),
			QeVendorID:    policy.GetHeaderPolicy().GetQeVendorId(),
		},
		TdQuoteBodyOptions: TdQuoteBodyOptions{
			MinimumTeeTcbSvn: policy.GetTdQuoteBodyPolicy().GetMinimumTeeTcbSvn(),
			MrSeam:           policy.GetTdQuoteBodyPolicy().GetMrSeam(),
			TdAttributes:     policy.GetTdQuoteBodyPolicy().GetTdAttributes(),
			Xfam:             policy.GetTdQuoteBodyPolicy().GetXfam(),
			MrTd:             policy.GetTdQuoteBodyPolicy().GetMrTd(),
			MrConfigID:       policy.GetTdQuoteBodyPolicy().GetMrConfigId(),
			MrOwner:          policy.GetTdQuoteBodyPolicy().GetMrOwner(),
			MrOwnerConfig:    policy.GetTdQuoteBodyPolicy().GetMrOwnerConfig(),
			Rtmrs:            policy.GetTdQuoteBodyPolicy().GetRtmrs(),
			ReportData:       policy.GetTdQuoteBodyPolicy().GetReportData(),
		},
	}
	if err := checkOptionsLengths(opts); err != nil {
		return nil, err
	}
	return opts, nil
}

func byteCheckRtmr(field string, size int, given, required [][]byte) error {
	if len(required) == 0 {
		return nil
	}
	if len(required) != rtmrsCount {
		return fmt.Errorf("RTMR field size(%d) is not equal to expected size(4)", len(required))
	}
	for i := range required {
		if len(required[i]) != size {
			return fmt.Errorf("RTMR[%d] should be 48 bytes, found %d", i, len(required[i]))
		}
		if !bytes.Equal(required[i], given[i]) {
			return fmt.Errorf("quote field %s is %s. Expect %s",
				field, hex.EncodeToString(given[i]), hex.EncodeToString(required[i]))
		}
	}
	return nil
}

func byteCheck(option, field string, size int, given, required []byte) error {
	if len(required) == 0 {
		return nil
	}
	if len(required) != size {
		return fmt.Errorf("option %v must be nil or %d bytes", option, size)
	}
	if !bytes.Equal(required, given) {
		return fmt.Errorf("quote field %s is %s. Expect %s",
			field, hex.EncodeToString(given), hex.EncodeToString(required))
	}
	return nil
}

func exactByteMatch(quote *pb.QuoteV4, opts *Options) error {
	givenRtmr := quote.GetTdQuoteBody().GetRtmrs()
	return multierr.Combine(
		byteCheck("MrSeam", "MR_SEAM", abi.MrSeamSize, quote.GetTdQuoteBody().GetMrSeam(), opts.TdQuoteBodyOptions.MrSeam),
		byteCheck("TdAttributes", "TD_ATTRIBUTES", abi.TdAttributesSize, quote.GetTdQuoteBody().GetTdAttributes(), opts.TdQuoteBodyOptions.TdAttributes),
		byteCheck("Xfam", "XFAM", abi.XfamSize, quote.GetTdQuoteBody().GetXfam(), opts.TdQuoteBodyOptions.Xfam),
		byteCheck("MrTd", "MR_TD", abi.MrTdSize, quote.GetTdQuoteBody().GetMrTd(), opts.TdQuoteBodyOptions.MrTd),
		byteCheck("MrConfigID", "MR_CONFIG_ID", abi.MrConfigIDSize, quote.GetTdQuoteBody().GetMrConfigId(), opts.TdQuoteBodyOptions.MrConfigID),
		byteCheck("MrOwner", "MR_OWNER", abi.MrOwnerSize, quote.GetTdQuoteBody().GetMrOwner(), opts.TdQuoteBodyOptions.MrOwner),
		byteCheck("MrOwnerConfig", "MR_OWNER_CONFIG", abi.MrOwnerConfigSize, quote.GetTdQuoteBody().GetMrOwnerConfig(), opts.TdQuoteBodyOptions.MrOwnerConfig),
		byteCheckRtmr("RTMRS", abi.RtmrSize, givenRtmr, opts.TdQuoteBodyOptions.Rtmrs),
		byteCheck("ReportData", "REPORT_DATA", abi.ReportDataSize, quote.GetTdQuoteBody().GetReportData(), opts.TdQuoteBodyOptions.ReportData),
		byteCheck("QeVendorID", "QE_VENDOR_ID", abi.QeVendorIDSize, quote.GetHeader().GetQeVendorId(), opts.HeaderOptions.QeVendorID),
	)
}

func isSvnHigherOrEqual(quoteSvn []byte, optionSvn []byte) bool {
	if optionSvn == nil {
		return true
	}
	for i := range quoteSvn {
		if quoteSvn[i] < optionSvn[i] {
			return false
		}
	}
	return true
}

func minVersionCheck(quote *pb.QuoteV4, opts *Options) error {
	if !isSvnHigherOrEqual(quote.GetTdQuoteBody().GetTeeTcbSvn(), opts.TdQuoteBodyOptions.MinimumTeeTcbSvn) {
		return fmt.Errorf("TEE TCB security-version number %d is less than the required minimum %d",
			quote.GetTdQuoteBody().GetTeeTcbSvn(), opts.TdQuoteBodyOptions.MinimumTeeTcbSvn)
	}
	qeSvn := uint16(binary.LittleEndian.Uint16(quote.GetHeader().GetQeSvn()))
	pceSvn := uint16(binary.LittleEndian.Uint16(quote.GetHeader().GetPceSvn()))
	if qeSvn < opts.HeaderOptions.MinimumQeSvn {
		return fmt.Errorf("QE security-version number %d is less than the required minimum %d",
			qeSvn, opts.HeaderOptions.MinimumQeSvn)
	}
	if pceSvn < opts.HeaderOptions.MinimumPceSvn {
		return fmt.Errorf("PCE security-version number %d is less than the required minimum %d",
			pceSvn, opts.HeaderOptions.MinimumPceSvn)
	}
	return nil
}

func validateXfam(value []byte, fixed1, fixed0 uint64) error {
	if len(value) == 0 {
		return nil
	}
	if len(value) != abi.XfamSize {
		return fmt.Errorf("xfam size is invalid")
	}
	xfam := binary.LittleEndian.Uint64(value[:])
	if xfam&fixed1 != fixed1 {
		return fmt.Errorf("unauthorized xfam 0x%x as xfamFixed1 0x%x bits are unset", xfam, fixed1)
	}
	if xfam&(^fixed0) != 0 {
		return fmt.Errorf("unauthorized xfam 0x%x as xfamFixed0 0x%x bits are set", xfam, fixed0)
	}
	return nil
}

func validateTdAttributes(value []byte, fixed1, fixed0 uint64) error {
	if len(value) == 0 {
		return nil
	}
	if len(value) != abi.TdAttributesSize {
		return fmt.Errorf("tdAttributes size is invalid")
	}
	tdAttributes := binary.LittleEndian.Uint64(value[:])
	if tdAttributes&fixed1 != fixed1 {
		return fmt.Errorf("unauthorized tdAttributes 0x%x as tdAttributesFixed1 0x%x bits are unset", tdAttributes, fixed1)
	}
	if tdAttributes&(^fixed0) != 0 {
		return fmt.Errorf("unauthorized tdAttributes 0x%x as tdAttributesFixed0 0x%x bits are set", tdAttributes, fixed0)
	}
	return nil
}

// TdxQuote validates fields of the protobuf representation of an attestation Quote against
// expectations. Does not check the attestation certificates or signature.
func TdxQuote(quote *pb.QuoteV4, options *Options) error {
	if options == nil {
		return vr.ErrOptionsNil
	}
	if err := abi.CheckQuoteV4(quote); err != nil {
		return fmt.Errorf("QuoteV4 invalid: %v", err)
	}
	return multierr.Combine(
		exactByteMatch(quote, options),
		minVersionCheck(quote, options),
		validateXfam(quote.GetTdQuoteBody().GetXfam(), xfamFixed1, xfamFixed0),
		validateTdAttributes(quote.GetTdQuoteBody().GetTdAttributes(), tdAttributesFixed1, tdAttributesFixed0),
	)
}

// RawTdxQuoteValidate checks the raw bytes representation of an attestation quote.
//
// Deprecated: Use RawTdxQuote instead. This function is no longer recommended for use.
func RawTdxQuoteValidate(raw []byte, options *Options) error {
	quote, err := abi.QuoteToProto(raw)
	if err != nil {
		return fmt.Errorf("could not convert raw bytes to QuoteV4: %v", err)
	}
	return TdxQuote(quote, options)
}

// RawTdxQuote checks the raw bytes representation of an attestation quote.
func RawTdxQuote(raw []byte, options *Options) error {
	quote, err := abi.QuoteToProto(raw)
	if err != nil {
		return fmt.Errorf("could not convert raw bytes to QuoteV4: %v", err)
	}
	return TdxQuote(quote, options)
}
