package pkg

import (
	"fmt"
	"time"
)

type Job struct {
	ID string
	F func() error
	time time.Time
}

type DispatchStatus struct {
	Type string
	ID int
	Status string
}

type Dispatcher struct {
	JobCounter     int
	JobQueue       chan *Job
	DispatchStatus chan *DispatchStatus
	WorkQueue      chan *Job
	WorkerQueue    chan *Worker
}

type Worker struct {
	ID int
	jobs chan *Job
	dispatchStatus chan *DispatchStatus
	Quit chan bool
}

type JobExecutable func() error

// CreateNewWorker adds a new worker to the worker queue from which the dispatcher will pull to execute jobs
func CreateNewWorker(id int, workerQueue chan *Worker, jobQueue chan *Job, dStatus chan *DispatchStatus) *Worker {
	w := &Worker{
		ID: id,
		jobs: jobQueue,
		dispatchStatus: dStatus,
	}

	go func() { workerQueue <- w }()
	return w
}

// Start handles job execution by a worker and quitting the worker when the task is completed.
func (w *Worker) Start() {
	go func() {
		for {
			select {
			case job := <- w.jobs:
				err := job.F()
				// If we get an error back from attempting to execute the job we really should retry the job at least once
				// and then give up on it. For now, however, I am just noting by printing that the job failed and continuing on
				if err != nil {
					fmt.Println("Failed to execute job!")
				}
				w.dispatchStatus <- &DispatchStatus{Type: "worker", ID: w.ID, Status: "quit"}
				w.Quit <- true
			case <- w.Quit:
				return
			}
		}
	}()
}

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		JobCounter:     0,
		JobQueue:  make(chan *Job),
		DispatchStatus: make(chan *DispatchStatus),
		WorkQueue:      make(chan *Job),
		WorkerQueue:    make(chan *Worker),
	}
	return d
}

// Start executes job after a certain amount of time
// Should the job only be dispatched after a certain amount of time? Or immediately & then sleep for duration?
func (d *Dispatcher) Start(numWorkers int) {
	if numWorkers < 1 {
		fmt.Println("Too few workers!")
		return
	}
	for i := 0; i<numWorkers; i++ {
		worker := CreateNewWorker(i, d.WorkerQueue, d.WorkQueue, d.DispatchStatus)
		worker.Start()
	}

	go func() {
		d.scanJobs()
	}()
}

func (d *Dispatcher)scanJobs() {
	for {
		select {
		case job := <- d.JobQueue:
			fmt.Printf("Got a job in the queue to dispatch: %s\n", job.ID)
			go func() { d.WorkQueue <- job }()
		case ds := <- d.DispatchStatus:
			fmt.Printf("Got a dispatch status:\n\tType[%s] - ID[%d] - Status[%s]\n", ds.Type, ds.ID, ds.Status)
			if ds.Type == "worker" {
				if ds.Status == "quit" {
					d.JobCounter--
				}
			}
		}
	}
}

// AddJob creates a job from the a jobExecutable, id and given execution time. Handles
// the delay by waiting for a duration before queueing the job for execution.
func (d *Dispatcher) AddJob(id string, je JobExecutable, executionTime time.Time) {
	job:= &Job{ID: id, F: je, time: executionTime}

	// If we have passed the desired execution time do not delay the job. Otherwise, determine
	// appropriate duration to wait before job execution.
	var duration time.Duration = 0
	if time.Now().Before(executionTime) {
		duration = executionTime.Sub(time.Now())
	}

	readyToExecute := make(chan bool)
	timer := time.AfterFunc(duration, func() {
		readyToExecute <- true
	})
	go func() {
		for {
			select {
			case _ = <-readyToExecute:
				// After the delay, queue the job for immediate execution
				d.JobQueue <- job
				d.JobCounter++
				timer.Stop()
				return
			default:
				// Sleeping for 1 second at a time but really this should be determined based on the DepositInterval
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (d *Dispatcher) Finished() bool {
	return d.JobCounter < 1
}
