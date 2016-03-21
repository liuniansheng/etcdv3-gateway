package gateway

// Config is the configuration for etcdv3 gateway.
type Config struct {
	// Gateway HTTP listening address.
	Addr string

	// Etcd endpoints
	EtcdAddrs []string
}
