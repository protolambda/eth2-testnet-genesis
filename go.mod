module github.com/protolambda/eth2-testnet-genesis

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/VictoriaMetrics/fastcache v1.7.0 // indirect
	github.com/ethereum/go-ethereum v1.10.9
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/herumi/bls-eth-go-binary v0.0.0-20210917013441-d37c07cfda4e
	github.com/holiman/uint256 v1.2.1
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/protolambda/ask v0.1.2
	github.com/protolambda/bls12-381-util v0.0.0-20210812140640-b03868185758 // indirect
	github.com/protolambda/zrnt v0.26.0
	github.com/protolambda/ztyp v0.2.2
	github.com/shirou/gopsutil v3.21.8+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0
	github.com/wealdtech/go-eth2-util v1.7.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/protolambda/zrnt => github.com/gballet/zrnt v0.10.2-0.20230223160905-39f8f10187e2

replace github.com/ethereum/go-ethereum => github.com/gballet/go-ethereum v1.10.24-0.20230307102830-acf5ede40e00
