module github.com/luxfi/adx

go 1.24.6

require (
	// CTV Ad Exchange dependencies
	github.com/gorilla/websocket v1.5.3
	github.com/prebid/openrtb/v20 v20.1.0
	github.com/shopspring/decimal v1.4.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.41.0
)

require (
	github.com/apple/foundationdb/bindings/go v0.0.0-20250822051852-75f408187073 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Note: Replace directives commented out for standalone repository
// Uncomment and adjust paths when integrating with full luxfi monorepo
// replace (
//	github.com/luxfi/consensus => ../consensus
//	github.com/luxfi/crypto => ../crypto
//	github.com/luxfi/database => ../database
//	github.com/luxfi/ids => ../ids
//	github.com/luxfi/log => ../log
//	github.com/luxfi/metric => ../metric
// )
