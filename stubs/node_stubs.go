package stubs

var CalculateWorldSegment = "Node.GetSegment"
var End = "Node.Quit"

type WorkerRequest struct {
	WholeWorld [][]uint8
	Start      int
	End        int
	Size       int
}

type WorkerResponse struct {
	Segment [][]uint8
}
