BIN := rkexport.exe 
BUILD_PROD_TEST_LDFLAGS := "-s -w"
BUILD_PROD_LDFLAGS := "-s -w -H=windowsgui"

.PHONY: test
test:
	GOOS=windows GOARCH=amd64 go build -ldflags=$(BUILD_PROD_TEST_LDFLAGS) -o $(BIN) .

.PHONY: prod
prod:
	GOOS=windows GOARCH=amd64 go build -ldflags=$(BUILD_PROD_LDFLAGS) -o $(BIN) .
