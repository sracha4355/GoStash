package tasks

// ---- Task Queue setup for a worker pool ----//
type Task interface {
	Run() error
}

type TaskQueue struct {
	Tasks    chan Task
	NumTasks int
}

func (q *TaskQueue) New(
	__tasks__ []Task,
) {
	q.NumTasks = len(__tasks__)
	q.Tasks = make(chan Task, q.NumTasks)
	for _, task := range __tasks__ {
		q.Tasks <- task
	}
}