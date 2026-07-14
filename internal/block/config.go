package block

import "fmt"

type Config struct {
	ExecutorID   string
	RuntimeEpoch uint32
	BlockSize    uint32
	NumBlocks    uint32
}

func (c Config) Validate() error {
	if c.BlockSize == 0 {
		return fmt.Errorf("block size must be greater than zero")
	}
	if c.NumBlocks == 0 {
		return fmt.Errorf("number of blocks must be greater than zero")
	}
	return nil
}
