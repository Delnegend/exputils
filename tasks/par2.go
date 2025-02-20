package tasks

import (
	"context"
	"exputils/utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func Par2(
	ctx context.Context,
	parentDir string,
	poolSize int,
	updateProgressBase func(func() float64) func(),
	sendWarning func(error),
) {
	_7zFiles := []os.DirEntry{}
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		sendWarning(fmt.Errorf("can't read directory: %w", err))
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".7z" {
			_7zFiles = append(_7zFiles, entry)
		}
	}

	if len(_7zFiles) == 0 {
		sendWarning(fmt.Errorf("no 7z files found"))
		return
	}

	processedFiles := 0
	var progressMutex sync.Mutex
	updateProgress := updateProgressBase(func() float64 {
		progressMutex.Lock()
		defer progressMutex.Unlock()
		processedFiles++
		return float64(processedFiles) / float64(len(_7zFiles))
	})

	// remove all .par2 in the directory
	if entries, err = os.ReadDir(parentDir); err != nil {
		sendWarning(fmt.Errorf("can't read directory: %w", err))
		return
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.ToLower(filepath.Ext(entry.Name())) == ".par2" {
				sendWarning(fmt.Errorf("there are .par2 files in the directory"))
				return
			}
		}
	}

	pool := utils.NewWorkerPool(ctx, poolSize)

	for _, file := range _7zFiles {
		input7zFile := filepath.Join(parentDir, file.Name())
		pool.Run(func() {
			defer updateProgress()

			cmd := exec.CommandContext(ctx, "par2j64.exe", "c", "/rr11", input7zFile+".par2", input7zFile)
			outputMsgBytes, err := cmd.CombinedOutput()
			outputMsgString := string(outputMsgBytes)
			switch {
			case err != nil && outputMsgString != "":
				sendWarning(fmt.Errorf("par2 error: %s", outputMsgString))
			case err != nil && outputMsgString == "":
				sendWarning(fmt.Errorf("par2 error: %w", err))
			}
		})
	}
}
