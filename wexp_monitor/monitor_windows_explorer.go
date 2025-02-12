package wexpmonitor

import (
	"context"
	"exputils/utils"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

const (
	winEventOutOfContext      = uintptr(0x0000)
	eventSystemForeground     = uintptr(0x0003)
	eventObjectValueChange    = uintptr(0x800E)
	eventObjectLocationChange = uintptr(0x800B)
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	setWinEventHook     = user32.NewProc("SetWinEventHook")
	unhookWinEvent      = user32.NewProc("UnhookWinEvent")
	getClassName        = user32.NewProc("GetClassNameW")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	isWindow            = user32.NewProc("IsWindow")
	translateMessage    = user32.NewProc("TranslateMessage")

	matchClasses = []string{
		"CabinetWClass",
		"ExploreWClass",
		"Microsoft.UI.Content.PopupWindowSiteBridge",
		"CtrlNotifySink",
	}
)

func init() {
	if user32 == nil {
		panic("failed to load user32.dll")
	}
	if setWinEventHook == nil {
		panic("failed to load SetWinEventHook")
	}
	if unhookWinEvent == nil {
		panic("failed to load UnhookWinEvent")
	}
	if getClassName == nil {
		panic("failed to load GetClassNameW")
	}
	if getForegroundWindow == nil {
		panic("failed to load GetForegroundWindow")
	}
	if isWindow == nil {
		panic("failed to load IsWindow")
	}
	if translateMessage == nil {
		panic("failed to load TranslateMessage")
	}
}

// This is actually more resource intensive than just polling the path
func MonitorWindowsExplorer(ctx context.Context, pathChan chan<- string, forceUpdateChan <-chan struct{}) {
	lastPath := ""
	debouncedGetLastViewedExplorerPath := utils.Debouncer(100*time.Millisecond, func() {
		time.Sleep(50 * time.Millisecond)
		if path, err := GetLastViewedExplorerPath(); err != nil {
			fmt.Println("Failed to get last viewed Explorer path:", err)
		} else if path != "" && path != lastPath {
			lastPath = path
			pathChan <- path
		}
	})

	go func() {
		for range forceUpdateChan {
			select {
			case <-ctx.Done():
				return
			default:
				debouncedGetLastViewedExplorerPath()
			}
		}
	}()

	hook, _, err := setWinEventHook.Call(
		eventSystemForeground,
		eventObjectValueChange,
		0,
		windows.NewCallback(func(hWinEventHook, event, hwnd, idObject, idChild, eventThread, eventTime uintptr) uintptr {
			select {
			case <-ctx.Done():
				return 0 // Exit callback if context is cancelled
			default:
				ret, _, _ := isWindow.Call(hwnd)
				if ret == 0 {
					return 0
				}

				if event == eventSystemForeground || event == eventObjectValueChange || event == eventObjectLocationChange {
					className := make([]uint16, 256)
					getClassName.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 256)
					class := syscall.UTF16ToString(className)

					for _, c := range matchClasses {
						if c == class {
							debouncedGetLastViewedExplorerPath()
							break
						}
					}
				}

				return 0
			}
		}),
		0,
		0,
		winEventOutOfContext,
	)
	if err != nil && err != windows.ERROR_SUCCESS {
		fmt.Println("Failed to set event hook:", err)
		os.Exit(1)
	}

	defer unhookWinEvent.Call(hook)

	var msg ole.Msg
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, exiting MonitorWindowsExplorer")
			return
		default:
			ret, _ := ole.GetMessage(&msg, 0, 0, 0)
			if ret == 0 {
				break
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			ole.DispatchMessage(&msg)
		}
	}
}
