//go:build ignore

// Explicitly compiled deterministic protocol simulation; not a native reader.
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
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(input), "projection.json"))
	if err != nil {
		panic(err)
	}
	var envelope struct {
		Catalog struct {
			Digest string `json:"digest"`
			Files  []struct {
				ID string `json:"id"`
			} `json:"files"`
		} `json:"catalog"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		panic(err)
	}
	fmt.Println(string(raw))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var q struct {
			Enumerate bool `json:"enumerate"`
		}
		if json.Unmarshal(scanner.Bytes(), &q) != nil {
			os.Exit(7)
		}
		ids := []string{}
		for _, f := range envelope.Catalog.Files {
			ids = append(ids, f.ID)
		}
		if q.Enumerate {
			if name == "dies" {
				os.Exit(7)
			}
			if name == "late" {
				time.Sleep(350 * time.Millisecond)
			}
			if name == "timeout" {
				time.Sleep(7 * time.Second)
			}
			complete := name != "partial"
			count := len(ids)
			if name == "truncated" && len(ids) > 0 {
				ids = ids[:len(ids)-1]
			}
			if name == "unknown" {
				ids = []string{"999"}
			}
			b, _ := json.Marshal(map[string]any{"ok": true, "complete": complete, "count": count, "digest": envelope.Catalog.Digest, "ids": ids})
			fmt.Println(string(b))
		} else {
			b, _ := json.Marshal(map[string]any{"ok": true, "ids": ids})
			fmt.Println(string(b))
		}
	}
}
