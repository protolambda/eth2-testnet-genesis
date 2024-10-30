# Eth2 Testnet Genesis State Creator

This repository contains a utility for generating an Eth2 testnet genesis state, eliminating the need to make deposits for all validators.

## Process Overview

1. Deploy a deposit contract.
2. Select an Eth1 block hash and corresponding timestamp as the starting point. The deposit contract contents must be empty at this block.
3. Set the minimum genesis time. This usually determines the first eligible Eth1 block starting point.
4. Ensure the selected Eth1 block is compatible (i.e., has an equal or later timestamp) with the minimum genesis time. If a future compared to the min. genesis time is chosen, the generated state will be invalid.
6. Adjust the actual genesis time by configuring the genesis delay (`actual genesis time = chosen Eth1 time + delay`).

## Outcome

- The deposit data tree at genesis will be empty, rather than filled with the genesis validators.
- The expected empty deposit tree root is `0xd70a234731285c6804c2a4f56711ddb8c82c99740f207854891028af34e27e5e` (32th order zero hash, with zero length-mixin).
- The validators will be pre-filled in the genesis state.
- The state will have an existing Eth1 block, an empty deposit tree, and a zero deposit count, from where deposits can continue.
- The state will have a zero deposit index, meaning block proposers won't have to search for non-existing deposits of the pre-filled validators.
- The deposit contract will be considered empty by Eth2 nodes at genesis. Any subsequent deposits will be appended to the validator set, as expected.

## Usage

### Subcommands:

- `phase0`: Create genesis state for phase0 beacon chain.
- `altair`: Create genesis state for Altair beacon chain.
- `bellatrix`: Create genesis state for Bellatrix beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs.
- `capella`: Create genesis state for Capella beacon chain, from execution-layer (only required if post-transition) and consensus-layer configs.
- `deneb`: Create genesis state for Deneb beacon chain, from execution-layer and consensus-layer configs.
- `version`: Print version and exit.

### Common Inputs:

- Eth1 config: A `genesis.json`, as specified in Geth.
- Eth2 config: A standard YAML file, as specified in Eth2.0 specs. Concatenation of configs of all relevant phases.
- Mnemonics: The `mnemonics.yaml` is formatted as shown below. It specifies the amount of validators for each mnemonic.
- Validators List: Alternatively, a file with a list of validators can be specified with the `--additional-validators` flag.

### Outputs:

- `genesis.ssz`: A state to start the network with.
- `tranches`: A directory with text files for each mnemonic, listing all pubkeys (1 per line). Useful for checking if keystores are generated correctly before genesis, and for tracking the validators.

### Example Usage:
- For bellatrix genesis state:
```bash
eth2-testnet-genesis bellatrix --config=config.yaml --mnemonics=mnemonics.yaml --eth1-config=genesis.json
```
- For capella genesis state:
```bash
eth2-testnet-genesis capella --config=config.yaml --mnemonics=mnemonics.yaml --eth1-config=genesis.json
```
- For capella genesis state with additional validators specified :
```bash
eth2-testnet-genesis capella --config=config.yaml --mnemonics=mnemonics.yaml --eth1-config=genesis.json --validators ./validators.yaml
```
- For capella genesis state for a shadowfork:
```bash
eth2-testnet-genesis capella --config=config.yaml --eth1-config="genesis.json" --mnemonics=mnemonics.yaml --shadow-fork-eth1-rpc=http://localhost:8545
```
- For deneb genesis state: like capella, but swap "capella" with "deneb". Options are the same.

*Make sure to set all `--preset-X` (where `X` is an upgrade name) flags when building a genesis for a custom preset (i.e. `minimal` test states).*

### Extra Details:

- To get additional information such as fork digest, genesis validators root, etc., run the `compute_genesis_details.py` script. Install dependencies with `pip install milagro-bls-binding==1.6.3 eth2spec==1.1.0a7` (use of a venv recommended).
- An alternate approach to get this information is to use `zcli`. E.g: `zcli pretty <bellatrix/capella>  BeaconState genesis.ssz > parsedState.json`
- If you want to fetch the EL block to embed in the genesis state from a live node, you can run the tool with the flag `--shadow-fork-eth1-rpc=http://<EL-JSON-RPC-URL>`

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

In addition, or as alternative to the mnemonic-based validator generation a file with a list of validators can be specified with the `--additional-validators` flag.

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

# individual 0x02 credentials can be set:
0x82fc9f31d6e768c57d09483e788b24444235f64d2cae5f2f8a9dd28b6e8ed6636a5f378febc762cfcd9f8ab808286608:020000000000000000000000CcCCccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC
0xb744b5466a214762ee17621dc4c75d1bba16417e20755f7c9c2485ea518580be50d2c87d70cc4ac393158eb34311c9a2:020000000000000000000000000000000000000000000000000000000000dEaD:100000000000

```

A validators list can be generated separately via:
```
export MNEMONIC="your mnemonic"
eth2-val-tools deposit-data --fork-version 0x00000000 --source-max 200 --source-min 0 --validators-mnemonic="$MNEMONIC" --withdrawals-mnemonic="$MNEMONIC" --as-json-list | jq ".[] | \"0x\" + .pubkey + \":\" + .withdrawal_credentials + \":32000000000\"" | tr -d '"' > validators.txt
```

## License

This project is licensed under the MIT License. See the LICENSE file for details.
