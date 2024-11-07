package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
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

// Create and initialize a new 2D grid with given dimensions
func initializeWorld(height, width int) [][]uint8 {
	world := make([][]uint8, height)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

func loadInitialState(p Params, c distributorChannels) [][]uint8 {
	world := initializeWorld(p.ImageHeight, p.ImageWidth)
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			value := <-c.ioInput
			world[x][y] = value

			if value == 255 {

				//c.events <- CellFlipped{
				//	CompletedTurns: 0,
				//	Cell:           util.Cell{X: x, Y: y},
				//}
			}
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	return world
}

func saveGameState(p Params, c distributorChannels, turns int, world [][]uint8) {
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turns)
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[x][y]
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	//c.events <- ImageOutputComplete{turns, filename}
}

// Send an RPC call to the server and retrieve the updated game state
func executeTurn(client *rpc.Client, req stubs.Request, res *stubs.Response) {
	if err := client.Call(stubs.Turn, req, res); err != nil {
	}
}

func getCount(client *rpc.Client, c distributorChannels) {
	res := new(stubs.ResponseAlive)
	if err := client.Call(stubs.Alive, stubs.RequestAlive{}, res); err != nil {
		fmt.Println(err)
	}
	c.events <- AliveCellsCount{res.Turn, res.NumAlive}
}

func runTicker(done chan bool, client *rpc.Client, c distributorChannels) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case _ = <-ticker.C:
			getCount(client, c)

		}
	}
}

// Manage client-server interaction and distribute work across routines
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	serverAddress := "127.0.0.1:8030"
	client, err := rpc.Dial("tcp", serverAddress)
	if err != nil {
		log.Fatal("dialing", err)
	}
	defer client.Close()

	initialWorld := loadInitialState(p, c)

	req := stubs.Request{
		OldWorld:    initialWorld,
		Turns:       p.Turns,
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	res := new(stubs.Response)

	done := make(chan bool)
	defer close(done)
	go runTicker(done, client, c)

	executeTurn(client, req, res)

	saveGameState(p, c, res.Turns, res.NewWorld)
	c.events <- FinalTurnComplete{
		CompletedTurns: res.Turns,
		Alive:          res.AliveCellLocation,
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{res.Turns, Quitting}
	done <- true
	close(c.events)
}
