package stubs

import "uk.ac.bris.cs/gameoflife/util"

var Turn = "Server.ProcessTurns"
var Alive = "Server.GetAliveCells"
var Quit = "Server.Quit"

var KeyPressHandler = "Server.HandleKeyPress"

type AliveCellsRequest struct {
}

type AliveCellsResponse struct {
	AliveCount int
}

type Response struct {
	NewWorld          [][]uint8
	Turns             int
	AliveCell         int
	AliveCellLocation []util.Cell
}

type Request struct {
	OldWorld    [][]uint8
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	Start       int
	End         int
}

type Empty struct {
}

type RequestAlive struct {
}
type ResponseAlive struct {
	Turn     int
	NumAlive int
}

type RequestKeyPress struct {
	KeyPress rune
}
type ResponseKey struct {
	World        [][]uint8
	Turn         int
	Acknowledged bool
}

type EmptyRes struct {
}

type EmptyReq struct {
}
