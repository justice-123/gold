package main

import (
	"flag"
	"log"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

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
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

func getNumAliveCells(height, width int, world [][]uint8) int {
	num := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == 255 {
				num++
			}
		}
	}
	return num
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

type Server struct{}

func (s *Server) ProcessTurns(req stubs.Request, res *stubs.Response) error {
	// 초기 OldWorld 설정
	currentWorld := req.OldWorld
	nextWorld := makeNewWorld(req.ImageHeight, req.ImageWidth)

	for t := 0; t < req.Turns; t++ {
		// 매 턴마다 nextWorld를 새롭게 계산
		nextWorld = calculateNextWorld(currentWorld, req.ImageHeight, req.ImageWidth)

		// 결과를 응답 구조체에 설정
		res.AliveCellLocation = getAliveCells(req.ImageHeight, req.ImageWidth, nextWorld)
		res.AliveCell = getNumAliveCells(req.ImageHeight, req.ImageWidth, nextWorld)
		res.Turns = t + 1

		// 로그 출력 (디버깅 용)
		log.Printf("Turn %d - Alive Cells: %d", t+1, res.AliveCell)

		// 다음 턴을 위해 world 교체
		currentWorld, nextWorld = nextWorld, currentWorld
	}

	// 최종 상태 설정
	res.NewWorld = currentWorld
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
	defer listener.Close()
	log.Println("Server listening on port", *serverPort)
	rpc.Accept(listener)
}
