package gol

import (
	"fmt"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

var wg, pause, executingKeyPress sync.WaitGroup
var turn int
var worldGlobal, oldWorldGlobal [][]uint8
var end, snapshot, getCount = false, false, false

const CellAlive = 255
const CellDead = 0

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type pixel struct {
	X     int
	Y     int
	Value uint8
}

type neighbourPixel struct {
	X int
	Y int
}

func getAliveCells(world [][]uint8, p Params) []util.Cell {
	// make the slice
	aliveCells := make([]util.Cell, 0)

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			// append every CellAlive cells
			if world[x][y] == CellAlive {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}

	return aliveCells
}

func getNumAliveCells(world [][]uint8, p Params) int {
	num := 0
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			// append every CellAlive cells
			if world[x][y] == CellAlive {
				num++
			}
		}
	}

	return num
}

func makeOutput(world [][]uint8, p Params, c distributorChannels) {

	// add a get output to the command channel
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)

	// add one pixel at a time to the output channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[x][y]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}

func makeOutputTurnWithOuput(world [][]uint8, p Params, c distributorChannels, turns int) {

	// add a get output to the command channel
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turns)
	c.ioFilename <- filename

	// add one pixel at a time to the output channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[x][y]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{turns, filename}
}

func combineChannelData(world [][]uint8, data chan pixel, c distributorChannels) [][]uint8 {
	length := len(data)

	for i := 0; i < length; i++ {
		// do not do work if currently paused
		pause.Wait()

		item := <-data
		x := item.X
		y := item.Y

		if item.Value != world[x][y] {
			world[x][y] = item.Value
			c.events <- CellFlipped{turn, util.Cell{X: x, Y: y}}
		}
	}
	return world
}

func combineChannelDataN(data chan neighbourPixel, p Params) [][]int {
	// Create a 2D slice to store the neighbour count
	neighbours := make([][]int, p.ImageHeight)
	for i := range neighbours {
		neighbours[i] = make([]int, p.ImageWidth)
	}

	length := len(data)
	for i := 0; i < length; i++ {
		// do not do work if currently paused
		pause.Wait()

		item := <-data
		neighbours[item.X][item.Y] += 1
	}
	return neighbours
}

func startWorkers(workerNum, numRows int, p Params, world [][]uint8, c chan pixel, neighbours [][]int) {
	// if there is only one worker
	if workerNum == 1 {
		wg.Add(1)
		go updateWorldWorker(0, numRows, neighbours, c, world, p)
		return
	}

	// if there is more than one worker
	//first worker
	wg.Add(1)
	go updateWorldWorker(0, numRows, neighbours, c, world, p)

	// spread work between workers up to the last whole multiple
	num := 1
	finishRow := numRows
	for i := numRows; i < p.ImageHeight-numRows; i += numRows {
		wg.Add(1)
		go updateWorldWorker(i, i+numRows, neighbours, c, world, p)
		num++
		finishRow += numRows
	}
	// final worker does the remaining rows
	wg.Add(1)
	go updateWorldWorker(finishRow, p.ImageHeight, neighbours, c, world, p)

}

func startWorkersNeighbours(workerNum, numRows int, p Params, world [][]uint8, neighbourChan chan neighbourPixel) {
	// if there is only one worker
	if workerNum == 1 {
		// add one to wait for this worker
		wg.Add(1)
		go calculateNewAlive(world, p, 0, numRows, neighbourChan)
		return
	}

	// if there is more than one worker
	// add one to wait for this worker
	wg.Add(1)
	go calculateNewAlive(world, p, 0, numRows, neighbourChan)

	// spread work between workers up to the last whole multiple
	finishRow := numRows
	for i := numRows; i < p.ImageHeight-numRows; i += numRows {
		// add one to wait for this worker
		wg.Add(1)
		go calculateNewAlive(world, p, i, i+numRows, neighbourChan)
		finishRow += numRows
	}

	// final worker does the remaining rows
	// add one to wait for this worker
	wg.Add(1)
	go calculateNewAlive(world, p, finishRow, p.ImageHeight, neighbourChan)
}

func calculateNewAliveParallel(world [][]uint8, p Params, workerNum int, c distributorChannels) [][]uint8 {
	// this says RoundUp(p.imageHeight / workNum)
	numRows := (p.ImageHeight + workerNum - 1) / workerNum

	// make channels for the world data and neighbour data
	// needs to be the size of the board
	dataChan := make(chan pixel, p.ImageWidth*p.ImageHeight)
	// needs to be the size of the board 8 times as we may send 8 neighbours for each pixel
	neighbourChan := make(chan neighbourPixel, 8*p.ImageWidth*p.ImageHeight)
	// close these channels after calculation
	defer close(dataChan)
	defer close(neighbourChan)

	// start workers for calculating neighbours
	startWorkersNeighbours(workerNum, numRows, p, world, neighbourChan)

	// wait for neighbours to be calculated, then recombine neighbours
	wg.Wait()
	neighbours := combineChannelDataN(neighbourChan, p)

	// start workers to make the world
	startWorkers(workerNum, numRows, p, world, dataChan, neighbours)

	//wait for neighbours to be calculated, then recombine world
	wg.Wait()
	world = combineChannelData(world, dataChan, c)

	//return world
	return world
}

func calculateNewAlive(world [][]uint8, p Params, start, end int, n chan neighbourPixel) {
	// state that this worker is done once the functions completes
	defer wg.Done()

	//get neighbours
	// for all cells, calculate how many neighbours it has
	for y := start; y < end; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			// do not do work if currently paused

			// if a cell is CellAlive
			if world[x][y] == CellAlive {
				// add 1 to all neighbours
				// i and j are the offset
				for i := -1; i <= 1; i++ {
					for j := -1; j <= 1; j++ {

						//for image wrap around
						xCoord := x + i
						if xCoord < 0 {
							xCoord = p.ImageWidth - 1
						} else {
							xCoord = xCoord % p.ImageWidth
						}

						// for image wrap around
						yCoord := y + j
						if yCoord < 0 {
							yCoord = p.ImageHeight - 1
						} else {
							yCoord = yCoord % p.ImageHeight
						}

						// if you are not offset, do not add one as this is yourself
						if !(i == 0 && j == 0) {
							n <- neighbourPixel{xCoord, yCoord}
						}
					}
				}
			}
		}
	}
}

func updateWorldWorker(start, end int, neighbours [][]int, c chan pixel, world [][]uint8, p Params) {
	// state that you are done once the function finishes
	defer wg.Done()

	// for all cells in your region
	for y := start; y < end; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			// do not do work if currently paused

			numNeighbours := neighbours[x][y]
			// you die with less than two or more than 3 neighbours (or stay dead)
			if numNeighbours < 2 || numNeighbours > 3 {
				c <- pixel{x, y, CellDead}
			} else if numNeighbours == 3 {
				// you become alive if you are dead and have exactly 3
				c <- pixel{x, y, CellAlive}
			}
		}
	}
}

func runAliveEvery2(c distributorChannels, p Params, done chan bool) {
	//make the ticker
	ticker := time.NewTicker(2 * time.Second)

	for {
		//see if we need to report the number of alive cells
		select {
		case <-done:
			return
		case _ = <-ticker.C:
			getCount = true
		}
	}
}

func runKeyPressController(c distributorChannels, p Params) {
	for key := range c.keyPresses {
		switch key {
		case 's':
			snapshot = true
			return
		case 'q':
			end = true
			return
		case 'p':
			c.events <- StateChange{turn, Paused}
			for {
				for key := range c.keyPresses {
					switch key {
					case 's':
						makeOutputTurnWithOuput(worldGlobal, p, c, turn)
					case 'p':
						c.events <- StateChange{turn, Executing}
						return
					case 'q':
						end = true
						return
					}
				}
			}
		}
	}
}

func executeTurns(p Params, c distributorChannels) {
	for turn < p.Turns {

		// call the function to calculate new CellAlive cells from old CellAlive cells
		newAlive := calculateNewAliveParallel(worldGlobal, p, p.Threads, c)

		// set the current state to the new CellAlive cells
		oldWorldGlobal = worldGlobal
		worldGlobal = newAlive

		// increase the number of turns passed
		turn++

		runKeyPressController(c, p)

		if end {
			return
		}
		if getCount {
			getCount = false
			c.events <- AliveCellsCount{turn, getNumAliveCells(worldGlobal, p)}
		}
		if snapshot {
			snapshot = false
			makeOutputTurnWithOuput(worldGlobal, p, c, turn)
		}

		c.events <- TurnComplete{turn}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// Create a 2D slice to store the world.
	worldGlobal = make([][]uint8, p.ImageHeight)
	for i := range worldGlobal {
		worldGlobal[i] = make([]uint8, p.ImageWidth)
	}

	turn = 0

	// add a get input to the command channel
	// give file name
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	cells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	// read each uint8 data off of the input channel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.ioInput
			if val == 255 {
				worldGlobal[x][y] = 255
				cells = append(cells, util.Cell{x, y})
			}
		}
	}
	c.events <- CellsFlipped{turn, cells}

	c.events <- StateChange{turn, Executing}

	// key press controller
	// create stop channel for quitting
	//go runKeyPressController(c, p)

	// create a new ticket and a channel to stop it
	// run the ticker
	done := make(chan bool)
	go runAliveEvery2(c, p, done)

	// Make sure that the Io has finished any input before moving on to processing
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Execute all turns of the Game of Life.
	executeTurns(p, c)

	//output the slice to a pgm
	if end {
		makeOutputTurnWithOuput(worldGlobal, p, c, turn)
	} else {
		makeOutput(worldGlobal, p, c)
	}

	//Report the final state using FinalTurnCompleteEvent.
	aliveCells := getAliveCells(worldGlobal, p)
	c.events <- FinalTurnComplete{turn, aliveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// stop the ticker
	done <- true

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	executingKeyPress.Wait()
	close(done)
	close(c.events)
}
