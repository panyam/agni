module github.com/panyam/agni

go 1.27.4

require (
	connectrpc.com/connect v1.21.1
	github.com/bodgit/sevenzip v1.6.5
	github.com/ebitengine/purego v0.8.4
	github.com/klauspost/compress v1.20.2
	github.com/panyam/goapplib v0.7.23
	github.com/panyam/goapplib/storage v0.7.23
	github.com/panyam/jaala/frontends/mcp v0.1.101
	github.com/panyam/jaala/frontends/nl v0.1.101
	github.com/panyam/massrelay v0.3.7
	github.com/panyam/mcpkit v0.8.21
	github.com/panyam/mcpkit/experimental/ext/events v0.8.21
	github.com/panyam/mcpkit/experimental/ext/events/clients/go v0.8.21
	github.com/panyam/mcpkit/experimental/ext/rooms v0.8.21
	github.com/panyam/mcpkit/experimental/ext/rooms/massrelay v0.8.21
	github.com/panyam/mcpkit/experimental/integrations/roomsevents v0.8.21
	github.com/panyam/oneauth v0.3.13
	github.com/panyam/oneauth/grpc v0.3.13
	github.com/panyam/oneauth/oauth2 v0.3.13
	github.com/panyam/servicekit v0.2.4
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/ulikunitz/xz v0.5.15
	golang.org/x/oauth2 v0.38.0
	golang.org/x/text v0.43.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/alexedwards/scs/v2 v2.9.0 // indirect
	github.com/andybalholm/brotli v1.2.7 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/fernet/fernet-go v0.0.0-20240119011108-303da6aec611 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/panyam/jaala/frontends/cypher v0.1.101 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/stangelandcl/ppmd v0.1.1 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
)

require (
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/panyam/gocurrent v0.1.2 // indirect
	github.com/panyam/goutils v0.1.15
	github.com/panyam/jaala v0.1.101
	github.com/panyam/templar v0.1.2 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.84.0
)

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)
