package disasm

import (
	"errors"
	"fmt"
	"strings"
)

// Instruction Set
//////////////////////////////////////

// Returns the first one line instruction in the form of an Instruction "struct" of a byte array that we are given
func Parse(in []byte, address int) (Instruction, error) {
	firstByte := in[0]
	var signed bool

	// Check if this is a signed operation
	instructions := unsignedInstructions
	if firstByte == 0xFE {
		signed = true
		firstByte = in[1]
		instructions = signedInstructions
	}

	if instruction, ok := instructions[firstByte]; ok {
		// We have it!
		instruction.Op = firstByte
		instruction.Signed = signed
		instruction.Address = address

		// Check for Indexed Addressing Mode Instruction Type
		if instruction.AddressingMode == "indexed" && instruction.VariableLength == true {
			if in[1]&1 == 1 {
				instruction.ByteLength++
				instruction.AddressingMode = "long-indexed"
			} else {
				instruction.AddressingMode = "short-indexed"
			}
		}

		// Check for Indirect Addressing Mode Instruction Type
		if instruction.AddressingMode == "indirect" {
			if in[1]&1 == 1 {
				instruction.AddressingMode = "indirect+"
				instruction.AutoIncrement = true
			}
		}

		// Adjust for signed instructions
		if signed {
			instruction.ByteLength++
			instruction.Signed = signed
			instruction.Mnemonic = "SGN " + instruction.Mnemonic
			instruction.RawOps = in[2:instruction.ByteLength]
		} else {
			instruction.RawOps = in[1:instruction.ByteLength]
		}

		instruction.Raw = in[0:instruction.ByteLength]

		// Build our Vars object from the VarStrings object
		if instruction.VarCount > 0 {

			if (firstByte & 0xf8) == 0x20 {
				instruction.doSJMP()
				instruction.doPseudo()

			} else if (firstByte & 0xf8) == 0x28 {
				instruction.doSCALL()
				instruction.doPseudo()

			} else if (firstByte & 0xf8) == 0x30 {
				instruction.doJBC()
				instruction.doPseudo()

			} else if (firstByte & 0xf8) == 0x38 {
				instruction.doJBS()
				instruction.doPseudo()

			} else if (firstByte & 0xf0) == 0xd0 {
				instruction.doCONDJMP()
				instruction.doPseudo()

			} else if (firstByte & 0xf0) == 0xf0 {
				instruction.doF0()
				instruction.doPseudo()

			} else if (firstByte & 0xf0) == 0xe0 {
				instruction.doE0()
				instruction.doPseudo()

			} else if (firstByte & 0xf0) == 0xc0 {
				instruction.doC0()
				instruction.doPseudo()

			} else if (firstByte & 0xe0) == 0 {
				instruction.do00()
				instruction.doPseudo()

			} else {
				instruction.doMIDDLE()
				instruction.doPseudo()
			}

		} else {
			instruction.Checked = true
		}

		return instruction, nil

	} else {
		return Instruction{ByteLength: 1}, errors.New("Unable to find instruction!")
	}

}

type Instruction struct {
	Op              byte
	Address         int
	XRefs           map[int][]XRef
	Calls           map[int][]Call
	Jumps           map[int][]Jump
	Raw             []byte
	RawOps          []byte
	Mnemonic        string
	ByteLength      int
	VarCount        int
	VarStrings      []string            // baop, breg (strings)
	Vars            map[string]Variable // baop, breg (assembled objects)
	PseudoCode      string
	PseudoString    string
	VarTypes        []string // dest, src, etc
	AddressingMode  string
	Description     string
	LongDescription string
	VariableLength  bool
	AutoIncrement   bool
	Flags           Flags
	Signed          bool
	Ignore          bool
	Reserved        bool
	Checked         bool
}

type Instructions []Instruction

func (inst Instructions) Len() int {
	return len(inst)
}

func (inst Instructions) Less(i, j int) bool {
	return inst[i].Address < inst[j].Address
}

func (inst Instructions) Swap(i, j int) {
	inst[i], inst[j] = inst[j], inst[i]
}

var VarObjs = map[string]Variable{
	"aa": {
		Description: "A 2-bit field within an opcode that selects the basic addressing mode used. This field is present only in those opcodes that allow addressing mode options. ",
		Bits:        2,
	},
	"baop": {
		Description: "A byte operand that is addressed by any addressing mode.",
		Bits:        8,
	},
	"bbb": {
		Description: "A 3-bit field within an opcode that selects a specific bit within a register.",
		Bits:        3,
	},
	"bitno": {
		Description: "A 3-bit field within an opcode that selects one of the eight bits in a byte. ",
		Bits:        3,
	},
	"breg": {
		Description: "A byte register in the internal register file. When it could be unclear whether this variable refers to a source or a destination register, it is prefixed with an S or a D. The value must be in the range of 00–FFH.",
		Bits:        8,
	},
	"cadd": {
		Description: "An address in the program code",
		//Bits:       0,
	},
	"Dbreg": {
		Description: "A byte register in the lower register file that serves as the destination of the instruction operation. ",
		Bits:        8,
	},
	"disp": {
		Description: "Displacement. The distance between the end of an instruction and the target label.",
		//Bits:       0,
	},
	"Dlreg": {
		Description: "A 32-bit register in the lower register file that serves as the destination of the instruction operation. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH.",
		Bits:        8,
	},
	"Dwreg": {
		Description: "A word register in the lower register file that serves as the destination of the instruction operation. Must be aligned on an address that is evenly divisible by 2. The value must be in the range of 00–FEH.",
		Bits:        8,
	},
	"lreg": {
		Description: "A 32-bit register in the lower register file. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH. ",
		Bits:        8,
	},
	"ptr2_reg": {
		Description: " A double-pointer register, used with the EBMOVI instruction. Must be aligned on an address that is evenly divisible by 8. The value must be in the range of 00–F8H. ",
		Bits:        8,
	},
	"preg": {
		Description: "A pointer register. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH. ",
		Bits:        8,
	},
	"Sbreg": {
		Description: "A byte register in the lower register file that serves as the source of the instruction operation.",
		Bits:        8,
	},
	"Slreg": {
		Description: "A 32-bit register in the lower register file that serves as the source of the instruction operation. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH.",
		Bits:        8,
	},
	"Swreg": {
		Description: "A word register in the lower register file that serves as the source of the instruction operation. Must be aligned on an address that is evenly divisible by 2. The value must be in the range of 00–FEH.",
		Bits:        8,
	},
	"treg": {
		Description: "A 24-bit register in the lower register file. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH.",
		Bits:        8,
	},
	"waop": {
		Description: "A word operand that is addressed by any addressing mode.",
		Bits:        16,
	},
	"w2_reg": {
		Description: "A double-word register in the lower register file. Must be aligned on an address that is evenly divisible by 4. The value must be in the range of 00–FCH. Although w2_reg is similar to lreg, there is a distinction: w2_reg consists of two halves, each containing a 16-bit address; lreg is indivisible and contains a 32-bit number.",
		//Bits:       0,
	},
	"wreg": {
		Description: "A word register in the lower register file. When it could be unclear whether this variable refers to a source or a destination register, it is prefixed with an S or a D. Must be aligned on an address that is evenly divisible by 2. The value must be in the range of 00–FEH.",
		//Bits:       0,
	},
	"xxx": {
		Description: "The three high-order bits of displacement",
		Bits:        3,
	},
}

type Flags struct{}

type Variable struct {
	Description string
	Type        string
	Value       string
	Bits        int
}

type XRef struct {
	String   string
	Mnemonic string
	XRefFrom int
	XRefTo   int
}

type Call struct {
	String   string
	Mnemonic string
	CallFrom int
	CallTo   int
}

type Jump struct {
	String   string
	Mnemonic string
	JumpFrom int
	JumpTo   int
}

// XRef
func (instr *Instruction) XRef(s string, v int) {
	//if v != 0x00 && instr.Mnemonic != "JBC" {
	if v > 0x02 {

		existing := instr.XRefs
		if existing == nil {
			instr.XRefs = make(map[int][]XRef)
		} else {
			for _, ins := range instr.XRefs[v] {
				if ins.XRefFrom == instr.Address {
					return
				}
			}
		}

		instr.XRefs[v] = append(existing[v], XRef{String: fmt.Sprintf(s, v), Mnemonic: instr.Mnemonic, XRefFrom: instr.Address, XRefTo: v})
	}
}

// Call
func (instr *Instruction) Call(s string, v int) {
	existing := instr.Calls
	if existing == nil {
		instr.Calls = make(map[int][]Call)
	}
	instr.Calls[v] = append(existing[v], Call{String: fmt.Sprintf(s, v), Mnemonic: instr.Mnemonic, CallFrom: instr.Address, CallTo: v})
}

// Jump
func (instr *Instruction) Jump(s string, v int) {
	existing := instr.Jumps
	if existing == nil {
		instr.Jumps = make(map[int][]Jump)
	}
	instr.Jumps[v] = append(existing[v], Jump{String: fmt.Sprintf(s, v), Mnemonic: instr.Mnemonic, JumpFrom: instr.Address, JumpTo: v})
}

// Do Pseudo
func (instr *Instruction) doPseudo() {
	var v [3]string

Loop:
	for _, varStr := range instr.VarStrings {

		if instr.Mnemonic == "DJNZ" || instr.Mnemonic == "DJNZW" {
			v[0] = instr.Vars["cadd"].Value
			v[1] = instr.Vars["breg"].Value
			break Loop
		}

		val := instr.Vars[varStr].Value
		val = strings.Replace(val, "[R_00 ~(Zero Register)]", "", 1)
		val = strings.Replace(val, "R_", "$r_", 1)
		val = strings.Replace(val, "[$r_00]", "", 1)
		val = strings.Replace(val, "$r_00", "0x00", 1)
		val = strings.Replace(val, "$r_02", "0x11", 1)
		val = strings.Replace(val, " ~(", " (", 1)
		val = strings.Replace(val, " ~", "", 1)
		val = strings.Replace(val, "$r_02 (Ones Register)", "0x11", 1)
		val = strings.Replace(val, " (Ones Register)", "", 1)
		val = strings.Replace(val, "#", "0x", 1)

		val = strings.Replace(val, " ( GP Reg RAM )", "", 1)

		switch instr.Vars[varStr].Type {
		case "DEST":
			val = strings.Replace(val, "0x000", "$r_", 1)
			val = strings.Replace(val, "0x", "$r_", 1)
			v[0] = val
		case "ADDR":
			v[0] = val
		case "PTRS":
			v[0] = val
		case "BYTEREG":
			v[2] = val
		default:
			v[1] = val
		}
	}

	switch instr.Mnemonic {

	case "CLR", "CLRB":
		instr.PseudoCode = fmt.Sprintf("%s = 0x00", v[0])

	case "EXT":
		instr.PseudoCode = fmt.Sprintf("SIGN EXTEND INT %s TO LONG INT", v[0])

	case "EXTB":
		instr.PseudoCode = fmt.Sprintf("SIGN EXTEND SHORT INT %s TO INT", v[0])

	case "JNST", "JNH", "JGT", "JNC", "JNVT", "JNV", "JGE", "JNE", "JST", "JH", "JLE", "JC", "JVT", "JV", "JLT", "JE":
		instr.PseudoCode = fmt.Sprintf("	JUMP TO: %s", v[0])

	case "JBS":
		instr.PseudoCode = fmt.Sprintf("if bitno: (%s) of %s is set { JUMP TO: %s }", v[1], v[2], v[0])

	case "JBC":
		instr.PseudoCode = fmt.Sprintf("if bitno: (%s) of %s is clear { JUMP TO: %s }", v[1], v[2], v[0])

	case "LJMP", "SJMP", "EBR", "EJMP":
		instr.PseudoCode = fmt.Sprintf("JUMP TO: %s", v[0])

	case "ECALL", "CALL", "SCALL", "LCALL":
		instr.PseudoCode = fmt.Sprintf("CALL SUB_ %s", v[0])

	case "PUSH":
		instr.PseudoCode = fmt.Sprintf("PUSH %s ONTO THE STACK", v[1])

	case "POP":
		instr.PseudoCode = fmt.Sprintf("POP THE STACK TO %s", v[0])

	case "CMPB", "CMP", "CMPL":
		instr.PseudoCode = fmt.Sprintf("if (%s == %s) {", v[0], v[1])

	case "ANDB", "AND", "ADDB":
		instr.PseudoCode = fmt.Sprintf("%s = %s & %s", v[0], v[0], v[1])

	case "ORB", "OR", "XOR", "XORB":
		instr.PseudoCode = fmt.Sprintf("%s = %s %s %s", v[0], v[0], instr.Mnemonic, v[1])

	case "NOT", "NOTB", "NEG", "NEGB":
		instr.PseudoCode = fmt.Sprintf("%s = %s %s %s", v[0], v[0], instr.Mnemonic, v[0])

	case "ADD", "ADDC", "ADDCB":
		instr.PseudoCode = fmt.Sprintf("%s = %s + %s", v[0], v[0], v[1])

	case "XCH", "XCHB":
		instr.PseudoCode = fmt.Sprintf("%s <=%s=> %s", v[0], instr.Mnemonic, v[1])

	case "SUB", "SUBC", "SUBCB", "SUBB":
		instr.PseudoCode = fmt.Sprintf("%s = %s - %s", v[0], v[0], v[1])

	case "MUL", "MULB", "MULU", "MULUB", "SGN MUL", "SGN MULB":
		instr.PseudoCode = fmt.Sprintf("%s = %s * %s", v[0], v[0], v[1])

	case "DIV", "DIVU", "DIVUB", "SGN DIVB", "SGN DIV":
		instr.PseudoCode = fmt.Sprintf("%s = %s / %s", v[0], v[0], v[1])

	case "SHR", "SHRL", "SHRAL", "SHRB":
		instr.PseudoCode = fmt.Sprintf("%s >> %s", v[0], v[1])

	case "SHL", "SHLL", "SHLB", "SHRA":
		instr.PseudoCode = fmt.Sprintf("%s << %s", v[0], v[1])

	case "DEC", "DECB":
		instr.PseudoCode = fmt.Sprintf("%s--", v[0])

	case "INC", "INCB":
		instr.PseudoCode = fmt.Sprintf("%s++", v[0])

	case "LD", "LDB", "ELD", "ELDB", "STB", "ESTB", "ST", "EST", "LDBZE", "LDBSE":
		instr.PseudoCode = fmt.Sprintf("%s = %s", v[0], v[1])

	case "NORML": // TODO
		instr.PseudoCode = fmt.Sprintf("NORMALIZE %s (todo)", v[0])

	case "BMOV", "BMOVI":
		instr.PseudoCode = fmt.Sprintf("BMOV %s count(%s) (todo)", v[0], v[1])

	case "DJNZ", "DJNZW":
		instr.PseudoCode = fmt.Sprintf("%s--; if ( %s != 0 ) { JUMP TO: %s }", v[1], v[1], v[0])

	default:
		instr.PseudoCode = fmt.Sprintf("########### %s = %s", v[0], v[1])
	}
}

// Get Offset
func getOffset(data []byte) int {
	b1 := byte(data[0])
	b2 := byte(data[1])

	//fmt.Printf("B1: 		0x%X 		%.8b \n", b1, b1)

	b1 = b1 & 0x07

	if b1&0x04 == 0x04 {
		b1 |= 0xFC
		//b3 = 0xFF
	}

	offset := int((int16(b1) << 8) | int16(b2))

	return offset
}

// SJMP
func (instr *Instruction) doSJMP() {
	vars := map[string]Variable{}

	offset := getOffset([]byte{instr.Op, instr.RawOps[0]})

	str := "0x%X"
	val := (instr.Address + instr.ByteLength) + offset

	instr.Jump(str, val)
	//instr.XRef(str, val)

	cadd := VarObjs["cadd"]
	cadd.Value = fmt.Sprintf("0x%X", val)

	cadd.Type = instr.VarTypes[0]
	vars["cadd"] = cadd
	instr.Vars = vars
	instr.Checked = true
}

// SCALL
func (instr *Instruction) doSCALL() {
	vars := map[string]Variable{}

	offset := getOffset([]byte{instr.Op, instr.RawOps[0]})

	cadd := VarObjs["cadd"]

	str := "0x%X"
	val := (instr.Address + instr.ByteLength) + offset

	//if val > 0x180000 {
	//	val = val & 0xFFFFF
	//}

	instr.Call(str, val)

	cadd.Value = fmt.Sprintf(str, val)
	cadd.Type = instr.VarTypes[0]
	vars["cadd"] = cadd
	instr.Vars = vars
	instr.Checked = true
}

// JBC
func (instr *Instruction) doJBC() {
	vars := map[string]Variable{}
	offset := int(instr.RawOps[1])

	breg := VarObjs["breg"]

	val := int(instr.RawOps[0])
	str := "R_%X"
	str = regName(str, val)
	instr.XRef(str, val)

	breg.Value = fmt.Sprintf(str, val)
	breg.Type = instr.VarTypes[0]
	vars["breg"] = breg

	bitno := VarObjs["bitno"]
	bitno.Value = fmt.Sprintf("%d", instr.Op&0x07)
	bitno.Type = instr.VarTypes[1]
	vars["bitno"] = bitno

	cadd := VarObjs["cadd"]

	val = int(instr.Address + instr.ByteLength + offset)
	str = "0x%X"
	str = regName(str, val)
	//instr.XRef(str, val)
	instr.Jump(str, val)

	cadd.Value = fmt.Sprintf(str, val)
	cadd.Type = instr.VarTypes[2]
	vars["cadd"] = cadd

	instr.Vars = vars
	instr.Checked = true
}

// JBS
func (instr *Instruction) doJBS() {
	vars := map[string]Variable{}
	offset := int(instr.RawOps[1])

	breg := VarObjs["breg"]

	val := int(instr.RawOps[0])
	str := "R_%X"
	str = regName(str, val)
	instr.XRef(str, val)

	breg.Value = fmt.Sprintf(str, val)
	breg.Type = instr.VarTypes[0]
	vars["breg"] = breg

	bitno := VarObjs["bitno"]
	bitno.Value = fmt.Sprintf("%d", instr.Op&0x07)
	bitno.Type = instr.VarTypes[1]
	vars["bitno"] = bitno

	cadd := VarObjs["cadd"]

	val = int(instr.Address + instr.ByteLength + offset)
	str = "0x%X"
	str = regName(str, val)
	//instr.XRef(str, val)
	instr.Jump(str, val)

	cadd.Value = fmt.Sprintf(str, val)
	cadd.Type = instr.VarTypes[2]
	vars["cadd"] = cadd

	instr.Vars = vars
	instr.Checked = true
}

// CONDJMP
func (instr *Instruction) doCONDJMP() {
	vars := map[string]Variable{}
	offset := int(instr.RawOps[0])

	str := "0x%X"
	val := instr.Address + instr.ByteLength + offset
	instr.Jump(str, val)
	//instr.XRef(str, val)

	cadd := VarObjs["cadd"]
	cadd.Value = fmt.Sprintf(str, val)
	cadd.Type = instr.VarTypes[0]
	vars["cadd"] = cadd

	instr.Vars = vars
	instr.Checked = true
}

// Fx OpCodes
func (instr *Instruction) doF0() {
	vars := map[string]Variable{}

	b1 := instr.RawOps[0]
	b2 := instr.RawOps[1]
	b3 := instr.RawOps[2]

	offset := int(b3)<<16 | int(b2)<<8 | int(b1)

	val := instr.Address + instr.ByteLength + offset
	val = val & 0x1FFFFF
	str := "0x%X"

	if instr.Mnemonic == "ECALL" {
		instr.Call(str, val)
	} else {
		instr.XRef(str, val)
	}

	cadd := VarObjs["cadd"]
	cadd.Value = fmt.Sprintf(str, val)
	cadd.Type = instr.VarTypes[0]
	vars["cadd"] = cadd

	instr.Vars = vars
	instr.Checked = true
}

// Ex OpCodes
func (instr *Instruction) doE0() {
	vars := map[string]Variable{}
	switch instr.Op {

	case 0xE0, 0xE1:
		// DJNZ, DJNZW
		offset := int(instr.RawOps[1])

		breg := VarObjs["breg"]

		val := int(instr.RawOps[0])
		str := "R_%X"
		str = regName(str, val)
		instr.XRef(str, val)

		breg.Value = fmt.Sprintf(str, val)
		breg.Type = instr.VarTypes[0]
		vars["breg"] = breg

		val = instr.Address + instr.ByteLength + offset
		str = "0x%X"
		instr.Jump(str, val)

		cadd := VarObjs["cadd"]
		cadd.Value = fmt.Sprintf(str, val)
		cadd.Type = instr.VarTypes[1]
		vars["cadd"] = cadd

		instr.Checked = true

	case 0xEA, 0xEB, 0xE8, 0xE9:
		// ELD, ELDB
		switch instr.AddressingMode {

		case "extended-indexed":

			b1 := instr.RawOps[1]
			b2 := instr.RawOps[2]
			b3 := instr.RawOps[3]

			offset := int(b3)<<16 | int(b2)<<8 | int(b1)

			offStr := "0x%06X"
			offStr = regName(offStr, offset)
			instr.XRef(offStr, offset)

			val := int(instr.RawOps[0])
			str := "[R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			treg := VarObjs["treg"]
			treg.Value = fmt.Sprintf(offStr+str+"]", offset, val)
			treg.Type = instr.VarTypes[1]

			_reg := VarObjs[instr.VarStrings[0]]

			val = int(instr.RawOps[4])
			str = "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			_reg.Value = fmt.Sprintf(str, val)
			_reg.Type = instr.VarTypes[0]

			vars["treg"] = treg
			vars[instr.VarStrings[0]] = _reg
			instr.Checked = true

		case "extended-indirect":

			val := int(instr.RawOps[0])
			str := "[R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			treg := VarObjs["treg"]
			treg.Value = fmt.Sprintf(str+"]", val)
			treg.Type = instr.VarTypes[1]

			val = int(instr.RawOps[1])
			str = "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			_reg := VarObjs[instr.VarStrings[0]]
			_reg.Value = fmt.Sprintf(str, val)
			_reg.Type = instr.VarTypes[0]

			vars["treg"] = treg
			vars[instr.VarStrings[0]] = _reg
			instr.Checked = true
		}

	case 0xE6:
		// EJMP

		b1 := instr.RawOps[0]
		b2 := instr.RawOps[1]
		b3 := instr.RawOps[2]

		offset := int(b3)<<16 | int(b2)<<8 | int(b1)

		val := instr.Address + instr.ByteLength + offset
		val = val & 0x1FFFFF

		str := "0x%X"
		str = regName(str, val)
		instr.Jump(str, val)

		cadd := VarObjs["cadd"]
		cadd.Value = fmt.Sprintf(str, val)
		cadd.Type = instr.VarTypes[0]
		vars["cadd"] = cadd

		instr.Checked = true

	case 0xE3:
		// BR / EBR

		val := int(instr.RawOps[0])

		if (instr.RawOps[0] & 0x01) == 0x00 {
			instr.Description = "BRANCH INDIRECT."
			instr.Mnemonic = "BR"
			instr.AddressingMode = "indirect"
			instr.VarStrings = []string{"wreg"}

		} else {
			val &= 0xFE
		}

		vo := VarObjs[instr.VarStrings[0]]
		str := "[R_%02X]"
		str = regName(str, val)
		instr.Jump(str, val)
		instr.XRef(str, val)

		vo.Value = fmt.Sprintf(str, val)
		vo.Type = instr.VarTypes[0]

		vars[instr.VarStrings[0]] = vo

		instr.Checked = true

	case 0xE7, 0xEF:
		// LJMP, LCALL

		b1 := instr.RawOps[0]
		b2 := instr.RawOps[1]

		offset := int(b2)<<8 | int(b1)

		cadd := VarObjs["cadd"]
		str := "0x%X"
		val := int(instr.Address + instr.ByteLength + offset)

		str = regName(str, val)
		if instr.Mnemonic == "LCALL" {
			instr.Call(str, val)
		} else {
			instr.Jump(str, val)
		}

		//instr.XRef(str, val)

		cadd.Value = fmt.Sprintf(str, val)
		cadd.Type = instr.VarTypes[0]
		vars["cadd"] = cadd
		instr.Checked = true

	}
	instr.Vars = vars
	//instr.Checked = true
}

//Cx OpCodes
func (instr *Instruction) doC0() {
	vars := map[string]Variable{}
	instr.Checked = true

	if instr.Op == 0xC1 || instr.Op == 0xC5 || instr.AddressingMode == "direct" {
		//BMOV / CMPL / all other direct
		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {

			val := int(instr.RawOps[b])
			str := "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			vo := VarObjs[varStr]
			vo.Value = fmt.Sprintf(str, val)
			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
			instr.Checked = true
		}

	} else {

		switch instr.AddressingMode {

		case "immediate":
			for i, varStr := range instr.VarStrings {
				vo := VarObjs[varStr]

				val := int(instr.RawOps[1])<<8 | int(instr.RawOps[0])
				str := "#%04X"
				str = regName(str, val)
				instr.XRef(str, val)

				vo.Value = fmt.Sprintf(str, val)
				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
			}
			instr.Checked = true

		case "indirect", "indirect+":
			b := len(instr.RawOps) - 1
			for i, varStr := range instr.VarStrings {
				str := "R_%02X"
				val := int(instr.RawOps[b] & 0xFE)
				if b == 0 {
					str = "[R_%02X]"
					if instr.AutoIncrement == true {
						str = str + "+"
						val = val & 0xFE
					}
				}

				str = regName(str, val)

				vo := VarObjs[varStr]
				vo.Value = fmt.Sprintf(str, val)
				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
				b--
			}
			instr.Checked = true

		case "indexed", "short-indexed":

			// byte offset
			b := len(instr.RawOps) - 1
			for i, varStr := range instr.VarStrings {
				vo := VarObjs[varStr]
				val := int(instr.RawOps[b])
				str := "R_%02X"
				str = regName(str, val)
				instr.XRef(str, val)

				if i+1 == instr.VarCount {

					offset := int(instr.RawOps[b])
					offStr := "0x%02X"
					offStr = regName(offStr, offset)
					instr.XRef(offStr, offset)

					val = int(instr.RawOps[b-1] & 0xFE)
					str = "[R_%02X"

					str = fmt.Sprintf(offStr+str+"]", offset, val)
					str = regName(str, val)
					vo.Value = str
				} else {
					vo.Value = fmt.Sprintf(str, val)
				}

				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
				b--
			}
			instr.Checked = true

		case "long-indexed":

			// word offset
			b := len(instr.RawOps) - 1
			for i, varStr := range instr.VarStrings {
				vo := VarObjs[varStr]
				val := int(instr.RawOps[b])
				str := "R_%02X"

				if i+1 == instr.VarCount {

					offset := int(instr.RawOps[b])<<8 | int(instr.RawOps[b-1])
					offStr := "0x%04X"
					offStr = regName(offStr, offset)
					instr.XRef(offStr, offset)

					val := int(instr.RawOps[b-2] & 0xFE)
					str := "[R_%02X"
					str = regName(str, val)
					instr.XRef(str, val)

					value := fmt.Sprintf(offStr+str+"]", offset, val)
					vo.Value = value
				} else {
					str = regName(str, val)
					vo.Value = fmt.Sprintf(str, val)
					instr.XRef(str, val)
				}

				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
				b--
			}
			instr.Checked = true
		}

	}

	instr.Vars = vars
	//instr.Checked = true

}

// 0x OpCodes
func (instr *Instruction) do00() {
	vars := map[string]Variable{}

	if instr.Op == 0x1F || instr.Op == 0x1D {
		switch instr.AddressingMode {

		case "extended-indexed":
			// ETSB

			b1 := byte(instr.RawOps[1])
			b2 := byte(instr.RawOps[2])
			b3 := byte(instr.RawOps[3])

			offset := int(b3)<<16 | int(b2)<<8 | int(b1)

			offStr := "0x%06X"
			offStr = regName(offStr, offset)
			instr.XRef(offStr, offset)

			val := int(instr.RawOps[0])
			str := "[R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			treg := VarObjs["treg"]
			treg.Value = fmt.Sprintf(offStr+str+"]", offset, val)
			treg.Type = instr.VarTypes[1]

			val = int(instr.RawOps[4])
			str = "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			_reg := VarObjs[instr.VarStrings[0]]
			_reg.Value = fmt.Sprintf(str, val)
			_reg.Type = instr.VarTypes[0]

			vars["treg"] = treg
			vars[instr.VarStrings[0]] = _reg
			instr.Vars = vars
			instr.Checked = true

		case "extended-indirect":

			val := int(instr.RawOps[0])
			str := "[R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			treg := VarObjs["treg"]
			treg.Value = fmt.Sprintf(str+"]", val)
			treg.Type = instr.VarTypes[1]

			val = int(instr.RawOps[1])
			str = "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			_reg := VarObjs[instr.VarStrings[0]]
			_reg.Value = fmt.Sprintf(str, val)
			_reg.Type = instr.VarTypes[0]

			vars["treg"] = treg
			vars[instr.VarStrings[0]] = _reg
			instr.Vars = vars
			instr.Checked = true
		}

	} else {

		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {
			vo := VarObjs[varStr]
			val := int(instr.RawOps[b])
			str := "R_%02X"
			str = regName(str, val)
			instr.XRef(str, val)

			if (instr.Op&0x08 == 0x08) && b == 0 && instr.Op != 0x0F && (instr.RawOps[0] < 0x10) {
				str = "#%02X"
			}

			vo.Value = fmt.Sprintf(str, val)

			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
		}

		instr.Vars = vars
		instr.Checked = true

	}
}

// Middle OpCodes ()
func (instr *Instruction) doMIDDLE() {
	vars := map[string]Variable{}

	switch instr.AddressingMode {

	case "direct":
		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {
			str := "R_%02X"
			val := int(instr.RawOps[b])
			str = regName(str, val)
			instr.XRef(str, val)
			vo := VarObjs[varStr]
			vo.Value = fmt.Sprintf(str, val)
			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
		}
		instr.Checked = true

	case "immediate":
		if instr.Op&0x10 == 0x10 {
			// byte const
			b := len(instr.RawOps) - 1
			for i, varStr := range instr.VarStrings {
				val := int(instr.RawOps[b])
				str := "R_%02X"
				str = regName(str, val)
				if b == 0 {
					str = "#%02X"
				} else {
					instr.XRef(str, val)
				}
				vo := VarObjs[varStr]
				vo.Value = fmt.Sprintf(str, val)
				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
				b--
			}

		} else {
			// word constant
			b := len(instr.RawOps) - 1
			for i, varStr := range instr.VarStrings {
				val := int(instr.RawOps[b])
				str := "R_%02X"
				str = regName(str, val)
				if b == 1 {
					str = "#%04X"
					val = int(instr.RawOps[1])<<8 | int(instr.RawOps[0])
				} else {
					instr.XRef(str, val)
				}

				vo := VarObjs[varStr]
				vo.Value = fmt.Sprintf(str, val)
				vo.Type = instr.VarTypes[i]
				vars[varStr] = vo
				b--
			}

		}
		instr.Checked = true

	case "indirect", "indirect+":
		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {
			str := "R_%02X"
			val := int(instr.RawOps[b] & 0xFE)
			str = regName(str, val)
			if b == 0 {
				str = "[R_%02X"
				if instr.AutoIncrement == true {
					str = str + "+"
					val = val & 0xFE
				}
				str = regName(str, val) + "]"
			}
			instr.XRef(str, val)

			vo := VarObjs[varStr]
			vo.Value = fmt.Sprintf(str, val)
			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
		}
		instr.Checked = true

	case "indexed", "short-indexed":

		// byte offset
		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {
			vo := VarObjs[varStr]
			str := "R_%02X"
			val := int(instr.RawOps[b])
			str = regName(str, val)
			instr.XRef(str, val)

			if i+1 == instr.VarCount {

				offset := int(instr.RawOps[b])
				offStr := "0x%02X"
				offStr = regName(offStr, offset)
				instr.XRef(offStr, offset)

				val := int(instr.RawOps[b-1] & 0xFE)
				str := "[R_%02X"
				str = regName(str, val)
				instr.XRef(str, val)

				value := fmt.Sprintf(offStr+str+"]", offset, val)
				vo.Value = value
			} else {
				vo.Value = fmt.Sprintf(str, val)
			}

			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
		}
		instr.Checked = true

	case "long-indexed":

		// word offset
		b := len(instr.RawOps) - 1
		for i, varStr := range instr.VarStrings {
			vo := VarObjs[varStr]
			val := int(instr.RawOps[b])
			str := "R_%02X"

			if i+1 == instr.VarCount {

				offset := int(instr.RawOps[b])<<8 | int(instr.RawOps[b-1])
				offStr := "0x%04X"
				offStr = regName(offStr, offset)
				instr.XRef(offStr, offset)

				val := int(instr.RawOps[b-2] & 0xFE)
				str := "[R_%02X"
				str = regName(str, val)
				instr.XRef(str, val)

				value := fmt.Sprintf(offStr+str+"]", offset, val)
				vo.Value = value
			} else {
				str = regName(str, val)
				vo.Value = fmt.Sprintf(str, val)
				instr.XRef(str, val)
			}

			vo.Type = instr.VarTypes[i]
			vars[varStr] = vo
			b--
		}
		instr.Checked = true

	}

	instr.Vars = vars
	//instr.Checked = true

}

var unsignedInstructions = map[byte]Instruction{
	0x00: Instruction{
		Mnemonic:        "SKIP",
		ByteLength:      2,
		VarCount:        0,
		VarTypes:        []string{"ByteReg"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "TWO BYTE NO-OPERATION.",
		LongDescription: "Does nothing. Control passes to the next sequentia instruction. This is actually a two-byte NOP i which the second byte can be any value an is simply ignored.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          true,
		Signed:          false,
		Reserved:        false,
	},
	0x01: Instruction{
		Mnemonic:        "CLR",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"wreg"},
		AddressingMode:  "direct",
		Description:     "CLEAR WORD.",
		LongDescription: "Clears the value of the operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x02: Instruction{
		Mnemonic:        "NOT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"wreg"},
		AddressingMode:  "direct",
		Description:     "COMPLEMENT WORD.",
		LongDescription: "Complements the value of the word operand (replaces each “1” with a “0” and each “0” with a “1”).",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x03: Instruction{
		Mnemonic:        "NEG",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"wreg"},
		AddressingMode:  "direct",
		Description:     "NEGATE INTEGER.",
		LongDescription: "Negates the value of the integer operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x04: Instruction{
		Mnemonic:        "XCH",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "EXCHANGE WORD.",
		LongDescription: "Exchanges the value of the source word operand with that of the destination word operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x05: Instruction{
		Mnemonic:        "DEC",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "DECREMENT WORD.",
		LongDescription: "Decrements the value of the operand by one.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x06: Instruction{
		Mnemonic:        "EXT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"lreg"},
		AddressingMode:  "direct",
		Description:     "SIGN-EXTEND INTEGER INTO LONGINTEGER.",
		LongDescription: "Sign-extends the low-order word of the operand throughout the high-order word of the operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x07: Instruction{
		Mnemonic:        "INC",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"wreg"},
		AddressingMode:  "direct",
		Description:     "INCREMENT WORD.",
		LongDescription: "Increments the value of the word operand by 1.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x08: Instruction{
		Mnemonic:        "SHR",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"wreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "LOGICAL RIGHT SHIFT WORD.",
		LongDescription: "Shifts the destination word operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The left bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x09: Instruction{
		Mnemonic:        "SHL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"wreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "SHIFT WORD LEFT.",
		LongDescription: "Shifts the destination word operand to the left as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The right bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0A: Instruction{
		Mnemonic:        "SHRA",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"wreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "ARITHMETIC RIGHT SHIFT WORD.",
		LongDescription: "Shifts the destination word operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. If the original high order bit value was “0,” zeros are shifted in. If the value was “1,” ones are shifted in. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0B: Instruction{
		Mnemonic:        "XCH",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "EXCHANGE WORD",
		LongDescription: "Exchanges the value of the source word operand with that of the destination word operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0C: Instruction{
		Mnemonic:        "SHRL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"lreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "LOGICAL RIGHT SHIFT DOUBLE-WORD.",
		LongDescription: "Shifts the destination double-word operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The left bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0D: Instruction{
		Mnemonic:        "SHLL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"lreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "SHIFT DOUBLE-WORD LEFT.",
		LongDescription: "Shifts the destination double-word operand to the left as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The right bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0E: Instruction{
		Mnemonic:        "SHRAL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"lreg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "ARITHMETIC RIGHT SHIFT DOUBLEWORD.",
		LongDescription: "Shifts the destination double-word operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. If the original high order bit value was “0,” zeros are shifted in. If the value was “1,” ones are shifted in.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x0F: Instruction{
		Mnemonic:        "NORML",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"lreg", "breg"},
		AddressingMode:  "direct",
		Description:     "NORMALIZE LONG-INTEGER.",
		LongDescription: "Normalizes the source (leftmost) long-integer operand. (That is, it shifts the operand to the left until its most significant bit is “1” or until it has performed 31 shifts). If the most significant bit is still “0” after 31 shifts, the instruction stops the process and sets the zero flag. The instruction stores the actual number of shifts performed in the destination (rightmost) operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x10: Instruction{
		Mnemonic:   "Reserved",
		ByteLength: 1,
		Reserved:   true,
	},
	0x11: Instruction{
		Mnemonic:        "CLRB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "CLEAR BYTE.",
		LongDescription: "Clears the value of the operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x12: Instruction{
		Mnemonic:        "NOTB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "COMPLEMENT BYTE.",
		LongDescription: "Complements the value of the byte operand (replaces each “1” with a “0” and each “0” with a “1”).",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x13: Instruction{
		Mnemonic:        "NEGB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "NEGATE SHORT-INTEGER.",
		LongDescription: "Negates the value of the short-integer operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x14: Instruction{
		Mnemonic:        "XCHB",
		ByteLength:      3, // Changed? was 2
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "EXCHANGE BYTE.",
		LongDescription: "Exchanges the value of the source byte operand with that of the destination byte operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x15: Instruction{
		Mnemonic:        "DECB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "DECREMENT BYTE.",
		LongDescription: "Decrements the value of the operand by one.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x16: Instruction{
		Mnemonic:        "EXTB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"wreg"},
		AddressingMode:  "direct",
		Description:     "SIGN-EXTEND SHORT-INTEGER INTO INTEGER.",
		LongDescription: "Sign-extends the low-order byte of the operand throughout the high-order byte of the operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x17: Instruction{
		Mnemonic:        "INCB",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"breg"},
		AddressingMode:  "direct",
		Description:     "INCREMENT BYTE.",
		LongDescription: "Increments the value of the byte operand by 1.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x18: Instruction{
		Mnemonic:        "SHRB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"breg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "LOGICAL RIGHT SHIFT BYTE.",
		LongDescription: "Shifts the destination byte operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The left bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x19: Instruction{
		Mnemonic:        "SHLB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"breg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "SHIFT BYTE LEFT.",
		LongDescription: "Shifts the destination byte operand to the left as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. The right bits of the result are filled with zeros. The last bit shifted out is saved in the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1A: Instruction{
		Mnemonic:        "SHRAB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"breg", "breg/#count"},
		AddressingMode:  "direct",
		Description:     "",
		LongDescription: "",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1B: Instruction{
		Mnemonic:        "XCHB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "COUNT"},
		VarStrings:      []string{"breg", "breg/#count"},
		AddressingMode:  "indexed",
		Description:     "ARITHMETIC RIGHT SHIFT BYTE.",
		LongDescription: "Shifts the destination byte operand to the right as many times as specified by the count operand. The count may be specified either as an immediate value in the range of 0 to 15 (0FH), inclusive, or as the content of any register (10–0FFH) with a value in the range of 0 to 31 (1FH), inclusive. If the original high order bit value was “0,” zeros are shifted in. If the value was “1,” ones are shifted in. The last bit shifted out is saved in the carry flag.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1C: Instruction{
		Mnemonic:        "EST",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"wreg", "treg"},
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED STORE WORD.",
		LongDescription: "Stores the value of the source (leftmost) word operand into the destination (rightmost) operand. This instruction allows you to move data from the lower register file to anywhere in the 16-Mbyte address space.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1D: Instruction{
		Mnemonic:        "EST",
		ByteLength:      6,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"wreg", "treg"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED STORE WORD.",
		LongDescription: "Stores the value of the source (leftmost) word operand into the destination (rightmost) operand. This instruction allows you to move data from the lower register file to anywhere in the 16-Mbyte address space.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1E: Instruction{
		Mnemonic:        "ESTB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"breg", "treg"},
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED STORE BYTE.",
		LongDescription: "Stores the value of the source (leftmost) byte operand into the destination (rightmost) operand. This instruction allows you to move data from the lower register file to anywhere in the 16- Mbyte address space.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x1F: Instruction{
		Mnemonic:        "ESTB",
		ByteLength:      6,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"breg", "treg"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED STORE BYTE.",
		LongDescription: "Stores the value of the source (leftmost) byte operand into the destination (rightmost) operand. This instruction allows you to move data from the lower register file to anywhere in the 16- Mbyte address space.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x20: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x21: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x22: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x23: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x24: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x25: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x26: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x27: Instruction{
		Mnemonic:        "SJMP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –1024 to +1023, inclusive.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x28: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x29: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2A: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2B: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2C: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2D: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2E: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x2F: Instruction{
		Mnemonic:        "SCALL",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		VariableLength:  false,
		AutoIncrement:   false,
		Description:     "SHORT CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –1024 to +1023.",
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x30: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x31: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x32: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x33: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x34: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x35: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x36: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x37: Instruction{
		Mnemonic:        "JBC",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS CLEAR.",
		LongDescription: "Tests the specified bit. If the bit is set, control passes to the next sequential instruction. If the bit is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x38: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x39: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3A: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3B: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3C: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3D: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3E: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x3F: Instruction{
		Mnemonic:        "JBS",
		ByteLength:      3,
		VarCount:        3,
		VarTypes:        []string{"BYTEREG", "BITNO", "ADDR"},
		VarStrings:      []string{"breg", "bitno", "cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF BIT IS SET.",
		LongDescription: "Tests the specified bit. If the bit is clear, control passes to the next sequential instruction. If the bit is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x40: Instruction{
		Mnemonic:        "AND",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the two source word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x41: Instruction{
		Mnemonic:        "AND",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the two source word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x42: Instruction{
		Mnemonic:        "AND",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the two source word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x43: Instruction{
		Mnemonic:        "AND",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the two source word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x44: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "direct",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the two source word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x45: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the two source word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x46: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the two source word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x47: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the two source word operands and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x48: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the first source word operand from the second, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x49: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the first source word operand from the second, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4A: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the first source word operand from the second, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4B: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dwreg", "Swreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the first source word operand from the second, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4C: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the two source word operands, using unsigned arithmetic, and stores the 32-bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4D: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the two source word operands, using unsigned arithmetic, and stores the 32-bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4E: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the two source word operands, using unsigned arithmetic, and stores the 32-bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4F: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the two source word operands, using unsigned arithmetic, and stores the 32-bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x50: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the two source byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x51: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the two source byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x52: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the two source byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x53: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the two source byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x54: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "direct",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the two source byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x55: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the two source byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x56: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the two source byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x57: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the two source byte operands and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x58: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the second source byte operand from the first, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x59: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the second source byte operand from the first, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5A: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the second source byte operand from the first, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5B: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"Dbreg", "Sbreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the second source byte operand from the first, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5C: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY BYTES, UNSIGNED.",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5D: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY BYTES, UNSIGNED.",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5E: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY BYTES, UNSIGNED.",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5F: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY BYTES, UNSIGNED.",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x60: Instruction{
		Mnemonic:        "AND",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the source and destination word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x61: Instruction{
		Mnemonic:        "AND",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the source and destination word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x62: Instruction{
		Mnemonic:        "AND",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the source and destination word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x63: Instruction{
		Mnemonic:        "AND",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL AND WORDS.",
		LongDescription: "ANDs the source and destination word operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x64: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the source and destination word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x65: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the source and destination word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x66: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the source and destination word operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x67: Instruction{
		Mnemonic:        "ADD",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "ADD WORDS.",
		LongDescription: "Adds the source and destination word operands and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x68: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x69: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6A: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6B: Instruction{
		Mnemonic:        "SUB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6C: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the source and destination word operands, using unsigned arithmetic, and stores the 32- bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6D: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the source and destination word operands, using unsigned arithmetic, and stores the 32- bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6E: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the source and destination word operands, using unsigned arithmetic, and stores the 32- bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6F: Instruction{
		Mnemonic:        "MULU",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY WORDS, UNSIGNED.",
		LongDescription: "Multiplies the source and destination word operands, using unsigned arithmetic, and stores the 32- bit result into the destination double-word operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x70: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the source and destination byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x71: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the source and destination byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",

		VariableLength: false,
		AutoIncrement:  false,
		Flags:          Flags{},
		Ignore:         false,
		Signed:         false,
		Reserved:       false,
	},
	0x72: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the source and destination byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",

		VariableLength: false,
		AutoIncrement:  false,
		Flags:          Flags{},
		Ignore:         false,
		Signed:         false,
		Reserved:       false,
	},
	0x73: Instruction{
		Mnemonic:        "ANDB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL AND BYTES.",
		LongDescription: "ANDs the source and destination byte operands and stores the result into the destination operand. The result has ones in only the bit positions in which both operands had a “1” and zeros in all other bit positions.",

		VariableLength: true,
		AutoIncrement:  false,
		Flags:          Flags{},
		Ignore:         false,
		Signed:         false,
		Reserved:       false,
	},
	0x74: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the source and destination byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x75: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the source and destination byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x76: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the source and destination byte operands and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x77: Instruction{
		Mnemonic:        "ADDB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "ADD BYTES.",
		LongDescription: "Adds the source and destination byte operands and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x78: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x79: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7A: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7B: Instruction{
		Mnemonic:        "SUBB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand, stores the result in the destination operand, and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7C: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY BYTES",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7D: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY BYTES",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7E: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY BYTES",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7F: Instruction{
		Mnemonic:        "MULUB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY BYTES",
		LongDescription: "Multiplies the source and destination operands, using unsigned arithmetic, and stores the word result into the destination operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x80: Instruction{
		Mnemonic:        "OR",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL OR WORDS.",
		LongDescription: "ORs the source word operand with the destination word operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x81: Instruction{
		Mnemonic:        "OR",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL OR WORDS.",
		LongDescription: "ORs the source word operand with the destination word operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x82: Instruction{
		Mnemonic:        "OR",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL OR WORDS.",
		LongDescription: "ORs the source word operand with the destination word operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x83: Instruction{
		Mnemonic:        "OR",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL OR WORDS.",
		LongDescription: "ORs the source word operand with the destination word operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x84: Instruction{
		Mnemonic:        "XOR",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL EXCLUSIVE-OR WORDS",
		LongDescription: "XORs the source word operand with the destination word operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x85: Instruction{
		Mnemonic:        "XOR",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL EXCLUSIVE-OR WORDS",
		LongDescription: "XORs the source word operand with the destination word operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x86: Instruction{
		Mnemonic:        "XOR",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL EXCLUSIVE-OR WORDS",
		LongDescription: "XORs the source word operand with the destination word operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x87: Instruction{
		Mnemonic:        "XOR",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL EXCLUSIVE-OR WORDS",
		LongDescription: "XORs the source word operand with the destination word operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x88: Instruction{
		Mnemonic:        "CMP",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "COMPARE WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x89: Instruction{
		Mnemonic:        "CMP",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "COMPARE WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8A: Instruction{
		Mnemonic:        "CMP",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "COMPARE WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8B: Instruction{
		Mnemonic:        "CMP",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "COMPARE WORDS.",
		LongDescription: "Subtracts the source word operand from the destination word operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8C: Instruction{
		Mnemonic:        "DIVU",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "direct",
		Description:     "DIVIDE WORDS, UNSIGNED.",
		LongDescription: "Divides the contents of the destination double-word operand by the contents of the source word operand, using unsigned arithmetic. It stores the quotient into the low-order word (i.e., the word with the lower address) of the destination operand and the remainder into the high-order word. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8D: Instruction{
		Mnemonic:        "DIVU",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "DIVIDE WORDS, UNSIGNED.",
		LongDescription: "Divides the contents of the destination double-word operand by the contents of the source word operand, using unsigned arithmetic. It stores the quotient into the low-order word (i.e., the word with the lower address) of the destination operand and the remainder into the high-order word. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8E: Instruction{
		Mnemonic:        "DIVU",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "DIVIDE WORDS, UNSIGNED.",
		LongDescription: "Divides the contents of the destination double-word operand by the contents of the source word operand, using unsigned arithmetic. It stores the quotient into the low-order word (i.e., the word with the lower address) of the destination operand and the remainder into the high-order word. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8F: Instruction{
		Mnemonic:        "DIVU",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "DIVIDE WORDS, UNSIGNED.",
		LongDescription: "Divides the contents of the destination double-word operand by the contents of the source word operand, using unsigned arithmetic. It stores the quotient into the low-order word (i.e., the word with the lower address) of the destination operand and the remainder into the high-order word. The following two statements are performed concurrently.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x90: Instruction{
		Mnemonic:        "ORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL OR BYTES.",
		LongDescription: "ORs the source byte operand with the destination byte operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x91: Instruction{
		Mnemonic:        "ORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL OR BYTES.",
		LongDescription: "ORs the source byte operand with the destination byte operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x92: Instruction{
		Mnemonic:        "ORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL OR BYTES.",
		LongDescription: "ORs the source byte operand with the destination byte operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x93: Instruction{
		Mnemonic:        "ORB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL OR BYTES.",
		LongDescription: "ORs the source byte operand with the destination byte operand and replaces the original destination operand with the result. The result has a “1” in each bit position in which either the source or destination operand had a “1”.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x94: Instruction{
		Mnemonic:        "XORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOGICAL EXCLUSIVE-OR BYTES.",
		LongDescription: "XORs the source byte operand with the destination byte operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x95: Instruction{
		Mnemonic:        "XORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOGICAL EXCLUSIVE-OR BYTES.",
		LongDescription: "XORs the source byte operand with the destination byte operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x96: Instruction{
		Mnemonic:        "XORB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOGICAL EXCLUSIVE-OR BYTES.",
		LongDescription: "XORs the source byte operand with the destination byte operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x97: Instruction{
		Mnemonic:        "XORB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOGICAL EXCLUSIVE-OR BYTES.",
		LongDescription: "XORs the source byte operand with the destination byte operand and stores the result in the destination operand. The result has ones in the bit positions in which either operand (but not both) had a “1” and zeros in all other bit positions.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x98: Instruction{
		Mnemonic:        "CMPB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "COMPARE BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x99: Instruction{
		Mnemonic:        "CMPB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "COMPARE BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9A: Instruction{
		Mnemonic:        "CMPB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "COMPARE BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9B: Instruction{
		Mnemonic:        "CMPB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "COMPARE BYTES.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9C: Instruction{
		Mnemonic:        "DIVUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "DIVIDE BYTES, UNSIGNED.",
		LongDescription: "This instruction divides the contents of the destination word operand by the contents of the source byte operand, using unsigned arithmetic. It stores the quotient into the low-order byte (i.e., the byte with the lower address) of the destination operand and the remainder into the high-order byte. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9D: Instruction{
		Mnemonic:        "DIVUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "DIVIDE BYTES, UNSIGNED.",
		LongDescription: "This instruction divides the contents of the destination word operand by the contents of the source byte operand, using unsigned arithmetic. It stores the quotient into the low-order byte (i.e., the byte with the lower address) of the destination operand and the remainder into the high-order byte. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9E: Instruction{
		Mnemonic:        "DIVUB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "DIVIDE BYTES, UNSIGNED.",
		LongDescription: "This instruction divides the contents of the destination word operand by the contents of the source byte operand, using unsigned arithmetic. It stores the quotient into the low-order byte (i.e., the byte with the lower address) of the destination operand and the remainder into the high-order byte. The following two statements are performed concurrently.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9F: Instruction{
		Mnemonic:        "DIVUB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "DIVIDE BYTES, UNSIGNED.",
		LongDescription: "This instruction divides the contents of the destination word operand by the contents of the source byte operand, using unsigned arithmetic. It stores the quotient into the low-order byte (i.e., the byte with the lower address) of the destination operand and the remainder into the high-order byte. The following two statements are performed concurrently.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA0: Instruction{
		Mnemonic:        "LD",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA1: Instruction{
		Mnemonic:        "LD",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA2: Instruction{
		Mnemonic:        "LD",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA3: Instruction{
		Mnemonic:        "LD",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA4: Instruction{
		Mnemonic:        "ADDC",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "ADD WORDS WITH CARRY.",
		LongDescription: "Adds the source and destination word operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA5: Instruction{
		Mnemonic:        "ADDC",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "ADD WORDS WITH CARRY.",
		LongDescription: "Adds the source and destination word operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA6: Instruction{
		Mnemonic:        "ADDC",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "ADD WORDS WITH CARRY.",
		LongDescription: "Adds the source and destination word operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA7: Instruction{
		Mnemonic:        "ADDC",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "ADD WORDS WITH CARRY.",
		LongDescription: "Adds the source and destination word operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA8: Instruction{
		Mnemonic:        "SUBC",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT WORDS WITH BORROW.",
		LongDescription: "Subtracts the source word operand from the destination word operand. If the carry flag was clear, SUBC subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xA9: Instruction{
		Mnemonic:        "SUBC",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT WORDS WITH BORROW.",
		LongDescription: "Subtracts the source word operand from the destination word operand. If the carry flag was clear, SUBC subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAA: Instruction{
		Mnemonic:        "SUBC",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT WORDS WITH BORROW.",
		LongDescription: "Subtracts the source word operand from the destination word operand. If the carry flag was clear, SUBC subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAB: Instruction{
		Mnemonic:        "SUBC",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT WORDS WITH BORROW.",
		LongDescription: "Subtracts the source word operand from the destination word operand. If the carry flag was clear, SUBC subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAC: Instruction{
		Mnemonic:        "LDBZE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOAD BYTE ZERO-EXTENDED.",
		LongDescription: "Zeroextends the value of the source byte operand and loads it into the destination word operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAD: Instruction{
		Mnemonic:        "LDBZE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOAD BYTE ZERO-EXTENDED.",
		LongDescription: "Zeroextends the value of the source byte operand and loads it into the destination word operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAE: Instruction{
		Mnemonic:        "LDBZE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOAD BYTE ZERO-EXTENDED.",
		LongDescription: "Zeroextends the value of the source byte operand and loads it into the destination word operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xAF: Instruction{
		Mnemonic:        "LDBZE",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOAD BYTE ZERO-EXTENDED.",
		LongDescription: "Zeroextends the value of the source byte operand and loads it into the destination word operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB0: Instruction{
		Mnemonic:        "LDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB1: Instruction{
		Mnemonic:        "LDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB2: Instruction{
		Mnemonic:        "LDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB3: Instruction{
		Mnemonic:        "LDB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB4: Instruction{
		Mnemonic:        "ADDCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "ADD BYTES WITH CARRY.",
		LongDescription: "Adds the source and destination byte operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB5: Instruction{
		Mnemonic:        "ADDCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "ADD BYTES WITH CARRY.",
		LongDescription: "Adds the source and destination byte operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB6: Instruction{
		Mnemonic:        "ADDCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "ADD BYTES WITH CARRY.",
		LongDescription: "Adds the source and destination byte operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB7: Instruction{
		Mnemonic:        "ADDCB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "ADD BYTES WITH CARRY.",
		LongDescription: "Adds the source and destination byte operands and the carry flag (0 or 1) and stores the sum into the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB8: Instruction{
		Mnemonic:        "SUBCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "SUBTRACT BYTES WITH BORROW.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. If the carry flag was clear, SUBCB subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xB9: Instruction{
		Mnemonic:        "SUBCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "SUBTRACT BYTES WITH BORROW.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. If the carry flag was clear, SUBCB subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBA: Instruction{
		Mnemonic:        "SUBCB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "SUBTRACT BYTES WITH BORROW.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. If the carry flag was clear, SUBCB subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBB: Instruction{
		Mnemonic:        "SUBCB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "SUBTRACT BYTES WITH BORROW.",
		LongDescription: "Subtracts the source byte operand from the destination byte operand. If the carry flag was clear, SUBCB subtracts 1 from the result. It stores the result in the destination operand and sets the carry flag as the complement of borrow.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBC: Instruction{
		Mnemonic:        "LDBSE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "LOAD BYTE SIGN-EXTENDED.",
		LongDescription: "Signextends the value of the source shortinteger operand and loads it into the destination integer operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBD: Instruction{
		Mnemonic:        "LDBSE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "LOAD BYTE SIGN-EXTENDED.",
		LongDescription: "Signextends the value of the source shortinteger operand and loads it into the destination integer operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBE: Instruction{
		Mnemonic:        "LDBSE",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "LOAD BYTE SIGN-EXTENDED.",
		LongDescription: "Signextends the value of the source shortinteger operand and loads it into the destination integer operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xBF: Instruction{
		Mnemonic:        "LDBSE",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "LOAD BYTE SIGN-EXTENDED.",
		LongDescription: "Signextends the value of the source shortinteger operand and loads it into the destination integer operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC0: Instruction{
		Mnemonic:        "ST",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "STORE WORD.",
		LongDescription: "Stores the value of the source (leftmost) word operand into the destination (rightmost) operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC1: Instruction{
		Mnemonic:        "BMOV",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"PTRS", "CNTREG"},
		VarStrings:      []string{"lreg", "wreg"},
		AddressingMode:  "",
		Description:     "BLOCK MOVE.",
		LongDescription: "Moves a block of word data from one location in memory to another. The source and destination addresses are calculated using indirect addressing with autoincrement.\n A long register (PTRS) addresses the source and destination pointers, which are stored in adjacent word registers. The source pointer (SRCPTR) is the low word and the destination pointer (DSTPTR) is the high word of PTRS.\n A word register (CNTREG) specifies thenumber of transfers. CNTREG must reside in the lower register file; it cannot be windowed. The blocks of word data can be located anywhere in page 00H, but should not overlap. Because the source (SRCPTR) and destination (DSTPTR) pointers are 16 bits wide, this instruction uses nonextended data moves. It cannot operate across page boundaries.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC2: Instruction{
		Mnemonic:        "ST",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "STORE WORD.",
		LongDescription: "Stores the value of the source (leftmost) word operand into the destination (rightmost) operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC3: Instruction{
		Mnemonic:        "ST",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "STORE WORD.",
		LongDescription: "Stores the value of the source (leftmost) word operand into the destination (rightmost) operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC4: Instruction{
		Mnemonic:        "STB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "direct",
		Description:     "STORE BYTE.",
		LongDescription: "Stores the value of the source (leftmost) byte operand into the destination (rightmost) operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC5: Instruction{
		Mnemonic:        "CMPL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"Dlreg", "Slreg"},
		AddressingMode:  "direct",
		Description:     "COMPARE LONG.",
		LongDescription: "Compares the magnitudes of two double-word (long) operands. The operands are specified using the direct addressing mode. The flags are altered, but the operands remain unaffected. If a borrow occurs, the carry flag is cleared; otherwise, it is set.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC6: Instruction{
		Mnemonic:        "STB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "STORE BYTE.",
		LongDescription: "Stores the value of the source (leftmost) byte operand into the destination (rightmost) operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC7: Instruction{
		Mnemonic:        "STB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"SRC", "DEST"},
		VarStrings:      []string{"breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "STORE BYTE.",
		LongDescription: "Stores the value of the source (leftmost) byte operand into the destination (rightmost) operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC8: Instruction{
		Mnemonic:        "PUSH",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"SRC"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "direct",
		Description:     "PUSH WORD.",
		LongDescription: "Pushes the word operand onto the stack.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xC9: Instruction{
		Mnemonic:        "PUSH",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"SRC"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "immediate",
		Description:     "PUSH WORD.",
		LongDescription: "Pushes the word operand onto the stack.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCA: Instruction{
		Mnemonic:        "PUSH",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"SRC"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "indirect",
		Description:     "PUSH WORD.",
		LongDescription: "Pushes the word operand onto the stack.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCB: Instruction{
		Mnemonic:        "PUSH",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"SRC"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "indexed",
		Description:     "PUSH WORD.",
		LongDescription: "Pushes the word operand onto the stack.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCC: Instruction{
		Mnemonic:        "POP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "direct",
		Description:     "POP WORD.",
		LongDescription: "Pops the word on top of the stack and places it at the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCD: Instruction{
		Mnemonic:        "BMOVI",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"PTRS", "CNTREG"},
		VarStrings:      []string{"lreg", "wreg"},
		AddressingMode:  "indirect",
		Description:     "INTERRUPTIBLE BLOCK MOVE.",
		LongDescription: "Moves a block of word data from one location in memory to another. The instruction is identical to BMOV, except that BMOVI is interruptible. The source and destination addresses are calculated using indirect addressing with autoincrement.\n A long register (PTRS) addresses the source and destination pointers, which are stored in adjacent word registers. The source pointer (SRCPTR) is the low word and the destination pointer (DSTPTR) is the high word of PTRS.\n A word register (CNTREG) specifies the number of transfers. CNTREG must reside in the lower register file; it cannot be windowed. The blocks of word data can be located anywhere in page 00H, but should not overlap. Because the source (SRCPTR) and destination (DSTPTR) pointers are 16 bits wide, this instruction uses nonexteneded data moves. It cannot operate across page boundaries. (If you need to cross page boundaries, use the EBMOVI instruction.)",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCE: Instruction{
		Mnemonic:        "POP",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "indirect",
		Description:     "POP WORD.",
		LongDescription: "Pops the word on top of the stack and places it at the destination operand.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xCF: Instruction{
		Mnemonic:        "POP",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"DEST"},
		VarStrings:      []string{"waop"},
		AddressingMode:  "indexed",
		Description:     "POP WORD.",
		LongDescription: "Pops the word on top of the stack and places it at the destination operand.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD0: Instruction{
		Mnemonic:        "JNST",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF STICKY BIT FLAG IS CLEAR.",
		LongDescription: "Tests the sticky bit flag. If the flag is set, control passes to the next sequential instruction. If the sticky bit flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD1: Instruction{
		Mnemonic:        "JNH",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF NOT HIGHER (UNSIGNED).",
		LongDescription: "Tests both the zero flag and the carry flag. If the carry flag is set and the zero flag is clear, control passes to the next sequential instruction. If either the carry flag is clear or the zero flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD2: Instruction{
		Mnemonic:        "JGT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF SIGNED GREATER THAN.",
		LongDescription: "Tests both the zero flag and the negative flag. If either flag is set, control passes to the next sequential instruction. If both flags are clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD3: Instruction{
		Mnemonic:        "JNC",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF CARRY FLAG IS CLEAR.",
		LongDescription: "Tests the carry flag. If the flag is set, control passes to the next sequential instruction. If the carry flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD4: Instruction{
		Mnemonic:        "JNVT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF OVERFLOW-TRAP FLAG IS CLEAR.",
		LongDescription: "Tests the overflow-trap flag. If the flag is set, this instruction clears the flag and passes control to the next sequential instruction. If the overflow-trap flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD5: Instruction{
		Mnemonic:        "JNV",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF OVERFLOW FLAG IS CLEAR.",
		LongDescription: "Tests the overflow flag. If the flag is set, control passes to the next sequential instruction. If the overflow flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD6: Instruction{
		Mnemonic:        "JGE",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF SIGNED GREATER THAN OR EQUAL.",
		LongDescription: "Tests the negative flag. If the negative flag is set, control passes to the next sequential instruction. If the negative flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD7: Instruction{
		Mnemonic:        "JNE",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF NOT EQUAL.",
		LongDescription: "Tests the zero flag. If the flag is set, control passes to the next sequential instruction. If the zero flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD8: Instruction{
		Mnemonic:        "JST",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF STICKY BIT FLAG IS SET.",
		LongDescription: "Tests the sticky bit flag. If the flag is clear, control passes to the next sequential instruction. If the sticky bit flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xD9: Instruction{
		Mnemonic:        "JH",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF HIGHER (UNSIGNED).",
		LongDescription: "Tests both the zero flag and the carry flag. If either the carry flag is clear or the zero flag is set, control passes to the next sequential instruction. If the carry flag is set and the zero flag is clear, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDA: Instruction{
		Mnemonic:        "JLE",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF SIGNED LESS THAN OR EQUAL.",
		LongDescription: "Tests both the negative flag and the zero flag. If both flags are clear, control passes to the next sequential instruction. If either flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDB: Instruction{
		Mnemonic:        "JC",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF CARRY FLAG IS SET.",
		LongDescription: "Tests the carry flag. If the carry flag is clear, control passes to the next sequential instruction. If the carry flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDC: Instruction{
		Mnemonic:        "JVT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF OVERFLOW-TRAP FLAG IS SET.",
		LongDescription: "Tests the overflow-trap flag. If the flag is clear, control passes to the next sequential instruction. If the overflow-trap flag is set, this instruction clears the flag and adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDD: Instruction{
		Mnemonic:        "JV",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF OVERFLOW FLAG IS SET.",
		LongDescription: "Tests the overflow flag. If the flag is clear, control passes to the next sequential instruction. If the overflow flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDE: Instruction{
		Mnemonic:        "JLT",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF SIGNED LESS THAN.",
		LongDescription: "Tests the negative flag. If the flag is clear, control passes to the next sequential instruction. If the negative flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xDF: Instruction{
		Mnemonic:        "JE",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "indexed",
		Description:     "JUMP IF EQUAL.",
		LongDescription: "Tests the zero flag. If the flag is clear, control passes to the next sequential instruction. If the zero flag is set, this instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE0: Instruction{
		Mnemonic:        "DJNZ",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"BREG", "ADDR"},
		VarStrings:      []string{"breg", "cadd"},
		AddressingMode:  "indexed",
		Description:     "DECREMENT AND JUMP IF NOT ZERO.",
		LongDescription: "Decrements the value of the byte operand by 1. If the result is 0, control passes to the next sequential instruction. If the result is not 0, the instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE1: Instruction{
		Mnemonic:        "DJNZW",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"WREG", "ADDR"},
		VarStrings:      []string{"wreg", "cadd"},
		AddressingMode:  "indexed",
		Description:     "DECREMENT AND JUMP IF NOT ZERO WORD.",
		LongDescription: "Decrements the value of the word operand by 1. If the result is 0, control passes to the next sequential instruction. If the result is not 0, the instruction adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –128 to +127.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE2: Instruction{
		Mnemonic:        "TIJMP",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"TBASE", "INDEX", "#MASK"}, // TODO XXX
		VarStrings:      []string{"TBASE", "INDEX", "#MASK"},
		AddressingMode:  "indexed",
		Description:     "TABLE INDIRECT JUMP.",
		LongDescription: "Causes execution to continue at an address selected from a table of addresses.\n The first word register, TBASE, contains the 16-bit address of the beginning of the jump table. TBASE can be located in RAM up to FEH without windowing or above FFH with windowing. The jump table itself can be placed at any nonreserved memory location on a word boundary in page FFH.\n The second word register, INDEX, contains the 16-bit address that points to a register containing a 7-bit value. This value is used to calculate the offset into the jump table. Like TBASE, INDEX can be located in RAM up to FEH without windowing or above FFH with windowing. Note that the 16-bit address contained in INDEX is absolute; it disregards any windowing that may be in effect when the TIJMP instruction is executed.\n The byte operand, #MASK, is 7-bit immediate data to mask INDEX. #MASK is ANDed with INDEX to determine the offset (OFFSET). OFFSET is multiplied by two, then added to the base address (TBASE) to determine the destination address (DEST X) in page FFH.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE3: Instruction{
		Mnemonic:        "EBR",
		ByteLength:      2,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"}, // TODO XXX
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED BRANCH INDIRECT.",
		LongDescription: "Continues execution at the address specified in the operand word register. This instruction is an unconditional indirect jump to anywhere in the 16-Mbyte address space.\n EBR shares its opcode (E3) with the BR instruction. To differentiate between the two, the compiler sets the least-significant bit of treg for the EBR instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE4: Instruction{
		Mnemonic:        "EBMOVI",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"PTRS", "CNTREG"},
		VarStrings:      []string{"prt2_reg", "wreg"},
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED INTERRUPTIBLE BLOCK MOVE.",
		LongDescription: "Moves a block of word data from one memory location to another. This instruction allows you to move blocks of up to 64K words between any two locations in the 16-Mbyte address space. This instruction is interruptible. The source and destination addresses are calculated using the extended indirect with autoincrement addressing mode. A quadword register (PTRS) addresses the 24-bit pointers, which are stored in adjacent doubleword registers. The source pointer (SRCPTR) is the low double-word and the destination pointer is the high double-word of PTRS. A word register (CNTREG) specifies the number of transfers. This register must reside in the lower register file; it cannot be windowed. The blocks of data can reside anywhere in memory, but should not overlap.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE5: Instruction{
		Mnemonic:   "Reserved",
		ByteLength: 1,
		Reserved:   true,
	},
	0xE6: Instruction{
		Mnemonic:        "EJMP",
		ByteLength:      4,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The operand may be any address in the entire address space. The offset must be in the range of +8,388,607 to –8,388,608 for 24-bit addresses. This instruction is an unconditional, relative jump to anywhere in the 16-Mbyte address space. It functions only in extended addressing mode.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE7: Instruction{
		Mnemonic:        "LJMP",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "long-indexed",
		Description:     "LONG JUMP.",
		LongDescription: "Adds to the program counter the offset between the end of this instruction and the target label, effecting the jump. The offset must be in the range of –32,768 to +32,767.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE8: Instruction{
		Mnemonic:        "ELD",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "treg"},
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand. This instruction allows you to move data from anywhere in the 16-Mbyte address space into the lower register file.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xE9: Instruction{
		Mnemonic:        "ELD",
		ByteLength:      6,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "treg"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED LOAD WORD.",
		LongDescription: "Loads the value of the source word operand into the destination operand. This instruction allows you to move data from anywhere in the 16-Mbyte address space into the lower register file.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xEA: Instruction{
		Mnemonic:        "ELDB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "treg"},
		AddressingMode:  "extended-indirect",
		Description:     "EXTENDED LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand. This instruction allows you to move data from anywhere in the 16-Mbyte address space into the lower register file.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xEB: Instruction{
		Mnemonic:        "ELDB",
		ByteLength:      6,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"breg", "treg"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED LOAD BYTE.",
		LongDescription: "Loads the value of the source byte operand into the destination operand. This instruction allows you to move data from anywhere in the 16-Mbyte address space into the lower register file.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xEC: Instruction{
		Mnemonic:        "DPTS",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "DISABLE PERIPHERAL TRANSACTION SERVER (PTS).",
		LongDescription: "Disables the peripheral transaction server (PTS).",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xED: Instruction{
		Mnemonic:        "EPTS",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "ENABLE PERIPHERAL TRANSACTION SERVER (PTS).",
		LongDescription: "Enables the peripheral transaction server (PTS).",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xEE: Instruction{
		Mnemonic:   "Reserved",
		ByteLength: 1,
		Reserved:   true,
	},
	0xEF: Instruction{
		Mnemonic:        "LCALL",
		ByteLength:      3,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "long-indexed",
		Description:     "LONG CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The offset must be in the range of –32,768 to +32,767.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF0: Instruction{
		Mnemonic:        "RET",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "indirect",
		Description:     "RETURN FROM SUBROUTINE.",
		LongDescription: "Pops the PC off the top of the stack.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF1: Instruction{
		Mnemonic:        "ECALL",
		ByteLength:      4,
		VarCount:        1,
		VarTypes:        []string{"ADDR"},
		VarStrings:      []string{"cadd"},
		AddressingMode:  "extended-indexed",
		Description:     "EXTENDED CALL.",
		LongDescription: "Pushes the contents of the program counter (the return address) onto the stack, then adds to the program counter the offset between the end of this instruction and the target label, effecting the call. The operand may be any address in the address space. \n This instruction is an unconditional relative call to anywhere in the 16-Mbyte address space. It functions only in extended addressing mode.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF2: Instruction{
		Mnemonic:        "PUSHF",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "PUSH FLAGS.",
		LongDescription: "Pushes the PSW onto the top of the stack, then clears it. Clearing the PSW disables interrupt servicing. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF3: Instruction{
		Mnemonic:        "POPF",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "POP FLAGS.",
		LongDescription: "Pops the word on top of the stack and places it into the PSW. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF4: Instruction{
		Mnemonic:        "PUSHA",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "PUSH ALL.",
		LongDescription: "This instruction is used instead of PUSHF, to support the eight additional interrupts. It pushes two words — PSW/INT_MASK and INT_MASK1/WSR — onto the stack.\n This instruction clears the PSW, INT_MASK, and INT_MASK1 registers and decrements the SP by 4. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF5: Instruction{
		Mnemonic:        "POPA",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "POP ALL.",
		LongDescription: "This instruction is used instead of POPF, to support the eight additional interrupts. It pops two words off the stack and places the first word into the INT_MASK1/WSR register pair and the second word into the PSW/INT_MASK register-pair. This instruction increments the SP by 4. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF6: Instruction{
		Mnemonic:        "IDLPD",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "immediate",
		Description:     "IDLE/POWERDOWN.",
		LongDescription: "Depending on the 8-bit value of the KEY operand, this instruction causes the device to: \n • enter idle mode, if KEY=1, \n • enter powerdown mode, if KEY=2, \n • execute a reset sequence, \n if KEY > 3. \n The bus controller completes any prefetch cycle in progress before the CPU stops or resets.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF7: Instruction{
		Mnemonic:        "TRAP",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "SOFTWARE TRAP.",
		LongDescription: "This instruction causes an interrupt call that is vectored through location FF2010H. The operation of this instruction is not affected by the state of the interrupt enable flag (I) in the PSW. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF8: Instruction{
		Mnemonic:        "CLRC",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "CLEAR CARRY FLAG.",
		LongDescription: "Clears the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xF9: Instruction{
		Mnemonic:        "SETC",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "SET CARRY FLAG.",
		LongDescription: "Sets the carry flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xFA: Instruction{
		Mnemonic:        "DI",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "DISABLE INTERRUPTS.",
		LongDescription: "Disables maskable interrupts. Interrupt calls cannot occur after this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xFB: Instruction{
		Mnemonic:        "EI",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "ENABLE INTERRUPTS.",
		LongDescription: "Enables maskable interrupts following the execution of the next statement. Interrupt calls cannot occur immediately following this instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xFC: Instruction{
		Mnemonic:        "CLRVT",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "CLEAR OVERFLOW-TRAP FLAG.",
		LongDescription: "Clears the overflow-trap flag.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xFD: Instruction{
		Mnemonic:        "NOP",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "NO OPERATION.",
		LongDescription: "Does nothing. Control passes to the next sequential instruction.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0xFE: Instruction{
		Mnemonic:       "(Note 2) Prefix for signed multiplication and division.",
		ByteLength:     1,
		VarCount:       0,
		VariableLength: false,
		AutoIncrement:  false,
		Flags:          Flags{},
		Ignore:         true,
		Signed:         false,
		Reserved:       false,
	},
	0xFF: Instruction{
		Mnemonic:        "RST",
		ByteLength:      1,
		VarCount:        0,
		AddressingMode:  "direct",
		Description:     "RESET SYSTEM.",
		LongDescription: "Initializes the PSW to zero, the PC to FF2080H, and the pins and SFRs to their reset values. Executing this instruction causes the RESET# pin to be pulled low for 16 state times.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
}

var signedInstructions = map[byte]Instruction{
	0x1C: Instruction{
		Mnemonic:        "MYSTERY",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "MYSTERY.",
		LongDescription: "MYSTERY",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4C: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the two source integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4D: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the two source integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4E: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the two source integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x4F: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"lreg", "wreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the two source integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5C: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the two source short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5D: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the two source short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5E: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      4,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the two source short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x5F: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      5,
		VarCount:        3,
		VarTypes:        []string{"DEST", "SRC1", "SRC2"},
		VarStrings:      []string{"wreg", "breg", "baop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the two source short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6C: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the source and destination integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6D: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the source and destination integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6E: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the source and destination integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x6F: Instruction{
		Mnemonic:        "MUL",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY INTEGERS.",
		LongDescription: "Multiplies the source and destination integer operands, using signed arithmetic, and stores the 32-bit result into the destination long-integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7C: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the source and destination short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7D: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the source and destination short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7E: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the source and destination short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x7F: Instruction{
		Mnemonic:        "MULB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "MULTIPLY SHORT-INTEGERS.",
		LongDescription: "Multiplies the source and destination short-integer operands, using signed arithmetic, and stores the 16-bit result into the destination integer operand. The sticky bit flag is undefined after the instruction is executed.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8C: Instruction{
		Mnemonic:        "DIV",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "direct",
		Description:     "DIVIDE INTEGERS.",
		LongDescription: "Divides the contents of the destination long-integer operand by the contents of the source integer word operand, using signed arithmetic. It stores the quotient into the low-order word of the destination (i.e., the word with the lower address) and the remainder into the high-order word.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8D: Instruction{
		Mnemonic:        "DIV",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "immediate",
		Description:     "DIVIDE INTEGERS.",
		LongDescription: "Divides the contents of the destination long-integer operand by the contents of the source integer word operand, using signed arithmetic. It stores the quotient into the low-order word of the destination (i.e., the word with the lower address) and the remainder into the high-order word.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8E: Instruction{
		Mnemonic:        "DIV",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indirect",
		Description:     "DIVIDE INTEGERS.",
		LongDescription: "Divides the contents of the destination long-integer operand by the contents of the source integer word operand, using signed arithmetic. It stores the quotient into the low-order word of the destination (i.e., the word with the lower address) and the remainder into the high-order word.",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x8F: Instruction{
		Mnemonic:        "DIV",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"lreg", "waop"},
		AddressingMode:  "indexed",
		Description:     "DIVIDE INTEGERS.",
		LongDescription: "Divides the contents of the destination long-integer operand by the contents of the source integer word operand, using signed arithmetic. It stores the quotient into the low-order word of the destination (i.e., the word with the lower address) and the remainder into the high-order word.",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9C: Instruction{
		Mnemonic:        "DIVB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "direct",
		Description:     "DIVIDE SHORT-INTEGERS.",
		LongDescription: "Divides the contents of the destination integer operand by the contents of the source short-integer operand, using signed arithmetic. It stores the quotient into the low-order byte of the destination (i.e., the word with the lower address) and the remainder into the highorder byte. ",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9D: Instruction{
		Mnemonic:        "DIVB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "immediate",
		Description:     "DIVIDE SHORT-INTEGERS.",
		LongDescription: "Divides the contents of the destination integer operand by the contents of the source short-integer operand, using signed arithmetic. It stores the quotient into the low-order byte of the destination (i.e., the word with the lower address) and the remainder into the highorder byte. ",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9E: Instruction{
		Mnemonic:        "DIVB",
		ByteLength:      3,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indirect",
		Description:     "DIVIDE SHORT-INTEGERS.",
		LongDescription: "Divides the contents of the destination integer operand by the contents of the source short-integer operand, using signed arithmetic. It stores the quotient into the low-order byte of the destination (i.e., the word with the lower address) and the remainder into the highorder byte. ",
		VariableLength:  false,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
	0x9F: Instruction{
		Mnemonic:        "DIVB",
		ByteLength:      4,
		VarCount:        2,
		VarTypes:        []string{"DEST", "SRC"},
		VarStrings:      []string{"wreg", "baop"},
		AddressingMode:  "indexed",
		Description:     "DIVIDE SHORT-INTEGERS.",
		LongDescription: "Divides the contents of the destination integer operand by the contents of the source short-integer operand, using signed arithmetic. It stores the quotient into the low-order byte of the destination (i.e., the word with the lower address) and the remainder into the highorder byte. ",
		VariableLength:  true,
		AutoIncrement:   false,
		Flags:           Flags{},
		Ignore:          false,
		Signed:          false,
		Reserved:        false,
	},
}
