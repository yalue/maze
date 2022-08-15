// This defines a library for generating 2D mazes.  The generated mazes satisfy
// the Maze interface, which includes go's image.Image interface.
package maze

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"time"
)

// All mazes returned by this library will support this interface. For now, it
// at least provides the Image interface so the mazes can be saved to files. In
// the future, additional functionality may be added to this.
type Maze interface {
	image.Image
	RegenerateFromSeed(seed int64) error
	ShowSolution(show bool) error
	// Returns a human-readable string about that maze, for providing debug
	// info such as the last set random seed.
	GetInfo() string
}

// Implements the disjoint set data structure from CLRS.
type disjointSet struct {
	parent *disjointSet
	rank   int
}

// Returns a new disjointSet containing only itself.
func newDisjointSet() *disjointSet {
	toReturn := disjointSet{
		rank: 0,
	}
	toReturn.parent = &toReturn
	return &toReturn
}

// Finds the unique "root" of a disjoint set. May adjust parent pointers.
func (s *disjointSet) findSet() *disjointSet {
	if s != s.parent {
		s.parent = s.parent.findSet()
	}
	return s.parent
}

// Adjusts both s and other to become part of the same set. May adjust parent
// pointers and ranks.
func (s *disjointSet) union(other *disjointSet) {
	x := s.findSet()
	y := other.findSet()
	if x.rank > y.rank {
		y.parent = x
		return
	}
	x.parent = y
	if x.rank == y.rank {
		y.rank++
	}
}

// The number of pixels across, in a square cell. Must be at least 5.
const cellPixels = 9

// Differentiates between different types of cells, i.e. whether they are part
// of the solution, or not part of the maze at all.
type cellState uint8

func (s cellState) String() string {
	switch s {
	case 0:
		return "normal"
	case 1:
		return "solutionPath"
	case 2:
		return "excluded"
	}
	return fmt.Sprintf("Unknown cellState: %d", uint8(s))
}

// A single "cell" of the grid-based maze. Can be drawn as an image
// individuallly.
type gridMazeCell struct {
	// Whether each of the cell's "walls" are present. The order is left, top,
	// right, bottom. Each entry is true if the wall is there.
	walls [4]bool
	// Determines how the cell is drawn, and other stuff.
	state cellState
	// Used for the disjoint-set method of maze generation.
	djSet *disjointSet
}

func (c *gridMazeCell) ColorModel() color.Model {
	return color.RGBAModel
}

func (c *gridMazeCell) Bounds() image.Rectangle {
	return image.Rect(0, 0, cellPixels, cellPixels)
}

// Takes an integer, 0 through 3, corresponding to the top-left, top-right,
// bottom-right, and bottom-left corners. Returns false only if both walls
// adjacent to the corner are clear.
func (c *gridMazeCell) cornerSet(n int) bool {
	if (n < 0) || (n > 3) {
		return true
	}
	if n == 3 {
		return c.walls[3] || c.walls[0]
	}
	return c.walls[n] || c.walls[n+1]
}

func (c *gridMazeCell) At(x, y int) color.Color {
	if (x < 0) || (y < 0) || (x >= cellPixels) || (y >= cellPixels) {
		return color.Transparent
	}
	// "Excluded" cells are always going to be blank.
	if c.state == 2 {
		return color.White
	}
	if x == 0 {
		if y == 0 {
			// Top left corner is clear only if it has no adjacent walls
			if c.cornerSet(0) {
				return color.Black
			}
			return color.White
		}
		if y == (cellPixels - 1) {
			// Bottom left corner
			if c.cornerSet(3) {
				return color.Black
			}
			return color.White
		}
		// The pixel is along the left wall
		if c.walls[0] {
			return color.Black
		}
		return color.White
	}
	if x == (cellPixels - 1) {
		if y == 0 {
			// Top right corner
			if c.cornerSet(1) {
				return color.Black
			}
			return color.White
		}
		if y == (cellPixels - 1) {
			// Bottom right corner
			if c.cornerSet(2) {
				return color.Black
			}
			return color.White
		}
		// The pixel is along the right wall
		if c.walls[2] {
			return color.Black
		}
		return color.White
	}
	// We already checked corners along with the left and right walls, so we
	// don't need to check them for the top and bottom walls.
	if y == 0 {
		if c.walls[1] {
			return color.Black
		}
		return color.White
	}
	if y == (cellPixels - 1) {
		if c.walls[3] {
			return color.Black
		}
		return color.White
	}
	// At this point, we're not along any wall. First, everything is simply
	// white if we're not "selected"
	if c.state != 1 {
		return color.White
	}
	// Selected cells are red, if more than two pixels away from an edge.
	if (x > 1) && (x < (cellPixels - 2)) && (y > 1) && (y < (cellPixels - 2)) {
		return color.RGBA{
			R: 230,
			G: 20,
			B: 20,
			A: 255,
		}
	}
	return color.White
}

// Resets the given cell's disjoint set entry and sets all of its walls. Does
// *not* change its cell state.
func initGridMazeCell(c *gridMazeCell) {
	c.djSet = newDisjointSet()
	for i := range c.walls {
		c.walls[i] = true
	}
}

// Used internally when generating a GridMaze.
type gridNeighborInfo struct {
	// The index of the first cell in the disjoint neighboring pair.
	baseIndex int
	// Will always be either 2 (right) or 3 (down) due to our implementation.
	neighborDirection int
}

// Satisfies the Maze interface. Basically a 2D array of cells. Create using
// NewGridMaze.
type GridMaze struct {
	// Width and height are numbers of cells
	width  int
	height int
	cells  []gridMazeCell
	// The indices of the start and end cells in the maze.
	startCellIndex int
	endCellIndex   int
	// Used internally when generating the maze, to avoid reallocating a slice
	// many times.
	neighbors []gridNeighborInfo
	// The seed that was initially used when creating the maze.
	randomSeed int64
	// The time required for the last generation.
	generationTime float64
}

// Allocates but does not initialize any maze cell contents.
func allocateMaze(width, height int) (*GridMaze, error) {
	if (width < 1) || (height < 1) {
		return nil, fmt.Errorf("width and height must be at least 1")
	}
	cellCount := width * height
	// Check for overflow.
	if cellCount <= 0 {
		return nil, fmt.Errorf("The maze's size was too big")
	}
	toReturn := &GridMaze{
		width:          int(width),
		height:         int(height),
		cells:          make([]gridMazeCell, cellCount),
		startCellIndex: -1,
		endCellIndex:   -1,
		neighbors:      nil,
	}
	return toReturn, nil
}

// Generates a maze. If the given RNG seed is not positive, a new seed will be
// selected based on the current time in nanoseconds.
func NewGridMazeWithSeed(width, height int, seed int64) (*GridMaze, error) {
	toReturn, e := allocateMaze(width, height)
	if e != nil {
		return nil, e
	}
	if seed <= 0 {
		seed = time.Now().UnixNano()
	}
	e = toReturn.RegenerateFromSeed(seed)
	if e != nil {
		return nil, fmt.Errorf("Error generating maze: %w", e)
	}
	return toReturn, nil
}

// We'll convert template colors to values of this type.
type templateCellType uint8

func (t templateCellType) String() string {
	switch t {
	case 0:
		return "valid"
	case 1:
		return "excluded"
	case 2:
		return "startCandidate"
	case 3:
		return "endCandidate"
	}
	return fmt.Sprintf("Invalid template cell type: %d", uint8(t))
}

// Converts an arbitrary color to what the type of cell represents. See the
// comment on NewGridMazeFromTemplate for how the mapping works.
func colorToTemplateCellType(c color.Color) templateCellType {
	r, g, b, _ := c.RGBA()
	r = r >> 8
	g = g >> 8
	b = b >> 8
	// Black pixels represent excluded cells
	if (r == 0) && (g == 0) && (b == 0) {
		return 1
	}
	// Green pixels are possible starting cells
	if (r == 0) && (g > 200) && (b == 0) {
		return 2
	}
	// Red pixels are possible ending cells
	if (r > 200) && (g == 0) && (b == 0) {
		return 3
	}
	// White pixels are standard maze cells
	if (r == 255) && (g == 255) && (b == 255) {
		return 0
	}
	// All other colors are treated as standard for now
	return 0
}

// Uses a "template" image to generate a maze. Each pixel in the template will
// correspond to one cell in the maze. The given seed will be ignored if not
// positive. The template image must use the following format:
//   - Green pixels are possible starting points (RGB = 0, >200, 0)
//   - Red pixels are possible ending points (RGB = >200, 0, 0)
//   - Black pixels are excluded cells
//   - White pixels are "normal" cells that will be part of the maze.
//   - Any other color will be treated as "excluded" by default, but this is
//     subject to change.
func NewGridMazeFromTemplate(templatePic image.Image, seed int64) (*GridMaze,
	error) {
	bounds := templatePic.Bounds().Canon()
	width := bounds.Dx()
	height := bounds.Dy()
	toReturn, e := allocateMaze(width, height)
	if e != nil {
		return nil, e
	}
	if seed <= 0 {
		seed = time.Now().UnixNano()
	}

	possibleStartIndices := make([]int, 0, 100)
	possibleEndIndices := make([]int, 0, 100)
	cellIndex := -1
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			cellIndex++
			cellType := colorToTemplateCellType(templatePic.At(col, row))
			switch cellType {
			case 0:
				// No need to do anything with standard cells
				break
			case 1:
				// Type 1 = excluded cells
				toReturn.cells[cellIndex].state = 2
			case 2:
				// Type 2 = possible start cell.
				possibleStartIndices = append(possibleStartIndices, cellIndex)
			case 3:
				// Type 3 = possible end cell.
				possibleEndIndices = append(possibleEndIndices, cellIndex)
			default:
				return nil, fmt.Errorf("Invalid template pixel type (%s)",
					cellType)
			}
		}
	}

	rng := rand.New(rand.NewSource(seed))
	if len(possibleStartIndices) != 0 {
		toReturn.startCellIndex = possibleStartIndices[rng.Intn(
			len(possibleStartIndices))]
	} else {
		if toReturn.cells[0].state == 2 {
			return nil, fmt.Errorf("No possible start locations marked, and " +
				"the top-left cell is excluded")
		}
		toReturn.startCellIndex = 0
	}
	if len(possibleEndIndices) != 0 {
		toReturn.endCellIndex = possibleEndIndices[rng.Intn(
			len(possibleEndIndices))]
	} else {
		if toReturn.cells[len(toReturn.cells)-1].state == 2 {
			return nil, fmt.Errorf("No possible end locations marked, and " +
				"the bottom-right cell is excluded")
		}
		toReturn.endCellIndex = len(toReturn.cells) - 1
	}

	// We've set up walls and chosen a start and end cell, so build the actual
	// maze now.
	e = toReturn.RegenerateFromSeed(seed)
	if e != nil {
		return nil, fmt.Errorf("Error generating maze: %w", e)
	}
	return toReturn, nil
}

// Initializes the list of neighbors that aren't connected yet. Must only be
// called by RegenerateFromSeed
func (m *GridMaze) initDisjointNeighbors() error {
	var initCount int
	// Each cell starts with a disconnected neighbor to its right, except for
	// the rightmost column.
	initCount += (m.width - 1) * m.height
	// Each cell starts with a disconnected neighbor below it, except for the
	// bottom row.
	initCount += (m.height - 1) * m.width
	if initCount == 0 {
		// We have a 1x1 "maze"
		return nil
	}
	m.neighbors = make([]gridNeighborInfo, 0, initCount)

	for row := 0; row < m.height; row++ {
		rowStartIdx := row * m.width
		for col := 0; col < m.width; col++ {
			index := rowStartIdx + col
			// We don't consider an "excluded" cell to be a disjoint neighbor,
			// because it will never be joined.
			if m.cells[index].state == 2 {
				continue
			}
			// Create an entry for the neighbor to the right, except if the
			// neighbor is excluded.
			if (col != (m.width - 1)) && (m.cells[index+1].state != 2) {
				m.neighbors = append(m.neighbors, gridNeighborInfo{
					baseIndex:         index,
					neighborDirection: 2,
				})
			}
			if row == (m.height - 1) {
				continue
			}
			// Create an entry for the neighbor below, also making sure it
			// isn't excluded.
			if m.cells[index+m.width].state != 2 {
				m.neighbors = append(m.neighbors, gridNeighborInfo{
					baseIndex:         index,
					neighborDirection: 3,
				})
			}
		}
	}
	return nil
}

// Returns the index of the neighboring cell from the gridNeighborInfo isntance
func (m *GridMaze) neighborIndex(n *gridNeighborInfo) (int, error) {
	if n.neighborDirection == 2 {
		// To the right
		return n.baseIndex + 1, nil
	}
	if n.neighborDirection == 3 {
		// Below
		return n.baseIndex + m.width, nil
	}
	return -1, fmt.Errorf("Internal error: neighbor not below or to the right")
}

// Remove any neighbors from the list of disjoint neighbors that are now
// connected.
func (m *GridMaze) updateDisjointNeighbors() error {
	i := 0
	for i < len(m.neighbors) {
		tmp := &(m.neighbors[i])
		cellA := &(m.cells[tmp.baseIndex])
		cellBIndex, e := m.neighborIndex(tmp)
		if e != nil {
			return e
		}
		cellB := &(m.cells[cellBIndex])
		// Leave this entry in the list if its describing two cells that still
		// aren't reachable from one another.
		if cellA.djSet.findSet() != cellB.djSet.findSet() {
			i++
			continue
		}
		// Cells A and B are reachable from one another, so remove this entry
		// from the list.
		m.neighbors[i] = m.neighbors[len(m.neighbors)-1]
		m.neighbors = m.neighbors[:len(m.neighbors)-1]
	}
	return nil
}

// Returns a pointer to a random entry in m.neighbors.  UNLESS the random entry
// selected corresponded to two cells that are already in the same set.  In
// such a case, this returns nil, nil.  Only returns a non-nil error if an
// internal error occurs.
func (m *GridMaze) sampleNeighbor(rng *rand.Rand) (*gridNeighborInfo, error) {
	toReturn := &(m.neighbors[rng.Intn(len(m.neighbors))])
	cellA := &(m.cells[toReturn.baseIndex])
	indexB, e := m.neighborIndex(toReturn)
	if e != nil {
		return nil, e
	}
	cellB := &(m.cells[indexB])
	if cellA.djSet.findSet() == cellB.djSet.findSet() {
		return nil, nil
	}
	return toReturn, nil
}

// Randomly selects and returns a gridNeighborInfo for two cells that can be
// joined. Updates the internal list of neighbors, etc. Returns nil, true, nil
// if the list is empty (indicating the maze is complete).
func (m *GridMaze) getDisjointNeighbor(rng *rand.Rand) (*gridNeighborInfo,
	bool, error) {
	if len(m.neighbors) == 0 {
		return nil, true, nil
	}
	// Start by seeing if a random sample returns a pair of cells that haven't
	// been joined yet.
	toReturn, e := m.sampleNeighbor(rng)
	if e != nil {
		return nil, false, e
	}
	if toReturn != nil {
		return toReturn, false, nil
	}
	// A random sample did *not* return two cells that haven't been joined, so
	// clean up the entire list to only contain non-joined cells.
	e = m.updateDisjointNeighbors()
	if e != nil {
		return nil, false, e
	}
	// The list may have become empty after updating.
	if len(m.neighbors) == 0 {
		return nil, true, nil
	}
	// Now, we know the list only contains non-joined neighbors, so picking one
	// randomly is guaranteed to be OK.
	toReturn, e = m.sampleNeighbor(rng)
	return toReturn, false, e
}

func (m *GridMaze) RegenerateFromSeed(seed int64) error {
	for i := range m.cells {
		initGridMazeCell(&(m.cells[i]))
	}
	e := m.initDisjointNeighbors()
	if e != nil {
		return fmt.Errorf("Error initializing maze state: %w", e)
	}
	m.randomSeed = seed
	rng := rand.New(rand.NewSource(seed))
	startTime := time.Now()

	for len(m.neighbors) != 0 {
		// Pick a random pair of cells to connect.
		tmp, done, e := m.getDisjointNeighbor(rng)
		if done {
			break
		}
		if e != nil {
			return e
		}
		cellA := &(m.cells[tmp.baseIndex])
		var cellB *gridMazeCell
		// Invalid directions have already been checked.
		if tmp.neighborDirection == 2 {
			// We're removing the wall between cellA and the cell to its right.
			cellB = &(m.cells[tmp.baseIndex+1])
			cellA.walls[2] = false
			cellB.walls[0] = false
		} else {
			// We're removing the wall between cellA and the cell below it.
			cellB = &(m.cells[tmp.baseIndex+m.width])
			cellA.walls[3] = false
			cellB.walls[1] = false
		}
		// Combine the sets containing the two cells.
		cellA.djSet.union(cellB.djSet)
	}

	m.generationTime = time.Since(startTime).Seconds()
	// Arbitrarily go from top right to bottom left if the start cell hasn't
	// been set.
	if m.startCellIndex == -1 {
		m.startCellIndex = 0
		m.endCellIndex = len(m.cells) - 1
	}
	return nil
}

// Removes any walls that only touch one other, reducing overall noise in the
// maze. Only does this *once*, so you can repeatedly call this function.
// However, calling it too much may eventually trivialize the entire maze.
func (m *GridMaze) ErodeWalls() error {
	// We use a copy to avoid removing multiple wall segments in a single call
	// to this function.
	origCells := make([]gridMazeCell, len(m.cells))
	copy(origCells, m.cells)

	// The way this works is that we basically check all four walls on the
	// bottom right corner of every cell except for the rightmost column and
	// bottommost row.
	for row := 0; row < (m.height - 1); row++ {
		rowStartIndex := row * m.width
		for col := 0; col < (m.width - 1); col++ {
			cellIndex := rowStartIndex + col
			// We remove walls that "stick out": not part of a corner or
			// touching another wall. Each bottom-right cell corner has a wall
			// sticking out if and only if it toucnes exactly one wall.
			wallCount := 0
			firstWall := 0

			// We only need to check two cells, the right and bottom wall of
			// the "main" cell, and the top and left of the "lower" cell, which
			// is diagonally across the corner.
			mainCell := &(origCells[cellIndex])
			if mainCell.walls[2] {
				wallCount++
				firstWall = 2
			}
			if mainCell.walls[3] {
				wallCount++
				if wallCount != 1 {
					continue
				}
				firstWall = 3
			}
			lowerCell := &(origCells[cellIndex+m.width+1])
			if lowerCell.walls[0] {
				wallCount++
				if wallCount != 1 {
					continue
				}
				firstWall = 0
			}
			if lowerCell.walls[1] {
				wallCount++
				if wallCount != 1 {
					continue
				}
				firstWall = 1
			}
			if wallCount == 0 {
				continue
			}
			// If we made it this far, we've seen exactly one wall, and set
			// firstWall to indicate which one it is.
			if firstWall == 0 {
				m.cells[cellIndex+m.width+1].walls[0] = false
				m.cells[cellIndex+m.width].walls[2] = false
			} else if firstWall == 1 {
				m.cells[cellIndex+m.width+1].walls[1] = false
				m.cells[cellIndex+1].walls[3] = false
			} else if firstWall == 2 {
				m.cells[cellIndex].walls[2] = false
				m.cells[cellIndex+1].walls[0] = false
			} else if firstWall == 3 {
				m.cells[cellIndex].walls[3] = false
				m.cells[cellIndex+m.width].walls[1] = false
			}
		}
	}
	return nil
}

// Used internally by ShowSolution.
func setDirRanking(currentCol, currentRow, targetCol, targetRow int,
	dirRanking []int) {
	colDiff := targetCol - currentCol
	absColDiff := colDiff
	if absColDiff < 0 {
		absColDiff = -absColDiff
	}
	rowDiff := targetRow - currentRow
	absRowDiff := rowDiff
	if absRowDiff < 0 {
		absRowDiff = -absRowDiff
	}
	if absRowDiff > absColDiff {
		// The row difference is bigger, so moving up or down is highest
		// priority.
		if rowDiff > 0 {
			// Moving down is best, up is worst
			dirRanking[0] = 3
			dirRanking[3] = 1
		} else {
			// Moving up is best, down is worst
			dirRanking[0] = 1
			dirRanking[3] = 3
		}
		if colDiff > 0 {
			// Moving right is better than left
			dirRanking[1] = 2
			dirRanking[2] = 0
		} else {
			// Moving left is better than right
			dirRanking[1] = 0
			dirRanking[2] = 2
		}
		return
	}
	// The highest priority is moving left or right
	if colDiff > 0 {
		// Moving right is best, left is worst
		dirRanking[0] = 2
		dirRanking[3] = 0
	} else {
		// Moving left is best, right is worst
		dirRanking[0] = 0
		dirRanking[3] = 2
	}
	if rowDiff > 0 {
		// Moving down is better than up
		dirRanking[1] = 3
		dirRanking[2] = 1
	} else {
		// Moving up is better than down
		dirRanking[1] = 1
		dirRanking[2] = 3
	}
}

// Used internally by ShowSolution.
func (m *GridMaze) isReachableAndUnvisited(currentIndex, currentCol,
	currentRow, moveDir int, visited []bool) (bool, int) {
	if m.cells[currentIndex].walls[moveDir] {
		return false, -1
	}
	dstIndex := currentIndex
	switch moveDir {
	case 0:
		if currentCol == 0 {
			return false, -1
		}
		dstIndex--
	case 1:
		if currentRow == 0 {
			return false, -1
		}
		dstIndex -= m.width
	case 2:
		if currentCol == (m.width - 1) {
			return false, -1
		}
		dstIndex++
	case 3:
		if currentRow == (m.height - 1) {
			return false, -1
		}
		dstIndex += m.width
	default:
		panic("Bad direction.")
	}
	// We assume that cell walls were removed reciprocally, and we already
	// checked that the currentCell does not have a wall in the relevant
	// direction.
	if visited[dstIndex] {
		return false, -1
	}
	return true, dstIndex
}

func (m *GridMaze) clearSolution() error {
	for i := range m.cells {
		// Don't change the cells with an "excluded" state.
		if m.cells[i].state == 1 {
			m.cells[i].state = 0
		}
	}
	return nil
}

func (m *GridMaze) ShowSolution(show bool) error {
	// We perform basically a depth-first search here, prioritizing moving in
	// whichever direction has the shortest manhattan distance to the target.
	if !show {
		return m.clearSolution()
	}
	var endRow, endCol int
	endCol = m.endCellIndex % m.width
	endRow = m.endCellIndex / m.width
	visited := make([]bool, len(m.cells))
	// Mark "excluded" cells as visited, just so the solution path will never
	// attempt to go through them.
	for i := range visited {
		if m.cells[i].state == 2 {
			visited[i] = true
		}
	}
	// These will be -1 to indicate either uninitialized or the end of the
	// path.
	parentIndices := make([]int, len(m.cells))
	for i := range parentIndices {
		parentIndices[i] = -1
	}

	// The initial capacity of this is arbitrary, but hopefully something big
	// enough that it won't need to be reallocated.
	dfsStack := make([]int, 0, len(m.cells)/2)
	dfsStack = append(dfsStack, m.startCellIndex)
	visited[m.startCellIndex] = true

	// Will be filled with some permutation of the values 0 through 3, (left,
	// up, right, down), where index 0 in this array is the best direction to
	// move (minimum manhattan distance), and index 3 is the worst. Ties may be
	// broken arbitrarily.
	var dirRanking [4]int

DFSLoop:
	for {
		// Select the next path starting-point from the top of the stack
		if len(dfsStack) == 0 {
			return fmt.Errorf("Internal error: failed to solve maze")
		}
		currentIndex := dfsStack[len(dfsStack)-1]
		dfsStack = dfsStack[:len(dfsStack)-1]
		currentCol := currentIndex % m.width
		currentRow := currentIndex / m.width
		if (currentRow == endRow) && (currentCol == endCol) {
			// Solution found! The parentIndices have already been set.
			break
		}

		// Follow the path as long as possible, minimizing manhattan distance
		// at each step.
		for {
			setDirRanking(currentCol, currentRow, endCol, endRow,
				dirRanking[:])
			moveDir := -1
			moveDst := -1
			for _, v := range dirRanking {
				okMove, dstIndex := m.isReachableAndUnvisited(currentIndex,
					currentCol, currentRow, v, visited)
				if !okMove {
					continue
				}
				if moveDst == -1 {
					// We found the next step in our move
					moveDst = dstIndex
					moveDir = v
					continue
				}
				// We've found a valid direction, but we already have chosen
				// our next step, so add it to the stack to test later.
				visited[dstIndex] = true
				parentIndices[dstIndex] = currentIndex
				dfsStack = append(dfsStack, dstIndex)
			}
			if moveDst < 0 {
				// Can't make any more moves along this path.
				break
			}
			// Move to the destination index
			visited[moveDst] = true
			parentIndices[moveDst] = currentIndex
			currentIndex = moveDst
			switch moveDir {
			case 0:
				currentCol--
			case 1:
				currentRow--
			case 2:
				currentCol++
			case 3:
				currentRow++
			}
			if (currentRow == endRow) && (currentCol == endCol) {
				break DFSLoop
			}
		}
	}

	// Highlight the path by following the chain of parent indices from the end
	index := m.endCellIndex
	for index >= 0 {
		m.cells[index].state = 1
		index = parentIndices[index]
	}

	return nil
}

func (m *GridMaze) GetInfo() string {
	return fmt.Sprintf("%dx%d grid maze with randon seed %d, generated in "+
		"%.03f seconds", m.width, m.height, m.randomSeed, m.generationTime)
}

// Returns the cell at the row and column.
func (m *GridMaze) getCell(col, row int) *gridMazeCell {
	return &(m.cells[col*m.width+row])
}

func (m *GridMaze) ColorModel() color.Model {
	return color.RGBAModel
}

func (m *GridMaze) Bounds() image.Rectangle {
	return image.Rect(0, 0, m.width*cellPixels, m.height*cellPixels)
}

func (m *GridMaze) At(x, y int) color.Color {
	if (x < 0) || (y < 0) || (x >= m.width*cellPixels) ||
		(y >= m.height*cellPixels) {
		return color.Transparent
	}
	// We delegate drawing of each pixel to the At() function for the cell it
	// falls into.
	row := x / cellPixels
	rowOffset := x % cellPixels
	col := y / cellPixels
	colOffset := y % cellPixels
	return m.getCell(col, row).At(rowOffset, colOffset)
}

// Satisfies the Image interface, surrounds an image with a solid-color border.
type imageBorder struct {
	pic         image.Image
	picBounds   image.Rectangle
	borderWidth int
	fillColor   color.Color
}

func (b *imageBorder) ColorModel() color.Model {
	return b.pic.ColorModel()
}

func (b *imageBorder) Bounds() image.Rectangle {
	tmp := b.picBounds
	w := b.borderWidth * 2
	return image.Rect(0, 0, tmp.Dx()+w, tmp.Dy()+w)
}

func (b *imageBorder) At(x, y int) color.Color {
	tmp := b.picBounds
	if (x < b.borderWidth) || (y < b.borderWidth) {
		return b.fillColor
	}
	if (x >= tmp.Dx()+b.borderWidth) || (y >= tmp.Dy()+b.borderWidth) {
		return b.fillColor
	}
	return b.pic.At(x-b.borderWidth+tmp.Min.X, y-b.borderWidth+tmp.Min.Y)
}

// Returns a new image, consisting of the given image surrounded by a border
// with the given width in pixels.
func AddImageBorder(pic image.Image, width int) image.Image {
	return &imageBorder{
		pic:         pic,
		picBounds:   pic.Bounds(),
		borderWidth: width,
		fillColor:   color.White,
	}
}
