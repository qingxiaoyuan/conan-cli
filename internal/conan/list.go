package conan

import (
	"context"
	"fmt"
	"strings"
)

type Binary struct {
	Settings map[string]string
	Options  map[string]string
}

func (c *Client) List(ctx context.Context, query, remote string) (map[string]any, Result, error) {
	args := []string{"list", query, "--format=json"}
	if remote != "" {
		args = append(args, "--remote="+remote)
	}
	var data map[string]any
	result, err := c.RunJSON(ctx, &data, args...)
	return data, result, err
}

func ExtractBinaries(data map[string]any) []Binary {
	var binaries []Binary
	walkBinaries(data, &binaries)
	return binaries
}

func walkBinaries(value any, binaries *[]Binary) {
	switch typed := value.(type) {
	case map[string]any:
		if info, ok := typed["info"].(map[string]any); ok {
			binary := Binary{Settings: stringMap(info["settings"]), Options: stringMap(info["options"])}
			if len(binary.Settings) > 0 || len(binary.Options) > 0 {
				*binaries = append(*binaries, binary)
			}
		}
		for _, child := range typed {
			walkBinaries(child, binaries)
		}
	case []any:
		for _, child := range typed {
			walkBinaries(child, binaries)
		}
	}
}

func stringMap(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for key, item := range raw {
		out[key] = fmt.Sprint(item)
	}
	return out
}

func ListHasReference(data map[string]any, reference string) bool {
	name := strings.SplitN(reference, "/", 2)[0]
	found := false
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == reference || strings.HasPrefix(key, name+"/") {
					found = true
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(data)
	return found
}
