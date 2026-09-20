//go:build ignore

// Bounded protocol fault emitter for correction tests only. Compile explicitly.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	input := os.Args[len(os.Args)-1]
	name := strings.TrimSuffix(filepath.Base(input), ".json")
	projection, err := os.ReadFile(filepath.Join(filepath.Dir(input), "projection.json"))
	if err != nil {
		panic(err)
	}
	switch name {
	case "shape":
		fmt.Println(`{"ok":true}`)
		time.Sleep(time.Second)
		return
	case "partial":
		fmt.Print(`{"ok":true`)
		os.Exit(7)
	case "timeout":
		time.Sleep(10 * time.Second)
		return
	case "numeric-startup":
		projection = []byte(strings.Replace(string(projection), `"id":"1"`, `"id":1`, 1))
	}
	fmt.Println(string(projection))
	if name == "failed-exit" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(7)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var q struct {
			Text string `json:"text"`
			Hash string `json:"hash"`
		}
		if json.Unmarshal(scanner.Bytes(), &q) != nil {
			os.Exit(7)
		}
		if q.Text == "delay" {
			time.Sleep(700 * time.Millisecond)
		}
		if q.Hash == "bbbb" {
			switch name {
			case "query-failed":
				fmt.Print(`{"ok":true,"ids":["2"]}`)
				os.Exit(7)
			case "invalid-id":
				fmt.Println(`{"ok":true,"ids":[9007199254740993]}`)
				continue
			case "unknown-id":
				fmt.Println(`{"ok":true,"ids":["999"]}`)
				continue
			case "duplicate-id":
				fmt.Println(`{"ok":true,"ids":["1","1"]}`)
				continue
			case "out-of-range":
				fmt.Println(`{"ok":true,"ids":["9223372036854775808"]}`)
				continue
			case "negative-id":
				fmt.Println(`{"ok":true,"ids":["-1"]}`)
				continue
			case "zero-id":
				fmt.Println(`{"ok":true,"ids":["0"]}`)
				continue
			case "leading-zero":
				fmt.Println(`{"ok":true,"ids":["01"]}`)
				continue
			case "exponent-id":
				fmt.Println(`{"ok":true,"ids":["1e0"]}`)
				continue
			}
			fmt.Println(`{"ok":true,"ids":["2"]}`)
		} else {
			fmt.Println(`{"ok":true,"ids":["1","2"]}`)
		}
	}
}
