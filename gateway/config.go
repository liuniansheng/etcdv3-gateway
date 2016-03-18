package gateway

type Config struct {
	// Gateway HTTP listening address.
	Addr string

	// Etcd endpoints
	EtcdAddrs []string
}
