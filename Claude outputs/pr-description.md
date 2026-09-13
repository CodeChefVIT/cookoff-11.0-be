## 📌 Description

Closes #42. `GET /result/:submission_id` was polling Postgres directly every 5s per open connection (`GetSubmissionStatusByID`, then `GetSubmissionResults` + `GetSubmissionByID` once done). With ~500 concurrent participants polling, this generated 500–1000 DB queries/sec competing with the worker's `FOR UPDATE` writes (score/balance updates) for connections.

Fix: the worker now builds the full result once, at the moment it finalizes a submission (last testcase callback), and caches it in Redis under `sub:status:<submission_id>` with a 30-minute TTL. `GetResult` polls that Redis key instead of Postgres — a cache hit returns immediately with zero DB queries. If Redis itself errors (not just a miss), it falls back to the original Postgres path so results still get delivered. No database migration required.

---

## 🛠️ Type of Change
- [x] `perf`: Performance optimizations
- [x] `fix`: A bug fix

---

## 🧪 Verification & Testing
- [x] Project compiles clean (`go build ./...`)
- [ ] Relocated tests ran and passed (`go test -v ./...`)
- [ ] Static checks and formatters ran (`go fmt` / `go vet` / `golangci-lint`)

Manually verified: submitted code, confirmed `sub:status:<id>` appears in Redis right after the worker finalizes the submission, and `GET /result/:id` returns from cache on the next poll instead of hitting Postgres. Also tested the failure path (partial testcases failing) still caches and returns correctly.

---

## 📋 Checklist
- [x] My commit messages conform to the **Conventional Commits** specification.
- [ ] I have updated the documentation mirror (inside `docs/`) for the modified packages.
- [x] No raw SQL queries have been hardcoded (all queries compiled via SQLC).
- [x] database mutations (if any) are wrapped in atomic database transactions (`pgx.Tx`).

---

**Files changed:** `internal/helpers/utils/redis.go`, `internal/workers/submission.go`, `internal/controllers/result.go`
