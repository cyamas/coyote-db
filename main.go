package main

import (
	"fmt"
	"time"

	"github.com/cyamas/coyote-db/server"
)

type Cluster struct {
	servers []*server.Server
	count   int
}

func NewCluster(addrs ...string) *Cluster {
	cluster := &Cluster{make([]*server.Server, 0, len(addrs)), 0}
	for i := range addrs {
		cluster.servers = append(cluster.servers, server.New(i, addrs))
		cluster.count++
	}
	return cluster
}

func (c *Cluster) Init() {
	for _, server := range c.servers {
		go server.Start()
	}
}

func (c *Cluster) ConnectCluster() {
	for _, server := range c.servers {
		server.ConnectToClustermates()
	}
}

func main() {
	cluster := NewCluster("localhost:6400", "localhost:6401", "localhost:6402", "localhost:6403", "localhost:6404")
	cluster.Init()
	cluster.ConnectCluster()
	for cluster.count > 0 {
		time.Sleep(time.Second * 1)
		fmt.Println(cluster.count, "servers running...")
	}
}
