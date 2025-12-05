#!/usr/bin/env python3

import sys, json, random

SAMPLES_PER_SEC = 10000.0

alltraces = json.loads(open(sys.argv[1]).read())

stacks = {}

for trace in alltraces:
    allspans_by_id = {}

    start = None
    finish = None
    for span in trace:
        if start is None or start > span["start"]:
            start = span["start"]
        if finish is None or finish < span["finish"]:
            finish = span["finish"]
        allspans_by_id[span["id"]] = span

    trace_duration_sec = (finish - start)/1000000000.0
    samples_per_trace = int(trace_duration_sec * SAMPLES_PER_SEC)

    for i in range(samples_per_trace):
        sample_time = start + i * ((finish-start)/samples_per_trace)

        spans_by_id = {}
        parents = set()
        # TODO: exhaustive search every time for overlaps is bad.
        for span in trace:
            if span["start"] > sample_time or span["finish"] < sample_time: continue
            spans_by_id[span["id"]] = span
            if "parent_id" in span:
                parents.add(span["parent_id"])

        leaves = []
        for id, span in spans_by_id.items():
            if id in parents: continue
            leaves.append(span)

        for span in leaves:
            stack = []
            walked = set()
            def walk(span):
                if span["id"] in walked: return
                walked.add(span["id"])
                stack.append((span, span["id"] in spans_by_id))
                if "parent_id" in span:
                    parent = spans_by_id.get(span["parent_id"])
                    if not parent:
                        parent = allspans_by_id.get(span["parent_id"])
                    if parent:
                        walk(parent)

            walk(span)
            stack_name = ";".join("[%s]%s.%s" % (active and "active" or "inactive",
                                                span["func"]["package"], span["func"]["name"])
                                for (span, active) in reversed(stack))
            stacks[stack_name] = stacks.get(stack_name, 0) + 1

for stack, count in stacks.items():
    print(stack, count)
