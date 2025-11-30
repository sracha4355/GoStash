package contracts
type Serializable[T any] interface {
	Serialize() ([]byte)
	SafeSerialize() ([]byte, error) 
}

