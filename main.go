package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pingcap/etcdv3-gateway/gateway"
)

var (
	addr      = flag.String("addr", ":20168", "Etcdv3 gateway HTTP listening address")
	etcdAddrs = flag.String("etcd", "127.0.0.1:2379", "Etcd endpoints, separated by comma")
)

func main() {
	flag.Parse()

	cfg := &gateway.Config{
		Addr:      *addr,
		EtcdAddrs: strings.Split(*etcdAddrs, ","),
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		fmt.Printf("create gateway failed %v", err)
		return
	}

	if err = gw.Run(); err != nil {
		fmt.Printf("run gateway failed %v", err)
	}
}
