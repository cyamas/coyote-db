package server

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
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
	id             int64
	addr           string
	rank           int
	currentTerm    int64
	votedFor       int64
	log            *log.Log
	commitIndex    int64
	lastApplied    int64
	nextIndex      int64
	matchIndex     int64
	store          store.Store
	clusterAddrs   []string
	clusterConns   map[int64]*grpc.ClientConn
	clusterCh      chan string
	heartbeatTimer *time.Timer
	leaderCh       chan bool
	clientCh       chan *coyote_db.Entry
	mu             sync.Mutex
}

func New(addrIdx int, clusterAddrs []string, clusterCh chan string) *Server {
	return &Server{
		id:           int64(addrIdx),
		addr:         clusterAddrs[addrIdx],
		clusterAddrs: clusterAddrs,
		log:          log.Init("./placeholder_filepath.json"),
		clusterConns: make(map[int64]*grpc.ClientConn),
		clusterCh:    clusterCh,
		votedFor:     -1,
		leaderCh:     make(chan bool),
		clientCh:     make(chan *coyote_db.Entry, 10),
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

func (s *Server) Run() {
	for {
		if s.rank == LEADER {
			nextHeartbeat := time.NewTimer(heartbeatInterval)
			select {
			case <-nextHeartbeat.C:
				s.sendHeartbeat()
				return
			case entry := <-s.clientCh:
				entries := []*coyote_db.Entry{entry}
				batchTimeout := time.After(10 * time.Millisecond)
				for {
					select {
					case entry := <-s.clientCh:
						entries = append(entries, entry)
					case <-batchTimeout:
						s.sendEntries(entries)
						break
					}
				}
			}
		} else {
			s.startHeartbeatTimer()
		}
	}
}

func (s *Server) sendEntries(entries []*coyote_db.Entry) {
}

func (s *Server) ConnectToClustermates() {
	for i, addr := range s.clusterAddrs {
		if int64(i) == s.id {
			continue
		}
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("failed to connect to clustermate at address %s. Error: %s\n", addr, err.Error())
		}
		s.clusterConns[int64(i)] = conn
	}
}

func (s *Server) startHeartbeatTimer() {
	s.heartbeatTimer = time.NewTimer(heartbeatTimeout)
	select {
	case <-s.heartbeatTimer.C:
		s.triggerElection()
	case <-s.leaderCh:
		return
	}
}

/*
 1. Start randomized election timer between 150-300 milliseconds
    ------- If timeout reached and hasn't cast a vote: proceed
    ------- Else return
 2. Increment term and rank of server, cast vote for itself
 3. Create a vote channel that returns a bool of whether or not a quorum was reached
 4. Create a context with a cancellation
 5. call s.broadCastVoteRequest and pass in the context and channel
 6. Select from whichever channel returns first:

-------- 1. New Leader has sent an AppendEntries request
-------- 2. Server has won election and becomes new leader
*/
func (s *Server) triggerElection() {
	for {
		electionTimeout := time.Duration(150+rand.Intn(150)) * time.Millisecond
		time.Sleep(electionTimeout)
		if s.votedFor != -1 {
			break
		}
		s.currentTerm++
		s.rank = CANDIDATE
		s.votedFor = s.id
		resultCh := make(chan bool)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.broadcastVoteRequest(ctx, resultCh)

		select {
		case newLeader := <-s.leaderCh:
			if newLeader {
				cancel()
				return
			}
		case success := <-resultCh:
			close(resultCh)
			if success {
				s.rank = LEADER
				s.votedFor = -1
				s.clusterCh <- fmt.Sprintf("Server %d has been elected LEADER...", s.id)
				s.sendHeartbeat()
				cancel()
				return
			} else {
				s.clusterCh <- fmt.Sprintf("Server %d has LOST the election", s.id)
				s.votedFor = -1
				s.rank = FOLLOWER
				cancel()
				return

			}
		}
	}
}

func (s *Server) sendHeartbeat() {
	req := &coyote_db.AppendEntriesRequest{
		Term:         s.currentTerm,
		LeaderID:     s.id,
		PrevLogIndex: s.getLastLogIndex(),
		PrevLogTerm:  s.getLastLogTerm(),
		LeaderCommit: s.commitIndex,
	}
	for id, conn := range s.clusterConns {
		client := coyote_db.NewStoreClient(conn)
		resp, err := client.AppendEntries(context.Background(), req)
		if err != nil {
			fmt.Printf("Error sending AppendEntries RPC to client %d: %s", id, err.Error())
		}
		if resp.Success {
			s.clusterCh <- fmt.Sprintf("Server %d has acknowledged %d as LEADER", id, s.id)
		}
	}
}

/*
1. if AppendEntriesRequest has lower term than s.currentTerm -> return false
2. if a log doesn't exist in the server -> return false
3. if the req.Term doesn't match on the entry at prevlogindex -> delete all entries at and after req.prevLogIndex and return false
4. send on leaderCh to stop any potential election happening, update state to match leader, and return a success response
*/
func (s *Server) AppendEntries(ctx context.Context, req *coyote_db.AppendEntriesRequest) (*coyote_db.AppendEntriesResponse, error) {
	resp := &coyote_db.AppendEntriesResponse{}
	if req.Term < s.currentTerm {
		resp.Term = s.currentTerm
		resp.Success = false
		return resp, nil
	}
	prevLogIndex := int(req.PrevLogIndex)
	if prevLogIndex > len(s.log.Entries)-1 {
		resp.Term = s.currentTerm
		resp.Success = false
		return resp, nil
	}

	if prevLogIndex >= 0 && s.log.Entries[prevLogIndex].Term != req.Term {
		resp.Term = s.currentTerm
		resp.Success = false
		return resp, nil
	}
	if req.Entries != nil {
		s.log.Lock()
		s.log.Entries = s.log.Entries[:prevLogIndex+1]
		s.log.Entries = append(s.log.Entries, req.Entries.Entry...)
		s.log.Unlock()
	}
	s.leaderCh <- true
	s.rank = FOLLOWER
	s.votedFor = -1
	s.currentTerm = req.Term
	resp.Success = true
	resp.Term = s.currentTerm
	return resp, nil
}

func (s *Server) quorumReached(votes int) bool {
	return len(s.clusterConns)-votes < votes
}

func (s *Server) broadcastVoteRequest(ctx context.Context, resultCh chan bool) {
	req := s.createVoteRequest()
	voteCount := 1

	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for id, conn := range s.clusterConns {
		wg.Add(1)
		go func(id int64, conn grpc.ClientConnInterface) {
			defer wg.Done()
			client := coyote_db.NewStoreClient(conn)
			resp, err := client.RequestVote(ctx, req)
			if err != nil {
				_ = fmt.Sprintf("Error Requesting Vote to Server %d: %s\n", id, err.Error())
				return
			}
			if !resp.VoteGranted && resp.Term > s.currentTerm {
				s.mu.Lock()
				s.rank = FOLLOWER
				s.currentTerm = resp.Term
				s.mu.Unlock()

				select {
				case resultCh <- false:
				default:
				}
				cancel()
				return
			}
			if resp.VoteGranted {
				mu.Lock()
				if s.quorumReached(voteCount) {
					cancel()
					return
				}
				voteCount++
				mu.Unlock()
				if s.quorumReached(voteCount) {
					select {
					case resultCh <- true:
					default:
					}
					cancel()
					return
				}
			}

		}(id, conn)
	}
	wg.Wait()

	select {
	case <-ctx.Done():
	default:
		resultCh <- false
	}
}

func (s *Server) createVoteRequest() *coyote_db.VoteRequest {
	return &coyote_db.VoteRequest{
		Term:         s.currentTerm,
		CandidateID:  s.id,
		LastLogIndex: s.getLastLogIndex(),
		LastLogTerm:  s.getLastLogTerm(),
	}

}

func (s *Server) RequestVote(ctx context.Context, req *coyote_db.VoteRequest) (*coyote_db.VoteResponse, error) {
	resp := &coyote_db.VoteResponse{}
	if req.Term < s.currentTerm {
		resp.Term = s.currentTerm
		resp.VoteGranted = false
		return resp, nil
	}
	if (s.votedFor == -1 || s.votedFor == req.CandidateID) && req.LastLogIndex >= s.getLastLogIndex() && req.LastLogTerm >= s.getLastLogTerm() {
		s.mu.Lock()
		s.votedFor = req.CandidateID
		s.mu.Unlock()
		resp.Term = s.currentTerm
		resp.VoteGranted = true
		return resp, nil
	}
	return nil, errors.New("Error handling vote request from clustermate")
}

func (s *Server) getLastLogIndex() int64 {
	return int64(len(s.log.Entries) - 1)
}

func (s *Server) getLastLogTerm() int64 {
	if len(s.log.Entries) == 0 {
		return 0
	}
	return int64(s.log.Entries[len(s.log.Entries)].Term)
}
