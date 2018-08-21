package bytecode

import (
	"encoding/binary"
	"fmt"
)

type Instructions []byte
type Opcode byte

const (
	OpConstant Opcode = iota // 1 operand: previous assigned number to constant
)

// Make instruction from op and operands (Big Endian)
func Make(op Opcode, operands ...int) []byte {
	definition, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	numBytes := 1
	for _, width := range definition.OperandWidths {
		numBytes += width
	}

	instruction := make([]byte, numBytes)

	// Add op
	instruction[0] = byte(op)

	// Add operands
	offset := 1
	for i, o := range operands {
		width := definition.OperandWidths[i]

		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		}

		offset += width
	}

	return instruction
}

func ReadUint16(i Instructions) uint16 {
	return binary.BigEndian.Uint16(i)
}

// For debugging
type Definition struct {
	Name          string // readability
	OperandWidths []int  // number of bytes each operand takes up
}

var definitions = map[Opcode]*Definition{
	OpConstant: {"OpConstant", []int{2}},
}

func Lookup(op byte) (*Definition, error) {
	definition, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("Opcode %d undefined", op)
	}

	return definition, nil
}
