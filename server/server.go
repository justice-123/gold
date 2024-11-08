package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var quitting = make(chan bool, 1)

type RestartInfo struct {
	restart bool
	turns   int
	world   [][]uint8
}

type BoolContainer struct {
	mu     sync.Mutex
	status bool
}

type IntContainer2 struct {
	mu    sync.Mutex
	value int
	turn  int
}

type IntContainer struct {
	mu   sync.Mutex
	turn int
}

type WorldContainer struct {
	mu    sync.Mutex
	world [][]uint8
	turn  int
}

func (c *BoolContainer) get() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.status
}

func (c *BoolContainer) setTrue() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = true
}

func (c *BoolContainer) setFalse() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = false
}

func (c *IntContainer2) getTurn() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.turn
}

func (c *IntContainer) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.turn
}

func (c *IntContainer) set(val int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.turn = val
}

func (c *IntContainer2) getCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.value
}

func (w *WorldContainer) getWorld() [][]uint8 {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.world
}
func (w *WorldContainer) getTurn() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.turn
}

func (w *WorldContainer) set(new [][]uint8, turn int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.world = new
	w.turn = turn
}

func (w *WorldContainer) setWorld(new [][]uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.world = new
}

func (w *WorldContainer) setTurn(new int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.turn = new
}

func (c *IntContainer2) set(turnsCompleted, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.value = count
	c.turn = turnsCompleted
}

var pausedTurn IntContainer
var getCount, paused, snapShot, quit, shut, clientQuit BoolContainer
var aliveCount IntContainer2
var waitingSnapShot, pause, waitingForCount, waitingPause, waitingShut, makingSnapshot sync.WaitGroup
var world, snapshotInfo WorldContainer
var restartInformation RestartInfo

func makeNewWorld(height, width int) [][]uint8 {
	newWorld := make([][]uint8, height)
	for i := range newWorld {
		newWorld[i] = make([]uint8, width)
	}
	return newWorld
}

func getAliveCells(height, width int, world [][]uint8) []util.Cell {
	aliveCells := make([]util.Cell, 0)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[x][y] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

func calculateNeighbor(x, y int, world [][]uint8, height, width int) int {
	aliveNeighbor := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			ny, nx := y+i, x+j
			if nx < 0 {
				nx = width - 1
			} else if nx == width {
				nx = 0
			} else {
				nx = nx % width
			}

			if ny < 0 {
				ny = height - 1
			} else if ny == height {
				ny = 0
			} else {
				ny = ny % height
			}

			if world[ny][nx] == 255 {
				if !(i == 0 && j == 0) {
					aliveNeighbor++
				}
			}
		}
	}
	return aliveNeighbor
}

func calculateNextWorld(world [][]uint8, height, width int) [][]uint8 {
	newWorld := makeNewWorld(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			neighbors := calculateNeighbor(x, y, world, height, width)
			if neighbors < 2 || neighbors > 3 {
				newWorld[y][x] = 0
			} else if neighbors == 3 {
				newWorld[y][x] = 255
			} else if neighbors == 2 && world[y][x] == 255 {
				newWorld[y][x] = 255
			} else {
				newWorld[y][x] = 0
			}
		}
	}
	return newWorld
}

func getAliveCellsFor(world [][]uint8, height, width int) int {
	count := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[x][y] == 255 {
				count++
			}
		}
	}
	return count
}

type Server struct{}

func (s *Server) GetAliveCells(_ stubs.RequestAlive, res *stubs.ResponseAlive) error {
	getCount.setTrue()
	waitingForCount.Add(1)
	waitingForCount.Wait()
	res.NumAlive = aliveCount.getCount()
	res.Turn = aliveCount.getTurn()
	return nil
}

func (s *Server) GetSnapshot(_ stubs.RequestAlive, res *stubs.ResponseSnapshot) error {
	// we want to get back the state of the board
	snapShot.setTrue()
	waitingSnapShot.Add(1)
	waitingSnapShot.Wait()
	res.Turns = snapshotInfo.getTurn()
	res.NewWorld = snapshotInfo.getWorld()
	return nil
}

func (s *Server) GetSnapshotPaused(_ stubs.RequestAlive, res *stubs.ResponseSnapshot) error {
	// we want to get back the state of the board
	makingSnapshot.Add(1)
	res.NewWorld = world.getWorld()
	res.Turns = world.turn
	makingSnapshot.Done()
	return nil
}

func (s *Server) PauseProcessing(_ stubs.EmptyReq, res *stubs.ResponseTurn) error {
	// pause the processing
	//waitingPause.Add(1)
	pause.Add(1)
	paused.setTrue()
	//waitingPause.Wait()
	res.Turn = pausedTurn.get()
	return nil
}
func (s *Server) Quit(_ stubs.EmptyReq, _ *stubs.EmptyRes) error {
	// quit once next turn is complete
	quit.setTrue()
	return nil
}

func (s *Server) ClientQuit(_ stubs.EmptyReq, _ *stubs.EmptyRes) error {
	clientQuit.setTrue()
	return nil
}

func (s *Server) ClientQuitPause(_ stubs.EmptyReq, _ *stubs.EmptyRes) error {
	clientQuit.setTrue()
	pause.Done()
	return nil
}

func (s *Server) UnpauseProcessing(_ stubs.EmptyReq, _ *stubs.EmptyRes) error {
	paused.setFalse()
	pause.Done()
	return nil
}
func (s *Server) ShutDistribute() error {
	shut.setTrue()
	waitingShut.Add(1)
	waitingShut.Wait()
	return nil
}

func (s *Server) ProcessTurns(req stubs.Request, res *stubs.Response) error {
	currentWorld := req.OldWorld
	nextWorld := makeNewWorld(req.ImageHeight, req.ImageWidth)
	turn := 0
	if req.Restart {
		if restartInformation.restart {
			currentWorld = restartInformation.world
			turn = restartInformation.turns
		} else {
			return errors.New("nothing to restart with")
		}
	}
	for turnNum := 0; turnNum < req.Turns; turnNum++ {
		// 매 턴마다 nextWorld를 새롭게 계산
		nextWorld = calculateNextWorld(currentWorld, req.ImageHeight, req.ImageWidth)

		// 결과를 응답 구조체에 설정
		//res.AliveCell = getNumAliveCells(req.ImageHeight, req.ImageWidth, nextWorld)
		turn = turnNum + 1

		// 다음 턴을 위해 world 교체
		currentWorld = nextWorld
		world.set(currentWorld, turn)

		makingSnapshot.Wait()

		if paused.get() {
			pausedTurn.set(turn)
			//waitingPause.Done()
			pause.Wait()
		}
		if quit.get() {
			break
		}
		if snapShot.get() {
			snapShot.setFalse()
			snapshotInfo.set(currentWorld, turn)
			waitingSnapShot.Done()
		}
		if getCount.get() {
			getCount.setFalse()
			aliveCount.set(turn, getAliveCellsFor(currentWorld, req.ImageHeight, req.ImageWidth))
			waitingForCount.Done()
		}
		if clientQuit.get() {
			restartInformation = RestartInfo{restart: true, turns: turn, world: currentWorld}
			clientQuit.setFalse()
			break
		}
	}

	res.Turns = turn
	res.NewWorld = currentWorld
	res.AliveCellLocation = getAliveCells(req.ImageHeight, req.ImageWidth, currentWorld)

	if quit.get() {
		quitting <- true
	}
	return nil
}

func main() {
	serverPort := flag.String("port", "8030", "Port to Listen")
	flag.Parse()

	rpc.Register(&Server{})
	listener, err := net.Listen("tcp", "127.0.0.1:"+*serverPort)
	if err != nil {
		log.Fatal("Listener error:", err)
	}
	log.Println("Server listening on port", *serverPort)
	defer listener.Close()
	go rpc.Accept(listener)

	_ = <-quitting
}
