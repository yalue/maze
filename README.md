Maze Generation in Go
=====================

This is nothing special, just for my own fun.


Usage
-----

The bulk of the logic is in the `github.com/yalue/maze` go package. It can be
used as a library:

```go
import (
    "github.com/yalue/maze"
    "image/png"
    "os"
)

func main() {
    // Generates a new maze, taking a width and height in cells.
    m, _ := maze.NewGridMaze(60, 40)
    // The returned maze satisfies the image.Image interface; save it to a
    // file.
    f, _ := os.Create("maze.png")
    png.Encode(f, m)
    f.Close()
}
```

Alternatively, there is a command-line tool in the `create_maze_image`
subdirectory:

```bash
cd create_maze_image
go build .

# View the command-line options required by the tool:
./create_maze_image -help
```

