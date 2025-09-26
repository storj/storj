// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package auditlogger

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CapsConfig defines limits to keep change sets manageable in size.
type CapsConfig struct {
	MaxChanges       int `help:"max changed fields recorded per event (0 = no limit)" default:"300"`
	MaxDepth         int `help:"max struct depth to recurse when comparing (0 = no limit)" default:"6"`
	MaxSliceElements int `help:"max slice/array elements compared (0 = no limit)" default:"50"`
	MaxMapEntries    int `help:"max map entries compared (0 = no limit)" default:"200"`
	MaxStringLen     int `help:"max string length recorded (0 = no limit)" default:"512"`
	MaxBytes         int `help:"max total JSON bytes recorded (0 = no limit)" default:"28672"` // 28KiB
}

// BuildChangeSet compares before and after states to build a changeset.
func BuildChangeSet(before, after any, caps CapsConfig) map[string]any {
	st := &capState{caps: caps}
	cs := st.build(before, after)

	if caps.MaxBytes > 0 {
		cs = pruneToByteCap(cs, caps.MaxBytes)
	}
	return cs
}

type capState struct {
	caps   CapsConfig
	added  int
	closed bool
}

func (st *capState) build(before, after any) map[string]any {
	// nil handling at root.
	if before == nil && after == nil {
		return nil
	}
	if before == nil || after == nil {
		out := make(map[string]any, 1)
		st.addChange(out, "value", before, after)
		return out
	}

	bv, av := reflect.ValueOf(before), reflect.ValueOf(after)

	// Pointer policy at root: deref both or neither; mixed → change.
	if bv.Kind() == reflect.Ptr || av.Kind() == reflect.Ptr {
		if bv.Kind() == reflect.Ptr && av.Kind() == reflect.Ptr {
			if bv.IsNil() && av.IsNil() {
				return nil
			}
			if bv.IsNil() || av.IsNil() {
				out := make(map[string]any, 1)
				st.addChange(out, "value", getPointerValue(bv), getPointerValue(av))
				return out
			}
			bv, av = bv.Elem(), av.Elem()
		} else {
			out := make(map[string]any, 1)
			st.addChange(out, "value", before, after)
			return out
		}
	}

	// Type mismatch after pointer normalization → change.
	if bv.Type() != av.Type() {
		out := make(map[string]any, 1)
		st.addChange(out, "value", before, after)
		return out
	}

	out := make(map[string]any)

	// Atomic short-circuit at root.
	if isAtomicType(bv.Type()) {
		st.emitIfDiffVal(out, "value", bv, av)
		return out
	}

	switch bv.Kind() {
	case reflect.Struct:
		st.compareStructFields(bv, av, "", out, 0)
	case reflect.Slice, reflect.Array:
		if isAtomicType(bv.Type()) {
			st.emitIfDiffVal(out, "value", bv, av)
			return out
		}
		st.compareSliceFields(bv, av, "value", out)
	case reflect.Map:
		st.compareMapFields(bv, av, "value", out)
	default:
		st.emitIfDiffVal(out, "value", bv, av)
	}
	return out
}

func (st *capState) compareStructFields(bv, av reflect.Value, prefix string, changes map[string]any, depth int) {
	if st.closed {
		return
	}

	// Depth cap → opaque compare.
	if st.depthExceeded(depth) {
		st.emitIfDiffVal(changes, prefix, bv, av)
		return
	}

	// Opaque structs (e.g., time.Time with no exported fields) → atomic compare.
	if !hasExportedFields(bv.Type()) {
		st.emitIfDiffVal(changes, prefix, bv, av)
		return
	}

	for i := 0; i < bv.NumField() && !st.closed; i++ {
		f := bv.Type().Field(i)
		if !f.IsExported() || shouldSkipField(f.Type) {
			continue
		}

		bf, af := bv.Field(i), av.Field(i)
		name := f.Name
		if prefix != "" {
			name = prefix + "." + name
		}

		// pointers inside struct: both-or-neither rule.
		if bf.Kind() == reflect.Ptr && af.Kind() == reflect.Ptr {
			if bf.IsNil() && af.IsNil() {
				continue
			}
			if bf.IsNil() || af.IsNil() {
				st.addChange(changes, name, getPointerValue(bf), getPointerValue(af))
				continue
			}
			bf, af = bf.Elem(), af.Elem()
		} else if bf.Kind() == reflect.Ptr || af.Kind() == reflect.Ptr {
			st.addChange(changes, name, toInterface(bf), toInterface(af))
			continue
		}

		// Nested atomic types → scalar compare.
		if isAtomicType(bf.Type()) {
			st.emitIfDiffVal(changes, name, bf, af)
			continue
		}

		switch bf.Kind() {
		case reflect.Struct:
			if !hasExportedFields(bf.Type()) {
				st.emitIfDiffVal(changes, name, bf, af)
				continue
			}
			st.compareStructFields(bf, af, name, changes, depth+1)
		case reflect.Slice, reflect.Array:
			if isAtomicType(bf.Type()) {
				st.emitIfDiffVal(changes, name, bf, af)
				continue
			}
			st.compareSliceFields(bf, af, name, changes)
		case reflect.Map:
			st.compareMapFields(bf, af, name, changes)
		default:
			st.emitIfDiffVal(changes, name, bf, af)
		}
	}
}

func (st *capState) compareSliceFields(bv, av reflect.Value, field string, changes map[string]any) {
	if st.closed {
		return
	}

	if isAtomicType(bv.Type()) {
		st.emitIfDiffVal(changes, field, bv, av)
		return
	}

	// Length-only signal when sizes differ.
	if bv.Len() != av.Len() {
		st.addChange(changes, field+".count", bv.Len(), av.Len())
		return
	}
	if bv.Len() == 0 {
		return
	}

	// Compare first N elements (if MaxSliceElements>0), else all.
	n := getCap(bv.Len(), st.caps.MaxSliceElements)

	// Struct elements → deep-compare sampled elements.
	if bv.Index(0).Kind() == reflect.Struct {
		for i := 0; i < n && !st.closed; i++ {
			elemChanges := make(map[string]any)
			st.compareStructFields(bv.Index(i), av.Index(i), "", elemChanges, 0)

			keys := make([]string, 0, len(elemChanges))
			for k := range elemChanges {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if st.closed {
					break
				}
				changes[fmt.Sprintf("%s[%d].%s", field, i, k)] = elemChanges[k]
			}
		}
		return
	}

	// Primitive slices: if small, record full diff; otherwise emit compact signal.
	if bv.Len() <= 10 {
		st.emitIfDiffVal(changes, field, bv, av)
	} else if !reflect.DeepEqual(bv.Interface(), av.Interface()) {
		st.addChange(changes, field+".changed", true, true)
		st.addChange(changes, field+".length", bv.Len(), av.Len())
	}
}

func (st *capState) compareMapFields(bv, av reflect.Value, prefix string, changes map[string]any) {
	if st.closed {
		return
	}

	// Only walk string-keyed maps; others → opaque compare.
	if bv.Kind() != reflect.Map || av.Kind() != reflect.Map ||
		bv.Type().Key().Kind() != reflect.String || av.Type().Key().Kind() != reflect.String {
		st.emitIfDiffVal(changes, prefix, bv, av)
		return
	}

	if bv.Len() != av.Len() {
		st.addChange(changes, prefix+".count", bv.Len(), av.Len())
	}

	// Gather and cap keys.
	keys := bv.MapKeys()
	ks := make([]string, 0, len(keys))
	for _, k := range keys {
		ks = append(ks, k.String())
	}
	sort.Strings(ks)
	ks = ks[:getCap(len(ks), st.caps.MaxMapEntries)]

	for _, k := range ks {
		if st.closed {
			return
		}
		bvv := bv.MapIndex(reflect.ValueOf(k))
		avv := av.MapIndex(reflect.ValueOf(k))

		// Presence deltas.
		if !bvv.IsValid() || !avv.IsValid() {
			st.addChange(changes, prefix+"."+k, toInterface(bvv), toInterface(avv))
			continue
		}

		// Atomic map values → scalar.
		if isAtomicType(bvv.Type()) {
			st.emitIfDiffVal(changes, prefix+"."+k, bvv, avv)
			continue
		}

		switch bvv.Kind() {
		case reflect.Struct:
			st.compareStructFields(bvv, avv, prefix+"."+k, changes, 0)
		case reflect.Slice, reflect.Array:
			if isAtomicType(bvv.Type()) {
				st.emitIfDiffVal(changes, prefix+"."+k, bvv, avv)
				continue
			}
			st.compareSliceFields(bvv, avv, prefix+"."+k, changes)
		case reflect.Map:
			st.compareMapFields(bvv, avv, prefix+"."+k, changes)
		default:
			st.emitIfDiffVal(changes, prefix+"."+k, bvv, avv)
		}
	}
}

func (st *capState) depthExceeded(depth int) bool {
	return st.caps.MaxDepth > 0 && depth >= st.caps.MaxDepth
}

func (st *capState) addChange(changes map[string]any, key string, oldV, newV any) {
	if st.closed {
		return
	}
	if st.caps.MaxChanges > 0 && st.added >= st.caps.MaxChanges {
		st.closed = true
		return
	}
	changes[key] = []any{st.truncateValue(oldV), st.truncateValue(newV)}
	st.added++
}

func (st *capState) emitIfDiffVal(ch map[string]any, key string, a, b reflect.Value) {
	if st.closed {
		return
	}
	if !reflect.DeepEqual(a.Interface(), b.Interface()) {
		st.addChange(ch, key, a.Interface(), b.Interface())
	}
}

func (st *capState) truncateValue(v any) any {
	if v == nil || st.caps.MaxStringLen <= 0 {
		return v
	}
	if s, ok := v.(string); ok && len(s) > st.caps.MaxStringLen {
		if st.caps.MaxStringLen <= 1 {
			return "…"
		}
		return s[:st.caps.MaxStringLen-1] + "…"
	}
	return v
}

func getCap(n, max int) int {
	if max > 0 && n > max {
		return max
	}
	return n
}

// isAtomicType treats as scalar (no deep walk).
func isAtomicType(t reflect.Type) bool {
	if t == nil {
		return false
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// time.Time.
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return true
	}
	// uuid.UUID.
	if strings.Contains(t.PkgPath(), "uuid") && t.Name() == "UUID" {
		return true
	}

	return false
}

func hasExportedFields(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

func getPointerValue(val reflect.Value) any {
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return nil
	}
	return val.Elem().Interface()
}

func shouldSkipField(fieldType reflect.Type) bool {
	kind := fieldType.Kind()
	if kind == reflect.Func || kind == reflect.Chan || kind == reflect.UnsafePointer || kind == reflect.Interface {
		return true
	}
	return false
}

func toInterface(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

// pruneToByteCap remove keys (lexicographic, from the end) until JSON fits.
func pruneToByteCap(ch map[string]any, capBytes int) map[string]any {
	if capBytes <= 0 {
		return ch
	}
	b, _ := json.Marshal(ch)
	if len(b) <= capBytes {
		return ch
	}

	keys := make([]string, 0, len(ch))
	for k := range ch {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make(map[string]any, len(ch))
	for k, v := range ch {
		out[k] = v
	}
	for i := len(keys) - 1; i >= 0; i-- {
		delete(out, keys[i])
		b, _ = json.Marshal(out)
		if len(b) <= capBytes {
			break
		}
	}
	return out
}
