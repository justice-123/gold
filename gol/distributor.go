package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPressed <-chan rune
}

func makeNewWorld(height, width int) [][]uint8 {
	oldWorld := make([][]uint8, height)
	for i := range oldWorld {
		oldWorld[i] = make([]uint8, width)
	}
	return oldWorld
}

func readFromFile(p Params, c distributorChannels) [][]uint8 {
	world := makeNewWorld(p.ImageHeight, p.ImageWidth)
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- 1
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			value := <-c.ioInput
			world[y][x] = value

			if world[y][x] == 255 {
				c.events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: x, Y: y},
				}
			}
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	return world
}

func makeOutputTurnWithTurnNum(p Params, c distributorChannels, turns int, world [][]uint8) {

	// add a get output to the command channel
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turns)
	c.ioFilename <- filename

	// add one pixel at a time to the output channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{turns, filename}
}

func makeCall(client *rpc.Client, req stubs.Request, res *stubs.Response) {
	err := client.Call(stubs.Turn, req, res)
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	server := "127.0.0.1:8030"
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		log.Fatal("dialing", err)
	}
	done := make(chan bool, 1)
	OldWorld := readFromFile(p, c)

	req := stubs.Request{
		OldWorld:    OldWorld,
		Turns:       p.Turns,
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	res := new(stubs.Response)

	makeCall(client, req, res)
	done <- true

	makeOutputTurnWithTurnNum(p, c, res.Turns, res.NewWorld)
	aliveCells := res.AliveCellLocation
	finalCompleteMsg := FinalTurnComplete{
		CompletedTurns: res.Turns,
		Alive:          aliveCells,
	}

	c.events <- finalCompleteMsg
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{res.Turns, Quitting}
	client.Close()
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
