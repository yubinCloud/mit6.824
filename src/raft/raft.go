package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// 打印一个 leader 的当前状态，即有哪些人给他投票才当选的
func (rf *Raft) debug() {
	log.Printf("ID %d, term %d: recv-votes %v", rf.me, rf.currentTerm, rf.recvVotes)
}

// 将 rf 转变为 follower 角色
func (rf *Raft) toFollower() {
	rf.votedFor = -1
	rf.voteCnt = 0
	rf.recvVotes = make([]bool, len(rf.peers))
	rf.role = RAFT_ROLE_FOLLOWER
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term := rf.currentTerm
	isleader := false
	if rf.role == RAFT_ROLE_LEADER {
		isleader = true
	}
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.hasReceived = true
	// 判断自己是否落后了
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}
	reply.Term = rf.currentTerm
	//  Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		return
	}
	// If votedFor is null or candidateId, and candidate's log is at least as up-to-date as receiver's log, grant vote
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
		return
	} else {
		reply.VoteGranted = false
	}
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) AppendEntriesHandler(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.hasReceived = true

	// 比较谁更 up-to-date
	// ..

	// 比较 term
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	// 如果 rf 是 candidate，表示当前任期有人成功当选 leader，此时 rf 应当转为 follower
	if rf.role == RAFT_ROLE_CANDIDATE {
		rf.currentTerm = args.Term
		rf.toFollower()
	}

	rf.lastFrom = args.LeaderId
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}
	reply.Success = true
	reply.Term = rf.currentTerm
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntriesHandler", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// 向一个 server 发送一个 AppendEntries 的请求
func (rf *Raft) sendAppendEntriesToOne(peerIndex int) {
	args := &AppendEntriesArgs{}
	args.LeaderId = rf.me
	args.Entries = make([]LogEntry, 0)
	reply := &AppendEntriesReply{}
	rf.mu.Lock()
	args.Term = rf.currentTerm
	rf.mu.Unlock()
	ok := rf.sendAppendEntries(peerIndex, args, reply)
	if !ok {
		return
	}
	if reply.Success {
		// 后面可能需要计数
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.toFollower()
		return
	}
}

// 向所有 server 发送 AppendEntries 的请求
func (rf *Raft) sendAppendEntriesToAll(entries []LogEntry) {
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		go rf.sendAppendEntriesToOne(i)
	}
}

// rf 向一个 server 索要投票
func (rf *Raft) askVote(peerIdx int) {
	rf.mu.Lock()
	role := rf.role
	if role != RAFT_ROLE_CANDIDATE { // 先判断一下自己还是不是 candidate，不是的话就不能再要票了
		rf.mu.Unlock()
		return
	}
	// 准备发送 RequestVote 的 RPC 请求
	args := &RequestVoteArgs{}
	args.Term = rf.currentTerm
	rf.mu.Unlock()

	args.CandidateId = rf.me
	args.LastLogIndex = 0
	args.LastLogTerm = 0
	reply := &RequestVoteReply{}
	ok := rf.sendRequestVote(peerIdx, args, reply)
	// 解析 RPC 响应的结果
	if !ok {
		return // 发送失败直接返回
	}
	rf.mu.Lock()
	if reply.Term > rf.currentTerm { // 比较一下 term
		rf.currentTerm = reply.Term
		rf.toFollower()
		rf.mu.Unlock()
		return
	}
	if !reply.VoteGranted { // 如果对方不投票，说明它已经投给了别人，直接 return 就好了
		rf.mu.Unlock()
		return
	} else { // 如果对方投票了，那就要更新一下票数，同时判断一下是否能够转变成 leader
		// 更新票数
		rf.voteCnt += 1
		rf.recvVotes[peerIdx] = true
		// 判断能否转变为 leader，但要*注意*，只有 candidate 才有转变的必要
		if rf.role == RAFT_ROLE_CANDIDATE && rf.voteCnt >= len(rf.peers)/2+1 {
			rf.role = RAFT_ROLE_LEADER
			rf.debug()
			rf.mu.Unlock()
			rf.sendAppendEntriesToAll(make([]LogEntry, 0))
		} else {
			rf.mu.Unlock()
		}
		return
	}
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.
		rf.mu.Lock()
		if rf.role == RAFT_ROLE_FOLLOWER || rf.role == RAFT_ROLE_CANDIDATE {
			if rf.hasReceived {
				rf.hasReceived = false
				rf.mu.Unlock()
			} else {
				// election timeout
				rf.role = RAFT_ROLE_CANDIDATE // convert to candidate
				// start election
				rf.currentTerm += 1 // Increment currentTerm
				// Vote for self
				rf.votedFor = rf.me
				rf.voteCnt = 1
				rf.recvVotes = make([]bool, len(rf.peers))
				rf.recvVotes[rf.me] = true
				rf.mu.Unlock()
				for i := 0; i < len(rf.peers); i++ {
					if i == rf.me {
						continue
					} else {
						go rf.askVote(i)
					}
				}
			}

			// pause for a random amount of time between 500 and 1000
			// milliseconds.
			ms := 800 + (rand.Int63() % 1000)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		} else {
			rf.mu.Unlock()
			rf.sendAppendEntriesToAll(make([]LogEntry, 0)) // 向所有 server 发送 heartbeat
			ms := 100
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}

	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.mu.Lock()
	rf.role = RAFT_ROLE_FOLLOWER
	rf.hasReceived = false
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.logs = make([]*LogEntry, 0, 10)
	rf.voteCnt = 0
	rf.recvVotes = make([]bool, len(peers))
	rf.mu.Unlock()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
