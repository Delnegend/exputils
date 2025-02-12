package wexpmonitor

import (
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

func GetLastViewedExplorerPath() (string, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return "", nil
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return "", nil
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", nil
	}
	defer shell.Release()

	windows_, err := oleutil.CallMethod(shell, "Windows")
	if err != nil {
		return "", nil
	}
	windowsDisp := windows_.ToIDispatch()
	defer windowsDisp.Release()

	count, err := oleutil.GetProperty(windowsDisp, "Count")
	if err != nil {
		return "", nil
	}
	c := int(count.Val)

	foregroundHwnd, _, err := getForegroundWindow.Call()
	if err != nil && err != windows.ERROR_SUCCESS {
		return "", nil
	}

	for i := 0; i < c; i++ {
		item, err := oleutil.CallMethod(windowsDisp, "Item", i)
		if err != nil {
			continue
		}
		window := item.ToIDispatch()
		if window == nil {
			continue
		}
		defer window.Release()

		name, err := oleutil.GetProperty(window, "Name")
		if err != nil {
			continue
		}
		if name.ToString() != "File Explorer" {
			continue
		}

		hwnd, err := oleutil.GetProperty(window, "HWND")
		if err != nil {
			continue
		}
		if uintptr(hwnd.Val) != foregroundHwnd {
			continue
		}

		document, err := oleutil.GetProperty(window, "Document")
		if err != nil {
			continue
		}
		docDisp := document.ToIDispatch()
		if docDisp == nil {
			continue
		}
		defer docDisp.Release()

		folder, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			continue
		}
		folderDisp := folder.ToIDispatch()
		if folderDisp == nil {
			continue
		}
		defer folderDisp.Release()

		self, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			continue
		}
		selfDisp := self.ToIDispatch()
		if selfDisp == nil {
			continue
		}
		defer selfDisp.Release()

		path, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			continue
		}
		return path.ToString(), nil
	}
	return "", nil
}
