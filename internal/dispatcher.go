package internal

import (
	"fmt"
	"time"
)

type Job struct {
	ID int
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

func CreateNewWorker(id int, workerQueue chan *Worker, jobQueue chan *Job, dStatus chan *DispatchStatus) *Worker {
	w := &Worker{
		ID: id,
		jobs: jobQueue,
		dispatchStatus: dStatus,
	}

	go func() { workerQueue <- w }()
	return w
}

func (w *Worker) Start() {
	go func() {
		for {
			select {
			case job := <- w.jobs:
				fmt.Printf("Worker[%d] executing job[%d].\n", w.ID, job.ID)
				job.F()
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
		JobQueue:       make(chan *Job),
		DispatchStatus: make(chan *DispatchStatus),
		WorkQueue:      make(chan *Job),
		WorkerQueue:    make(chan *Worker),
	}
	return d
}

// Start executes job after a certain amount of time
// Should the job only be dispatched after a certain amount of time? Or immediately & then sleep for duration?
func (d *Dispatcher) Start(numWorkers int) {
	for i := 0; i<numWorkers; i++ {
		worker := CreateNewWorker(i, d.WorkerQueue, d.WorkQueue, d.DispatchStatus)
		worker.Start()
	}

	go func() {
		for {
			select {
			case job := <- d.JobQueue:
				fmt.Printf("Got a job in the queue to dispatch: %d\n", job.ID)
				d.WorkQueue <- job
			case ds := <- d.DispatchStatus:
				fmt.Printf("Got a dispatch status:\n\tType[%s] - ID[%d] - Status[%s]\n", ds.Type, ds.ID, ds.Status)
				if ds.Type == "worker" {
					if ds.Status == "quit" {
						d.JobCounter--
					}
				}
			}
		}
	}()
}

func (d *Dispatcher) AddJob(je JobExecutable, time time.Time) {
	j := &Job{ID: d.JobCounter, F: je, time: time}
	go func() { d.JobQueue <- j }()
	d.JobCounter++
	fmt.Printf("jobCounter is now: %d\n", d.JobCounter)
}

func (d *Dispatcher) Finished() bool {
	fmt.Printf("Finished Dispatcher \n")
	return d.JobCounter < 1
}
