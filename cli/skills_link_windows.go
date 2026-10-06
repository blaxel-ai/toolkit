package cli

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
func createSkillDirectoryLink(target, _ string, link string) (err error) {
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	target = strings.TrimPrefix(target, `\\?\`)
	substitute, err := windows.UTF16FromString(`\??\` + target)
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
