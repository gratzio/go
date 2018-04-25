package lumen

import (
	"strconv"
)

func (g *AddressGenerator) Generate(index uint32) (string, error) {
	key := index + 100
	return "5" + strconv.FormatUint(uint64(key), 10), nil
}
