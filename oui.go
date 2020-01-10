package oui

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"unicode"
)

const (
	OUI_FILE = "oui.csv"
	OUI_URL  = "http://standards-oui.ieee.org/oui/oui.csv"
)

type Organization struct {
	Name    string
	Address string
}

type atomicBool int32

func (ab *atomicBool) Set() {
	atomic.StoreInt32((*int32)(ab), 1)
}

func (ab *atomicBool) Unset() {
	atomic.StoreInt32((*int32)(ab), 0)
}

func (ab *atomicBool) IsSet() bool {
	return atomic.LoadInt32((*int32)(ab)) == 1
}

var (
	ouis    = make(map[string]Organization)
	loaded  = new(atomicBool)
	loading = new(atomicBool)
)

func Download() {
	fmt.Printf("Opening output file %s: ", OUI_FILE)

	outFile, err := os.OpenFile(OUI_FILE, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Failed")
		panic(err)
	}
	defer outFile.Close()

	fmt.Println("Success")

	fmt.Printf("Downloading OUI file from %s: ", OUI_URL)

	res, err := http.Get(OUI_URL)
	if err != nil {
		fmt.Println("Failed")
		panic(err)
	}

	var writtenBytes int64
	totalBytes := res.ContentLength

	for writtenBytes < totalBytes {
		written, err := io.CopyN(outFile, res.Body, 1024)
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("Failed")
			panic(err)
		}
		writtenBytes += written
		fmt.Printf("\rDownloading OUI file from %s: %3d%%", OUI_URL, 100*writtenBytes/totalBytes)
	}
	res.Body.Close()

	fmt.Printf("\rDownloading OUI file from %s: Success\n", OUI_URL)
}

func Load() error {
	if loaded.IsSet() {
		return nil
	}
	if loading.IsSet() {
		for loading.IsSet() {
		}
		return nil
	}
	loading.Set()

	ouiFile, err := os.Open(OUI_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			Download()
			ouiFile, err = os.Open(OUI_FILE)
		}
		if err != nil {
			return fmt.Errorf("OUI Load|Open File: %s", err)
		}
	}
	defer ouiFile.Close()

	reader := csv.NewReader(ouiFile)

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("OUI Load|CSV Read Header: %s", err)
	}

	if len(header) != 4 {
		return fmt.Errorf("OUI Load|Invalid CSV: Columns required: 4, found: %d", len(header))
	}

	for {
		line, err := reader.Read()

		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("OUI Load|CSV Read Row: %s", err)
		}
		ouis[line[1]] = Organization{
			Name:    strings.TrimSpace(line[2]),
			Address: strings.TrimSpace(line[3]),
		}
	}

	loaded.Set()
	loading.Unset()
	return nil
}

func Query(address string) (org Organization, err error) {
	if !loaded.IsSet() {
		err = Load()
		if err != nil {
			return
		}
	}

	var oui string

	for _, ch := range strings.ToUpper(address) {
		if unicode.Is(unicode.ASCII_Hex_Digit, ch) {
			oui += string(ch)
			if len(oui) == 6 {
				break
			}
		}
	}

	if len(oui) != 6 {
		err = fmt.Errorf("OUI Query|Parse Address: Hex digits required: 6, found: %d", len(oui))
		return
	}

	org, ok := ouis[oui]
	if !ok {
		err = fmt.Errorf("OUI Query|Find Assignment: Assignment not found for %s", oui)
		return
	}

	return
}
