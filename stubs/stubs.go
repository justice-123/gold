package stubs

import "uk.ac.bris.cs/gameoflife/util"

var Turn = "Server.ProcessTurns"
var Alive = "Server.GetNumAliveCells"

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

type ResponseAlive struct {
	Turn     int
	NumAlive int
}
