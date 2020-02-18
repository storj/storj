// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"time"
)

// Note to readers: All of the tag iteration and field searching code does not bother to
// handle escaped spaces or commas because all of the keys we intend to migrate do not
// contain them. In particular, we have no package import paths with spaces or commas and
// it is impossible to have a function name with a space or comma in it. Additionally, all
// of the monkit/v3 field names do not have spaces or commas. This could become invalid if
// someone decides to break it, but this code is also temporary.

// MetricDowngrade downgrades known v3 metrics into v2 versions for backwards compat.
type MetricDowngrade struct {
	dest MetricDest
}

// NewMetricDowngrade constructs a MetricDowngrade that passes known v3 metrics as
// v2 metrics to the provided dest.
func NewMetricDowngrade(dest MetricDest) *MetricDowngrade {
	return &MetricDowngrade{
		dest: dest,
	}
}

// Metric implements MetricDest
func (k *MetricDowngrade) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	comma := bytes.IndexByte(key, ',')
	if comma < 0 {
		return nil
	}

	if string(key[:comma]) == "function_times" {
		return k.handleFunctionTimes(application, instance, key[comma+1:], val, ts)
	}
	if string(key[:comma]) == "function" {
		return k.handleFunction(application, instance, key[comma+1:], val, ts)
	}

	v2key, ok := knownMetrics[string(key[:comma])]
	if !ok {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(v2key)+1+len(key)-space)
	out = append(out, v2key...)
	out = append(out, '.')
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func (k *MetricDowngrade) handleFunctionTimes(application, instance string, key []byte, val float64, ts time.Time) error {
	var name, kind, scope string
	iterateTags(key, func(tag []byte) {
		if len(tag) < 6 {
			return
		}
		switch {
		case string(tag[:5]) == "name=":
			name = string(tag[5:])
		case string(tag[:5]) == "kind=":
			kind = string(tag[5:])
		case string(tag[:6]) == "scope=":
			scope = string(tag[6:])
		}
	})

	if name == "" || kind == "" || scope == "" {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(scope)+1+len(name)+1+len(kind)+7+(len(key)-space))
	out = append(out, scope...)
	out = append(out, '.')
	out = append(out, name...)
	out = append(out, '.')
	out = append(out, kind...)
	out = append(out, "_times_"...)
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func (k *MetricDowngrade) handleFunction(application, instance string, key []byte, val float64, ts time.Time) error {
	var name, scope string
	iterateTags(key, func(tag []byte) {
		if len(tag) < 6 {
			return
		}
		switch {
		case string(tag[:5]) == "name=":
			name = string(tag[5:])
		case string(tag[:6]) == "scope=":
			scope = string(tag[6:])
		}
	})

	if name == "" || scope == "" {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(scope)+1+len(name)+1+(len(key)-space))
	out = append(out, scope...)
	out = append(out, '.')
	out = append(out, name...)
	out = append(out, '.')
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func iterateTags(key []byte, cb func([]byte)) {
	for len(key) > 0 {
		comma := bytes.IndexByte(key, ',')
		if comma == -1 {
			break
		}
		cb(key[:comma])
		key = key[comma+1:]
	}
	space := bytes.IndexByte(key, ' ')
	if space >= 0 {
		cb(key[:space])
	}
}

var knownMetrics = map[string]string{
	"total.bytes":                                    "storj.io/storj/satellite/accounting/tally.total.bytes",
	"total.inline_bytes":                             "storj.io/storj/satellite/accounting/tally.total.inline_bytes",
	"total.inline_segments":                          "storj.io/storj/satellite/accounting/tally.total.inline_segments",
	"total.objects":                                  "storj.io/storj/satellite/accounting/tally.total.objects",
	"total.remote_bytes":                             "storj.io/storj/satellite/accounting/tally.total.remote_bytes",
	"total.remote_segments":                          "storj.io/storj/satellite/accounting/tally.total.remote_segments",
	"total.segments":                                 "storj.io/storj/satellite/accounting/tally.total.segments",
	"audit_contained_nodes":                          "storj.io/storj/satellite/audit.audit_contained_nodes",
	"audit_contained_nodes_global":                   "storj.io/storj/satellite/audit.audit_contained_nodes_global",
	"audit_contained_percentage":                     "storj.io/storj/satellite/audit.audit_contained_percentage",
	"audit_fail_nodes":                               "storj.io/storj/satellite/audit.audit_fail_nodes",
	"audit_fail_nodes_global":                        "storj.io/storj/satellite/audit.audit_fail_nodes_global",
	"audit_failed_percentage":                        "storj.io/storj/satellite/audit.audit_failed_percentage",
	"audit_offline_nodes":                            "storj.io/storj/satellite/audit.audit_offline_nodes",
	"audit_offline_nodes_global":                     "storj.io/storj/satellite/audit.audit_offline_nodes_global",
	"audit_offline_percentage":                       "storj.io/storj/satellite/audit.audit_offline_percentage",
	"audit_success_nodes":                            "storj.io/storj/satellite/audit.audit_success_nodes",
	"audit_success_nodes_global":                     "storj.io/storj/satellite/audit.audit_success_nodes_global",
	"audit_successful_percentage":                    "storj.io/storj/satellite/audit.audit_successful_percentage",
	"audit_total_nodes":                              "storj.io/storj/satellite/audit.audit_total_nodes",
	"audit_total_nodes_global":                       "storj.io/storj/satellite/audit.audit_total_nodes_global",
	"audit_total_pointer_nodes":                      "storj.io/storj/satellite/audit.audit_total_pointer_nodes",
	"audit_total_pointer_nodes_global":               "storj.io/storj/satellite/audit.audit_total_pointer_nodes_global",
	"audit_unknown_nodes":                            "storj.io/storj/satellite/audit.audit_unknown_nodes",
	"audit_unknown_nodes_global":                     "storj.io/storj/satellite/audit.audit_unknown_nodes_global",
	"audit_unknown_percentage":                       "storj.io/storj/satellite/audit.audit_unknown_percentage",
	"audited_percentage":                             "storj.io/storj/satellite/audit.audited_percentage",
	"reverify_contained":                             "storj.io/storj/satellite/audit.reverify_contained",
	"reverify_contained_global":                      "storj.io/storj/satellite/audit.reverify_contained_global",
	"reverify_contained_in_segment":                  "storj.io/storj/satellite/audit.reverify_contained_in_segment",
	"reverify_fails":                                 "storj.io/storj/satellite/audit.reverify_fails",
	"reverify_fails_global":                          "storj.io/storj/satellite/audit.reverify_fails_global",
	"reverify_offlines":                              "storj.io/storj/satellite/audit.reverify_offlines",
	"reverify_offlines_global":                       "storj.io/storj/satellite/audit.reverify_offlines_global",
	"reverify_successes":                             "storj.io/storj/satellite/audit.reverify_successes",
	"reverify_successes_global":                      "storj.io/storj/satellite/audit.reverify_successes_global",
	"reverify_total_in_segment":                      "storj.io/storj/satellite/audit.reverify_total_in_segment",
	"reverify_unknown":                               "storj.io/storj/satellite/audit.reverify_unknown",
	"reverify_unknown_global":                        "storj.io/storj/satellite/audit.reverify_unknown_global",
	"graceful_exit_fail_max_failures_percentage":     "storj.io/storj/satellite/gracefulexit.graceful_exit_fail_max_failures_percentage",
	"graceful_exit_fail_validation":                  "storj.io/storj/satellite/gracefulexit.graceful_exit_fail_validation",
	"graceful_exit_final_bytes_transferred":          "storj.io/storj/satellite/gracefulexit.graceful_exit_final_bytes_transferred",
	"graceful_exit_final_pieces_failed":              "storj.io/storj/satellite/gracefulexit.graceful_exit_final_pieces_failed",
	"graceful_exit_final_pieces_succeess":            "storj.io/storj/satellite/gracefulexit.graceful_exit_final_pieces_succeess",
	"graceful_exit_init_node_age_seconds":            "storj.io/storj/satellite/gracefulexit.graceful_exit_init_node_age_seconds",
	"graceful_exit_init_node_audit_success_count":    "storj.io/storj/satellite/gracefulexit.graceful_exit_init_node_audit_success_count",
	"graceful_exit_init_node_audit_total_count":      "storj.io/storj/satellite/gracefulexit.graceful_exit_init_node_audit_total_count",
	"graceful_exit_init_node_piece_count":            "storj.io/storj/satellite/gracefulexit.graceful_exit_init_node_piece_count",
	"graceful_exit_success":                          "storj.io/storj/satellite/gracefulexit.graceful_exit_success",
	"graceful_exit_successful_pieces_transfer_ratio": "storj.io/storj/satellite/gracefulexit.graceful_exit_successful_pieces_transfer_ratio",
	"graceful_exit_transfer_piece_fail":              "storj.io/storj/satellite/gracefulexit.graceful_exit_transfer_piece_fail",
	"graceful_exit_transfer_piece_success":           "storj.io/storj/satellite/gracefulexit.graceful_exit_transfer_piece_success",
	"download_failed_not_enough_pieces_uplink":       "storj.io/storj/satellite/orders.download_failed_not_enough_pieces_uplink",
	"checker_segment_age":                            "storj.io/storj/satellite/repair/checker.checker_segment_age",
	"checker_segment_healthy_count":                  "storj.io/storj/satellite/repair/checker.checker_segment_healthy_count",
	"checker_segment_time_until_irreparable":         "storj.io/storj/satellite/repair/checker.checker_segment_time_until_irreparable",
	"checker_segment_total_count":                    "storj.io/storj/satellite/repair/checker.checker_segment_total_count",
	"remote_files_checked":                           "storj.io/storj/satellite/repair/checker.remote_files_checked",
	"remote_files_lost":                              "storj.io/storj/satellite/repair/checker.remote_files_lost",
	"remote_segments_checked":                        "storj.io/storj/satellite/repair/checker.remote_segments_checked",
	"remote_segments_lost":                           "storj.io/storj/satellite/repair/checker.remote_segments_lost",
	"remote_segments_needing_repair":                 "storj.io/storj/satellite/repair/checker.remote_segments_needing_repair",
	"download_failed_not_enough_pieces_repair":       "storj.io/storj/satellite/repair/repairer.download_failed_not_enough_pieces_repair",
	"healthy_ratio_after_repair":                     "storj.io/storj/satellite/repair/repairer.healthy_ratio_after_repair",
	"healthy_ratio_before_repair":                    "storj.io/storj/satellite/repair/repairer.healthy_ratio_before_repair",
	"repair_attempts":                                "storj.io/storj/satellite/repair/repairer.repair_attempts",
	"repair_failed":                                  "storj.io/storj/satellite/repair/repairer.repair_failed",
	"repair_nodes_unavailable":                       "storj.io/storj/satellite/repair/repairer.repair_nodes_unavailable",
	"repair_partial":                                 "storj.io/storj/satellite/repair/repairer.repair_partial",
	"repair_segment_pieces_canceled":                 "storj.io/storj/satellite/repair/repairer.repair_segment_pieces_canceled",
	"repair_segment_pieces_failed":                   "storj.io/storj/satellite/repair/repairer.repair_segment_pieces_failed",
	"repair_segment_pieces_successful":               "storj.io/storj/satellite/repair/repairer.repair_segment_pieces_successful",

	"repair_segment_pieces_total": "storj.io/storj/satellite/repair/repairer.repair_segment_pieces_total",

	"repair_segment_size": "storj.io/storj/satellite/repair/repairer.repair_segment_size",
	"repair_success":      "storj.io/storj/satellite/repair/repairer.repair_success",

	"repair_unnecessary": "storj.io/storj/satellite/repair/repairer.repair_unnecessary",

	"segment_repair_count": "storj.io/storj/satellite/repair/repairer.segment_repair_count",

	"segment_time_until_repair": "storj.io/storj/satellite/repair/repairer.segment_time_until_repair",
	"time_for_repair":           "storj.io/storj/satellite/repair/repairer.time_for_repair",
	"time_since_checker_queue":  "storj.io/storj/satellite/repair/repairer.time_since_checker_queue",

	"audit_reputation_alpha":                          "storj.io/storj/satellite/satellitedb.audit_reputation_alpha",
	"audit_reputation_beta":                           "storj.io/storj/satellite/satellitedb.audit_reputation_beta",
	"open_file_in_trash":                              "storj.io/storj/storage/filestore.open_file_in_trash",
	"satellite_contact_request":                       "storj.io/storj/storagenode/contact.satellite_contact_request",
	"satellite_gracefulexit_request":                  "storj.io/storj/storagenode/gracefulexit.satellite_gracefulexit_request",
	"allocated_bandwidth":                             "storj.io/storj/storagenode/monitor.allocated_bandwidth",
	"used_bandwidth":                                  "storj.io/storj/storagenode/monitor.used_bandwidth",
	"download_stripe_failed_not_enough_pieces_uplink": "storj.io/storj/uplink/eestream.download_stripe_failed_not_enough_pieces_uplink",
}
