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

// BinaryRef is one prebuilt package ID tied to a recipe name/version.
type BinaryRef struct {
	Name      string
	Version   string
	Channel   string
	Reference string
	Settings  map[string]string
	Options   map[string]string
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

func ExtractBinaryRefs(data map[string]any) []BinaryRef {
	var refs []BinaryRef
	walkBinaryRefs(data, Recipe{}, &refs)
	return refs
}

func walkBinaryRefs(value any, current Recipe, refs *[]BinaryRef) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if rec, ok := parseRecipeKey(key); ok {
				rec.Channel = recipeChannel(key)
				walkBinaryRefs(child, rec, refs)
				continue
			}
			if key == "info" {
				if current.Name == "" {
					continue
				}
				info, _ := child.(map[string]any)
				ref := BinaryRef{
					Name:      current.Name,
					Version:   current.Version,
					Channel:   current.Channel,
					Reference: current.Reference,
					Settings:  stringMap(info["settings"]),
					Options:   stringMap(info["options"]),
				}
				if len(ref.Settings) > 0 || len(ref.Options) > 0 {
					*refs = append(*refs, ref)
				}
				continue
			}
			walkBinaryRefs(child, current, refs)
		}
	case []any:
		for _, child := range typed {
			walkBinaryRefs(child, current, refs)
		}
	}
}

func recipeChannel(key string) string {
	parts := strings.SplitN(key, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	userChannel := strings.SplitN(parts[1], "/", 2)
	if len(userChannel) != 2 {
		return ""
	}
	return strings.TrimSpace(userChannel[1])
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
