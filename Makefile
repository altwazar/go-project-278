build:
	go build -o bin/urlshortener ./main.go
test:
	go test -v ./...
