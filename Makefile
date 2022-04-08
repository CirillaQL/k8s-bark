BINARY_NAME=k8s-bark

build:
	GOARCH=amd64 GOOS=linux go build -o ./bin/${BINARY_NAME} ./main.go

clean:
	go clean
	rm -rf ./bin/${BINARY_NAME}