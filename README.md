# `eth2-testnet-genesis`

This is a tool to create an Eth2 testnet genesis state, without having to make deposits for all validators.

The steps are:
- Deploy a deposit contract
- Take an Eth1 block hash and corresponding timestamp as starting point. The deposit contract contents must be empty at this block.
- Configure the minimum genesis time. This normally specifies what the above would be, the first eligible eth1 block starting point.
  Make sure to pick an eth1 block that is compatible (has equal or later timestamp) than the minimum genesis time.
  If you pick a time in the future, the generated state is invalid.
- Configure the genesis delay to adjust the actual genesis time (`actual genesis time = chosen eth1 time + delay`)

The result:
- The deposit data tree at genesis will be empty, instead of filled with the genesis validators.
  - The expected empty deposit tree root is `0xd70a234731285c6804c2a4f56711ddb8c82c99740f207854891028af34e27e5e`  (32th order zero hash, with zero length-mixin)
- The validators will be pre-filled in the genesis state.
- The state has an existing eth1 block, empty deposit tree, and zero deposit count, from where deposits can continue.
- The state has a zero deposit index: block proposers won't have to search for non-existing deposits of the pre-filled validators.
- The deposit contract is considered to be empty by Eth2 nodes at genesis. Any deposits will be appended to the validator set, as expected.

## Usage

```
Sub commands:
  phase0     Create genesis state for phase0 beacon chain
  altair     Create genesis state for Altair beacon chain
  bellatrix  Create genesis state for Bellatrix beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs
  capella    Create genesis state for Capella beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs
  version    Print version and exit
```

### Phase0

```
Create genesis state for phase0 beacon chain

Flags/args:
      --config string            Eth2 spec configuration, name or path to YAML (default "mainnet")
      --eth1-block bytes32       Eth1 block hash to put into state (default 0000000000000000000000000000000000000000000000000000000000000000)
      --legacy-config string     Eth2 legacy configuration (combined config and presets), name or path to YAML (default "mainnet")
      --mnemonics string         File with YAML of key sources (default "mnemonics.yaml")
      --validators string        File with list of validators (default "")
      --preset-altair string     Eth2 altair spec preset, name or path to YAML (default "mainnet")
      --preset-merge string      Eth2 merge spec preset, name or path to YAML (default "mainnet")
      --preset-phase0 string     Eth2 phase0 spec preset, name or path to YAML (default "mainnet")
      --preset-sharding string   Eth2 sharding spec preset, name or path to YAML (default "mainnet")
      --state-output string      Output path for state file (default "genesis.ssz")
      --timestamp uint           Eth1 block timestamp (default 1623884847)
      --tranches-dir string      Directory to dump lists of pubkeys of each tranche in (default "tranches")
```

### Altair

```
Create genesis state for altair beacon chain

Flags/args:
      --config string            Eth2 spec configuration, name or path to YAML (default "mainnet")
      --eth1-block bytes32       Eth1 block hash to put into state (default 0000000000000000000000000000000000000000000000000000000000000000)
      --legacy-config string     Eth2 legacy configuration (combined config and presets), name or path to YAML (default "mainnet")
      --mnemonics string         File with YAML of key sources (default "mnemonics.yaml")
      --validators string        File with list of validators (default "")
      --preset-altair string     Eth2 altair spec preset, name or path to YAML (default "mainnet")
      --preset-merge string      Eth2 merge spec preset, name or path to YAML (default "mainnet")
      --preset-phase0 string     Eth2 phase0 spec preset, name or path to YAML (default "mainnet")
      --preset-sharding string   Eth2 sharding spec preset, name or path to YAML (default "mainnet")
      --state-output string      Output path for state file (default "genesis.ssz")
      --timestamp uint           Eth1 block timestamp (default 1623884908)
      --tranches-dir string      Directory to dump lists of pubkeys of each tranche in (default "tranches")
```

### Merge

```
Create genesis state for Merge beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs

Flags/args:
      --legacy-config             Eth2 legacy configuration (combined config and presets), name or path to YAML (default: mainnet) (type: string)
      --config                    Eth2 spec configuration, name or path to YAML (default: mainnet) (type: string)
      --preset-phase0             Eth2 phase0 spec preset, name or path to YAML (default: mainnet) (type: string)
      --preset-altair             Eth2 altair spec preset, name or path to YAML (default: mainnet) (type: string)
      --preset-merge              Eth2 merge spec preset, name or path to YAML (default: mainnet) (type: string)
      --preset-sharding           Eth2 sharding spec preset, name or path to YAML (default: mainnet) (type: string)
      --eth1-config               Path to config JSON for eth1. No transition yet if empty. (default: engine_genesis.json) (type: string)
      --eth1-block                If not transitioned: Eth1 block hash to put into state. (default: 0000000000000000000000000000000000000000000000000000000000000000) (type: bytes32)
      --eth1-timestamp            If not transitioned: Eth1 block timestamp (default: 1633434014) (type: uint64)
      --mnemonics                 File with YAML of key sources (default: mnemonics.yaml) (type: string)
      --validators string         File with list of validators (default "") (type: string)
      --state-output              Output path for state file (default: genesis.ssz) (type: string)
      --tranches-dir              Directory to dump lists of pubkeys of each tranche in (default: tranches) (type: string)
```

## Common inputs

### Eth1 config

A `genesis.json`, as specified in [Geth](https://github.com/ethereum/go-ethereum/blob/307156cc46f7241dfad702f72b8ed2e338770337/core/genesis.go#L49).

### Eth2 config

A standard YAML file, as specified in [Eth2.0 specs](https://github.com/ethereum/eth2.0-specs/tree/dev/configs). Concatenation of configs of all relevant phases.

### Mnemonics

The `mnemonics.yaml` is formatted as:

```yaml
- mnemonic: "reward base tuna ..."  # a 24 word BIP 39 mnemonic
  count: 100  # amount of validators
- mnemonic: "hint dizzy fog ..."
  count: 9000
# ... more
```

### Validators List

In addition, or as alternative to the mnemonic-based validator generation a file with a list of validators can be specified with the `--validators` flag.

The list of validators is formatted as:

```text
# <validator pubkey>:<withdrawal credentials>[:<balance>]
0x9824e447621e4b3bca7794b91c664cc0b43322a70b1881b2f804e3a990a3965a64bfe7f098cb4c0396cd0c89218de0b4:001547805ff0547da9e51a7463a6a0c603eeda01dd930f7016185f0642b9ecaf:32000000000
0xace5689384f87725790499fb5261b586d7dfb7d86058f0a909856272ba02df9929dcdb4b1ea529b02b948b3a1dca4d57:008aa7b9c37bf27e7c49a3185a3e721c7a02c94da7a0b6ad5f88f1b0477d3b88:32000000000
# ... more

# balance is optional and defaults to 32000000000:
0xa33dfc09b4031e8c520469024c0ef419cc148f71d7b9501f58f2e54fc644462f208119791e57c5c9b33bf5e47f705060:00b84654c946dc68b353384426a29a3c5d736d9f751c192d5038206e93f79d73

# individual 0x01 credentials can be set:
0x82fc9f31d6e768c57d09483e788b24444235f64d2cae5f2f8a9dd28b6e8ed6636a5f378febc762cfcd9f8ab808286608:010000000000000000000000CcCCccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC
0xb744b5466a214762ee17621dc4c75d1bba16417e20755f7c9c2485ea518580be50d2c87d70cc4ac393158eb34311c9a2:010000000000000000000000000000000000000000000000000000000000dEaD
```

A validators list can be generated separately via:
```
export MNEMONIC="your mnemonic"
eth2-val-tools deposit-data --fork-version 0x00000000 --source-max 200 --source-min 0 --validators-mnemonic="$MNEMONIC" --withdrawals-mnemonic="$MNEMONIC" --as-json-list | jq ".[] | \"0x\" + .pubkey + \":\" + .withdrawal_credentials + \":32000000000\"" | tr -d '"' > validators.txt
``` 

## Outputs

The output will be:
- `genesis.ssz`: a state to start the network with
- `tranches`: a dir with text files for each mnemonic, listing all pubkeys (1 per line).
  Useful for to check if keystores are generated correctly before genesis, and to track the validators.

## Computing extra details

To get info such as fork digest, genesis validators root, etc. run the `compute_genesis_details.py` script.
Install dependencies with (use of a `venv` recommended):
```
pip install milagro-bls-binding==1.6.3 eth2spec==1.1.0a7
```

## License

MIT. See [`LICENSE`](./LICENSE) file.

