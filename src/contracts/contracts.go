package contracts
type Serializable interface {
	Serialize() ([]byte)
}

type Len interface {
	Len() int
}
