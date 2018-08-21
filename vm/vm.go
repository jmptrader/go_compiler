package vm

import (
	"fmt"
	"go_interpreter/bytecode"
	"go_interpreter/compiler"
	"go_interpreter/object"
)

const stackCapacity = 2048

type VM struct {
	constants    []object.Object       // Constants generated by compiler
	instructions bytecode.Instructions // Instructions generated by compiler
	stack        []object.Object
	stackPointer int // stack[stackPointer-1] is top of stack
}

func BuildVM(bytecode *compiler.Bytecode) *VM {
	return &VM{
		instructions: bytecode.Instructions,
		constants:    bytecode.Constants,
		stack:        make([]object.Object, stackCapacity),
		stackPointer: 0,
	}
}

// Fetch-decode-execute cycle (instruction cycle)
func (vm *VM) Run() error {
	for i := 0; i < len(vm.instructions); i++ {
		// Fetch
		op := bytecode.Opcode(vm.instructions[i])

		// Decode
		switch op {
		case bytecode.OpConstant:
			constIndex := bytecode.ReadUint16(vm.instructions[i+1:])
			i += 2

			// Execute
			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Get top element in stack
func (vm *VM) Top() object.Object {
	if vm.stackPointer == 0 {
		return nil
	} else {
		return vm.stack[vm.stackPointer-1]
	}
}

// Push onto stack
func (vm *VM) push(o object.Object) error {
	if vm.stackPointer >= stackCapacity {
		return fmt.Errorf("Stack overflow")
	}

	vm.stack[vm.stackPointer] = o
	vm.stackPointer++
	return nil
}
