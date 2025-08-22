module github.com/luxfi/adx

go 1.24.6

require (
	github.com/luxfi/crypto v1.3.2
	github.com/luxfi/ids v1.0.2
	github.com/luxfi/log v1.1.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/crypto v0.41.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20250813145105-42675adae3e6 // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/luxfi/consensus => ../consensus
	github.com/luxfi/crypto => ../crypto
	github.com/luxfi/database => ../database
	github.com/luxfi/ids => ../ids
	github.com/luxfi/log => ../log
	github.com/luxfi/metric => ../metric
)
