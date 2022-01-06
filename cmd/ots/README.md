# ots

ots is a command-line interface to [One-Time Secret](https://onetimesecret.com).

## Installation

```
go install github.com/corbaltcode/go-onetimesecret/cmd/ots
```

## Setup

ots requires a username and API key from [onetimesecret.com](https://onetimesecret.com). You can provide these in one of three ways:

1. On the command line with the `-username` and `-key` options
2. In the environment variables `OTS_USERNAME` and `OTS_KEY`
3. In the config file `$XDG_CONFIG_HOME/ots/config.toml`

The config file is formatted as follows:

```
username = "my-username"
key = "my-key"
```

## Storing, Retrieving, and Destroying Secrets

`ots put` stores a secret and prints the _secret key_ and _metadata key_:

```
$ ots put 'what is essential is invisible to the eye'
hdjk6p0ozf61o7n6pbaxy4in8zuq7sm	ifipvdpeo8oy6r8ryjbu8y7rhm9kty9
```

The secret key is used to retrieve the secret with `ots get`:

```
$ ots get hdjk6p0ozf61o7n6pbaxy4in8zuq7sm
what is essential is invisible to the eye
```

The metadata key, which should be kept private, is used to get metadata with `ots meta` (see below) and destroy the secret with `ots burn`:

```
$ ots burn ifipvdpeo8oy6r8ryjbu8y7rhm9kty9
ifipvdpeo8oy6r8ryjbu8y7rhm9kty9
```

## Generating Secrets

To generate a short, unique secret, use `ots gen`:

```
$ ots gen
rVjbS$twCJkS	44nwhy7v4fnabayqc5auv4ogh0nfr20	flsdlaun6hwczqu9utmc0vts5xj9xu1
```

The secret, secret key, and metadata key are printed, separated by tabs.

## Protecting Secrets

To protect a secret with a passphrase, provide the `-passphrase` option to `ots put` or `ots gen`:

```
$ ots put -passphrase xyzzy 'what is essential is invisible to the eye'
p76qypostz0dkfu3eokwwf33cx6pjtt	9i9dyake8yjnpacsymvplhgr4lki05d
```

Then provide the passphrase to `ots get` or `ots burn`:

```
$ ots get -passphrase xyzzy p76qypostz0dkfu3eokwwf33cx6pjtt
what is essential is invisible to the eye
```

## Avoiding Leaks

In general, secrets should not be supplied on the command line because they may be leaked in command history, logs, `ps` listings, and the like.

If `ots` sees the passphrase `-`, it reads a line from stdin and uses that as the passphrase instead (without the line terminator). For example:

```
$ ots gen -passphrase -
```

Similarly, if no positional argument is supplied to `ots put`, `ots` reads the secret from stdin:

```
$ ots put
```

## Multiline Secrets

If stdin is not a terminal, `ots put` reads from stdin until EOF. This means you can store multiline secrets by redirecting input:

```
$ echo -e -n 'what is essential\nis invisible\nto the eye' | ots put
ourkuoh5a2a9a0kjv5tk8xo4i9xzrbe	k7vp5azb8j2b1noti3z99w6j2hxr08z

$ ots get ourkuoh5a2a9a0kjv5tk8xo4i9xzrbe
what is essential
is invisible
to the eye
```

## Metadata

`ots meta` prints a secret's metadata. This includes the metadata key, secret key, customer ID (username), time to live, state (_new_, _viewed_, _received_, or _burned_), and other data.

```
$ ots gen | cut -f 3
nwizsd2nmtcb92oiy93o1nf3vv28pgo

$ ots meta nwizsd2nmtcb92oiy93o1nf3vv28pgo
jonah@corbalt.com	nwizsd2nmtcb92oiy93o1nf3vv28pgo ...
```

## Output Format

By default, `ots` prints tab-separated values, one record per line. If the `-json` option is given, `ots` prints JSON. This makes the output easier to read and lets you use `jq`:

```
$ ots gen | cut -f 3
bz70207ov6fwthe6kqo6e7ij1ol2sdi

$ ots meta -json bz70207ov6fwthe6kqo6e7ij1ol2sdi
{
	"CustomerID": "jonah@jonahb.com",
	"MetadataKey": "bz70207ov6fwthe6kqo6e7ij1ol2sdi",
	"SecretKey": "7yptfx0saapleidh2fcq1de9i7ba18n",
	"InitialMetadataTTL": 2419200,
	"MetadataTTL": 2419189,
	"SecretTTL": 1209588,
	"State": "viewed",
	"Updated": "2021-11-26T18:55:14-05:00",
	"Created": "2021-11-26T18:55:14-05:00",
	"ObfuscatedRecipient": "",
	"HasPassphrase": false
}

$ ots meta -json bz70207ov6fwthe6kqo6e7ij1ol2sdi | jq -r '.State'
viewed
```
