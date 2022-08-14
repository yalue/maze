// This defines a basic executable for generating an image of a maze.
package main

import (
	"flag"
	"fmt"
	"github.com/yalue/maze"
	"image/png"
	"os"
)

func run() int {
	var cellsWide, cellsHigh, erodeAmount int
	var randomSeed int64
	var showSolution bool
	var outFilename string
	flag.IntVar(&cellsWide, "cells_wide", 20,
		"The width of the maze, in grid cells.")
	flag.IntVar(&cellsHigh, "cells_high", 20,
		"The height of the maze, in grid cells.")
	flag.IntVar(&erodeAmount, "erode_amount", 0,
		"The amount by which to \"erode\" small walls.")
	flag.Int64Var(&randomSeed, "random_seed", -1,
		"If positive, specifies the random seed to use.")
	flag.BoolVar(&showSolution, "show_solution", false,
		"If set, shows the solution of the maze.")
	flag.StringVar(&outFilename, "output_file", "",
		"The name of the .png file to which the maze will be saved.")
	flag.Parse()
	if (cellsWide < 1) || (cellsHigh < 1) || (outFilename == "") {
		fmt.Println("Invalid or missing argument.")
		fmt.Println("Run with -help for more information.")
		return 1
	}
	var e error
	var m *maze.GridMaze
	if randomSeed > 0 {
		m, e = maze.NewGridMazeWithSeed(cellsWide, cellsHigh, randomSeed)
	} else {
		m, e = maze.NewGridMaze(cellsWide, cellsHigh)
	}
	if e != nil {
		fmt.Printf("Failed generating maze: %s\n", e)
		return 1
	}
	fmt.Printf("Generated %s OK.\n", m.GetInfo())
	if erodeAmount > 0 {
		fmt.Printf("Eroding maze walls %d steps.\n", erodeAmount)
		for i := 0; i < erodeAmount; i++ {
			e = m.ErodeWalls()
			if e != nil {
				fmt.Printf("Error eroding walls: %s\n", e)
				return 1
			}
		}
	}
	if showSolution {
		fmt.Printf("Finding solution to the maze.\n")
		e = m.ShowSolution(true)
		if e != nil {
			fmt.Printf("Error finding solution: %s\n", e)
			return 1
		}
	}
	f, e := os.Create(outFilename)
	if e != nil {
		fmt.Printf("Error creating output file %s: %s\n", outFilename, e)
		return 1
	}
	defer f.Close()
	e = png.Encode(f, maze.AddImageBorder(m, 5))
	if e != nil {
		fmt.Printf("Error writing image to %s: %s\n", outFilename, e)
		return 1
	}
	fmt.Printf("Image %s written OK.\n", outFilename)
	return 0
}

func main() {
	os.Exit(run())
}
