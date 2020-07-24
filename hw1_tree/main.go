package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
)

func printInfo(output io.Writer, info os.FileInfo, isLast bool) {
	var size string
	if !info.IsDir() {
		if info.Size() == 0 {
			size = " (empty)"
		} else {
			size = " (" + strconv.FormatInt(info.Size(), 10) + "b)"
		}
	}

	if isLast {
		fmt.Fprintf(output, "└───%s%s\n", info.Name(), size)
	} else {
		fmt.Fprintf(output, "├───%s%s\n", info.Name(), size)
	}
}

func dirTreeRecursive(output io.Writer, dir string, margins []string, printFiles bool) error {
	filesAndDirs, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	dirsNum := 0
	for _, file := range filesAndDirs {
		if file.IsDir() {
			dirsNum++
		}
	}

	var currentDir int
	var isLast bool
	for index, fileData := range filesAndDirs {
		isLast = index == (len(filesAndDirs)-1) || (currentDir == dirsNum-1 && !printFiles)

		if fileData.IsDir() || printFiles {
			for i := 0; i < len(margins); i++ {
				fmt.Fprintf(output, margins[i])
			}

			printInfo(output, fileData, isLast)
		}

		if fileData.IsDir() {
			currentDir++

			if !printFiles && currentDir == dirsNum {
				margins = append(margins, "\t")
				dirTreeRecursive(output, dir+"/"+fileData.Name(), margins, printFiles)
				margins = append(margins[:len(margins)-1], margins[len(margins):]...)
				continue
			}

			if isLast {
				margins = append(margins, "\t")
				dirTreeRecursive(output, dir+"/"+fileData.Name(), margins, printFiles)
			} else {
				margins = append(margins, "│\t")
				dirTreeRecursive(output, dir+"/"+fileData.Name(), margins, printFiles)
				margins = append(margins[:len(margins)-1], margins[len(margins):]...)
			}
		}
	}

	return nil
}

func dirTree(output io.Writer, path string, printFiles bool) error {
	return dirTreeRecursive(output, path, make([]string, 0), printFiles)
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
