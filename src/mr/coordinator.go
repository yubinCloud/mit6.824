package mr

import (
	"log"
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
	log.Print("map 任务全部完成")
	reply.TaskType = TERMINATE_TASK
	c.mu.Unlock()

	return nil
}

func (c *Coordinator) MapperTaskCompletedCallback(args *ReportMapperTaskCompletedArgs, reply *ReportMapperTaskCompletedReply) error {
	inputFile := args.InputFilename
	c.mu.Lock()
	mapTask, isOk := c.InprogressMapperTasks[inputFile]
	if !isOk {
		return nil
	}
	if mapTask.TaskId != args.MapperId {
		log.Fatal("report task 的 taskId 错误")
		return nil
	}
	delete(c.InprogressMapperTasks, inputFile)
	c.CompletedMapperTasks = append(c.CompletedMapperTasks, mapTask)
	c.mu.Unlock()
	return nil
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
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
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		MapperNum:             len(files),
		ReducerNum:            nReduce,
		MapperTasks:           make(map[string]*Task),
		IdleMapperTasks:       []*Task{},
		InprogressMapperTasks: make(map[string]*Task),
		CompletedMapperTasks:  []*Task{},
	}

	c.MapperTasks = make(map[string]*Task)
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

	c.server()
	return &c
}
