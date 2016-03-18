package gateway

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/juju/errors"
)

const (
	etcdTimeout    = 3 * time.Second
	requestTimeout = 3 * time.Second

	// Etcd use "/v2/keys" for its http v2 prefix, so
	// here we use /v3/keys.
	keysPrefix = "/v3/keys"
)

type Gateway struct {
	cfg    *Config
	client *clientv3.Client
}

func NewGateway(cfg *Config) (*Gateway, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.EtcdAddrs,
		DialTimeout: etcdTimeout,
	})

	if err != nil {
		return nil, errors.Trace(err)
	}

	gw := &Gateway{
		cfg:    cfg,
		client: client,
	}

	return gw, nil
}

func (gw *Gateway) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc(keysPrefix, gw.handleKeys)
	mux.HandleFunc(keysPrefix+"/", gw.handleKeys)

	s := &http.Server{
		Addr:         gw.cfg.Addr,
		Handler:      mux,
		ReadTimeout:  requestTimeout,
		WriteTimeout: requestTimeout,
	}

	err := s.ListenAndServe()
	return errors.Trace(err)
}

func (gw *Gateway) handleKeys(w http.ResponseWriter, r *http.Request) {
	var err error

	switch r.Method {
	case http.MethodGet:
		err = gw.handleGetKeys(w, r)
	case http.MethodPut, http.MethodPost:
		err = gw.handlePutKeys(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

func (gw *Gateway) handleGetKeys(w http.ResponseWriter, r *http.Request) error {
	key := strings.TrimPrefix(r.URL.Path, keysPrefix)

	kv := clientv3.KV(gw.client)

	// TODO: parse url args and set there options.
	var opts []clientv3.OpOption

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := kv.Get(ctx, key, opts...)
	cancel()

	if err != nil {
		return errors.Trace(err)
	}

	if len(resp.Kvs) == 0 {
		http.NotFound(w, r)
		return nil
	}

	// TODO: use content type for response, now we use json default.
	value, err := json.Marshal(resp)
	if err != nil {
		return errors.Trace(err)
	}

	w.Write(value)

	return nil
}

func (gw *Gateway) handlePutKeys(w http.ResponseWriter, r *http.Request) error {
	key := strings.TrimPrefix(r.URL.Path, keysPrefix)

	val, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return errors.Trace(err)
	}

	var (
		// TODO: parse url args and set there options.
		cmps []clientv3.Cmp
		opts []clientv3.OpOption
	)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	resp, err := gw.client.Txn(ctx).
		If(cmps...).
		Then(clientv3.OpPut(key, string(val), opts...)).
		Commit()
	cancel()

	if err != nil {
		return errors.Trace(err)
	} else if !resp.Succeeded {
		return errors.Errorf("put value for key %q failed", key)
	}

	value, err := json.Marshal(resp)
	if err != nil {
		return errors.Trace(err)
	}

	w.Write(value)
	return nil
}
