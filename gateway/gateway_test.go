package gateway

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/coreos/etcd/clientv3"
	. "github.com/pingcap/check"
)

func TestServer(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testGatewaySuite{})

var (
	addr     = flag.String("addr", "127.0.0.1:8080", "Gateway HTTP listening address")
	testEtcd = flag.String("etcd", "127.0.0.1:2378", "Etcd gPRC endpoints, separated by comma")
)

type testGatewaySuite struct {
	gw *Gateway
}

func (s *testGatewaySuite) SetUpSuite(c *C) {
	cfg := &Config{
		Addr:      *addr,
		EtcdAddrs: strings.Split(*testEtcd, ","),
	}

	gw, err := NewGateway(cfg)
	c.Assert(err, IsNil)

	s.gw = gw

	go s.gw.Run()
}

func (s *testGatewaySuite) TearDownSuite(c *C) {

}

func getURL(key string) string {
	return fmt.Sprintf("http://%s%s/%s", *addr, keysPrefix, key)
}

func (s *testGatewaySuite) TestPutGet(c *C) {
	url := getURL("foo")

	value := "hello world"
	resp, err := http.Post(url, "application/octet-stream", strings.NewReader(value))
	c.Assert(err, IsNil)

	_, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	resp.Body.Close()

	resp, err = http.Get(url)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	resp.Body.Close()

	msg := clientv3.GetResponse{}
	err = json.Unmarshal(data, &msg)
	c.Assert(err, IsNil)

	c.Assert(msg.Kvs, HasLen, 1)
	c.Assert(string(msg.Kvs[0].Value), Equals, value)
}
