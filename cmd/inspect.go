//go:build linux

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dck/internal/container"
)

func Inspect(args []string) {
	showSensitive := false
	var names []string
	for _, arg := range args {
		if arg == "--sensitive" {
			showSensitive = true
			continue
		}
		names = append(names, arg)
	}
	if len(names) < 1 {
		fmt.Println("Usage: dck inspect [--sensitive] <container> [<container>...]")
		os.Exit(1)
	}

	for _, name := range names {
		c, err := container.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		data, err := json.Marshal(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inspecting %s: %v\n", name, err)
			continue
		}
		var view map[string]interface{}
		if err := json.Unmarshal(data, &view); err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing inspection %s: %v\n", name, err)
			continue
		}
		if !showSensitive {
			redactInspection(view)
		}
		data, err = json.MarshalIndent(view, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting inspection %s: %v\n", name, err)
			continue
		}
		fmt.Println(string(data))
	}
}

func redactInspection(view map[string]interface{}) {
	delete(view, "env")
	if secrets, ok := view["secrets"].([]interface{}); ok {
		for _, item := range secrets {
			if secret, ok := item.(map[string]interface{}); ok {
				delete(secret, "data")
			}
		}
	}
	if configs, ok := view["configs"].([]interface{}); ok {
		for _, item := range configs {
			if config, ok := item.(map[string]interface{}); ok {
				delete(config, "data")
			}
		}
	}
	for key := range view {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			delete(view, key)
		}
	}
}
