package raft

import "log"

// Debugging
const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

func (rf *Raft) GetLog(index int) *LogEntry {
	return rf.logs[index-rf.lastIncluedIndex-1]
}

func (rf *Raft) GetLogTerm(index int) int {
	if index == rf.lastIncluedIndex {
		return rf.lastIncluedTerm
	}
	return rf.logs[index-rf.lastIncluedIndex-1].Term
}

func (rf *Raft) LogsLen() int {
	return rf.lastIncluedIndex + len(rf.logs) + 1
}

func (rf *Raft) RealIndex(vIndex int) int {
	return vIndex - rf.lastIncluedIndex - 1
}

func copyLogs(oldEntries []*LogEntry) []*LogEntry {
	newEntries := make([]*LogEntry, 0, len(oldEntries)+1)
	newEntries = append(newEntries, oldEntries...)
	return newEntries
}
