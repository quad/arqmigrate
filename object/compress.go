package object

import (
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

func Decompress(ibs []byte) ([]byte, error) {
	var expectedLength uint32
	c, err := binary.Decode(ibs, binary.BigEndian, &expectedLength)
	if err != nil {
		return nil, fmt.Errorf("invalid length: %w", err)
	} else if c != 4 {
		return nil, fmt.Errorf("too short, no length: %w", err)
	}

	obs := make([]byte, expectedLength)
	actualLength, err := lz4.UncompressBlock(ibs[4:], obs)
	if err != nil {
		return nil, fmt.Errorf("invalid lz4 block: %w", err)
	} else if expectedLength != uint32(actualLength) {
		return nil, fmt.Errorf("expected %v bytes in lz4 block, got %v bytes", expectedLength, actualLength)
	}

	return obs, nil
}

func Compress(ibs []byte) ([]byte, error) {
	headerSize := 4
	obs := make([]byte, lz4.CompressBlockBound(len(ibs))+headerSize)
	binary.BigEndian.PutUint32(obs, uint32(len(ibs)))
	actualLength, err := lz4.CompressBlock(ibs, obs[headerSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}

	return obs[:headerSize+actualLength], nil
}
