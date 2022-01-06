# go-onetimesecret

go-onetimesecret is a Go client for [One-Time Secret](https://onetimesecret.com). It includes a command-line interface, [ots](https://github.com/corbaltcode/go-onetimesecret/tree/main/cmd/ots).

## Installation

```
$ go get github.com/corbaltcode/go-onetimesecret
```

## Creating a Client

All operations are performed by calling methods on a `Client`. Create a `Client` by supplying your username (email) and password from [onetimesecret.com](https://onetimesecret.com).

```
import ots "github.com/corbaltcode/go-onetimesecret"

client := ots.Client{
  Username: "user@example.com",
  Key: "my-api-key",
}
```

## Storing & Retrieving Secrets

Use `Client.Put` and `Client.Get` to store and retrieve secrets. Once a secret has been retrieved, it's gone.

```
metadata, err := client.Put("the launch codes", "", 0, "")
if err != nil { ... }

secret, err := client.Get(metadata.SecretKey, "")
if err != nil { ... }

// prints "the launch codes"
print(secret)

// now the secret is gone
secret, err = client.Get(metadata.SecretKey, "")
if errors.Is(err, ots.ErrNotFound) {
  // handle error
}
```

## Using a Passphrase

Protect a secret by providing a passphrase to `Client.Put` and `Client.Generate` (see below). The passphrase will be required to retrieve or destroy the secret.

```
passphrase := xyzzy

metadata, err := client.Put("the launch codes", passphrase, 0, "")
if err != nil { ... }

secret, err = client.Get(metadata.SecretKey, "wrong passphrase")
if errors.Is(err, ots.ErrNotFound) {
  // handle error
}

secret, err := client.Get(metadata.SecretKey, passphrase)
if err != nil { ... }

// prints "the launch codes"
print(secret)
```

## Generating Secrets

One-Time Secret can generate short, unique secrets.

```
passphrase := "xyzzy"

secret, metadata, err := client.Generate(passphrase, 0, "")
if err != nil { ... }

// prints the generated secret
print(secret)
```

## Destroying Secrets

Destroy a secret by passing the metadata key and passphrase, if necessary, to `Client.Burn`.

```
passphrase := "xyzzy"

metadata, err := client.Put("the launch codes", passphrase, 0, "")
if err != nil { ... }

// destroys the secret
metadata, err = client.Burn(metadata.MetadataKey, passphrase)
if err != nil { ... }

// now the secret is gone
metadata, err = client.Burn(metadata.MetadataKey, passphrase)
if errors.Is(err, ots.ErrNotFound) {
  // handle error
}
```

## Sharing Secrets

Use `Metadata.SecretURL` to get a URL for sharing the secret:

```
metadata, err := client.Generate("", 0, "")
if err != nil { ... }

url, err := metadata.SecretURL()
if err != nil { ... }

// prints "https://onetimesecret.com/secret/<secret-key>"
print(url.String())
```

## Testing

Set the environment variables `OTS_USERNAME` and `OTS_KEY`, then:

```
go test ./...
```

## Contributing

Submit issues and pull requests to [corbaltcode/go-onetimesecret](https://github.com/corbaltcode/go-onetimesecret) on GitHub.
