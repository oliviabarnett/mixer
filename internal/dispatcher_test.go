package internal

import (
	"testing"
	"time"
)

func NewMockDispatcher(jobQueue chan *Job, dispatchStatus chan *DispatchStatus, workQueue chan *Job, workerQueue chan *Worker) *Dispatcher {
	d := &Dispatcher{
		JobCounter:     0,
		JobQueue:       jobQueue,
		DispatchStatus: dispatchStatus,
		WorkQueue:      workQueue,
		WorkerQueue:    workerQueue,
	}
	return d
}

func TestDispatcher_AddJob(t *testing.T) {
	var jobQueue = make(chan *Job)
	var dispatchStatus = make(chan *DispatchStatus)
	var workQueue = make(chan *Job)
	var workerQueue = make(chan *Worker)
	executionTime := time.Now()
	destinationAddress := "destination"

	dispatcher := NewMockDispatcher(jobQueue, dispatchStatus, workQueue, workerQueue)

	job := func() error {
		return nil
	}
	dispatcher.AddJob(destinationAddress, job, executionTime)

	var createdJobs []*Job
	for i := range jobQueue {
		createdJobs = append(createdJobs, i)
		if len(createdJobs) > 0 {
			break
		}
	}
	if len(createdJobs) == 0 {
		t.Errorf("Did not successfully add job to job queue")
	}
}

func TestScanJobs(t *testing.T) {
	var jobQueue = make(chan *Job)
	var dispatchStatus = make(chan *DispatchStatus)
	var workQueue = make(chan *Job)
	var workerQueue = make(chan *Worker)

	dispatcher := NewMockDispatcher(jobQueue, dispatchStatus, workQueue, workerQueue)

	go func() {
		dispatcher.scanJobs()
	}()

	jobFunc := func() error {
		return nil
	}
	job := &Job{ID: "myJob", F: jobFunc, time: time.Now()}
	jobQueue <- job

	dispatchedJob := <- workQueue

	if dispatchedJob == nil {
		t.Errorf("ScanJobs did not successfully add job to work queue")
	}

	if dispatcher.JobCounter != 0 {
		t.Errorf("ScanJobs did not decrement job counter")
	}
}

func TestWorker_Start(t *testing.T) {
	var jobQueue = make(chan *Job)
	var dispatchStatus = make(chan *DispatchStatus)

	worker := Worker{
		ID:             1,
		jobs:           jobQueue,
		dispatchStatus: dispatchStatus,
		Quit:          	nil,
	}

	worker.Start()

	var executedJob = make(chan bool)
	jobFunc := func() error {
		executedJob <- true
		return nil
	}
	job := &Job{ID: "myJob", F: jobFunc, time: time.Now()}
	jobQueue <- job

	didExecute := <- executedJob
	if !didExecute {
		t.Errorf("Did not execute scheduled job")
	}
}