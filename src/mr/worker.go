package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)
import "log"
import "net/rpc"
import "hash/fnv"

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for {
		task, err := AskTask()
		if err != nil {
			return
		}
		log.Printf("handle map task %d", task.TaskId)
		isExit := taskHandler(task, mapf, reducef)
		if isExit {
			return
		}
	}
}

func taskHandler(task *AskTaskReply, mapf func(string, string) []KeyValue, reducef func(string, []string) string) (isExit bool) {
	taskType := task.TaskType
	if taskType == TERMINATE_TASK {
		log.Printf("worker pid %d exists", os.Getgid())
		return true
	}
	if taskType == MAPPER_TASK {
		err := execMapTask(task, mapf)
		if err != nil {
			log.Fatalf("map 任务（ID：%v）执行失败", task.TaskId)
			return false
		}
		err = reportMapCompleted(task.TaskId, task.InputFileName)
		if err != nil {
			log.Fatalf("Failed to report map task %d", task.TaskId)
			return false
		}
		return false
	}
	if taskType == EMPTY_TASK {
		time.Sleep(time.Duration(2) * time.Second)
		return false
	}
	log.Fatalf("Unexpected task type: %v", taskType)
	return true
}

func execMapTask(task *AskTaskReply, mapf func(string, string) []KeyValue) error {
	log.Printf("Opening file: %v", task.InputFileName)
	content, err := os.ReadFile(task.InputFileName)
	if err != nil {
		log.Fatalf("cannot read %v", task.InputFileName)
	}
	intermediate := mapf(task.InputFileName, string(content))
	log.Printf("Storing intermediate kv")
	storeIntermediateKv(intermediate, task.TaskId, task.ReduceNum)
	return nil
}

func reportMapCompleted(mapperId int, inputFilename string) error {
	log.Printf("map task %d completed.", mapperId)
	err := ReportMapperTaskCompleted(mapperId, inputFilename)
	return err
}

func storeIntermediateKv(intermediate []KeyValue, mapperId int, reduceNum int) {
	sort.Sort(ByKey(intermediate))
	buckets := make([][]*KeyValue, reduceNum)
	print(len(buckets))
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var bucket []*KeyValue
		for k := i; k < j; k++ {
			bucket = append(bucket, &intermediate[k])
		}
		key := intermediate[i].Key
		bucketIdx := ihash(key) % reduceNum
		buckets[bucketIdx] = append(buckets[bucketIdx], bucket...)
		i = j
	}
	for reducerId, bucket := range buckets {
		tmpfile, err := os.CreateTemp("./", fmt.Sprintf("mr-%d-%d-*", mapperId, reducerId))
		println(tmpfile.Name())
		if err != nil {
			log.Fatalf("Can't create tmpfile %v", tmpfile.Name())
			return
		}
		enc := json.NewEncoder(tmpfile)
		for _, kv := range bucket {
			println("Encoding KV: " + kv.Key + "-" + kv.Value)
			err := enc.Encode(kv)
			if err != nil {
				log.Fatalf("Can't encoder kv: " + kv.Key + "-" + kv.Value)
				return
			}
		}
		err = os.Rename("./"+tmpfile.Name(), fmt.Sprintf("./mr-%d-%d", mapperId, reducerId))
		if err != nil {
			log.Fatalf("Failed to rename file mr-%d-%d", mapperId, reducerId)
		}
	}
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func AskTask() (*AskTaskReply, error) {
	args := AskTaskArgs{}
	reply := AskTaskReply{}

	ok := call("Coordinator.AssignTask", &args, &reply)
	if ok {
		return &reply, nil
	} else {
		return &reply, errors.New("call failed")
	}
}

func ReportMapperTaskCompleted(mapperId int, inputFilename string) error {
	args := ReportMapperTaskCompletedArgs{MapperId: mapperId, InputFilename: inputFilename}
	reply := ReportMapperTaskCompletedReply{}

	ok := call("Coordinator.MapperTaskCompletedCallback", &args, &reply)
	if ok {
		return nil
	} else {
		return errors.New("call failed")
	}
}
