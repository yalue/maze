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

const arrowLength = 16

// Returns 0 = left, 1 = up, 2 = right, and 3 = down. The given angle must be
// between 0 and 360, if it isn't this will simply return 2.
func angleToArrowDir(angle float32) int {
	if (angle > 45) && (angle <= 135) {
		return 1
	} else if (angle > 135) && (angle <= 225) {
		return 0
	} else if (angle > 225) && (angle < 315) {
		return 3
	}
	return 2
}

func getArrowForAngle(angle float32, arrowColor color.Color) image.Image {
	var tmp image.Image
	dir := angleToArrowDir(angle)
	if dir == 1 {
		tmp = image_utils.UpArrow(arrowColor)
	} else if dir == 0 {
		tmp = image_utils.LeftArrow(arrowColor)
	} else if dir == 3 {
		tmp = image_utils.DownArrow(arrowColor)
	} else {
		tmp = image_utils.RightArrow(arrowColor)
	}
	return tmp
}

// Returns an arrow pointing in the direction of the given angle, or at least
// as close to it as we can get (for now). The given angle must be between 0
// and 360 (inclusive).
func getOutlinedArrow(angle float32, arrowColor color.Color) image.Image {
	outerArrow := image_utils.ResizeImage(getArrowForAngle(angle, arrowColor),
		arrowLength, arrowLength)
	innerArrow := image_utils.ResizeImage(getArrowForAngle(angle, color.White),
		arrowLength/2, arrowLength/2)
	toReturn := image_utils.NewCompositeImage()
	toReturn.AddImage(outerArrow, image.Pt(0, 0))
	toReturn.AddImage(innerArrow, image.Pt(arrowLength/4, arrowLength/4))
	return image_utils.ToRGBA(toReturn)
}

// If the tip of the arrow is supposed to be at the given pt (or the tail of
// the arrow, if "away" is true), this returns the top-left where the square
// image returned by getOutlinedArrow should be drawn.
func getArrowTopLeft(pt image.Point, angle float32, away bool) image.Point {
	dir := angleToArrowDir(angle)
	halfLength := arrowLength / 2
	switch dir {
	case 0:
		// Pointing left
		if away {
			// The arrow's tail is at pt, but it's pointing to the left, so it
			// needs to be shifted so the whole arrow is to the right of pt.
			return image.Pt(pt.X-arrowLength-1, pt.Y-halfLength)
		}
		// pt is to the left of the arrow, so just offset it a pixel to the
		// right.
		return image.Pt(pt.X+1, pt.Y-halfLength)
	case 1:
		// Pointing up
		if away {
			return image.Pt(pt.X-halfLength, pt.Y-arrowLength-1)
		}
		return image.Pt(pt.X-halfLength, pt.Y+1)
	case 2:
		// Pointing right
		if away {
			return image.Pt(pt.X+1, pt.Y-halfLength)
		}
		return image.Pt(pt.X-arrowLength-1, pt.Y-halfLength)
	case 3:
		// Pointing down
		if away {
			return image.Pt(pt.X-halfLength, pt.Y+1)
		}
		return image.Pt(pt.X-halfLength, pt.Y-arrowLength-1)
	default:
		break
	}
	panic("Invalid arrow direction!")
	return image.Pt(0, 0)
}

// Adds "decorations" to the maze, including start and end arrows. Rasterizes
// the maze to an image.RGBA.
func drawMazeDecorations(m maze.Maze) (*image.RGBA, error) {
	info := m.GetInfo()
	decorated := image_utils.NewCompositeImage()
	mazePic := image_utils.ToRGBA(m)
	e := decorated.AddImage(mazePic, image.Pt(0, 0))
	if e != nil {
		return nil, fmt.Errorf("Error setting base maze image: %w", e)
	}
	blueColor := color.RGBA{100, 120, 255, 255}
	greenColor := color.RGBA{40, 180, 70, 255}

	startArrow := getOutlinedArrow(info.StartAngle, greenColor)
	startArrowPos := getArrowTopLeft(info.StartPoint, info.StartAngle,
		false)
	e = decorated.AddImage(startArrow, startArrowPos)
	if e != nil {
		return nil, fmt.Errorf("Error adding start arrow: %w", e)
	}

	endArrow := getOutlinedArrow(info.EndAngle, blueColor)
	endArrowPos := getArrowTopLeft(info.EndPoint, info.EndAngle, true)
	e = decorated.AddImage(endArrow, endArrowPos)
	if e != nil {
		return nil, fmt.Errorf("Error adding end arrow: %w", e)
	}

	toReturn := image_utils.ToRGBA(decorated)
	return toReturn, nil
}

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
	finalPic, e := drawMazeDecorations(m)
	if e != nil {
		fmt.Printf("Error adding maze decorations: %s\n", e)
		return 1
	}
	f, e := os.Create(outFilename)
	if e != nil {
		fmt.Printf("Error creating output file %s: %s\n", outFilename, e)
		return 1
	}
	defer f.Close()
	//e = png.Encode(f, image_utils.AddImageBorder(finalPic, color.White, 5))
	e = png.Encode(f, finalPic)
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
