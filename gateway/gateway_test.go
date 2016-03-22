package gateway

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
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
	addr     = flag.String("addr", "127.0.0.1:20168", "Gateway HTTP listening address")
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

func getKey(key string) string {
	return path.Join("/test_gateway", key)
}

func getURL(key string, params url.Values) string {
	key = getKey(key)

	if len(params) == 0 {
		return fmt.Sprintf("http://%s%s%s", *addr, keysPrefix, key)
	}

	return fmt.Sprintf("http://%s%s%s?%s", *addr, keysPrefix, key, params.Encode())
}

const bodyType = "application/octet-stream"

func testPut(c *C, url string, value string) {
	resp, err := http.Post(url, bodyType, strings.NewReader(value))
	c.Assert(err, IsNil)

	_, err = ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	resp.Body.Close()
}

func testGet(c *C, url string) *clientv3.GetResponse {
	resp, err := http.Get(url)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	resp.Body.Close()

	msg := clientv3.GetResponse{}
	err = json.Unmarshal(data, &msg)
	c.Assert(err, IsNil, Commentf("%q", data))
	return &msg
}

func (s *testGatewaySuite) TestPutGet(c *C) {
	url := getURL("a", nil)

	value := "hello world"
	testPut(c, url, value)

	msg := testGet(c, url)

	c.Assert(msg.Kvs, HasLen, 1)
	c.Assert(string(msg.Kvs[0].Value), Equals, value)
}

func (s *testGatewaySuite) TestGetOptions(c *C) {
	testPut(c, getURL("fooa", nil), "bara")
	testPut(c, getURL("foob", nil), "barb")
	testPut(c, getURL("fooaa", nil), "bar")

	params := url.Values{}
	params.Add("prefix", "true")
	msg := testGet(c, getURL("foo", params))
	c.Assert(msg.Kvs, HasLen, 3, Commentf("%+v", msg))

	params = url.Values{}
	params.Add("range-end", getKey("g"))
	msg = testGet(c, getURL("foo", params))
	c.Assert(msg.Kvs, HasLen, 3, Commentf("%+v", msg))

	params = url.Values{}
	params.Add("order", "desc")
	params.Add("sort-by", "value")
	params.Add("prefix", "true")
	msg = testGet(c, getURL("foo", params))
	c.Assert(msg.Kvs, HasLen, 3)
	c.Assert(string(msg.Kvs[0].Value), Equals, "barb")
	c.Assert(string(msg.Kvs[2].Value), Equals, "bar")
}
