// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"database/sql/driver"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

const (
	retentionModeNone                        = "0"
	retentionModeCompliance                  = "1"
	retentionModeGovernance                  = "2"
	retentionModeComplianceAndGovernanceMask = "3"
	retentionModeLegalHold                   = "4"
	retentionModesComplianceAndGovernance    = "(" + retentionModeCompliance + "," + retentionModeGovernance + ")"
)

// Constants for encoding an object's retention mode and legal hold status
// as a single value in the retention_mode column of the objects table.
const (
	// retentionModeMask is a bit mask used to identify bits related to storj.RetentionMode.
	retentionModeMask = 0b11

	// legalHoldFlag is a bit flag signifying that an object version is locked in legal hold
	// and cannot be deleted or modified until the legal hold is removed.
	legalHoldFlag = 0b100
)

// verifyObjectLockAndRetention checks that constraints for object lock and retention are correctly set.
func (obj *Object) verifyObjectLockAndRetention() error {
	if err := obj.Retention.Verify(); err != nil {
		return err
	}
	if obj.ExpiresAt != nil && (obj.LegalHold || obj.Retention.Enabled()) {
		return Error.New("object expiration must not be set if Object Lock configuration is set")
	}
	return nil
}

func checkExpiresAtWithObjectLock(object Object, segments transposedSegmentList, retention Retention, legalHold bool) error {
	if !retention.Enabled() && !legalHold {
		return nil
	}
	for _, e := range segments.ExpiresAts {
		if e != nil {
			return ErrObjectExpiration.New(noLockWithExpirationSegmentsErrMsg)
		}
	}
	if object.ExpiresAt != nil && !object.ExpiresAt.IsZero() {
		return ErrObjectExpiration.New(noLockWithExpirationErrMsg)
	}
	return nil
}

// Retention represents an object version's Object Lock retention configuration.
type Retention struct {
	Mode        storj.RetentionMode
	RetainUntil time.Time
}

// RetentionMode implements scanning for retention_mode column.
type RetentionMode struct {
	Mode      storj.RetentionMode
	LegalHold bool
}

// Enabled returns whether the retention configuration is enabled.
func (r *Retention) Enabled() bool {
	return r.Mode != storj.NoRetention
}

// Active returns whether the retention configuration is enabled and active as of the given time.
func (r *Retention) Active(now time.Time) bool {
	return r.Enabled() && now.Before(r.RetainUntil)
}

// ActiveNow returns whether the retention configuration is enabled and active as of the current time.
func (r *Retention) ActiveNow() bool {
	return r.Active(time.Now())
}

// Verify verifies the retention configuration.
func (r *Retention) Verify() error {
	if r.Mode == storj.GovernanceMode {
		if r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must be set if retention mode is set")
		}
		return nil
	}
	return r.verifyWithoutGovernance()
}

func (r *Retention) isProtected(bypassGovernance bool, now time.Time) bool {
	return r.Active(now) && !(bypassGovernance && r.Mode == storj.GovernanceMode)
}

// verifyWithoutGovernance verifies the retention configuration. It's used by metabase DB methods that haven't
// yet been adjusted to support governance mode, so it treats governance mode as invalid.
func (r *Retention) verifyWithoutGovernance() error {
	switch r.Mode {
	case storj.ComplianceMode:
		if r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must be set if retention mode is set")
		}
	case storj.NoRetention:
		if !r.RetainUntil.IsZero() {
			return errs.New("retention period expiration must not be set if retention mode is not set")
		}
	default:
		return errs.New("invalid retention mode %d", r.Mode)
	}
	return nil
}

// Value implements the sql/driver.Valuer interface.
func (r RetentionMode) Value() (driver.Value, error) {
	if int64(r.Mode)&retentionModeMask != int64(r.Mode) {
		return nil, Error.New("invalid retention mode")
	}

	val := int64(r.Mode)
	if r.LegalHold {
		val |= legalHoldFlag
	}

	return val, nil
}

func (r *RetentionMode) set(v int64) {
	r.Mode = storj.RetentionMode(v & retentionModeMask)
	r.LegalHold = v&legalHoldFlag != 0
}

// Scan implements the sql.Scanner interface.
func (r *RetentionMode) Scan(val interface{}) error {
	if val == nil {
		*r = RetentionMode{}
		return nil
	}
	if v, ok := val.(int64); ok {
		r.set(v)
		return nil
	}
	return Error.New("unable to scan %T", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (r RetentionMode) EncodeSpanner() (interface{}, error) {
	return r.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (r *RetentionMode) DecodeSpanner(val interface{}) error {
	switch v := val.(type) {
	case *string:
		if v == nil {
			*r = RetentionMode{}
			return nil
		}
		iVal, err := strconv.ParseInt(*v, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", *v, err)
		}
		r.set(iVal)
		return nil
	case string:
		iVal, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", v, err)
		}
		r.set(iVal)
		return nil
	case int64:
		r.set(v)
		return nil
	default:
		return r.Scan(val)
	}
}

type lockModeWrapper struct {
	retentionMode *storj.RetentionMode
	legalHold     *bool
}

// Value implements the sql/driver.Valuer interface.
func (r lockModeWrapper) Value() (driver.Value, error) {
	var val int64
	if r.retentionMode != nil {
		val = int64(*r.retentionMode)
	}
	if r.legalHold != nil && *r.legalHold {
		val |= legalHoldFlag
	}
	if val == 0 {
		return nil, nil
	}
	return val, nil
}

// Clear resets to the default values.
func (r lockModeWrapper) Clear() {
	if r.retentionMode != nil {
		*r.retentionMode = storj.NoRetention
	}
	if r.legalHold != nil {
		*r.legalHold = false
	}
}

// Set from am encoded value.
func (r lockModeWrapper) Set(val int64) {
	if r.retentionMode != nil {
		*r.retentionMode = storj.RetentionMode(val & retentionModeMask)
	}
	if r.legalHold != nil {
		*r.legalHold = val&legalHoldFlag != 0
	}
}

// Scan implements the sql.Scanner interface.
func (r lockModeWrapper) Scan(val interface{}) error {
	if val == nil {
		r.Clear()
		return nil
	}
	if v, ok := val.(int64); ok {
		r.Set(v)
		return nil
	}
	return Error.New("unable to scan %T", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (r lockModeWrapper) EncodeSpanner() (interface{}, error) {
	return r.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (r lockModeWrapper) DecodeSpanner(val interface{}) error {
	if strPtrVal, ok := val.(*string); ok {
		if strPtrVal == nil {
			r.Clear()
			return nil
		}
		val = strPtrVal
	}
	if strVal, ok := val.(string); ok {
		iVal, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return Error.New("unable to parse %q as int64: %w", strVal, err)
		}
		r.Set(iVal)
		return nil
	}
	return r.Scan(val)
}

type timeWrapper struct {
	*time.Time
}

// Value implements the sql/driver.Valuer interface.
func (t timeWrapper) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return *t.Time, nil
}

// Scan implements the sql.Scanner interface.
func (t timeWrapper) Scan(val interface{}) error {
	if val == nil {
		*t.Time = time.Time{}
		return nil
	}
	if v, ok := val.(time.Time); ok {
		*t.Time = v
		return nil
	}
	return Error.New("unable to scan %T into time.Time", val)
}

// EncodeSpanner implements the spanner.Encoder interface.
func (t timeWrapper) EncodeSpanner() (interface{}, error) {
	return t.Value()
}

// DecodeSpanner implements the spanner.Decoder interface.
func (t timeWrapper) DecodeSpanner(val interface{}) error {
	if strPtrVal, ok := val.(*string); ok {
		if strPtrVal == nil {
			*t.Time = time.Time{}
			return nil
		}
		val = strPtrVal
	}
	if strVal, ok := val.(string); ok {
		tVal, err := time.Parse(time.RFC3339Nano, strVal)
		if err != nil {
			return Error.New("unable to parse %q as time.Time: %w", strVal, err)
		}
		*t.Time = tVal
		return nil
	}
	return t.Scan(val)
}
