# Checker
Reads the event log and stores after quiescing and asserts the seven invariants. Dedupes by event id, keeping the first occurrence in log order — the copy the projection applied; a retried report differs only in `ingest_ts`. A test passes only if this passes.
