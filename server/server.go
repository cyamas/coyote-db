package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cyamas/coyote-db/log"
	coyote_db "github.com/cyamas/coyote-db/proto"
	"github.com/cyamas/coyote-db/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	FOLLOWER = iota
	CANDIDATE
	LEADER
)

const (
	heartbeatInterval = time.Duration(100 * time.Millisecond)
	heartbeatTimeout  = time.Duration(300 * time.Millisecond)
)

type Server struct {
	coyote_db.UnimplementedStoreServer
	id             int
	addr           string
	rank           int
	currentTerm    int
	votedFor       int
	log            *log.Log
	commitIndex    int
	lastApplied    int
	nextIndex      int
	matchIndex     int
	store          store.Store
	clusterAddrs   []string
	clusterConns   map[int]*grpc.ClientConn
	heartbeatTimer *time.Timer
}

func New(addrIdx int, clusterAddrs []string) *Server {
	return &Server{
		id:           addrIdx,
		addr:         clusterAddrs[addrIdx],
		clusterAddrs: clusterAddrs,
		log:          log.Init("./test/filepath.json"),
		clusterConns: make(map[int]*grpc.ClientConn),
		votedFor:     -1,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	coyote_db.RegisterStoreServer(grpcServer, s)
	return grpcServer.Serve(ln)
}

func (s *Server) ConnectToClustermates() {
	for i, addr := range s.clusterAddrs {
		if i == s.id {
			continue
		}
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("failed to connect to clustermate at address %s. Error: %s\n", addr, err.Error())
		}
		s.clusterConns[i] = conn
	}
}

func (s *Server) startHeartbeatTimer() {
	s.heartbeatTimer = time.NewTimer(heartbeatTimeout)
	for {
		select {
		case <-s.heartbeatTimer.C:
			s.triggerElection()
		}
	}
}

func (s *Server) triggerElection() {
	s.currentTerm++
	s.rank = CANDIDATE
	s.votedFor = s.id
}

func (s *Server) broadcastVoteRequest() {
	req := &coyote_db.VoteRequest{
		Term:         int64(s.currentTerm),
		CandidateID:  int64(s.id),
		LastLogIndex: int64(s.getLastLogIndex()),
		LastLogTerm:  int64(s.getLastLogTerm()),
	}
	for id, conn := range s.clusterConns {
		client := coyote_db.NewStoreClient(conn)
		resp, err := client.RequestVote(context.Background(), req)
		fmt.Println("placeholder resp", resp)
		if err != nil {
			fmt.Printf("Error Requesting Vote to Server %d: %s", id, err.Error())
		}
	}
}

func (s *Server) RequestVote(ctx context.Context, req *coyote_db.VoteRequest) (*coyote_db.VoteResponse, error) {
	resp := &coyote_db.VoteResponse{}
	if req.Term < int64(s.currentTerm) {
		resp.Term = int64(s.currentTerm)
		resp.VoteGranted = false
		return resp, nil
	}
	if s.votedFor == -1 && req.LastLogIndex >= int64(s.getLastLogIndex()) && req.LastLogTerm >= int64(s.getLastLogTerm()) {
		resp.Term = int64(s.currentTerm)
		resp.VoteGranted = true
		return resp, nil
	}
	return nil, errors.New("Error handling vote request from clustermate")
}

func (s *Server) getLastLogIndex() int {
	return len(s.log.Entries)
}

func (s *Server) getLastLogTerm() int {
	if len(s.log.Entries) == 0 {
		return 0
	}
	return s.log.Entries[len(s.log.Entries)].Term
}
