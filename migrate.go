package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type V2Client struct {
	keysAPI        client.KeysAPI
	srcAPIEndpoint string
	srcPrefix      string
	srcDomain      string
	dstAPIEndpoint string
	dstDomain      string
	httpClient     *http.Client
	lock           *sync.RWMutex
}

func NewV2Client(endpoints []string, srcAPIEndpoint, srcPrefix, srcDomain, dstAPIEndpoint, dstDomain string) (*V2Client, error) {
	cfg := client.Config{
		Endpoints:               endpoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}

	return &V2Client{
		keysAPI:        client.NewKeysAPI(c),
		srcAPIEndpoint: srcAPIEndpoint,
		srcPrefix:      srcPrefix,
		srcDomain:      srcDomain,
		dstAPIEndpoint: dstAPIEndpoint,
		dstDomain:      dstDomain,
		httpClient:     http.DefaultClient,
		lock:           &sync.RWMutex{},
	}, nil
}

func (v *V2Client) MigrateFrozen() error {
	frozen, err := v.GetFrozen()
	if err != nil {
		return err
	}

	for _, f := range frozen {
		if err := v.POSTFrozenRecord(f); err != nil {
			logrus.Error(err)
		}
	}

	return nil
}

func (v *V2Client) MigrateRecords() error {
	tokens, err := v.GetTokens()
	if err != nil {
		return err
	}

	das := make([]*Domain, 0)
	dts := make([]*Domain, 0)

	for _, t := range tokens {
		if err := v.POSTTokenRecord(t); err != nil {
			logrus.Error(err)
			continue
		}

		da, err := v.QueryARecord(t)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if len(da.Hosts) >= 1 {
			das = append(das, da)
		}

		dt, err := v.QueryTXTRecord(t)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if dt.Text != "" {
			t, err := convertToMap(dt.Text)
			if err == nil {
				dt.Text = t["text"]
			}
			dts = append(dts, dt)
		}
	}

	for _, a := range das {
		if err := v.POSTRecord(a); err != nil {
			logrus.Error(err)
			continue
		}
	}

	for _, t := range dts {
		if err := v.POSTRecord(t); err != nil {
			logrus.Error(err)
			continue
		}
	}

	return nil
}

func (v *V2Client) GetTokens() ([]Token, error) {
	path := "/token_origin"
	opts := &client.GetOptions{Recursive: true}
	tokens := make([]Token, 0)

	resp, err := v.keysAPI.Get(context.Background(), path, opts)
	if err != nil {
		return tokens, err
	}

	for _, n := range resp.Node.Nodes {
		token := Token{
			Path:       n.Key,
			Token:      n.Value,
			Expiration: n.Expiration,
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (v *V2Client) GetFrozen() ([]Frozen, error) {
	path := fmt.Sprintf("%s/%s", v.srcPrefix, "_frozen")
	opts := &client.GetOptions{Recursive: true}

	frozen := make([]Frozen, 0)

	resp, err := v.keysAPI.Get(context.Background(), path, opts)
	if err != nil {
		return frozen, err
	}

	for _, n := range resp.Node.Nodes {
		f := Frozen{
			Path:       n.Key,
			Expiration: n.Expiration,
		}
		frozen = append(frozen, f)
	}

	return frozen, nil
}

func (v *V2Client) QueryARecord(t Token) (d *Domain, err error) {
	fqdn := getDomainFromToken(t.Path)
	url := fmt.Sprintf("%s/v1/domain/%s", v.srcAPIEndpoint, fqdn)

	req, err := v.Request(http.MethodGet, url, nil)
	if err != nil {
		return d, errors.Wrapf(err, "QueryARecord: failed to build a request")
	}

	token, err := generateToken(t.Token)
	if err != nil {
		return d, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	o, err := v.Do(req)
	if err != nil {
		return d, errors.Wrap(err, "QueryARecord: failed to execute a request")
	}

	return &o.Data, nil
}

func (v *V2Client) QueryTXTRecord(t Token) (d *Domain, err error) {
	fqdn := getDomainFromToken(t.Path)
	// only migrate acme txt record
	url := fmt.Sprintf("%s/v1/domain/_acme-challenge.%s/txt", v.srcAPIEndpoint, fqdn)

	req, err := v.Request(http.MethodGet, url, nil)
	if err != nil {
		return d, errors.Wrapf(err, "QueryTXTRecord: failed to build a request")
	}

	token, err := generateToken(t.Token)
	if err != nil {
		return d, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	o, err := v.Do(req)
	if err != nil {
		return d, errors.Wrap(err, "QueryTXTRecord: failed to execute a request")
	}

	return &o.Data, nil
}

func (v *V2Client) POSTFrozenRecord(f Frozen) error {
	v.lock.RLock()
	defer v.lock.RUnlock()
	url := fmt.Sprintf("%s/v1/migrate/frozen", v.dstAPIEndpoint)

	f.Path = strings.Split(f.Path, "/")[3]

	body, err := v.JSONBody(f)
	if err != nil {
		return err
	}

	req, err := v.Request(http.MethodPost, url, body)
	if err != nil {
		return errors.Wrapf(err, "POSTFrozenRecord: failed to build a request")
	}

	_, err = v.Do(req)
	if err != nil {
		return errors.Wrap(err, "POSTFrozenRecord: failed to execute a request")
	}

	return nil
}

func (v *V2Client) POSTTokenRecord(t Token) error {
	v.lock.RLock()
	defer v.lock.RUnlock()
	url := fmt.Sprintf("%s/v1/migrate/token", v.dstAPIEndpoint)
	fqdn := getDomainFromToken(t.Path)

	if v.srcDomain != v.dstDomain {
		t.Path = fmt.Sprintf("%s.%s", strings.Split(fqdn, ".")[0], v.dstDomain)
	}

	body, err := v.JSONBody(t)
	if err != nil {
		return err
	}

	req, err := v.Request(http.MethodPost, url, body)
	if err != nil {
		return errors.Wrapf(err, "POSTTokenRecord: failed to build a request")
	}

	_, err = v.Do(req)
	if err != nil {
		return errors.Wrap(err, "POSTTokenRecord: failed to execute a request")
	}

	return nil
}

func (v *V2Client) POSTRecord(d *Domain) error {
	v.lock.RLock()
	defer v.lock.RUnlock()
	url := fmt.Sprintf("%s/v1/migrate/record", v.dstAPIEndpoint)

	if v.srcDomain != v.dstDomain {
		if d.Text == "" {
			d.Fqdn = fmt.Sprintf("%s.%s", strings.Split(d.Fqdn, ".")[0], v.dstDomain)
		} else {
			d.Fqdn = fmt.Sprintf("%s.%s.%s", strings.Split(d.Fqdn, ".")[0], strings.Split(d.Fqdn, ".")[1], v.dstDomain)
		}
	}

	body, err := v.JSONBody(d)
	if err != nil {
		return err
	}

	req, err := v.Request(http.MethodPost, url, body)
	if err != nil {
		return errors.Wrapf(err, "POSTRecord: failed to build a request")
	}

	_, err = v.Do(req)
	if err != nil {
		return errors.Wrap(err, "POSTRecord: failed to execute a request")
	}

	return nil
}

func (v *V2Client) Request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

func (v *V2Client) Do(req *http.Request) (Response, error) {
	var data Response

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, errors.Wrap(err, "read response body error")
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, errors.Wrapf(err, "decode response error: %s", string(body))
	}

	logrus.Debugf("got response entry: %+v", data)

	if code := resp.StatusCode; code < 200 || code > 300 {
		if data.Message != "" {
			return data, errors.Errorf("got request error: %s", data.Message)
		}
	}

	return data, nil
}

func (v *V2Client) JSONBody(payload interface{}) (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(payload)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func getDomainFromToken(src string) string {
	ss := strings.Split(src, "/")
	return strings.Join(strings.Split(ss[2], "_"), ".")
}

func generateToken(origin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(origin), bcrypt.MinCost)
	if err != nil {
		logrus.Errorf("failed to generate token with %s, err: %v", origin, err)
		return "", err
	}
	token := base64.StdEncoding.EncodeToString(hash)

	return token, nil
}

func convertToMap(value string) (map[string]string, error) {
	var v map[string]string
	err := json.Unmarshal([]byte(value), &v)
	return v, err
}
