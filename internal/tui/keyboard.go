package tui

import "github.com/gdamore/tcell/v2"

const (

	// KeyMenuOp is the operation corresponding to the activation of the Menu view.
	KeyMenuOp KeyOp = iota
	// KeyPreviewOp is the operation corresponding to the activation of the Preview table.
	KeyPreviewOp
)

var (
	KeyMapping = map[KeyOp]tcell.Key{
		KeyMenuOp:    tcell.KeyCtrlA,
		KeyPreviewOp: tcell.KeyCtrlD,
	}
)
