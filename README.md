# sia-antfarm
[![Build Status](https://travis-ci.org/NebulousLabs/Sia-Ant-Farm.svg?branch=travis)](https://travis-ci.org/NebulousLabs/Sia)

sia-antfarm is a collection of utilities for performing complex, end-to-end
tests of the [Sia](https://github.com/NebulousLabs/Sia) platform.  These test
are long-running and offer superior insight into high-level
network behaviour than Sia's existing automated testing suite.

# Prerequisites

You must have `siad` installed.  If it's outside of your path, provide its location using the `-siad` flag.  For all flags, see `sia-antfarm -h`.

# Install

`make`

# Running a sia-antfarm

This repository contains one utility, `sia-antfarm`. `sia-antfarm` starts up a number of `siad` instances, using jobs and configuration options parsed from the input `config.json`. `sia-antfarm` takes on flag, `-config`, which is a path to a JSON file defining the ants and their jobs. See `nebulous-configs/` for some examples that we use to test Sia.

An example `config.json`:

`config.json:`
```json
{
	"antconfigs": 
	[ 
		{
			"jobs": [
				"gateway"
			]
		},
		{
			"jobs": [
				"gateway"
			]
		},
		{
			"jobs": [
				"gateway"
			]
		},
		{
			"jobs": [
				"gateway"
			]
		},
		{
			"apiaddr": "127.0.0.1:9980",
			"jobs": [
				"gateway",
				"mining"
			]
		}
	],
	"autoconnect": true
}
```

This `config.json` creates 5 ants, with four running the `gateway` job
and one running a `gateway` and a `mining` job.  If `HostAddr`, `APIAddr`, or
`RPCAddr` are not specified, they will be set to a random port.  If
`autoconnect` is set to `false`, the ants will not automatically be made peers
of eachother.

## Available configuration options:

```
{
	'ListenAddress': the listen address that the `sia-antfarm` API listens on
	'AntConfigs': an array of `AntConfig` objects, defining the ants to run on this antfarm
	'AutoConnect': a boolean which automatically bootstraps the antfarm if provided
	'ExternalFarms': an array of strings, where each string is the api address of an external antfarm to connect to.
}
```

`AntConfig`s have the following options:
```
{
	'APIAddr': the api address for the ant to listen on, by default an unused localhost: bind address will be used.
	'RPCAddr': the RPC address for the ant to listen on, by default an unused bind address will be used.
	'HostAddr': the Host address for the ant to listen on, by default an unused bind address will be used.
	'SiaDirectory': the data directory to use for this ant, by default a unique directory in `./antfarm-data` will be generated and used.
	'SiadPath': the path to the `siad` binary, by default the `siad` in your path will be used.
	'Jobs': an array of jobs for this ant to run. available jobs include: ['miner', 'host', 'renter', 'gateway']
	'DesiredCurrency': a minimum (integer) amount of SiaCoin that this Ant will attempt to maintain by mining currency. This is mutually exclusive with the `miner` job.
	'UpgradePath': an array of version strings to upgrade to. Each upgrade will be triggered after `UpgradeDelay` seconds.
	'UpgradeDir': path to a directory containing the binaries to use for upgrades. This directory should be in the format of 'dir/version-platform-arch/siad'.
	'UpgradeDelay': number of seconds to wait between upgrades.
}
```



# License

The MIT License (MIT)

