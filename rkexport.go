package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	WIN_EXT  = ".exe"
	JSON_EXT = ".json"
)

func main() {
	//Configuration file: first argument or PROG_FILE_NAME.json
	var ini_file string
	if len(os.Args) >= 2 {
		ini_file = os.Args[1]
	} else {
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(os.Args[0]), WIN_EXT) {
			ini_file, _ = strings.CutSuffix(strings.ToLower(os.Args[0]), WIN_EXT)
		} else {
			ini_file = os.Args[0]
		}
		ini_file += JSON_EXT
	}

	app := NewApp()
	conf_data, err := os.ReadFile(ini_file)
	if err != nil {
		panic(fmt.Sprintf("os.ReadFile() failed: %v", err))
	}
	if err := app.LoadConfig(conf_data); err != nil {
		panic(fmt.Sprintf("app.LoadConfig() failed: %v", err))
	}
	if err := app.Start(); err != nil {
		panic(fmt.Sprintf("app.Start() failed: %v", err))
	}
}
