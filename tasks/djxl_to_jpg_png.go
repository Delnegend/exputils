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

func replaceExt(path, newExt string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + newExt
}

func DjxlToJpgPng(
	ctx context.Context,
	parentDir string,
	poolSize int,
	updateProgressBase func(func() float64) func(),
	sendWarning func(error),
) {
	jxlFiles := []os.DirEntry{}
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		sendWarning(fmt.Errorf("can't read directory: %w", err))
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".jxl" {
			jxlFiles = append(jxlFiles, entry)
		}
	}

	if len(jxlFiles) == 0 {
		sendWarning(fmt.Errorf("no jxl files found"))
		return
	}

	processedFiles := 0
	var progressMutex sync.Mutex

	updateProgress := updateProgressBase(func() float64 {
		progressMutex.Lock()
		defer progressMutex.Unlock()
		processedFiles++
		return float64(processedFiles) / float64(len(jxlFiles))
	})

	pool := utils.NewWorkerPool(ctx, poolSize)
	defer pool.Close()

	canContinue := true
	for _, file := range jxlFiles {
		inputJxlFile := filepath.Join(parentDir, file.Name())
		outputPngFile := replaceExt(inputJxlFile, ".png")
		outputJpgFile := replaceExt(inputJxlFile, ".jpg")

		// output file already exists
		if _, err := os.Stat(outputJpgFile); err == nil {
			sendWarning(fmt.Errorf("possible output file '%s' already exists", outputJpgFile))
			canContinue = false
		}
		if _, err := os.Stat(outputPngFile); err == nil {
			sendWarning(fmt.Errorf("possible output file '%s' already exists", outputPngFile))
			canContinue = false
		}
	}

	if !canContinue {
		return
	}

	for _, file := range jxlFiles {
		fileName := file.Name()
		pool.Run(func() {
			inputJxlFile := filepath.Join(parentDir, fileName)
			outputPngFile := replaceExt(inputJxlFile, ".png")
			outputJpgFile := replaceExt(inputJxlFile, ".jpg")

			// try reconstruct original jpg
			cmd := exec.CommandContext(ctx, "djxl", inputJxlFile, outputJpgFile)
			outputMsgBytes, err := cmd.CombinedOutput()
			outputMsgString := string(outputMsgBytes)
			switch {
			// error with message
			case err != nil && outputMsgString != "":
				sendWarning(fmt.Errorf("djxl failed to run: %s", outputMsgString))
				updateProgress()
				return
				// error without message
			case err != nil && outputMsgString == "":
				sendWarning(fmt.Errorf("djxl failed to run but didn't output anything"))
				updateProgress()
				return
				// success, reconstructed original jpg
			case err == nil && !strings.Contains(outputMsgString, "Warning: could not decode losslessly to JPEG"):
				_, err := os.Stat(outputJpgFile)
				if errors.Is(err, os.ErrNotExist) {
					sendWarning(fmt.Errorf("output file '%s' not created", outputJpgFile))
				} else if err != nil {
					sendWarning(fmt.Errorf("can't check if output file exists: %w", err))
				}
				updateProgress()
				return
			}

			if err := os.Remove(outputJpgFile); err != nil {
				sendWarning(fmt.Errorf("can't remove output file: %w", err))
				updateProgress()
				return
			}

			// jxl -> png
			cmd = exec.CommandContext(ctx, "djxl", inputJxlFile, outputPngFile)
			outputMsgBytes, err = cmd.CombinedOutput()
			outputMsgString = string(outputMsgBytes)

			switch {
			// error with message
			case err != nil && outputMsgString != "":
				sendWarning(fmt.Errorf("djxl failed to run: %s", outputMsgString))
				updateProgress()
				return
			// error without message
			case err != nil && outputMsgString == "":
				sendWarning(fmt.Errorf("djxl failed to run but didn't output anything"))
				updateProgress()
				return
			// error expect message not found
			case err == nil && !strings.Contains(outputMsgString, "Decoded to pixels."):
				sendWarning(fmt.Errorf("expecting 'Decoded to pixels.' in output: %s", outputMsgString))
				updateProgress()
				return
			}

			// check output file exists
			_, err = os.Stat(outputPngFile)
			if errors.Is(err, os.ErrNotExist) {
				sendWarning(fmt.Errorf("output file '%s' not created", outputPngFile))
			} else if err != nil {
				sendWarning(fmt.Errorf("can't check if output file exists: %w", err))
			}

			updateProgress()
		})
	}

	pool.Wait()
	updateProgressBase(func() float64 { return 1 })()
}
