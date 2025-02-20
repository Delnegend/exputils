package tasks

import (
	"context"
	"errors"
	"exputils/utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func Cjxl(
	ctx context.Context,
	parentDir string,
	poolSize int,
	outputLossy bool,
	updateProgressBase func(func() float64) func(),
	sendWarning func(error),
) {
	jpgPngFiles := []os.DirEntry{}
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		sendWarning(err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileExt := strings.ToLower(filepath.Ext(entry.Name()))
		if fileExt == ".jpg" || fileExt == ".png" {
			jpgPngFiles = append(jpgPngFiles, entry)
		}
	}

	if len(jpgPngFiles) == 0 {
		sendWarning(nil)
		return
	}

	fileNamesWithoutExt := []string{}

	// check if output files already exist, or 2 files jpg and png
	// with the same name might result in the same output jxl file
	for _, inputFile := range jpgPngFiles {
		outputFile := utils.ReplaceExt(filepath.Join(parentDir, inputFile.Name()), ".jxl")
		if _, err := os.Stat(outputFile); err == nil {
			sendWarning(fmt.Errorf("possible output file '%s' already exists", outputFile))
			return
		}

		withoutExt := utils.ReplaceExt(inputFile.Name(), "")
		if utils.Contains(fileNamesWithoutExt, withoutExt) {
			sendWarning(fmt.Errorf("duplicate possible output file for '%s'", inputFile.Name()))
			return
		}
		fileNamesWithoutExt = append(fileNamesWithoutExt, withoutExt)
	}

	processedFiles := 0
	var progressMutex sync.Mutex
	updateProgress := updateProgressBase(func() float64 {
		progressMutex.Lock()
		defer progressMutex.Unlock()
		processedFiles++
		return float64(processedFiles) / float64(len(jpgPngFiles))
	})

	pool := utils.NewWorkerPool(ctx, poolSize)

	distance := "0"
	if outputLossy {
		distance = "1"
	}

	for _, file := range jpgPngFiles {
		fileName := file.Name()
		pool.Run(func() {
			defer updateProgress()

			inputFile := filepath.Join(parentDir, fileName)
			outputFile := utils.ReplaceExt(inputFile, ".jxl")

			// convert jpg/png to jxl
			cmd := exec.CommandContext(ctx, "djxl", inputFile, outputFile, "-d", distance, "-e", "9")
			outputMsgBytes, err := cmd.CombinedOutput()
			outputMsgString := string(outputMsgBytes)
			switch {
			case err != nil && outputMsgString != "":
				sendWarning(fmt.Errorf("djxl error: %s", outputMsgString))
				return
			case err != nil && outputMsgString == "":
				sendWarning(fmt.Errorf("djxl error: %w", err))
				return
			}

			// check output file exists
			_, err = os.Stat(outputFile)
			if errors.Is(err, os.ErrNotExist) {
				sendWarning(fmt.Errorf("output file '%s' not created", outputFile))
			} else if err != nil {
				sendWarning(fmt.Errorf("can't check if output file exists: %w", err))
			}
		})
	}
}
