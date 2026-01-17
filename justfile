
test:
	@rm -f test/* && \
	mkdir -p test && \
	go fmt ./... && \
	go vet ./... && \
	staticcheck ./... && \
	errcheck ./... && \
	revive -config ~/.revive.toml ./... && \
	gosec -quiet ./... && \
	govulncheck ./... && \
	go test ./... -race -vet=all -shuffle=on -count=1 -timeout=30s -coverprofile=test/coverage.out && \
	go tool cover -func=test/coverage.out | tail -1 && \
	go tool cover -html=test/coverage.out -o test/coverage.html
