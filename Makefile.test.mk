##@ Test

TEST_TARGET ?= "./..."

.PHONY: test/setup
test/setup:
	@docker compose -f docker-compose.tests.yaml down -v --remove-orphans ## cleanup previous data
	@docker compose -f docker-compose.tests.yaml up -d
	@sleep 3
	@docker compose -f docker-compose.tests.yaml exec crdb1 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f docker-compose.tests.yaml exec crdb2 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f docker-compose.tests.yaml exec crdb3 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f docker-compose.tests.yaml exec crdb4 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f docker-compose.tests.yaml exec crdb5 bash -c 'cockroach sql --insecure -e "create database testcockroach;"'
	@docker compose -f docker-compose.tests.yaml exec crdb4 bash -c 'cockroach sql --insecure -e "create database testmetabase;"'
	@docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database teststorj;"'
	@docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "create database testmetabase;"'
	@docker compose -f docker-compose.tests.yaml exec postgres bash -c 'echo "postgres" | psql -U postgres -c "ALTER ROLE postgres CONNECTION LIMIT -1;"'

.PHONY: test/postgres
test/postgres: test/setup ## Run tests against Postgres (developer)
	@env \
		STORJ_TEST_POSTGRES='postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable' \
		STORJ_TEST_COCKROACH='omit' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f docker-compose.tests.yaml down -v; \
		}
	@docker compose -f docker-compose.tests.yaml down -v
	@echo done

.PHONY: test/cockroach
test/cockroach: test/setup ## Run tests against CockroachDB (developer)
	@env \
		STORJ_TEST_COCKROACH_NODROP='true' \
		STORJ_TEST_POSTGRES='omit' \
		STORJ_TEST_COCKROACH="cockroach://root@localhost:26356/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26357/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26358/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26359/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26360/testcockroach?sslmode=disable' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f docker-compose.tests.yaml down -v; \
		}
	@docker compose -f docker-compose.tests.yaml down -v
	@echo done

.PHONY: test
test: test/setup ## Run tests against CockroachDB and Postgres (developer)
	@env \
		STORJ_TEST_COCKROACH_NODROP='true' \
		STORJ_TEST_POSTGRES='postgres://postgres:postgres@localhost:5532/teststorj?sslmode=disable' \
		STORJ_TEST_COCKROACH="cockroach://root@localhost:26356/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26357/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26358/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH="$$STORJ_TEST_COCKROACH;cockroach://root@localhost:26359/testcockroach?sslmode=disable" \
		STORJ_TEST_COCKROACH_ALT='cockroach://root@localhost:26360/testcockroach?sslmode=disable' \
		STORJ_TEST_LOG_LEVEL='info' \
		go test -parallel 4 -p 6 -vet=off -race -v -cover -coverprofile=.coverprofile $(TEST_TARGET) || { \
			docker compose -f docker-compose.tests.yaml rm -fs; \
		}
	@docker compose -f docker-compose.tests.yaml rm -fs
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
		storj.io/storj/cmd/multinode

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

.PHONY: test-satellite-ui
test-satellite-ui: ## Run playwright ui tests
	@echo "Running ${@}"
	cd web/satellite;\
		npm install;\
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
	@./testsuite/wasm/check-size.sh
