package onetimesecret

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrDestroyed is returned when a secret URL is requested but the secret has
// been destroyed.
var ErrDestroyed = errors.New("onetimesecret: burned or retrieved")

// ErrInvalid is returned when the client attempts to store an empty secret.
var ErrInvalid = errors.New("onetimesecret: invalid argument")

// ErrNotFound is returned when there is no secret with the provided metadata
// key or secret key, or an incorrect passphrase is provided.
var ErrNotFound = errors.New("onetimesecret: unknown secret")

var baseURL url.URL

func init() {
	baseURL = url.URL{Scheme: "https", Host: "onetimesecret.com"}
}

type SecretState string

const (
	SecretStateOther    SecretState = "other"
	SecretStateBurned   SecretState = "burned"
	SecretStateNew      SecretState = "new"
	SecretStateReceived SecretState = "received"
	SecretStateViewed   SecretState = "viewed"
)

type SystemStatus string

const (
	SystemStatusOther   SystemStatus = "other"
	SystemStatusNominal SystemStatus = "nominal"
	SystemStatusOffline SystemStatus = "offline"
)

type Metadata struct {
	CustomerID          string
	MetadataKey         string
	SecretKey           string
	InitialMetadataTTL  int
	MetadataTTL         int
	SecretTTL           int
	State               SecretState
	Updated             time.Time
	Created             time.Time
	ObfuscatedRecipient string
	HasPassphrase       bool
}

// SecretURL returns a URL that allows retrieving the secret. If the secret has
// been destroyed, SecretURL returns ErrDestroyed.
func (m Metadata) SecretURL() (*url.URL, error) {
	if m.SecretKey == "" {
		return nil, ErrDestroyed
	}
	u := baseURL
	u.Path += "secret/" + url.PathEscape(m.SecretKey)
	return &u, nil
}

// MetadataURL returns a URL that allows retrieving the secret, burning the
// secret, and viewing its metadata.
func (m Metadata) MetadataURL() *url.URL {
	u := baseURL
	u.Path += "private/" + url.PathEscape(m.MetadataKey)
	return &u
}

func (m *Metadata) fromKeyResponse(kr keyResponse) {
	m.CustomerID = kr.CustomerID
	m.MetadataKey = kr.MetadataKey
	m.SecretKey = kr.SecretKey
	m.InitialMetadataTTL = kr.TTL
	m.MetadataTTL = kr.MetadataTTL
	m.SecretTTL = kr.SecretTTL
	m.State = parseSecretState(kr.State)
	m.Updated = time.Unix(int64(kr.Updated), 0)
	m.Created = time.Unix(int64(kr.Created), 0)
	if len(kr.Recipient) > 0 {
		m.ObfuscatedRecipient = kr.Recipient[0]
	}
	m.HasPassphrase = kr.PassphraseRequired
}

type PartialMetadata struct {
	CustomerID         string
	MetadataKey        string
	InitialMetadataTTL int
	MetadataTTL        int
	SecretTTL          int
	State              SecretState
	Updated            time.Time
	Created            time.Time
	Recipient          string
}

func (m *PartialMetadata) fromKeyResponse(kr keyResponse) {
	m.CustomerID = kr.CustomerID
	m.MetadataKey = kr.MetadataKey
	m.InitialMetadataTTL = kr.TTL
	m.MetadataTTL = kr.MetadataTTL
	m.SecretTTL = kr.SecretTTL
	m.State = parseSecretState(kr.State)
	m.Updated = time.Unix(int64(kr.Updated), 0)
	m.Created = time.Unix(int64(kr.Created), 0)
	if len(kr.Recipient) > 0 {
		m.Recipient = kr.Recipient[0]
	}
}

// A Client allows access to One-Time Secret.
type Client struct {
	Username string
	Key      string
}

// Get retrieves a secret given a secret key and, if necessary, a passphrase.
// If there is no secret with the given secret key or the passphrase is
// incorrect, Get returns ErrNotFound.
func (c *Client) Get(secretKey string, passphrase string) (string, error) {
	v := url.Values{}
	v.Add("passphrase", passphrase)
	path := "secret/" + url.PathEscape(secretKey)

	var kr keyResponse
	err := c.do("POST", path, v, nil, &kr)
	if err != nil {
		return "", err
	}

	return kr.Value, nil
}

// Put stores a secret with an optional passphrase and TTL in seconds and
// returns the new secret's metadata. If the secret is empty, Put returns
// ErrInvalid.
func (c *Client) Put(secret string, passphrase string, secretTTL int, recipient string) (Metadata, error) {
	v := url.Values{}
	v.Add("secret", secret)
	v.Add("passphrase", passphrase)
	v.Add("ttl", fmt.Sprint(secretTTL))
	v.Add("recipient", recipient)

	var kr keyResponse
	err := c.do("POST", "share", v, nil, &kr)
	if err != nil {
		return Metadata{}, err
	}

	m := Metadata{}
	m.fromKeyResponse(kr)
	return m, nil
}

// Generate creates a short, unique secret with an optional passphrase and TTL,
// returning the secret and its metadata.
func (c *Client) Generate(passphrase string, secretTTL int, recipient string) (string, Metadata, error) {
	v := url.Values{}
	v.Add("passphrase", passphrase)
	v.Add("ttl", fmt.Sprint(secretTTL))
	v.Add("recipient", recipient)

	var kr keyResponse
	err := c.do("POST", "generate", v, nil, &kr)
	if err != nil {
		return "", Metadata{}, err
	}

	m := Metadata{}
	m.fromKeyResponse(kr)
	return kr.Value, m, nil
}

// Burn destroys a secret given its metadata key and, if necessary, passphrase.
// If there is no secret with the given metadata key or the passphrase is
// incorrect, Burn returns ErrNotFound.
func (c *Client) Burn(metadataKey string, passphrase string) (Metadata, error) {
	v := url.Values{}
	v.Add("passphrase", passphrase)

	var br burnResponse
	path := "private/" + url.PathEscape(metadataKey) + "/burn"
	err := c.do("POST", path, v, nil, &br)
	if err != nil {
		return Metadata{}, err
	}

	m := Metadata{}
	m.fromKeyResponse(br.State)
	return m, nil
}

// GetMetadata returns metadata for a secret given a metadata key. If there is
// no secret with the given metadata key, GetMetadata returns ErrNotFound.
func (c *Client) GetMetadata(metadataKey string) (Metadata, error) {
	var kr keyResponse
	path := "private/" + url.PathEscape(metadataKey)
	err := c.do("POST", path, url.Values{}, nil, &kr)
	if err != nil {
		return Metadata{}, err
	}

	m := Metadata{}
	m.fromKeyResponse(kr)
	return m, nil
}

// GetRecentMetadata returns partial metadata for recently created secrets.
func (c *Client) GetRecentMetadata() ([]PartialMetadata, error) {
	var krs []keyResponse
	err := c.do("GET", "private/recent", url.Values{}, nil, &krs)
	if err != nil {
		return nil, err
	}

	ms := []PartialMetadata{}
	for _, kr := range krs {
		m := PartialMetadata{}
		m.fromKeyResponse(kr)
		ms = append(ms, m)
	}

	return ms, nil
}

// GetSystemStatus returns the status of the One-Time Secret system.
func (c *Client) GetSystemStatus() (SystemStatus, error) {
	r := systemStatusResponse{}
	err := c.do("GET", "status", url.Values{}, nil, &r)
	if err != nil {
		return "", err
	}
	return parseSystemStatus(r.Status), nil
}

func (c *Client) do(method string, path string, query url.Values, body io.Reader, out interface{}) error {
	u := baseURL
	u.Path += "api/v1/" + path
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}
	req.URL.RawQuery = query.Encode()
	req.SetBasicAuth(c.Username, c.Key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er errorResponse
		err = json.Unmarshal(respBody, &er)
		if err != nil {
			return err
		}
		switch er.Message {
		case "You did not provide anything to share":
			return ErrInvalid
		case "Unknown secret":
			return ErrNotFound
		default:
			return fmt.Errorf("error: %v", er.Message)
		}
	}

	err = json.Unmarshal(respBody, out)
	if err != nil {
		return err
	}

	return nil
}

func parseSecretState(s string) SecretState {
	switch s {
	case "burned":
		return SecretStateBurned
	case "new":
		return SecretStateNew
	case "received":
		return SecretStateReceived
	case "viewed":
		return SecretStateViewed
	default:
		return SecretStateOther
	}
}

func parseSystemStatus(s string) SystemStatus {
	switch s {
	case "nominal":
		return SystemStatusNominal
	case "offline":
		return SystemStatusOffline
	default:
		return SystemStatusOther
	}
}

type burnResponse struct {
	State          keyResponse `json:"state"`
	SecretShortkey string      `json:"secret_shortkey"`
}

type errorResponse struct {
	Message string `json:"message"`
}

type keyResponse struct {
	CustomerID         string   `json:"custid"`
	MetadataKey        string   `json:"metadata_key"`
	SecretKey          string   `json:"secret_key"`
	TTL                int      `json:"ttl"`
	MetadataTTL        int      `json:"metadata_ttl"`
	SecretTTL          int      `json:"secret_ttl"`
	State              string   `json:"state"`
	Updated            int      `json:"updated"`
	Created            int      `json:"created"`
	Recipient          []string `json:"recipient"`
	Value              string   `json:"value"`
	PassphraseRequired bool     `json:"passphrase_required"`
}

type systemStatusResponse struct {
	Status string `json:"status"`
}
