GoshWorker is a high-performance Go-lang worker pool. It consists of 4 main elements: 1. Global pool. A Global pool is used to spawn and keep goshworkers globally. 
2. Gosh Pools. One or more Gosh pools can be created by taking of workers from it, and creating local goshworker pools. Goshworkers are recycled by a Global Pool after 
GoshPools are finalized. Gosh Pools are concurrency limiting goroutine pools. They limit the concurrency of task execution, not the number of tasks queued. 
They never block submitting tasks, no matter how many tasks are queued.
3. GoshWorker. A goshworker is a external process wrapped in a shell-like layer created by the gosh "Go-Shell" exec Api.
It does not actually invoke the system shell rather, it uses golangs os/exec library to create a similar shell-like wrapper,
while improving and overhauling the standard exec library and is the core component of goshworker. The shell like layer allows for more 
complex and consistent job control, automation, and speed. A goshworker can be command-line command, or even another Golang executable file.
Write the Main file, compile the executable, the process will be invoked with the Goshpool and inputs and outputs will be piped in/out streamingly with buffered pipes.
4. Task. A task is a io process that defines the input the worker command will be piped into the command, and also deals with the resulting output. 
If something goes awry during a task, then the task will return an error. 

This pool is meant to be as performant as possible, suggestions for improvents are welcome.
