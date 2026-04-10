package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	afBluetooth   = 31
	btprotoRFCOMM = 3
	rfcommChannel = 15
)

var deviceAddr = [6]byte{0x42, 0x13, 0x09, 0xEB, 0xBE, 0x2C} // 2C:BE:EB:09:13:42 reversed

var commands = map[string][]byte{
	"transparency": mustHex("5560010ff00300cb010700c5af"),
	"off":          mustHex("5560010ff00300cd010500c447"),
	"high":         mustHex("5560010ff00300cf010100e66f"),
	"low":          mustHex("5560010ff00300d7010300e70f"),
}

var queryCmd = mustHex("5560011ec001000c039819")

var ancByteMap = map[byte]string{
	1: "high",
	3: "low",
	5: "off",
	7: "transparency",
}

type sockaddrRC struct {
	family  uint16
	bdaddr  [6]byte
	channel uint8
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func connect() (int, error) {
	fd, err := syscall.Socket(afBluetooth, syscall.SOCK_STREAM, btprotoRFCOMM)
	if err != nil {
		return -1, fmt.Errorf("socket: %w", err)
	}

	addr := sockaddrRC{
		family:  afBluetooth,
		bdaddr:  deviceAddr,
		channel: rfcommChannel,
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_CONNECT,
		uintptr(fd),
		uintptr(unsafe.Pointer(&addr)),
		unsafe.Sizeof(addr),
	)
	if errno != 0 {
		syscall.Close(fd)
		return -1, fmt.Errorf("connect: %s", errno)
	}

	return fd, nil
}

func sendCommand(cmd []byte) error {
	fd, err := connect()
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	_, err = syscall.Write(fd, cmd)
	return err
}

func queryANC() (string, error) {
	fd, err := connect()
	if err != nil {
		return "", err
	}
	defer syscall.Close(fd)

	if _, err := syscall.Write(fd, queryCmd); err != nil {
		return "", err
	}

	buf := make([]byte, 32)
	n, err := syscall.Read(fd, buf)
	if err != nil {
		return "", err
	}

	if n >= 10 {
		if mode, ok := ancByteMap[buf[9]]; ok {
			return mode, nil
		}
		return fmt.Sprintf("unknown(%d)", buf[9]), nil
	}
	return "unknown", nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: earctl <command>

Commands:
  high           Set ANC to high
  low            Set ANC to low
  off            Turn ANC off
  transparency   Set transparency mode
  get            Query current ANC mode
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	arg := strings.ToLower(os.Args[1])

	if arg == "get" || arg == "--get" {
		mode, err := queryANC()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(mode)
		return
	}

	cmd, ok := commands[arg]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", arg)
		usage()
		os.Exit(1)
	}

	if err := sendCommand(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ANC set to: %s\n", arg)
}
