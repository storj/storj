##@ Test

TEST_TARGET ?= "./..."
TEST_COMPOSE_FILE ?= testsuite/docker-compose.tests.yaml

.PHONY: test/setup
test/setup:
	@docker compose -f $(TEST_COMPOSE_FILE) down -v --remove-orphans ## cleanup previous data
	@docker compose -f $(TEST_COMPOSE_FILE) up -d
	@sleep 3
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb1 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb2 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb3 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb4 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb5 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec crdb4 bash -c 'cockroach sql --insecure -e "create database testmetabase;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database teststorj;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database testmetabase;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec postgres bash -c 'echo "postgres" | psql -U postgres -c "ALTER ROLE postgres CONNECTION LIMIT -1;"'
	@# TiDB runs testsuite/tidb-init.sql via bootstrap-sql-file before
	@# accepting connections, so once the port answers we're ready.
	@until bash -c 'exec 3<>/dev/tcp/127.0.0.1/4400' >/dev/null 2>&1; do sleep 1; done

.PHONY: test/postgres
test/postgres: test/setup ## Run tests against Postgres (developer)
	@env \
		STORJ_TEST_POSTGRES='postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable' \
		STORJ_TEST_COCKROACH='omit' \
		STORJ_TEST_TIDB='omit' \
		STORJ_TEST_SPANNER='omit' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f $(TEST_COMPOSE_FILE) down -v; \
		}
	@docker compose -f $(TEST_COMPOSE_FILE) down -v
	@echo done

.PHONY: test/setup/tidb
test/setup/tidb:
	@docker compose -f $(TEST_COMPOSE_FILE) down -v --remove-orphans
	@docker compose -f $(TEST_COMPOSE_FILE) up -d postgres tidb
	@sleep 3
	@docker compose -f $(TEST_COMPOSE_FILE) exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database teststorj;"'
	@docker compose -f $(TEST_COMPOSE_FILE) exec postgres bash -c 'echo "postgres" | psql -U postgres -c "ALTER ROLE postgres CONNECTION LIMIT -1;"'
	@# TiDB runs testsuite/tidb-init.sql via bootstrap-sql-file before
	@# accepting connections, so once the port answers we're ready.
	@until bash -c 'exec 3<>/dev/tcp/127.0.0.1/4400' >/dev/null 2>&1; do sleep 1; done

.PHONY: test/tidb
test/tidb: test/setup/tidb ## Run metabase tests against TiDB (developer)
	@env \
		STORJ_TEST_POSTGRES='omit' \
		STORJ_TEST_COCKROACH='omit' \
		STORJ_TEST_SPANNER='omit' \
		STORJ_TEST_TIDB='tidb://root@localhost:4400/testmetabase?parseTime=true!!master=postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f $(TEST_COMPOSE_FILE) down -v; \
		}
	@docker compose -f $(TEST_COMPOSE_FILE) down -v
	@echo done

.PHONY: test/cockroach
test/cockroach: test/setup ## Run tests against CockroachDB (developer)
	@env \
		STORJ_TEST_COCKROACH_NODROP='true' \
		STORJ_TEST_POSTGRES='omit' \
		STORJ_TEST_TIDB='omit' \
		STORJ_TEST_SPANNER='omit' \
		STORJ_TEST_COCKROACH="cockroach://root@localhost:26356/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26357/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26358/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26359/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26360/testcockroach?sslmode=disable' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f $(TEST_COMPOSE_FILE) down -v; \
		}
	@docker compose -f $(TEST_COMPOSE_FILE) down -v
	@echo done

.PHONY: test
test: test/setup ## Run tests against CockroachDB and Postgres (developer)
	@env \
		STORJ_TEST_COCKROACH_NODROP='true' \
		STORJ_TEST_TIDB='omit' \
		STORJ_TEST_SPANNER='omit' \
		STORJ_TEST_POSTGRES='postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable' \
		STORJ_TEST_COCKROACH="cockroach://root@localhost:26356/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26357/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26358/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26359/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26360/testcockroach?sslmode=disable' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f $(TEST_COMPOSE_FILE) rm -fs; \
		}
	@docker compose -f $(TEST_COMPOSE_FILE) rm -fs
	@echo done

.PHONY: install-sim
install-sim: ## install storj-sim
	@echo "Running ${@}"
	go install -race -v \
		storj.io/storj/cmd/satellite \
		storj.io/storj/cmd/storagenode \
		storj.io/storj/cmd/storj-sim \
		storj.io/storj/cmd/versioncontrol \
		storj.io/storj/cmd/uplink \
		storj.io/storj/cmd/identity \
		storj.io/storj/cmd/certificates \
		storj.io/storj/cmd/multinode \
		storj.io/storj/cmd/jobq

	## install the latest stable version of Gateway-ST
	go install -race -v storj.io/gateway@latest

.PHONY: test-sim
test-sim: ## Test source with storj-sim (jenkins)
	@echo "Running ${@}"
	@./testsuite/basic/start-sim.sh

.PHONY: test-sim-redis-unavailability
test-sim-redis-unavailability: ## Test source with Redis availability with storj-sim (jenkins)
	@echo "Running ${@}"
	@./testsuite/redis/start-sim.sh


.PHONY: test-certificates
test-certificates: ## Test certificate signing service and storagenode setup (jenkins)
	@echo "Running ${@}"
	@./testsuite/test-certificates.sh

.PHONY: test-sim-backwards-compatible
test-sim-backwards-compatible: ## Test uploading a file with lastest release (jenkins)
	@echo "Running ${@}"
	@./testsuite/backward-compatibility/start-sim.sh

.PHONY: test/integration/ui
test/integration/ui: ## Run playwright ui tests
	@echo "Running ${@}"
	cd web/satellite;\
		npm ci;\
		npm run wasm-dev;\
		npm run build;

	cd testsuite/playwright-ui;\
		npm ci;\
		npx playwright install --with-deps;\
		STORJ_TEST_SATELLITE_WEB='../../web/satellite' \
		go test -race -run TestRun -timeout 15m -count 1 ./...

.PHONY: test-wasm-size
test-wasm-size: ## Test that the built .wasm code has not increased in size
	@echo "Running ${@}"
	@./web/satellite/wasm/check-size.sh

##@ Integration Test

.PHONY: test/rolling-upgrade/cockroach
test/rolling-upgrade/cockroach: # Run rolling upgrade test with CockroachDB
	./testsuite/rolling-upgrade/run-cockroach.sh

.PHONY: test/rolling-upgrade/postgres
test/rolling-upgrade/postgres: # Run rolling upgrade test with PostgreSQL
	./testsuite/rolling-upgrade/run-postgres.sh

.PHONY: test/uplink-versions/postgres
test/uplink-versions/postgres: # Run uplink versions test with PostgreSQL
	./testsuite/uplink-versions/run-postgres.sh
