build:
	@go build -o gproxy ./cmd/app
run:
	@go build -o gproxy ./cmd/app/
	@sudo ./gproxy
