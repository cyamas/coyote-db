package main

import (
	"fmt"
	"time"

	"github.com/cyamas/coyote-db/server"
)

func main() {
	cluster := NewCluster(":6400", ":6401", ":6402", ":6403", ":6404")
	cluster.Init()
	time.Sleep(1 * time.Second)
	cluster.ConnectCluster()
	cluster.Run()
	for msg := range cluster.ch {
		fmt.Println(msg)
	}
}

type Cluster struct {
	servers []*server.Server
	count   int
	ch      chan string
}

func NewCluster(addrs ...string) *Cluster {
	cluster := &Cluster{make([]*server.Server, 0, len(addrs)), 0, make(chan string, 100)}
	for i := range addrs {
		cluster.servers = append(cluster.servers, server.New(i, addrs, cluster.ch))
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
	fmt.Println("cluster servers are connected")
}

func (c *Cluster) Run() {
	for _, server := range c.servers {
		go server.Run()
	}

}
