package stubs

import "uk.ac.bris.cs/gameoflife/util"

var Turn = "Server.ProcessTurns"

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

type RequestAliveCells struct {
	CurrentWorld [][]uint8
}

type ResponseAliveCells struct {
	AliveCells         int
	AliveCellsLocation []util.Cell
}

type Server struct {
	ServerAddress string
	Callback      string
}

type ServerId struct {
	ServerId int
	Status   bool
}
