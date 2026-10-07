package agentsetup

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Directory junctions work without Developer Mode or the symlink privilege.
// They use absolute targets, so moving the staged junction does not retarget it.
// Set the mount-point reparse data directly: do not shell out with user paths.
func CreateSkillDirectoryLink(target, _ string, link string) (err error) {
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	substitutePath := strings.TrimPrefix(target, `\\?\`)
	if strings.HasPrefix(substitutePath, `\\`) {
		substitutePath = `UNC\` + strings.TrimPrefix(substitutePath, `\\`)
	}
	substitute, err := windows.UTF16FromString(`\??\` + substitutePath)
	if err != nil {
		return err
	}
	printName, err := windows.UTF16FromString(target)
	if err != nil {
		return err
	}
	// Eight-byte reparse header, eight-byte mount-point header, UTF-16 names.
	length := 16 + 2*(len(substitute)+len(printName))
	if length > windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE {
		return fmt.Errorf("skill junction target is too long: %s", target)
	}
	buffer := make([]byte, length)
	binary.LittleEndian.PutUint32(buffer[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:6], uint16(length-8))
	binary.LittleEndian.PutUint16(buffer[10:12], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(buffer[12:14], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(buffer[14:16], uint16(2*(len(printName)-1)))
	offset := 16
	for _, name := range [][]uint16{substitute, printName} {
		for _, character := range name {
			binary.LittleEndian.PutUint16(buffer[offset:offset+2], character)
			offset += 2
		}
	}
	if err := os.Mkdir(link, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(link)
		}
	}()
	path, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var returned uint32
	return windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil)
}

// Since Go 1.23, junctions have ModeIrregular rather than ModeSymlink, and
// filepath.EvalSymlinks does not resolve them. Follow the opened Windows handle
// instead, including junctions in parent components, without walking cycles.
func EvalSkillLinks(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	buffer := make([]uint16, 256)
	flags := uint32(0) // Normalized name with a DOS volume name.
	for {
		length, err := windows.GetFinalPathNameByHandle(windows.Handle(file.Fd()), &buffer[0], uint32(len(buffer)), flags)
		if err == windows.ERROR_PATH_NOT_FOUND && flags == 0 {
			flags = 1 // VOLUME_NAME_GUID, for mounted volumes without a drive letter.
			continue
		}
		if err != nil {
			return "", &os.PathError{Op: "resolve", Path: name, Err: err}
		}
		if length < uint32(len(buffer)) {
			resolved := windows.UTF16ToString(buffer[:length])
			if strings.HasPrefix(resolved, `\\?\UNC\`) {
				return `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`), nil
			}
			if strings.HasPrefix(resolved, `\\?\`) && len(resolved) > 6 && resolved[5] == ':' {
				return resolved[4:], nil
			}
			return resolved, nil
		}
		if length > 65535 {
			return "", fmt.Errorf("resolved skill path is too long: %s", name)
		}
		buffer = make([]uint16, int(length)+1)
	}
}
