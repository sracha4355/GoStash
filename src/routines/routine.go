package routines
import "github.com/sracha4355/GoStash/src/utils"

type anyFunc func(...any) any
type Routine[Goroutine anyFunc] struct {
	id      int64
	routine Goroutine
	tags    map[string]interface{}
}

func NewRoutine[Goroutine anyFunc](
	idp *utils.IDProvider,
	routine Goroutine,
	tags *map[string]interface{},
) *Routine[Goroutine] {
	_tags_ :=  make(map[string]interface{})
	if tags != nil {
		for k, v := range *tags {
			_tags_[k] = v
		}
	}
	return &Routine[Goroutine]{
		id: idp.GetID(),
		routine: routine,
		tags:    _tags_,
	}
}

func (r *Routine[Goroutine]) ID() int64 {
	return r.id
}

func (r *Routine[Goroutine]) Go(args...any) {
	go r.routine(args...)
}

//---- decorators/wrappers for Routine struct ----//

