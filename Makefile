.PHONY: int-tests int-tests-grpc lint start proto thrift dev thrift-py

start:
	docker compose up -d
	until docker compose exec -T db pg_isready -U admin -d app; do sleep 1; done
	go build ./cmd/server
	./server

dev:
	docker compose up -d
	until docker compose exec -T db pg_isready -U admin -d app; do sleep 1; done
	air

proto:
	buf generate

thrift:
	mkdir -p api/thrift/gen
	thrift --gen go -out api/thrift/gen api/thrift/user.thrift
	rm -rf api/thrift/gen/thriftpb/user_service-remote

thrift-py:
	mkdir -p clients/python/gen
	thrift --gen py -out clients/python/gen api/thrift/user.thrift
	rm -f clients/python/gen/thriftpb/UserService-remote

lint:
	golangci-lint run ./...

SPECS_DIR ?= ../realworld

int-tests:
	docker compose -f compose.test.yaml up -d
	until docker compose -f compose.test.yaml exec -T test_db pg_isready -U admin -d test-app; do sleep 1; done
	go build ./cmd/server
	./server -env .env_test & echo $$! > server.pid
	sleep 2
	HOST=http://localhost:8097 $(SPECS_DIR)/specs/api/run-api-tests-hurl.sh; \
	RESULT=$$?; \
	kill $$(cat server.pid) 2>/dev/null || true; \
	rm -f server.pid; \
	docker compose -f compose.test.yaml exec -T test_db psql -U admin -d test-app -c "TRUNCATE TABLE users CASCADE;"; \
	docker compose -f compose.test.yaml down; \
	exit $$RESULT


int-tests-grpc:
	docker compose -f compose.test.yaml up -d
	until docker compose -f compose.test.yaml exec -T test_db pg_isready -U admin -d test-app; do sleep 1; done
	go build ./cmd/server
	lsof -ti :8097 -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	lsof -ti :8098 -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	./server -env .env_test & echo $$! > server.pid
	sleep 2
	GRPC_HOST=localhost:8098 go test -tags integration ./test/grpc/; \
	RESULT=$$?; \
	kill $$(cat server.pid) 2>/dev/null || true; \
	rm -f server.pid; \
	docker compose -f compose.test.yaml exec -T test_db psql -U admin -d test-app -c "TRUNCATE TABLE users CASCADE;"; \
	docker compose -f compose.test.yaml down; \
	exit $$RESULT
