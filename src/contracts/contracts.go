package contracts
type Serializable interface {
	Serialize() ([]byte)
	//Len() int
}

