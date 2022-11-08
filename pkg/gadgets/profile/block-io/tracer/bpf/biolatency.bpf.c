// SPDX-License-Identifier: GPL-2.0
// Copyright (c) 2020 Wenbo Zhang
#include <vmlinux/vmlinux.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>
#include "biolatency.h"
#include "bits.bpf.h"

#define MAX_ENTRIES	10240

const volatile bool filter_cg = false;
const volatile bool targ_per_disk = false;
const volatile bool targ_per_flag = false;
const volatile bool targ_queued = false;
const volatile bool targ_ms = false;
const volatile bool filter_dev = false;
const volatile __u32 targ_dev = 0;

struct request_queue___x {
	struct gendisk *disk;
} __attribute__((preserve_access_index));

struct {
	__uint(type, BPF_MAP_TYPE_CGROUP_ARRAY);
	__type(key, u32);
	__type(value, u32);
	__uint(max_entries, 1);
} cgroup_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, MAX_ENTRIES);
	__type(key, struct request *);
	__type(value, u64);
} start SEC(".maps");

static struct hist initial_hist;

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, MAX_ENTRIES);
	__type(key, struct hist_key);
	__type(value, struct hist);
} hists SEC(".maps");

static __always_inline
int trace_rq_start(struct request *rq, int issue)
{
	if (filter_cg && !bpf_current_task_under_cgroup(&cgroup_map, 0))
		return 0;

	if (issue && targ_queued && BPF_CORE_READ(rq, q, elevator))
		return 0;

	u64 ts = bpf_ktime_get_ns();

	if (filter_dev) {
		struct request_queue___x *q = (void *)BPF_CORE_READ(rq, q);
		struct gendisk *disk;
		u32 dev;

		if (bpf_core_field_exists(q->disk))
			disk = BPF_CORE_READ(q, disk);
		else
			disk = BPF_CORE_READ(rq, rq_disk);

		dev = disk ? MKDEV(BPF_CORE_READ(disk, major),
				BPF_CORE_READ(disk, first_minor)) : 0;
		if (targ_dev != dev)
			return 0;
	}
	bpf_map_update_elem(&start, &rq, &ts, 0);
	return 0;
}

static int handle_block_rq_insert(__u64 *ctx)
{
	/**
	 * commit a54895fa (v5.11-rc1) changed tracepoint argument list
	 * from TP_PROTO(struct request_queue *q, struct request *rq)
	 * to TP_PROTO(struct request *rq)
	 */
#ifdef KERNEL_BEFORE_5_11
	return trace_rq_start((void *)ctx[1], false);
#else /* !KERNEL_BEFORE_5_11 */
	return trace_rq_start((void *)ctx[0], false);
#endif /* !KERNEL_BEFORE_5_11 */
}

static int handle_block_rq_issue(__u64 *ctx)
{
	/**
	 * commit a54895fa (v5.11-rc1) changed tracepoint argument list
	 * from TP_PROTO(struct request_queue *q, struct request *rq)
	 * to TP_PROTO(struct request *rq)
	 */
#ifdef KERNEL_BEFORE_5_11
	return trace_rq_start((void *)ctx[1], true);
#else /* !KERNEL_BEFORE_5_11 */
	return trace_rq_start((void *)ctx[0], true);
#endif /* !KERNEL_BEFORE_5_11 */
}

static int handle_block_rq_complete(struct request *rq, int error, unsigned int nr_bytes)
{
	if (filter_cg && !bpf_current_task_under_cgroup(&cgroup_map, 0))
		return 0;

	u64 slot, *tsp, ts = bpf_ktime_get_ns();
	struct hist_key hkey = {};
	struct hist *histp;
	s64 delta;

	tsp = bpf_map_lookup_elem(&start, &rq);
	if (!tsp)
		return 0;
	delta = (s64)(ts - *tsp);
	if (delta < 0)
		goto cleanup;

	if (targ_per_disk) {
		struct request_queue___x *q = (void *)BPF_CORE_READ(rq, q);
		struct gendisk *disk;

		if (bpf_core_field_exists(q->disk))
			disk = BPF_CORE_READ(q, disk);
		else
			disk = BPF_CORE_READ(rq, rq_disk);

		hkey.dev = disk ? MKDEV(BPF_CORE_READ(disk, major),
					BPF_CORE_READ(disk, first_minor)) : 0;
	}
	if (targ_per_flag)
		hkey.cmd_flags = BPF_CORE_READ(rq, cmd_flags);

	histp = bpf_map_lookup_elem(&hists, &hkey);
	if (!histp) {
		bpf_map_update_elem(&hists, &hkey, &initial_hist, 0);
		histp = bpf_map_lookup_elem(&hists, &hkey);
		if (!histp)
			goto cleanup;
	}

	if (targ_ms)
		delta /= 1000000U;
	else
		delta /= 1000U;
	slot = log2l(delta);
	if (slot >= MAX_SLOTS)
		slot = MAX_SLOTS - 1;
	__sync_fetch_and_add(&histp->slots[slot], 1);

cleanup:
	bpf_map_delete_elem(&start, &rq);
	return 0;
}

SEC("tp_btf/block_rq_insert")
int ig_profio_ins(u64 *ctx)
{
	return handle_block_rq_insert(ctx);
}

SEC("tp_btf/block_rq_issue")
int ig_profio_iss(u64 *ctx)
{
	return handle_block_rq_issue(ctx);
}

SEC("tp_btf/block_rq_complete")
int BPF_PROG(ig_profio_done, struct request *rq, int error,
	     unsigned int nr_bytes)
{
	return handle_block_rq_complete(rq, error, nr_bytes);
}

SEC("raw_tp/block_rq_insert")
int ig_profio_ins_raw(u64 *ctx)
{
	return handle_block_rq_insert(ctx);
}

SEC("raw_tp/block_rq_issue")
int ig_profio_iss_raw(u64 *ctx)
{
	return handle_block_rq_issue(ctx);
}

SEC("raw_tp/block_rq_complete")
int BPF_PROG(ig_profio_done_raw, struct request *rq, int error,
	     unsigned int nr_bytes)
{
	return handle_block_rq_complete(rq, error, nr_bytes);
}

char LICENSE[] SEC("license") = "GPL";
