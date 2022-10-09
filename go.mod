module github.com/protolambda/eth2-testnet-genesis

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/VictoriaMetrics/fastcache v1.7.0 // indirect
	github.com/ethereum/go-ethereum v1.10.23
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/herumi/bls-eth-go-binary v0.0.0-20210917013441-d37c07cfda4e
	github.com/holiman/uint256 v1.2.0
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/protolambda/ask v0.1.2
	github.com/protolambda/bls12-381-util v0.0.0-20210812140640-b03868185758 // indirect
	github.com/protolambda/zrnt v0.26.0
	github.com/protolambda/ztyp v0.2.1
	github.com/shirou/gopsutil v3.21.8+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0
	github.com/wealdtech/go-eth2-util v1.7.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/ethereum/go-ethereum v1.10.23 => github.com/gballet/go-ethereum v1.10.24-0.20221005144906-f0e4ab07b54a
