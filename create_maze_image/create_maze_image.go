// This defines a basic executable for generating an image of a maze.
package main

import (
	"flag"
	"fmt"
	"github.com/yalue/image_utils"
	"github.com/yalue/maze"
	"image"
	"image/color"
	"image/png"
	"os"
)

func run() int {
	var cellsWide, cellsHigh, erodeAmount int
	var randomSeed int64
	var showSolution bool
	var outFilename, templateImage string
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
	flag.StringVar(&templateImage, "template_image", "",
		"An optional path to a PNG-format image to use as a layout "+
			"template. Wil ignore cells_wide and cells_high if used.")
	flag.Parse()
	if (cellsWide < 1) || (cellsHigh < 1) || (outFilename == "") {
		fmt.Println("Invalid or missing argument.")
		fmt.Println("Run with -help for more information.")
		return 1
	}
	var e error
	var m *maze.GridMaze
	if templateImage != "" {
		f, e := os.Open(templateImage)
		if e != nil {
			fmt.Printf("Error opening template image %s: %s\n", templateImage,
				e)
			return 1
		}
		pic, _, e := image.Decode(f)
		f.Close()
		if e != nil {
			fmt.Printf("Error parsing template image %s: %s\n", templateImage,
				e)
			return 1
		}
		m, e = maze.NewGridMazeFromTemplate(pic, randomSeed)
	} else {
		m, e = maze.NewGridMazeWithSeed(cellsWide, cellsHigh, randomSeed)
	}
	if e != nil {
		fmt.Printf("Failed generating maze: %s\n", e)
		return 1
	}
	tmp := m.GetInfo()
	fmt.Printf("Generated %s OK.\n", tmp.DebugInfo)
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
	e = png.Encode(f, image_utils.AddImageBorder(m, color.White, 5))
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
