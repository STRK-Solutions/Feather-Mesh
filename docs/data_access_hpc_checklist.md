# Peer data-access HPC acceptance checklist

Local CI proves only the single-host implementation. Run and attach this record
on the target shared filesystem before describing peer access as HPC-accepted.

| Check | Required evidence | Status |
| --- | --- | --- |
| Separate identities | Producer publishes; allowed consumer resolves/reads and denied consumer fails for the configured peer route. Record uid/group and ACL/mount policy. | Pending: target identities unavailable locally |
| Manifest atomicity | Concurrent provider publishes and readers repeatedly refresh/resolve on the target mount. Preserve complete old/new JSON records only; record filesystem/lock behavior. | Pending: target filesystem unavailable locally |
| Multi-node cache | Two allocated nodes refresh/read the same project; show cache paths are host/process-local and no SQLite WAL cache is shared. | Pending: multi-node allocation unavailable locally |
| Lifecycle | Remove/retarget a peer link and withdraw a version while discovery and STAC pagination run. New access must fail; already-open library handles are documented as non-revocable. | Pending: target filesystem unavailable locally |
| Read performance | Record Rasterio window size/block layout/CRS/nodata and Polars fixed-shard query timing/memory allocation for representative data. | Pending: representative HPC data unavailable locally |
| Staging recovery | Exercise source/output aliases, overlap, failed overwrite, and receipt-write failure on disposable target-mount fixtures; verify prior destinations/provider bytes. | Pending: target filesystem unavailable locally |

Local evidence is recorded in the [implementation workplan](../data_access_implementation_workplan.md#8-execution-record-for-the-implementing-agent). Do not replace any row above with local permission mocks.
