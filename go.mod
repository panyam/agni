module github.com/panyam/agni

go 1.26.4

require (
	connectrpc.com/connect v1.20.0
	github.com/panyam/goapplib v0.1.1
	github.com/panyam/servicekit v0.1.2
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/panyam/gocurrent v0.1.1 // indirect
	github.com/panyam/goutils v0.1.13 // indirect
	github.com/panyam/templar v0.1.2 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/grpc v1.82.1 // indirect
)

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)
