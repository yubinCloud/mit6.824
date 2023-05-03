package mr

import (
	"log"
	"strings"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type Task struct {
	TaskType        string
	TaskId          int
	filename        string
	assignBeginTime time.Time
}

type Coordinator struct {
	// Your definitions here.
	mu         sync.Mutex
	MapperNum  int // Map 的数量 M
	ReducerNum int // Reduce 的数量 R

	MapperTasks           map[string]*Task
	IdleMapperTasks       []*Task
	InprogressMapperTasks map[string]*Task
	CompletedMapperTasks  []*Task

	IntermediateKvLocList [][]string // 记录所有中间结果文件，intermediateKv[1] 代表是 reducer 1 所需要的所有中间结果文件

	ReducerTasks           []Task
	IdleReducerTasks       []*Task
	InprogressReducerTasks map[int]*Task
	CompletedReducerTasks  []*Task

	ReduceOutputFiles []string
}

// Your code here -- RPC handlers for the worker to call.

func (c *Coordinator) fetchMapTask() *Task {
	// 从 IdleMapperTasks 中 pop 出一个 task
	mapTask := c.IdleMapperTasks[len(c.IdleMapperTasks)-1]
	c.IdleMapperTasks = c.IdleMapperTasks[:len(c.IdleMapperTasks)-1]
	// 将 pop 的 task 放入 InProgress 中
	c.InprogressMapperTasks[mapTask.filename] = mapTask

	mapTask.assignBeginTime = time.Now()
	return mapTask
}

func (c *Coordinator) fetchReduceTask() *Task {
	// 从 IdleReducerTasks 中 pop 出一个 task
	reduceTask := c.IdleReducerTasks[len(c.IdleReducerTasks)-1]
	c.IdleReducerTasks = c.IdleReducerTasks[:len(c.IdleReducerTasks)-1]
	// 将 pop 的 task 放入 InProgress 中
	c.InprogressReducerTasks[reduceTask.TaskId] = reduceTask

	reduceTask.assignBeginTime = time.Now()
	return reduceTask
}

func (c *Coordinator) AssignTask(args *AskTaskArgs, reply *AskTaskReply) error {
	c.mu.Lock()
	if len(c.IdleMapperTasks) != 0 { // 仍存在未分配的 map 任务
		mapTask := c.fetchMapTask()
		c.mu.Unlock()
		reply.TaskType = MAPPER_TASK
		reply.TaskId = mapTask.TaskId
		reply.InputFileName = mapTask.filename
		reply.ReduceNum = c.ReducerNum
		log.Printf("assign map task %d", mapTask.TaskId)
		return nil
	}
	if len(c.InprogressMapperTasks) != 0 { // map 任务都已分配，但存在未完成的 map 任务
		c.mu.Unlock()
		reply.TaskType = EMPTY_TASK
		log.Print("assign empty task")
		return nil
	}
	if len(c.IdleReducerTasks) != 0 { // 仍存在未分配的 reduce 任务
		reduceTask := c.fetchReduceTask()
		c.mu.Unlock()
		reply.TaskType = REDUCER_TASK
		reply.TaskId = reduceTask.TaskId
		reply.InputFileName = strings.Join(c.IntermediateKvLocList[reduceTask.TaskId], "|")
		reply.ReduceNum = c.ReducerNum
		log.Printf("assign reduce task %d", reduceTask.TaskId)
		return nil
	}
	if len(c.InprogressReducerTasks) != 0 { // reduce 任务都已分配，但存在未完成的 reduce 任务
		c.mu.Unlock()
		reply.TaskType = EMPTY_TASK
		log.Print("assign empty task")
		return nil
	}
	c.mu.Unlock()
	reply.TaskType = TERMINATE_TASK
	return nil
}

func (c *Coordinator) MapperTaskCompletedCallback(args *ReportMapperTaskCompletedArgs, reply *ReportMapperTaskCompletedReply) error {
	inputFile := args.InputFilename
	c.mu.Lock()
	mapTask, isOk := c.InprogressMapperTasks[inputFile]
	if !isOk {
		c.mu.Unlock()
		return nil
	}
	if mapTask.TaskId != args.MapperId {
		log.Fatal("report task 的 taskId 错误")
		c.mu.Unlock()
		return nil
	}
	delete(c.InprogressMapperTasks, inputFile)
	c.CompletedMapperTasks = append(c.CompletedMapperTasks, mapTask)
	for _, intermediateFileInfo := range args.IntermediateFiles {
		reducerId := intermediateFileInfo.ReducerId
		fname := intermediateFileInfo.FileName
		c.IntermediateKvLocList[reducerId] = append(c.IntermediateKvLocList[reducerId], fname)
	}
	c.mu.Unlock()
	log.Printf("map task %d completed", mapTask.TaskId)
	return nil
}

func (c *Coordinator) ReducerTaskCompletedCallback(args *ReportReducerTaskCompletedArgs, reply *ReportReducerTaskCompletedReply) error {
	c.mu.Lock()
	reduceTask, isOk := c.InprogressReducerTasks[args.ReducerId]
	if !isOk {
		c.mu.Unlock()
		return nil
	}
	delete(c.InprogressReducerTasks, args.ReducerId)
	c.CompletedReducerTasks = append(c.CompletedReducerTasks, reduceTask)
	c.ReduceOutputFiles = append(c.ReduceOutputFiles, args.OutFile)
	c.mu.Unlock()
	log.Printf("reduce task %d completed", args.ReducerId)
	return nil
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

const EXPIRED_TIME = time.Duration(8) * time.Second

func (c *Coordinator) checkTasks() {
	for {
		c.mu.Lock()
		now := time.Now()
		for k, task := range c.InprogressMapperTasks {
			dur := now.Sub(task.assignBeginTime)
			if dur > EXPIRED_TIME {
				delete(c.InprogressMapperTasks, k)
				c.IdleMapperTasks = append(c.IdleMapperTasks, task)
			}
		}
		for idx, task := range c.InprogressReducerTasks {
			dur := now.Sub(task.assignBeginTime)
			if dur > EXPIRED_TIME {
				delete(c.InprogressReducerTasks, idx)
				c.IdleReducerTasks = append(c.IdleReducerTasks, task)
			}
		}
		c.mu.Unlock()
	}
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go c.checkTasks()
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here
	c.mu.Lock()
	if len(c.CompletedReducerTasks) == c.ReducerNum {
		log.Print("all tasks have been completed!")
		log.Printf("output files: %v", c.ReduceOutputFiles)
		ret = true
	}
	c.mu.Unlock()
	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		MapperNum:              len(files),
		ReducerNum:             nReduce,
		MapperTasks:            make(map[string]*Task),
		IdleMapperTasks:        []*Task{},
		InprogressMapperTasks:  make(map[string]*Task),
		CompletedMapperTasks:   []*Task{},
		IntermediateKvLocList:  make([][]string, nReduce),
		ReducerTasks:           make([]Task, nReduce),
		IdleReducerTasks:       []*Task{},
		InprogressReducerTasks: make(map[int]*Task),
		CompletedReducerTasks:  []*Task{},
		ReduceOutputFiles:      []string{},
	}

	for idx, filename := range files {
		mapperTask := Task{
			TaskType:        MAPPER_TASK,
			TaskId:          idx,
			filename:        filename,
			assignBeginTime: time.Time{},
		}
		c.MapperTasks[filename] = &mapperTask
		c.IdleMapperTasks = append(c.IdleMapperTasks, &mapperTask)
	}
	// reverse
	for i, j := 0, len(c.IdleMapperTasks)-1; i < j; i, j = i+1, j-1 {
		c.IdleMapperTasks[i], c.IdleMapperTasks[j] = c.IdleMapperTasks[j], c.IdleMapperTasks[i]
	}

	for reducerId := nReduce - 1; reducerId >= 0; reducerId-- {
		taskPtr := &c.ReducerTasks[reducerId]
		taskPtr.TaskType = REDUCER_TASK
		taskPtr.TaskId = reducerId
		c.IdleReducerTasks = append(c.IdleReducerTasks, taskPtr)
	}

	c.server()
	return &c
}
