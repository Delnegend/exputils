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

func Artefact(
	ctx context.Context,
	parentDir string,
	poolSize int,
	updateProgressBase func(func() float64) func(),
	sendWarning func(error),
) {
	jpgFiles := []os.DirEntry{}
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		sendWarning(err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".jpg" {
			jpgFiles = append(jpgFiles, entry)
		}
	}

	if len(jpgFiles) == 0 {
		sendWarning(fmt.Errorf("no jpg files found"))
		return
	}

	for _, file := range jpgFiles {
		inputJpgFile := filepath.Join(parentDir, file.Name())
		outputPngFile := utils.ReplaceExt(inputJpgFile, ".png")

		// output file already exists
		if _, err := os.Stat(outputPngFile); err == nil {
			sendWarning(fmt.Errorf("possible output file '%s' already exists", outputPngFile))
			return
		}
	}

	processedFiles := 0
	var progressMutex sync.Mutex
	updateProgress := updateProgressBase(func() float64 {
		progressMutex.Lock()
		defer progressMutex.Unlock()
		processedFiles++
		return float64(processedFiles) / float64(len(jpgFiles))
	})

	pool := utils.NewWorkerPool(ctx, poolSize)

	for _, file := range jpgFiles {
		fileName := file.Name()
		pool.Run(func() {
			defer updateProgress()

			inputJpgFile := filepath.Join(parentDir, fileName)
			outputPngFile := utils.ReplaceExt(inputJpgFile, ".png")

			cmd := exec.CommandContext(ctx, "artefact", inputJpgFile, "-o", outputPngFile, "-i", "50")
			outputMsgBytes, err := cmd.CombinedOutput()
			outputMsgString := string(outputMsgBytes)
			switch {
			case err != nil && outputMsgString != "":
				sendWarning(fmt.Errorf("artefact error: %s", outputMsgString))
				return
			case err != nil && outputMsgString == "":
				sendWarning(fmt.Errorf("artefact error: %s", err))
				return
			}

			// check output file exists
			_, err = os.Stat(outputPngFile)
			if errors.Is(err, os.ErrNotExist) {
				sendWarning(fmt.Errorf("output file '%s' not created", outputPngFile))
			} else if err != nil {
				sendWarning(fmt.Errorf("can't check if output file exists: %w", err))
			}
		})
	}

	pool.WaitAndClose()
	updateProgressBase(func() float64 { return 1.0 })()
}
