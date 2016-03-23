package gateway

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/schema"
	"github.com/juju/errors"
)

const (
	etcdTimeout    = 3 * time.Second
	requestTimeout = 3 * time.Second

	// Etcd use "/v2/keys" for its http v2 prefix, so
	// here we use /v3/keys.
	keysPrefix = "/v3/keys"

	contentTypeKey = "Content-Type"
)

// Gateway is a HTTP gateway to communicate with Etcd with v3 protocol.
type Gateway struct {
	cfg    *Config
	client *clientv3.Client
}

// NewGateway creates the gateway.
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

// Run runs the gateway server.
func (gw *Gateway) Run() error {
	mux := http.NewServeMux()

	keysHandler := &keysHandler{gw: gw}
	mux.Handle(keysPrefix, keysHandler)
	mux.Handle(keysPrefix+"/", keysHandler)

	s := &http.Server{
		Addr:         gw.cfg.Addr,
		Handler:      mux,
		ReadTimeout:  requestTimeout,
		WriteTimeout: requestTimeout,
	}

	err := s.ListenAndServe()
	return errors.Trace(err)
}

func writeResponse(w http.ResponseWriter, r *http.Request, resp interface{}) error {
	contentType := r.Header.Get(contentTypeKey)

	// TODO: check application/x-protobuf and return protobuf format.
	// use json for default
	value, err := json.Marshal(resp)

	if err != nil {
		return errors.Trace(err)
	}

	w.Header().Set(contentTypeKey, contentType)
	w.Write(value)

	return nil
}

// Schema decoder can be used safely in multi goroutines.
var decoder = schema.NewDecoder()

func init() {
	decoder.IgnoreUnknownKeys(true)
}

type keysHandler struct {
	gw *Gateway
}

func (h *keysHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	switch r.Method {
	case http.MethodGet:
		err = h.Get(w, r)
	case http.MethodPut, http.MethodPost:
		err = h.Put(w, r)
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

var sortOrderMap = map[string]clientv3.SortOrder{
	"asc":  clientv3.SortAscend,
	"desc": clientv3.SortDescend,
	// Default is None sort.
	"": clientv3.SortNone,
}

var sortByMap = map[string]clientv3.SortTarget{
	"create":  clientv3.SortByCreatedRev,
	"key":     clientv3.SortByKey,
	"modify":  clientv3.SortByModifiedRev,
	"value":   clientv3.SortByValue,
	"version": clientv3.SortByVersion,
	// Default is sort by key.
	"": clientv3.SortByKey,
}

func (h *keysHandler) parseGetOptions(r *http.Request) ([]clientv3.OpOption, error) {
	var opts []clientv3.OpOption

	option := struct {
		Limit    int64  `schema:"limit"`
		Order    string `schema:"order"`
		SortBy   string `schema:"sort-by"`
		Prefix   bool   `schema:"prefix"`
		RangeEnd string `schema:"range-end"`
	}{}

	if err := decoder.Decode(&option, r.Form); err != nil {
		return nil, errors.Trace(err)
	}

	if len(option.RangeEnd) > 0 {
		if option.Prefix {
			return nil, errors.New("too many arguments for range with prefix, only need one")
		}

		opts = append(opts, clientv3.WithRange(option.RangeEnd))
	}

	if option.Limit > 0 {
		opts = append(opts, clientv3.WithLimit(option.Limit))
	}

	order, ok := sortOrderMap[strings.ToLower(option.Order)]
	if !ok {
		return nil, errors.Errorf("bad sort order %v", option.Order)
	}

	sortBy, ok := sortByMap[strings.ToLower(option.SortBy)]
	if !ok {
		return nil, errors.Errorf("bad sort target %v", option.SortBy)
	}

	opts = append(opts, clientv3.WithSort(sortBy, order))

	if option.Prefix {
		opts = append(opts, clientv3.WithPrefix())
	}

	return opts, nil
}

func (h *keysHandler) Get(w http.ResponseWriter, r *http.Request) error {
	gw := h.gw

	key := strings.TrimPrefix(r.URL.Path, keysPrefix)

	kv := clientv3.KV(gw.client)

	if err := r.ParseForm(); err != nil {
		return errors.Trace(err)
	}

	opts, err := h.parseGetOptions(r)
	if err != nil {
		return errors.Trace(err)
	}

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

	writeResponse(w, r, resp)
	return nil
}

func (h *keysHandler) Put(w http.ResponseWriter, r *http.Request) error {
	gw := h.gw

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

	writeResponse(w, r, resp)
	return nil
}
