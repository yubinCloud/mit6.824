package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
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
type ByKey []*KeyValue

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
		log.Printf("recv task, type: %v, id: %d", task.TaskType, task.TaskId)
		isExit := taskHandler(task, mapf, reducef)
		if isExit {
			return
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
}

func taskHandler(task *AskTaskReply, mapf func(string, string) []KeyValue, reducef func(string, []string) string) (isExit bool) {
	taskType := task.TaskType
	if taskType == TERMINATE_TASK {
		log.Printf("worker pid %d exists", os.Getgid())
		return true
	}
	if taskType == EMPTY_TASK {
		time.Sleep(time.Duration(2) * time.Second)
		return false
	}
	if taskType == MAPPER_TASK {
		intermediateFiles, err := execMapTask(task, mapf)
		if err != nil {
			log.Fatalf("map 任务（ID：%v）执行失败", task.TaskId)
			return false
		}
		err = reportMapCompleted(task.TaskId, task.InputFileName, intermediateFiles)
		if err != nil {
			log.Fatalf("Failed to report map task %d", task.TaskId)
			return false
		}
		return false
	}
	if taskType == REDUCER_TASK {
		outFile, err := execReduceTask(task, reducef)
		if err != nil {
			log.Fatalf("reduce 任务（ID：%d）执行失败", task.TaskId)
			return false
		}
		err = reportReduceCompleted(task.TaskId, outFile)
		if err != nil {
			log.Fatalf("Failed to report reduce task %d", task.TaskId)
			return false
		}
		return false
	}
	log.Fatalf("Unexpected task type: %v", taskType)
	return true
}

func execMapTask(task *AskTaskReply, mapf func(string, string) []KeyValue) ([]IntermediateFileInfo, error) {
	log.Printf("exec map task %d", task.TaskId)
	log.Printf("Reading file %v", task.InputFileName)
	content, err := os.ReadFile(task.InputFileName)
	if err != nil {
		log.Fatalf("cannot read %v", task.InputFileName)
	}
	intermediate := mapf(task.InputFileName, string(content))
	intermediateFileInfos, err := storeIntermediateKv(intermediate, task.TaskId, task.ReduceNum)
	return intermediateFileInfos, nil
}

func reportMapCompleted(mapperId int, inputFilename string, intermediateFiles []IntermediateFileInfo) error {
	log.Printf("map task %d completed.", mapperId)
	err := ReportMapperTaskCompleted(mapperId, inputFilename, intermediateFiles)
	return err
}

func execReduceTask(task *AskTaskReply, reducef func(string, []string) string) (string, error) {
	log.Printf("exec reduce task %d", task.TaskId)
	intermediate, err := readIntermediate(task)
	if err != nil {
		return "", err
	}
	sort.Sort(ByKey(intermediate))
	outFileName := fmt.Sprintf("mr-out-%d", task.TaskId)
	outTempFile, err := os.CreateTemp("./", outFileName+"-*")
	if err != nil {
		log.Fatal("Failed to create temp file " + outFileName)
	}
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Key)
		}
		outV := reducef(intermediate[i].Key, values)
		fmt.Fprintf(outTempFile, "%v %v\n", intermediate[i].Key, outV)
		i = j
	}
	err = os.Rename("./"+outTempFile.Name(), "./"+outFileName)
	if err != nil {
		log.Fatal("Failed to rename file " + outFileName)
	}
	outTempFile.Close()
	return outFileName, nil
}

func reportReduceCompleted(reducerId int, outFile string) error {
	log.Printf("reduce task %d completed", reducerId)
	err := ReportReducerTaskCompleted(reducerId, outFile)
	return err
}

func readIntermediate(task *AskTaskReply) ([]*KeyValue, error) {
	var intermediate []*KeyValue
	inputFiles := strings.Split(task.InputFileName, "|")
	for _, inputFile := range inputFiles {
		f, err := os.Open("./" + inputFile)
		if err != nil {
			log.Fatalf("Failed to open file %v", inputFile)
			return nil, err
		}
		dec := json.NewDecoder(f)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, &kv)
		}
	}
	return intermediate, nil
}

func storeIntermediateKv(intermediate []KeyValue, mapperId int, reduceNum int) ([]IntermediateFileInfo, error) {
	buckets := make([][]KeyValue, reduceNum)
	for _, kv := range intermediate {
		bucketIdx := ihash(kv.Key) % reduceNum
		buckets[bucketIdx] = append(buckets[bucketIdx], kv)
	}
	var intermediateFileInfos []IntermediateFileInfo
	for reducerId, bucket := range buckets {
		mrName := fmt.Sprintf("mr-%d-%d", mapperId, reducerId)
		tmpfile, err := os.CreateTemp("./", mrName+"-*")
		if err != nil {
			log.Fatalf("Can't create tmpfile %v", tmpfile.Name())
			return nil, errors.New("Failed to create temp file")
		}
		enc := json.NewEncoder(tmpfile)
		for _, kv := range bucket {
			err := enc.Encode(kv)
			if err != nil {
				log.Fatalf("Can't encoder kv: " + kv.Key + "-" + kv.Value)
				return nil, err
			}
		}
		err = os.Rename(tmpfile.Name(), "./"+mrName)
		if err != nil {
			log.Fatal("Failed to rename file " + mrName)
			return nil, err
		}
		tmpfile.Close()
		intermediateFileInfo := IntermediateFileInfo{
			ReducerId: reducerId,
			FileName:  mrName,
		}
		intermediateFileInfos = append(intermediateFileInfos, intermediateFileInfo)
	}
	return intermediateFileInfos, nil
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

func ReportMapperTaskCompleted(mapperId int, inputFilename string, intermediateFiles []IntermediateFileInfo) error {
	args := ReportMapperTaskCompletedArgs{
		MapperId:          mapperId,
		InputFilename:     inputFilename,
		IntermediateFiles: intermediateFiles,
	}
	reply := ReportMapperTaskCompletedReply{}

	ok := call("Coordinator.MapperTaskCompletedCallback", &args, &reply)
	if ok {
		return nil
	} else {
		return errors.New("call failed")
	}
}

func ReportReducerTaskCompleted(reducerId int, outFile string) error {
	args := ReportReducerTaskCompletedArgs{
		ReducerId: reducerId,
		OutFile:   outFile,
	}
	reply := ReportReducerTaskCompletedReply{}

	ok := call("Coordinator.ReducerTaskCompletedCallback", &args, &reply)
	if ok {
		return nil
	} else {
		return errors.New("call failed")
	}
}
