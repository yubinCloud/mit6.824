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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// 打印一个 leader 的当前状态，即有哪些人给他投票才当选的
func (rf *Raft) debug() {
	log.Printf("ID %d, term %d: recv-votes %v", rf.me, rf.currentTerm, rf.recvVotes)
}

func (rf *Raft) showLogs() string {
	logTerms := make([]string, 0)
	for i := 0; i < len(rf.logs); i++ {
		logTerms = append(logTerms, strconv.Itoa(i))
	}
	s := "[" + strings.Join(logTerms, ",") + "]"
	return s
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
	lastLogIndex := len(rf.logs) - 1
	lastLogTerm := rf.logs[lastLogIndex].Term
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		if lastLogTerm < args.LastLogTerm || (lastLogTerm == args.LastLogTerm && lastLogIndex <= args.LastLogIndex) {
			rf.votedFor = args.CandidateId
			reply.VoteGranted = true
			DPrintf("# recv RequestVote, %d vote for %d at term %d", rf.me, args.CandidateId, rf.currentTerm)
			return
		}
	}
	reply.VoteGranted = false
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
	DPrintf("# send RequestVote, from %d to %d at term %d", rf.me, server, args.Term)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) AppendEntriesHandler(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.hasReceived = true
	DPrintf("# recv AppendEntries, rf %d recevie AppendEntries, args: %v, before logs %s", rf.me, args, rf.showLogs())

	// 1. Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}

	// 如果 rf 是 candidate，表示当前任期有人成功当选 leader，此时 rf 应当转为 follower
	if rf.currentTerm == args.Term && rf.role == RAFT_ROLE_CANDIDATE {
		rf.toFollower()
	}

	rf.lastFrom = args.LeaderId
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}
	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm
	if len(rf.logs) <= args.PrevLogIndex {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	// 3. If an existing entry conflicts with a new one (same index but different terms), delete the existing entry and all that follow it
	if rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		rf.logs = rf.logs[:args.PrevLogIndex]
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	// 4. Append any new entries not already in the log
	if len(args.Entries) > 0 {
		rf.logs = append(rf.logs, args.Entries...)
	}
	// 5.  If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = len(rf.logs) - 1
		if rf.commitIndex > args.LeaderCommit {
			rf.commitIndex = args.LeaderCommit
		}
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			applyMsg := ApplyMsg{}
			applyMsg.CommandValid = true
			applyMsg.Command = rf.logs[i].Command
			applyMsg.CommandIndex = i
			rf.applyCh <- applyMsg
		}
		rf.lastApplied = rf.commitIndex
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
	rf.mu.Lock()
	term := rf.currentTerm
	isLeader := rf.role == RAFT_ROLE_LEADER
	// 如果不是 leader，立刻返回 false
	if !isLeader {
		rf.mu.Unlock()
		return -1, term, false
	}
	// 给自己追加 log entry
	entry := &LogEntry{
		Term:    term,
		Command: command,
	}
	index := len(rf.logs)
	rf.logs = append(rf.logs, entry)
	rf.mu.Unlock()
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

// return value 表示是否要退出
func (rf *Raft) retryLoopSendAppendEntries(peerIndex int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	// 一直尝试，直至成功 server 响应
	ok := false
	for {
		ok = rf.sendAppendEntries(peerIndex, args, reply)
		if ok {
			break
		}
		rf.mu.Lock()
		if rf.killed() || rf.role != RAFT_ROLE_LEADER {
			rf.mu.Unlock()
			return true
		}
		rf.mu.Unlock()
		time.Sleep(15 * time.Millisecond)
	}
	return false
}

// 向一个 server 发送一个 AppendEntries 的请求
func (rf *Raft) sendAppendEntriesToOne(peerIndex int) {
	args := &AppendEntriesArgs{}
	args.LeaderId = rf.me
	args.Entries = make([]*LogEntry, 0)
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

func (rf *Raft) logReplication(peer int) {
	found := false // 表示是否找到 leader 与 follower 的差异点
	for !rf.killed() && !found {
		rf.mu.Lock()
		nextIndex := rf.nextIndex[peer]
		prevLogIndex := nextIndex - 1

		var entries []*LogEntry
		if nextIndex == len(rf.logs) {
			entries = make([]*LogEntry, 0)
		} else {
			entries = rf.logs[nextIndex:]
		}
		args := &AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  rf.logs[prevLogIndex].Term,
			Entries:      entries,
			LeaderCommit: rf.commitIndex,
		}
		rf.mu.Unlock()
		reply := &AppendEntriesReply{}
		ok := false
		DPrintf("# send AppendEntries: from %d to peer %d, term %d: prevLogIndex %d", rf.me, peer, rf.currentTerm, prevLogIndex)
		for !ok && !rf.killed() {
			ok = rf.sendAppendEntries(peer, args, reply)
			if !ok {
				time.Sleep(10 * time.Millisecond)
			}
		}
		rf.mu.Lock()
		if reply.Success {
			rf.nextIndex[peer] = nextIndex + len(entries)
			rf.matchIndex[peer] = rf.nextIndex[peer] - 1
			rf.mu.Unlock()
			found = true
			break
		} else {
			if reply.Term > args.Term {
				if rf.currentTerm < reply.Term {
					rf.currentTerm = reply.Term
					rf.toFollower()
				}
				rf.mu.Unlock()
				return
			}
			if reply.Term == args.Term {
				rf.nextIndex[peer]--
				rf.mu.Unlock()
				continue
			} else {
				rf.mu.Unlock()
				continue
			}
		}
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
	args.CandidateId = rf.me
	args.LastLogIndex = len(rf.logs) - 1
	args.LastLogTerm = rf.logs[args.LastLogIndex].Term
	rf.mu.Unlock()
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
			for i := 0; i < len(rf.peers); i++ {
				rf.nextIndex[i] = len(rf.logs)
				rf.matchIndex[i] = 0
			}
			rf.debug()
			rf.mu.Unlock()
			// 立刻发送一次 heartbeat
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				go rf.logReplication(i)
			}
		} else {
			rf.mu.Unlock()
		}
		return
	}
}

func (rf *Raft) checkLeaderCommit() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	for n := len(rf.logs) - 1; n > rf.commitIndex; n-- {
		if rf.logs[n].Term == rf.currentTerm {
			cnt := 0
			for i := 0; i < len(rf.peers); i++ {
				if rf.matchIndex[i] >= n {
					cnt++
				}
			}
			if cnt >= len(rf.peers)/2+1 {
				rf.commitIndex = n
				for j := rf.lastApplied + 1; j <= rf.commitIndex; j++ {
					applyMsg := ApplyMsg{}
					applyMsg.CommandValid = true
					applyMsg.Command = rf.logs[j].Command
					applyMsg.CommandIndex = j
					rf.applyCh <- applyMsg
				}
				rf.lastApplied = rf.commitIndex
				break
			}
		}
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
			ms := 900 + (rand.Int63() % 1100)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		} else {
			rf.mu.Unlock()
			// rf.sendAppendEntriesToAll(make([]LogEntry, 0)) // 向所有 server 发送 heartbeat
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				go rf.logReplication(i)
			}
			ms := 130
			time.Sleep(time.Duration(ms) * time.Millisecond)
			rf.checkLeaderCommit()
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
	rf.applyCh = applyCh

	// Your initialization code here (2A, 2B, 2C).
	rf.mu.Lock()
	rf.role = RAFT_ROLE_FOLLOWER
	rf.hasReceived = false
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.logs = make([]*LogEntry, 1, 10)
	rf.logs[0] = &LogEntry{
		Term:    0,
		Command: nil,
	}
	rf.voteCnt = 0
	rf.recvVotes = make([]bool, len(peers))
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	rf.mu.Unlock()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
