// This defines a library for generating 2D mazes. Basic usage:
//
//	// Create a maze with a random seed (based on time)
//	maze, _ := NewGridMaze(cellsWide, cellsHigh)
//	// OPTIONAL: Regenerate the maze using a specific seed
//	maze.RegenerateFromSeed(1337)
//	// OPTIONAL: Highlight the solution to the maze
//	maze.ShowSolution(true)
//
// The "maze" object satisfies the Maze interface, which includes go's
// image.Image interface.
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

// A single "cell" of the grid-based maze. Can be drawn as an image
// individuallly.
type gridMazeCell struct {
	// Whether each of the cell's "walls" are present. The order is left, top,
	// right, bottom. Each entry is true if the wall is there.
	walls [4]bool
	// True if the cell is "selected". This will color the middle pixels red if
	// set. Used for drawing the solution.
	selected bool
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
	if !c.selected {
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

func NewGridMazeWithSeed(width, height int, seed int64) (*GridMaze, error) {
	if (width < 1) || (height < 1) {
		return nil, fmt.Errorf("width and height must be at least 1")
	}
	cellCount := width * height
	// Check for overflow.
	if cellCount <= 0 {
		return nil, fmt.Errorf("The maze's size was too big")
	}
	toReturn := &GridMaze{
		width:     int(width),
		height:    int(height),
		cells:     make([]gridMazeCell, cellCount),
		neighbors: nil,
	}
	e := toReturn.RegenerateFromSeed(seed)
	if e != nil {
		return nil, fmt.Errorf("Error generating maze: %w", e)
	}
	return toReturn, nil
}

// Returns a maze built out of square cells and a random seed.
func NewGridMaze(width, height int) (*GridMaze, error) {
	return NewGridMazeWithSeed(width, height, time.Now().UnixNano())
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
			// Create an entry for the neighbor to the right
			if col != (m.width - 1) {
				m.neighbors = append(m.neighbors, gridNeighborInfo{
					baseIndex:         index,
					neighborDirection: 2,
				})
			}
			if row == (m.height - 1) {
				continue
			}
			// Create an entry for the neighbor below
			m.neighbors = append(m.neighbors, gridNeighborInfo{
				baseIndex:         index,
				neighborDirection: 3,
			})
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
	// TODO: Randomly choose a start and end cell.
	m.startCellIndex = 0
	m.endCellIndex = len(m.cells) - 1
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

func (m *GridMaze) ShowSolution(show bool) error {
	return fmt.Errorf("Showing the GridMaze solution is not yet supported")
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
