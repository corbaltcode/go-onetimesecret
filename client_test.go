package onetimesecret

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

var c Client

const ttlAllowedError = 3

func init() {
	c = Client{
		Username: mustGetenv("OTS_USERNAME"),
		Key:      mustGetenv("OTS_KEY"),
	}
}

func TestGet(t *testing.T) {
	want := randStr()
	meta, err := c.Put(want, "", 0, "")
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	got, err := c.Get(meta.SecretKey, "")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != want {
		t.Errorf("got secret %v (want %v)", got, want)
	}
}

func TestGetWithPassphrase(t *testing.T) {
	want := randStr()
	passphrase := randStr()
	meta, err := c.Put(want, passphrase, 0, "")
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	got, err := c.Get(meta.SecretKey, passphrase)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != want {
		t.Errorf("got secret %v (want %v)", got, want)
	}
}

func TestGetWrongPassphrase(t *testing.T) {
	meta, err := c.Put(randStr(), "right", 0, "")
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	_, err = c.Get(meta.SecretKey, "wrong")
	if err != ErrNotFound {
		t.Errorf("got error %v (want %v)", err, ErrNotFound)
	}
}

func TestGetNonexistent(t *testing.T) {
	_, err := c.Get(randStr(), "")
	if err != ErrNotFound {
		t.Errorf("got error %v (want %v)", err, ErrNotFound)
	}
}

func TestPut(t *testing.T) {
	ttl := 60 + rand.Intn(1000)
	recipient := "foo@example.com"
	obfuscatedRecipient := "fo*****@e*****.com"
	meta, err := c.Put(randStr(), randStr(), ttl, recipient)
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if meta.State != SecretStateNew {
		t.Errorf("wrong State %v (want %v)", meta.State, SecretStateNew)
	}
	if !meta.HasPassphrase {
		t.Errorf("wrong HasPassphrase %v (want %v)", meta.HasPassphrase, true)
	}
	if ttl-meta.SecretTTL > ttlAllowedError {
		t.Errorf("wrong SecretTTL %v (want %v)", meta.SecretTTL, ttl)
	}
	if 2*ttl-meta.MetadataTTL > ttlAllowedError {
		t.Errorf("wrong MetadataTTL %v (want %v)", meta.MetadataTTL, 2*ttl)
	}
	if meta.InitialMetadataTTL != 2*ttl {
		t.Errorf("wrong InitialMetadataTTL %v (want %v)", meta.InitialMetadataTTL, 2*ttl)
	}
	if meta.ObfuscatedRecipient != obfuscatedRecipient {
		t.Errorf("wrong Recipient %v (want %v)", meta.ObfuscatedRecipient, obfuscatedRecipient)
	}
}

func TestPutNothing(t *testing.T) {
	_, err := c.Put("", "", 0, "")
	if err != ErrInvalid {
		t.Errorf("got error %v (want %v)", err, ErrInvalid)
	}
}

func TestGenerate(t *testing.T) {
	ttl := 60 + rand.Intn(1000)
	recipient := "foo@example.com"
	obfuscatedRecipient := "fo*****@e*****.com"
	secret, meta, err := c.Generate(randStr(), ttl, recipient)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if secret == "" {
		t.Errorf("empty secret")
	}
	if meta.State != SecretStateNew {
		t.Errorf("wrong State %v (want %v)", meta.State, SecretStateNew)
	}
	if !meta.HasPassphrase {
		t.Errorf("wrong HasPassphrase %v (want %v)", meta.HasPassphrase, true)
	}
	if ttl-meta.SecretTTL > ttlAllowedError {
		t.Errorf("wrong SecretTTL %v (want %v)", meta.SecretTTL, ttl)
	}
	if 2*ttl-meta.MetadataTTL > ttlAllowedError {
		t.Errorf("wrong MetadataTTL %v (want %v)", meta.MetadataTTL, 2*ttl)
	}
	if meta.InitialMetadataTTL != 2*ttl {
		t.Errorf("wrong InitialMetadataTTL %v (want %v)", meta.InitialMetadataTTL, 2*ttl)
	}
	if meta.ObfuscatedRecipient != obfuscatedRecipient {
		t.Errorf("wrong Recipient %v (want %v)", meta.ObfuscatedRecipient, obfuscatedRecipient)
	}
}

func randStr() string {
	return fmt.Sprint(rand.Int())
}

func mustGetenv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("missing env var: %v", key))
	}
	return val
}
