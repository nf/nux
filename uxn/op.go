package uxn

import (
	"strings"
)

// Op represents a Uxn opcode.
type Op byte

// Short reports whether the opcode has the short flag set.
func (b Op) Short() bool { return b&0x20 > 0 && b&0x9f > 0 }

// Return reports whether the opcode has the return flag set.
func (b Op) Return() bool { return b&0x40 > 0 && b&0x9f > 0 }

// Keep reports whether the opcode has the keep flag set.
func (b Op) Keep() bool { return b&0x80 > 0 && b&0x1f > 0 }

// Base returns the opcode without any flags set.
func (b Op) Base() Op {
	switch {
	case b&0x1f > 0:
		return b & 0x1f
	case b&0x9f == 0:
		return b
	case b&0x9f == 0x80:
		return LIT
	default:
		panic("unreachable")
	}
}

// StackVal holds the position and width of a value on the stack.
type StackVal struct {
	Index int // from top of stack (1 is first, 0 is none)
	Size  int // 1 for byte, 2 for short
}

// StackArgs reports the stack arguments consumed by Op.
// If any of the args are zero then they not consumed.
func (op Op) StackArgs() (a, b, c StackVal) {
	type v = StackVal
	w, t := 1, 1
	if op.Short() {
		w, t = 2, 2
	}
	switch op.Base() {
	case JMP, JSR, STH, INC, POP, DUP:
		a = v{w, t}
	case SWP, EQU, NEQ, GTH, LTH, ADD, SUB, MUL, DIV, AND, ORA, EOR:
		a = v{w, t}
		b = v{w * 2, t}
	case ROT:
		a = v{w, t}
		b = v{w * 2, t}
		c = v{w * 3, t}
	case NIP, OVR:
		a = v{w * 2, t}
	case JCN:
		a = v{w, t}
		b = v{w + 1, 1}
	case LDZ, LDR, DEI, JCI:
		a = v{1, 1}
	case STZ, STR, DEO, SFT:
		a = v{1, 1}
		b = v{1 + w, t}
	case LDA:
		a = v{2, 2}
	case STA:
		a = v{2, 2}
		b = v{2 + w, t}
	}
	return
}

const (
	BRK Op = iota
	INC
	POP
	NIP
	SWP
	ROT
	DUP
	OVR
	EQU
	NEQ
	GTH
	LTH
	JMP
	JCN
	JSR
	STH
	LDZ
	STZ
	LDR
	STR
	LDA
	STA
	DEI
	DEO
	ADD
	SUB
	MUL
	DIV
	AND
	ORA
	EOR
	SFT
	JCI
	INC2
	POP2
	NIP2
	SWP2
	ROT2
	DUP2
	OVR2
	EQU2
	NEQ2
	GTH2
	LTH2
	JMP2
	JCN2
	JSR2
	STH2
	LDZ2
	STZ2
	LDR2
	STR2
	LDA2
	STA2
	DEI2
	DEO2
	ADD2
	SUB2
	MUL2
	DIV2
	AND2
	ORA2
	EOR2
	SFT2
	JMI
	INCr
	POPr
	NIPr
	SWPr
	ROTr
	DUPr
	OVRr
	EQUr
	NEQr
	GTHr
	LTHr
	JMPr
	JCNr
	JSRr
	STHr
	LDZr
	STZr
	LDRr
	STRr
	LDAr
	STAr
	DEIr
	DEOr
	ADDr
	SUBr
	MULr
	DIVr
	ANDr
	ORAr
	EORr
	SFTr
	JSI
	INC2r
	POP2r
	NIP2r
	SWP2r
	ROT2r
	DUP2r
	OVR2r
	EQU2r
	NEQ2r
	GTH2r
	LTH2r
	JMP2r
	JCN2r
	JSR2r
	STH2r
	LDZ2r
	STZ2r
	LDR2r
	STR2r
	LDA2r
	STA2r
	DEI2r
	DEO2r
	ADD2r
	SUB2r
	MUL2r
	DIV2r
	AND2r
	ORA2r
	EOR2r
	SFT2r
	LIT
	INCk
	POPk
	NIPk
	SWPk
	ROTk
	DUPk
	OVRk
	EQUk
	NEQk
	GTHk
	LTHk
	JMPk
	JCNk
	JSRk
	STHk
	LDZk
	STZk
	LDRk
	STRk
	LDAk
	STAk
	DEIk
	DEOk
	ADDk
	SUBk
	MULk
	DIVk
	ANDk
	ORAk
	EORk
	SFTk
	LIT2
	INC2k
	POP2k
	NIP2k
	SWP2k
	ROT2k
	DUP2k
	OVR2k
	EQU2k
	NEQ2k
	GTH2k
	LTH2k
	JMP2k
	JCN2k
	JSR2k
	STH2k
	LDZ2k
	STZ2k
	LDR2k
	STR2k
	LDA2k
	STA2k
	DEI2k
	DEO2k
	ADD2k
	SUB2k
	MUL2k
	DIV2k
	AND2k
	ORA2k
	EOR2k
	SFT2k
	LITr
	INCkr
	POPkr
	NIPkr
	SWPkr
	ROTkr
	DUPkr
	OVRkr
	EQUkr
	NEQkr
	GTHkr
	LTHkr
	JMPkr
	JCNkr
	JSRkr
	STHkr
	LDZkr
	STZkr
	LDRkr
	STRkr
	LDAkr
	STAkr
	DEIkr
	DEOkr
	ADDkr
	SUBkr
	MULkr
	DIVkr
	ANDkr
	ORAkr
	EORkr
	SFTkr
	LIT2r
	INC2kr
	POP2kr
	NIP2kr
	SWP2kr
	ROT2kr
	DUP2kr
	OVR2kr
	EQU2kr
	NEQ2kr
	GTH2kr
	LTH2kr
	JMP2kr
	JCN2kr
	JSR2kr
	STH2kr
	LDZ2kr
	STZ2kr
	LDR2kr
	STR2kr
	LDA2kr
	STA2kr
	DEI2kr
	DEO2kr
	ADD2kr
	SUB2kr
	MUL2kr
	DIV2kr
	AND2kr
	ORA2kr
	EOR2kr
	SFT2kr
)

func (o Op) String() string { return opStrings[o] }

var opStrings = strings.Fields(`
	BRK
	INC
	POP
	NIP
	SWP
	ROT
	DUP
	OVR
	EQU
	NEQ
	GTH
	LTH
	JMP
	JCN
	JSR
	STH
	LDZ
	STZ
	LDR
	STR
	LDA
	STA
	DEI
	DEO
	ADD
	SUB
	MUL
	DIV
	AND
	ORA
	EOR
	SFT
	JCI
	INC2
	POP2
	NIP2
	SWP2
	ROT2
	DUP2
	OVR2
	EQU2
	NEQ2
	GTH2
	LTH2
	JMP2
	JCN2
	JSR2
	STH2
	LDZ2
	STZ2
	LDR2
	STR2
	LDA2
	STA2
	DEI2
	DEO2
	ADD2
	SUB2
	MUL2
	DIV2
	AND2
	ORA2
	EOR2
	SFT2
	JMI
	INCr
	POPr
	NIPr
	SWPr
	ROTr
	DUPr
	OVRr
	EQUr
	NEQr
	GTHr
	LTHr
	JMPr
	JCNr
	JSRr
	STHr
	LDZr
	STZr
	LDRr
	STRr
	LDAr
	STAr
	DEIr
	DEOr
	ADDr
	SUBr
	MULr
	DIVr
	ANDr
	ORAr
	EORr
	SFTr
	JSI
	INC2r
	POP2r
	NIP2r
	SWP2r
	ROT2r
	DUP2r
	OVR2r
	EQU2r
	NEQ2r
	GTH2r
	LTH2r
	JMP2r
	JCN2r
	JSR2r
	STH2r
	LDZ2r
	STZ2r
	LDR2r
	STR2r
	LDA2r
	STA2r
	DEI2r
	DEO2r
	ADD2r
	SUB2r
	MUL2r
	DIV2r
	AND2r
	ORA2r
	EOR2r
	SFT2r
	LIT
	INCk
	POPk
	NIPk
	SWPk
	ROTk
	DUPk
	OVRk
	EQUk
	NEQk
	GTHk
	LTHk
	JMPk
	JCNk
	JSRk
	STHk
	LDZk
	STZk
	LDRk
	STRk
	LDAk
	STAk
	DEIk
	DEOk
	ADDk
	SUBk
	MULk
	DIVk
	ANDk
	ORAk
	EORk
	SFTk
	LIT2
	INC2k
	POP2k
	NIP2k
	SWP2k
	ROT2k
	DUP2k
	OVR2k
	EQU2k
	NEQ2k
	GTH2k
	LTH2k
	JMP2k
	JCN2k
	JSR2k
	STH2k
	LDZ2k
	STZ2k
	LDR2k
	STR2k
	LDA2k
	STA2k
	DEI2k
	DEO2k
	ADD2k
	SUB2k
	MUL2k
	DIV2k
	AND2k
	ORA2k
	EOR2k
	SFT2k
	LITr
	INCkr
	POPkr
	NIPkr
	SWPkr
	ROTkr
	DUPkr
	OVRkr
	EQUkr
	NEQkr
	GTHkr
	LTHkr
	JMPkr
	JCNkr
	JSRkr
	STHkr
	LDZkr
	STZkr
	LDRkr
	STRkr
	LDAkr
	STAkr
	DEIkr
	DEOkr
	ADDkr
	SUBkr
	MULkr
	DIVkr
	ANDkr
	ORAkr
	EORkr
	SFTkr
	LIT2r
	INC2kr
	POP2kr
	NIP2kr
	SWP2kr
	ROT2kr
	DUP2kr
	OVR2kr
	EQU2kr
	NEQ2kr
	GTH2kr
	LTH2kr
	JMP2kr
	JCN2kr
	JSR2kr
	STH2kr
	LDZ2kr
	STZ2kr
	LDR2kr
	STR2kr
	LDA2kr
	STA2kr
	DEI2kr
	DEO2kr
	ADD2kr
	SUB2kr
	MUL2kr
	DIV2kr
	AND2kr
	ORA2kr
	EOR2kr
	SFT2kr
`)
