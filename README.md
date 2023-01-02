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

