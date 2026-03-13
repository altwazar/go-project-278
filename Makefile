build:
	go build -o bin/urlshortener ./main.go
test:
	go test -v ./... -race
test-integration:
	go test -tags=integration ./... -v
test-integration-local:
	TESTCONTAINERS_RYUK_DISABLED=true DOCKER_HOST=unix://${XDG_RUNTIME_DIR}/podman/podman.sock go test -tags=integration ./... -v
