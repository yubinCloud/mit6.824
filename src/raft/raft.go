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

	"bytes"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labgob"
	"6.5840/labrpc"
)

// 打印一个 leader 的当前状态，即有哪些人给他投票才当选的
func (rf *Raft) debug() {
	DPrintf("$$$$ rf %d win election at term %d", rf.me, rf.currentTerm)
}

func (rf *Raft) showLogs() string {
	logTerms := make([]string, 0)
	for i := 0; i < len(rf.logs); i++ {
		logTerms = append(logTerms, strconv.Itoa(rf.logs[i].Term))
	}
	s := "[..., <" + strconv.Itoa(rf.lastIncluedIndex) + ">, " + strings.Join(logTerms, ",") + "]"
	return s
}

// 将 rf 转变为 follower 角色
func (rf *Raft) toFollower() {
	rf.votedFor = -1
	rf.voteCnt = 0
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

func (rf *Raft) encodeRaftState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	e.Encode(rf.lastIncluedIndex)
	e.Encode(rf.lastIncluedTerm)
	e.Encode(rf.commitIndex)
	e.Encode(rf.lastApplied)
	e.Encode(rf.nextIndex)
	e.Encode(rf.matchIndex)
	raftState := w.Bytes()
	return raftState
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
	raftState := rf.encodeRaftState()
	rf.persister.Save(raftState, rf.persister.ReadSnapshot())
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
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var logs []*LogEntry
	var lastIncludedIndex int
	var lastIncluedTerm int
	var commitIndex int
	var lastApplied int
	var nextIndex []int
	var matchIndex []int
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&logs) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncluedTerm) != nil ||
		d.Decode(&commitIndex) != nil ||
		d.Decode(&lastApplied) != nil ||
		d.Decode(&nextIndex) != nil ||
		d.Decode(&matchIndex) != nil {
		panic("resume error")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.logs = logs
		rf.lastIncluedIndex = lastIncludedIndex
		rf.lastIncluedTerm = lastIncluedTerm
		rf.commitIndex = commitIndex
		rf.lastApplied = lastIncludedIndex // 大坑！由于崩溃后会按照最新的闪照开始，所有这里的 lastApplied 值应当是 lastIncludedIndex 而不是上一次的 lastApplied
		rf.nextIndex = nextIndex
		rf.matchIndex = matchIndex
	}
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	DPrintf("# create snapshot, rf %d, indudeIndex: %d", rf.me, index)
	// Your code here (2D).
	if index == 0 {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if index <= rf.lastIncluedIndex {
		return
	}
	// 记录 snapshot 的相关信息，注意，这三个语句的顺序不能更换
	rf.lastIncluedTerm = rf.GetLogTerm(index)
	rf.logs = copyLogs(rf.logs[rf.RealIndex(index)+1:])
	rf.lastIncluedIndex = index
	rf.persister.Save(rf.encodeRaftState(), snapshot)
}

func (rf *Raft) notMoreUpTodate(args *RequestVoteArgs) bool {
	lastLogIndex := rf.LogsLen() - 1
	lastLogTerm := rf.GetLogTerm(lastLogIndex)
	return lastLogTerm < args.LastLogTerm || (lastLogTerm == args.LastLogTerm && lastLogIndex <= args.LastLogIndex)
}

func (rf *Raft) hasNotVoted(candidateId int) bool {
	return rf.votedFor == -1 || rf.votedFor == candidateId
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	DPrintf("# recv RequestVote, %d at term %d heard a RequestVote from %d with args-term %d", rf.me, rf.currentTerm, args.CandidateId, args.Term)
	defer rf.mu.Unlock()
	//  Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		return
	}
	// 判断自己是否落后了
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}
	// 往下就是 args.Term == rf.currentTerm 了
	reply.Term = rf.currentTerm
	// If votedFor is null or candidateId, and candidate's log is at least as up-to-date as receiver's log, grant vote
	if rf.notMoreUpTodate(args) && rf.hasNotVoted(args.CandidateId) {
		rf.votedFor = args.CandidateId
		rf.persist()
		reply.VoteGranted = true
		rf.lastRecv = time.Now()
		DPrintf("# vote RequestVote, %d vote for %d at term %d", rf.me, args.CandidateId, rf.currentTerm)
		return
	} else {
		reply.VoteGranted = false
		return
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
	DPrintf("# send RequestVote, from %d to %d at term %d", rf.me, server, args.Term)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// 0 死了之后，再活过来就收不到信息了
func (rf *Raft) AppendEntriesHandler(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	DPrintf("# recv AppendEntries, rf %d recevie AppendEntries at term %d, args: %v, before logs %s", rf.me, rf.currentTerm, args, rf.showLogs())

	// 1. Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}
	rf.lastRecv = time.Now()
	// update term
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}

	// 如果 rf 是 candidate，表示当前任期有人成功当选 leader，此时 rf 应当转为 follower
	if rf.currentTerm == args.Term && rf.role == RAFT_ROLE_CANDIDATE {
		rf.toFollower()
	}

	rf.lastFrom = args.LeaderId

	// 2. Reply false if log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm
	if rf.LogsLen() <= args.PrevLogIndex {
		reply.Success = false
		reply.Term = rf.currentTerm
		reply.ConflictTerm = -1
		reply.ConflictIndex = rf.LogsLen()
		rf.persist()
		return
	}
	// 3. If an existing entry conflicts with a new one (same index but different terms), delete the existing entry and all that follow it
	if rf.RealIndex(args.PrevLogIndex) >= 0 && rf.GetLogTerm(args.PrevLogIndex) != args.PrevLogTerm {
		// 为了快速寻找到 conflict entry，这里将错误的 term 整个跳过
		reply.ConflictTerm = rf.GetLogTerm(args.PrevLogIndex)
		index := args.PrevLogIndex
		for rf.RealIndex(index) >= 0 && rf.GetLogTerm(index) == reply.ConflictTerm {
			index--
		}
		reply.ConflictIndex = index + 1
		reply.Success = false
		reply.Term = rf.currentTerm
		rf.persist()
		return
	}
	// 判断 follower 中 log 是否已经拥有 args.Entries 的所有条目，全部有则匹配
	isMatch := true
	nextIndex := args.PrevLogIndex + 1
	end := rf.LogsLen() - 1
	for i := 0; isMatch && i < len(args.Entries); i++ {
		// 如果args.Entries还有元素，而log已经达到结尾，则不匹配
		if end < nextIndex+i {
			isMatch = false
		} else if rf.GetLogTerm(nextIndex+i) != args.Entries[i].Term {
			isMatch = false
		}
	}
	// 如果存在冲突的条目，再进行日志复制
	if isMatch == false {
		rf.logs = append(rf.logs[:rf.RealIndex(nextIndex)], args.Entries...)
	} else {
		// isMatch 说明该 RPC 请求是过期的
		reply.Success = true
		reply.Term = 0
	}

	// 5.  If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = rf.LogsLen() - 1
		if rf.commitIndex > args.LeaderCommit {
			rf.commitIndex = args.LeaderCommit
		}
	}
	reply.Success = true
	reply.Term = rf.currentTerm
	rf.persist()
	DPrintf("# save AppendEntries, rf %d save AppendEntries at term %d, args: %v, after logs %s", rf.me, rf.currentTerm, args, rf.showLogs())
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntriesHandler", args, reply)
	DPrintf("# reply AppendEntries: rf %d reply args %v with resp %v", server, args, reply)
	return ok
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf("# recv InstallSnapshot, rf %d heared snapshot from rf %d with args lastIncludedIndex %d lastIncludedTerm %d, before logs: %s", rf.me, args.LeaderId, args.LastIncludedIndex, args.LastIncludedTerm, rf.showLogs())
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		DPrintf("# save InstallSnapshot fail, reason: outdated term.")
		return
	}
	rf.lastRecv = time.Now()
	if args.Term > rf.currentTerm || (args.Term == rf.currentTerm && rf.role == RAFT_ROLE_CANDIDATE) {
		rf.currentTerm = args.Term
		rf.toFollower()
		rf.persist()
	}
	reply.Term = rf.currentTerm
	// outdated snapshot
	// if args.LastIncludedIndex < rf.commitIndex {
	// 	DPrintf("# save InstallSnapshot fail, reason: outdated snapshot. args index %d, my commit index %d", args.LastIncludedIndex, rf.commitIndex)
	// 	return
	// }
	// 删除 rf.logs 中无用的条目
	if args.LastIncludedIndex > rf.LogsLen()-1 || rf.GetLogTerm(args.LastIncludedIndex) != args.LastIncludedTerm {
		rf.logs = make([]*LogEntry, 0)
	} else {
		rf.logs = copyLogs(rf.logs[rf.RealIndex(args.LastIncludedIndex)+1:])
		// if rf.commitIndex < args.LastIncludedIndex {
		// 	rf.commitIndex = args.LastIncludedIndex
		// }
		// if rf.lastApplied < args.LastIncludedIndex {
		// 	rf.lastApplied = args.LastIncludedIndex
		// }
	}
	rf.commitIndex = args.LastIncludedIndex
	rf.lastApplied = args.LastIncludedIndex
	rf.lastIncluedIndex = args.LastIncludedIndex
	rf.lastIncluedTerm = args.LastIncludedTerm

	go func() {
		applyMsg := ApplyMsg{
			SnapshotValid: true,
			Snapshot:      args.Data,
			SnapshotTerm:  args.LastIncludedTerm,
			SnapshotIndex: args.LastIncludedIndex,
		}
		rf.applyCh <- applyMsg
	}()
	rf.persister.Save(rf.encodeRaftState(), args.Data)
	DPrintf("# save InstallSnapshot, install snapshot: lastIncludedIndex %d lastIncludedTerm %d, after logs: %s", args.LastIncludedIndex, args.LastIncludedTerm, rf.showLogs())
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	DPrintf("# reply InstallSnapshot: rf %d args lastIncluedIndex %d lastIncludedTerm %d with resp term %d", server, args.LastIncludedIndex, args.LastIncludedTerm, reply.Term)
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
	index := rf.LogsLen()
	rf.logs = append(rf.logs, entry)
	rf.matchIndex[rf.me] = rf.LogsLen() - 1 // 太臭臭了，这是个大坑！Start 的时候要先更新自己的 matchIndex
	rf.persist()
	rf.mu.Unlock()
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		go rf.logReplication(i)
	}
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
	DPrintf("$ killed, rf %d is killed", rf.me)
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) logReplication(peer int) {
	found := false // 表示是否找到 leader 与 follower 的差异点
	for !rf.killed() && !found {
		rf.mu.Lock()
		if rf.role != RAFT_ROLE_LEADER {
			rf.mu.Unlock()
			return
		}
		nextIndex := rf.nextIndex[peer]
		prevLogIndex := nextIndex - 1

		if prevLogIndex >= rf.lastIncluedIndex {
			var entries []*LogEntry
			if nextIndex == rf.LogsLen() {
				entries = make([]*LogEntry, 0)
			} else {
				entries = rf.logs[rf.RealIndex(nextIndex):]
			}
			args := &AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  rf.GetLogTerm(prevLogIndex),
				Entries:      entries,
				LeaderCommit: rf.commitIndex,
			}
			DPrintf("# send AppendEntries: from %d to peer %d, term %d: prevLogIndex %d, logs state: %s", rf.me, peer, rf.currentTerm, prevLogIndex, rf.showLogs())
			rf.mu.Unlock()
			reply := &AppendEntriesReply{}
			ok := rf.sendAppendEntries(peer, args, reply)
			if !ok {
				return // peer 崩溃的话，直接放弃
			}
			rf.mu.Lock()
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.toFollower()
				rf.persist()
				rf.mu.Unlock()
				return
			}
			if reply.Term < rf.currentTerm {
				rf.mu.Unlock()
				return
			}
			if reply.Success {
				if nextIndex+len(entries) > rf.nextIndex[peer] {
					rf.nextIndex[peer] = nextIndex + len(entries)
					rf.matchIndex[peer] = rf.nextIndex[peer] - 1
				}
				rf.persist()
				rf.mu.Unlock()
				found = true
				return
			} else {
				if reply.Term == args.Term {
					if reply.ConflictTerm != 0 {
						rf.nextIndex[peer] = reply.ConflictIndex
					} else {
						rf.nextIndex[peer]--
					}
					rf.persist()
					rf.mu.Unlock()
					continue
				} else {
					rf.persist()
					rf.mu.Unlock()
					return
				}
			}
		} else {
			// 准备 InstallSnapshot
			args := &InstallSnapshotArgs{
				Term:              rf.currentTerm,
				LeaderId:          rf.me,
				LastIncludedIndex: rf.lastIncluedIndex,
				LastIncludedTerm:  rf.lastIncluedTerm,
				Data:              rf.persister.ReadSnapshot(),
			}
			rf.mu.Unlock()
			reply := &InstallSnapshotReply{}
			DPrintf("# send InstallSnapshot: from %d to peer %d, term %d: lastIncludedIndex %d, lastIncludedTerm: %d, log state: %s", rf.me, peer, rf.currentTerm, rf.lastIncluedIndex, rf.lastIncluedTerm, rf.showLogs())
			ok := rf.sendInstallSnapshot(peer, args, reply)
			if !ok {
				return
			} else {
				rf.mu.Lock()
				if reply.Term < rf.currentTerm {
					rf.mu.Unlock()
					return
				} else if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.toFollower()
					rf.persist()
					rf.mu.Unlock()
					return
				} else {
					if args.LastIncludedIndex+1 > rf.nextIndex[peer] {
						rf.nextIndex[peer] = args.LastIncludedIndex + 1
						rf.matchIndex[peer] = args.LastIncludedIndex
						rf.mu.Unlock()
						continue
					} else {
						rf.mu.Unlock()
						return
					}
				}
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
	args.LastLogIndex = rf.LogsLen() - 1
	args.LastLogTerm = rf.GetLogTerm(args.LastLogIndex)
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
		rf.persist()
		rf.mu.Unlock()
		return
	}
	// 过期的响应直接丢弃
	if reply.Term < rf.currentTerm {
		rf.mu.Unlock()
		return
	}
	if !reply.VoteGranted { // 如果对方不投票，说明它已经投给了别人，直接 return 就好了
		rf.mu.Unlock()
		return
	} else { // 如果对方投票了，那就要更新一下票数，同时判断一下是否能够转变成 leader
		// 更新票数
		rf.voteCnt += 1
		// 判断能否转变为 leader，但要*注意*，只有 candidate 才有转变的必要
		if rf.role == RAFT_ROLE_CANDIDATE && rf.voteCnt >= len(rf.peers)/2+1 {
			rf.role = RAFT_ROLE_LEADER
			for i := 0; i < len(rf.peers); i++ {
				rf.nextIndex[i] = rf.LogsLen()
				rf.matchIndex[i] = 0
			}
			rf.persist()
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
	DPrintf("# check commit, rf %d, matchIndex: %v, commitIndex: %d", rf.me, rf.matchIndex, rf.commitIndex)
	defer rf.mu.Unlock()
	for n := rf.LogsLen() - 1; n > rf.commitIndex; n-- {
		if rf.GetLogTerm(n) < rf.currentTerm {
			break
		} else if rf.GetLogTerm(n) == rf.currentTerm {
			cnt := 0
			for i := 0; i < len(rf.peers); i++ {
				if rf.matchIndex[i] >= n {
					cnt++
				}
			}
			if cnt >= len(rf.peers)/2+1 {
				rf.commitIndex = n
				rf.persist()
				break
			}
		}
	}
}

func (rf *Raft) applier() {
	for rf.killed() == false {
		rf.mu.Lock()
		commitIndex := rf.commitIndex
		lastApplied := rf.lastApplied
		applyMsgList := make([]*ApplyMsg, 0, commitIndex-lastApplied)
		logIdxList := make([]string, 0, commitIndex-lastApplied)
		for i := lastApplied + 1; i <= commitIndex; i++ {
			commandValid := true
			if i == 0 {
				DPrintf("######## WARNING! rf %d has lastApplied %d and commitIndex %d", rf.me, rf.lastApplied, rf.commitIndex)
				commandValid = false
			}
			applyMsg := &ApplyMsg{
				CommandValid: commandValid,
				Command:      rf.GetLog(i).Command,
				CommandIndex: i,
			}
			applyMsgList = append(applyMsgList, applyMsg)
			logIdxList = append(logIdxList, strconv.Itoa(i))
		}
		rf.mu.Unlock()
		if len(logIdxList) > 0 {
			s := "[" + strings.Join(logIdxList, ",") + "]"
			DPrintf("# prepare commit, rf %d, commit log index: %s", rf.me, s)
		}
		for _, msg := range applyMsgList {
			rf.applyCh <- *msg
		}

		rf.mu.Lock()
		if commitIndex > rf.lastApplied {
			rf.lastApplied = commitIndex
		}
		rf.mu.Unlock()
		time.Sleep(time.Duration(50) * time.Millisecond)
	}
}

// 并行地发起投票
func (rf *Raft) parallelAskVotes() {
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		} else {
			go rf.askVote(i)
		}
	}
}

func (rf *Raft) ticker() {
	electionTimeout := time.Duration(800+(rand.Int()%400)) * time.Millisecond
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.
		rf.mu.Lock()
		if rf.role == RAFT_ROLE_FOLLOWER || rf.role == RAFT_ROLE_CANDIDATE {
			if time.Since(rf.lastRecv) < electionTimeout {
				rf.mu.Unlock()
				// continue
			} else {
				electionTimeout = time.Duration(800+(rand.Int()%400)) * time.Millisecond
				rf.lastRecv = time.Now()
				DPrintf("$ election-timeout, rf %d prepare to leader election at term %d", rf.me, rf.currentTerm+1)
				// election timeout
				rf.role = RAFT_ROLE_CANDIDATE // convert to candidate
				// start election
				rf.currentTerm += 1 // Increment currentTerm
				// Vote for self
				rf.votedFor = rf.me
				rf.voteCnt = 1
				rf.persist()
				rf.mu.Unlock()
				// 发起投票
				rf.parallelAskVotes()
			}
		} else {
			DPrintf("# prepare heartbeat, rf %d at term %d as %d to send heartbeat.", rf.me, rf.currentTerm, rf.role)
			rf.mu.Unlock()
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				go rf.logReplication(i)
			}
			rf.checkLeaderCommit()
		}
		timeout := 80
		time.Sleep(time.Duration(timeout) * time.Millisecond)
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
	rand.Seed(time.Now().UnixNano())
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh

	// Your initialization code here (2A, 2B, 2C).
	rf.mu.Lock()
	rf.role = RAFT_ROLE_FOLLOWER
	rf.lastRecv = time.Now()
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.logs = make([]*LogEntry, 1, 10)
	rf.logs[0] = &LogEntry{
		Term:    0,
		Command: nil,
	}
	rf.lastIncluedIndex = -1
	rf.lastIncluedTerm = 0
	rf.voteCnt = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.mu.Unlock()

	// start ticker goroutine to start elections
	go rf.ticker()
	go rf.applier()

	return rf
}
