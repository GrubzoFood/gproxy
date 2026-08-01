build:
	@go build -o gproxy ./cmd/app
run:
	@go build -o gproxy ./cmd/app/
	@sudo GPROXY_DEV=true ./gproxy 2>&1 | sudo tee /var/log/gproxy.log
