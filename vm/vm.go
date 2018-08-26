package vm

import (
	"fmt"
	"github.com/fatih/color"
	"go_interpreter/bytecode"
	"go_interpreter/compiler"
	"go_interpreter/object"
)

var PRINT_VM = false

const stackCapacity = 2048
const GlobalCapacity = 65536 // Upper limit on number of global bindings
const frameCapacity = 1024   // Upper limit on number of frames

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants    []object.Object // Constants generated by compiler
	stack        []object.Object // Stack for operands
	stackPointer int             // stack[stackPointer-1] is top of stack
	globals      []object.Object // Globals
	frames       []*Frame        // Stack of frames
	framesIndex  int             // Top of stack of frames
}

func BuildVM(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := BuildFrame(mainFn, 0)
	frames := make([]*Frame, frameCapacity)
	frames[0] = mainFrame

	return &VM{
		constants:    bytecode.Constants,
		stack:        make([]object.Object, stackCapacity),
		stackPointer: 0,
		globals:      make([]object.Object, GlobalCapacity),
		frames:       frames,
		framesIndex:  1, // Since mainFrame is already on the frame stack
	}
}

func BuildStatefulVM(bytecode *compiler.Bytecode, g []object.Object) *VM {
	vm := BuildVM(bytecode)
	vm.globals = g
	return vm
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

// Fetch-decode-execute cycle (instruction cycle)
func (vm *VM) Run() error {
	var ip int
	var instructions bytecode.Instructions
	var op bytecode.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		if PRINT_VM {
			color.Red("On frame %v", vm.framesIndex-1)
		}

		// Fetch
		vm.currentFrame().ip++
		ip = vm.currentFrame().ip
		instructions = vm.currentFrame().Instructions()
		op = bytecode.Opcode(instructions[ip])

		if PRINT_VM {
			def, _ := bytecode.Lookup(byte(op))
			color.Cyan("Current opcode: %s", def.Name)
		}

		// Decode & Execute
		switch op {
		case bytecode.OpGetLocal:
			localIndex := bytecode.ReadUint8(instructions[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+int(localIndex)])
			if err != nil {
				return err
			}
		case bytecode.OpSetLocal:
			// Get index of binding
			localIndex := bytecode.ReadUint8(instructions[ip+1:])
			vm.currentFrame().ip += 1
			// Get current frame
			frame := vm.currentFrame()
			// Save the binding to the location on the stack
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()
		case bytecode.OpReturnNothing:
			frame := vm.popFrame()
			vm.stackPointer = frame.basePointer - 1 // Reset back to base pointer and also pop function
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case bytecode.OpReturnValue:
			returnValue := vm.pop() // Pop return value off of stack
			frame := vm.popFrame()
			vm.stackPointer = frame.basePointer - 1 // Reset back to base pointer and also pop function
			err := vm.push(returnValue)
			if err != nil {
				return err
			}
		case bytecode.OpCall:
			// Get number of arguments to function
			numArgs := bytecode.ReadUint8(instructions[ip+1:])
			vm.currentFrame().ip += 1

			err := vm.callFunction(int(numArgs))
			if err != nil {
				return err
			}
		case bytecode.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndex(left, index)
			if err != nil {
				return err
			}
		case bytecode.OpHash:
			numElements := int(bytecode.ReadUint16(instructions[ip+1:]))
			vm.currentFrame().ip += 2

			hash, err := vm.buildHash(vm.stackPointer-numElements, vm.stackPointer)
			if err != nil {
				return err
			}
			vm.stackPointer -= numElements

			err = vm.push(hash)
			if err != nil {
				return err
			}
		case bytecode.OpArray:
			numElements := int(bytecode.ReadUint16(instructions[ip+1:]))
			vm.currentFrame().ip += 2

			array := vm.buildArray(vm.stackPointer-numElements, vm.stackPointer)
			vm.stackPointer -= numElements

			err := vm.push(array)
			if err != nil {
				return err
			}
		case bytecode.OpGetGlobal:
			globalIndex := bytecode.ReadUint16(instructions[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}
		case bytecode.OpSetGlobal:
			globalIndex := bytecode.ReadUint16(instructions[ip+1:])
			vm.currentFrame().ip += 2
			vm.globals[globalIndex] = vm.pop()
		case bytecode.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case bytecode.OpJumpNotTruthy:
			position := int(bytecode.ReadUint16(instructions[ip+1:]))
			// Skip over operand
			vm.currentFrame().ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = position - 1
			}
		case bytecode.OpJump:
			position := int(bytecode.ReadUint16(instructions[ip+1:]))
			// -1 because loop increments ip
			vm.currentFrame().ip = position - 1
		case bytecode.OpConstant:
			constIndex := bytecode.ReadUint16(instructions[ip+1:])
			// Skip over operand
			vm.currentFrame().ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}
		case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}
		case bytecode.OpPop:
			vm.pop()
		case bytecode.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}
		case bytecode.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}
		case bytecode.OpEqual, bytecode.OpNotEqual, bytecode.OpGreater:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}
		case bytecode.OpBang:
			err := vm.executeBang()
			if err != nil {
				return err
			}
		case bytecode.OpMinus:
			err := vm.executeMinus()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper method for call
func (vm *VM) callFunction(numArgs int) error {
	fn, ok := vm.stack[vm.stackPointer-1-numArgs].(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("Calling non-function")
	}
	if numArgs != fn.NumParameters {
		return fmt.Errorf(
			"Wrong number of arguments. Expected=%d, Actual=%d",
			fn.NumParameters,
			numArgs)
	}

	// basePointer is vm.stackPointer - numArgs
	frame := BuildFrame(fn, vm.stackPointer-numArgs)
	vm.pushFrame(frame)
	vm.stackPointer = frame.basePointer + fn.NumLocals
	return nil
}

// Helper method for index
func (vm *VM) executeIndex(left, index object.Object) error {
	if left.Type() == object.ARRAY_OBJECT && index.Type() == object.INTEGER_OBJECT {
		return vm.executeArrayIndex(left, index)
	} else if left.Type() == object.HASH_OBJECT {
		return vm.executeHashIndex(left, index)
	} else {
		return fmt.Errorf("Index operator not supported for %s", left.Type())
	}
}

// Helper method for array index
func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value

	if i < 0 || i > int64(len(arrayObject.Elements)-1) {
		return vm.push(Null)
	} else {
		return vm.push(arrayObject.Elements[i])
	}
}

// Helper method for hash index
func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)
	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("Unusable as hash key")
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	} else {
		return vm.push(pair.Value)
	}
}

// Helper method for hashmaps
func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)
	for i := startIndex; i < endIndex; i += 2 {
		// Get key and value, and create a pair
		key := vm.stack[i]
		value := vm.stack[i+1]
		pair := object.HashPair{Key: key, Value: value}

		// Check if key is hashable
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("Key is unhashable")
		}

		// Hash the key
		hashedPairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

// Helper method for arrays
func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	array := make([]object.Object, endIndex-startIndex)
	for i := startIndex; i < endIndex; i++ {
		array[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: array}
}

// Helper method for conditionals
func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

// Helper method to execute -
func (vm *VM) executeMinus() error {
	value := vm.pop()

	if value.Type() != object.INTEGER_OBJECT {
		return fmt.Errorf("Unsupported type: %s", value.Type())
	}

	return vm.push(&object.Integer{Value: -value.(*object.Integer).Value})
}

// Helper method to execute !
func (vm *VM) executeBang() error {
	value := vm.pop()

	switch value {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

// Helper method to execute !=, >, ==
func (vm *VM) executeComparison(op bytecode.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJECT || right.Type() == object.INTEGER_OBJECT {
		return vm.executeIntegerComparison(left, op, right)
	}

	switch op {
	case bytecode.OpEqual:
		return vm.push(toBooleanObject(right == left))
	case bytecode.OpNotEqual:
		return vm.push(toBooleanObject(right != left))
	default:
		return fmt.Errorf("Unknown operator: %s %d %s", left.Type(), op, right.Type())
	}
}

// Helper method to execute !=, >, == for integers
func (vm *VM) executeIntegerComparison(
	left object.Object, op bytecode.Opcode, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case bytecode.OpEqual:
		return vm.push(toBooleanObject(leftValue == rightValue))
	case bytecode.OpNotEqual:
		return vm.push(toBooleanObject(leftValue != rightValue))
	case bytecode.OpGreater:
		return vm.push(toBooleanObject(leftValue > rightValue))
	default:
		return fmt.Errorf("Unknown operator: %d", op)
	}
}

// Helper method to convert bool to boolean objects
func toBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	} else {
		return False
	}
}

// Helper method to execute +,-,*,/
func (vm *VM) executeBinaryOperation(op bytecode.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJECT && right.Type() == object.INTEGER_OBJECT {
		leftValue := left.(*object.Integer).Value
		rightValue := right.(*object.Integer).Value

		var result int64

		switch op {
		case bytecode.OpAdd:
			result = leftValue + rightValue
		case bytecode.OpSub:
			result = leftValue - rightValue
		case bytecode.OpMul:
			result = leftValue * rightValue
		case bytecode.OpDiv:
			result = leftValue / rightValue
		default:
			return fmt.Errorf("Unsupported operator for integer: %s", op)
		}

		return vm.push(&object.Integer{Value: result})
	} else if left.Type() == object.STRING_OBJECT && right.Type() == object.STRING_OBJECT {
		if op != bytecode.OpAdd {
			return fmt.Errorf("Unsupported operator for string: %s", op)
		}

		leftValue := left.(*object.String).Value
		rightValue := right.(*object.String).Value

		return vm.push(&object.String{Value: leftValue + rightValue})
	} else {
		return fmt.Errorf("Unsupported types for binary operation: %s %s", left.Type(), right.Type())
	}
}

// Get last popped element (for debugging)
func (vm *VM) LastPopped() object.Object {
	return vm.stack[vm.stackPointer]
}

// Push to stack
func (vm *VM) push(o object.Object) error {
	if vm.stackPointer >= stackCapacity {
		return fmt.Errorf("Stack overflow")
	}

	vm.stack[vm.stackPointer] = o
	vm.stackPointer++
	return nil
}

// Pop from stack
func (vm *VM) pop() object.Object {
	o := vm.stack[vm.stackPointer-1]
	vm.stackPointer--
	return o
}
