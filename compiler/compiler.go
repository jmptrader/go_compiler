package compiler

import (
	"go_interpreter/ast"
	"go_interpreter/bytecode"
	"go_interpreter/object"
)

type Bytecode struct {
	Instructions bytecode.Instructions // Instructions generated by compiler
	Constants    []object.Object       // Constants evaluated by compiler
}

// Translates AST to bytecode
type Compiler struct {
	instructions bytecode.Instructions // Generated bytecode
	constants    []object.Object       // Constant pool
}

func BuildCompiler() *Compiler {
	return &Compiler{
		instructions: bytecode.Instructions{},
		constants:    []object.Object{},
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, statement := range node.Statements {
			err := c.Compile(statement)
			if err != nil {
				return err
			}
		}
	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
	case *ast.Infix:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}
	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: node.Value}
		c.emit(bytecode.OpConstant, c.addConstant(integer))
	}

	return nil
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
	}
}

// Helper method for adding constant to constant pool
func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1 // Return the constant's index
}

// Helper method for adding instruction
func (c *Compiler) addInstruction(instruction []byte) int {
	position := len(c.instructions)
	c.instructions = append(c.instructions, instruction...)
	return position
}

// Helper method to generate an instruction and add it to the results in memory
func (c *Compiler) emit(op bytecode.Opcode, operands ...int) int {
	instruction := bytecode.Make(op, operands...)
	position := c.addInstruction(instruction)
	return position // Return's starting position of newly emitted instruction
}
